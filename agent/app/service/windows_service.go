package service

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/1Panel-dev/1Panel/agent/app/dto/request"
	"github.com/1Panel-dev/1Panel/agent/app/dto/response"
	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/app/repo"
	"github.com/1Panel-dev/1Panel/agent/global"
	"github.com/1Panel-dev/1Panel/agent/utils/common"
	"github.com/1Panel-dev/1Panel/agent/utils/controller"
	"github.com/1Panel-dev/1Panel/agent/utils/controller/manager"
	"github.com/1Panel-dev/1Panel/agent/utils/platform"
	"github.com/ulikunitz/xz"
)

type WindowsServiceService struct{}

const windowsServiceNotRegisteredMessage = "service artifacts generated, service not registered"

// Controller entry points are indirected through package-level variables so tests
// can swap them for fakes and assert the exact stop/start/enable sequence a
// register/operate/delete path issues. Production code must call these variables,
// never controller.* directly, on the managed-service (windows/linux) paths.
//
// The managed* implementations deliberately bypass controller.LoadServiceName:
// that helper lowercases the name and appends ".service" via loadProcessedName,
// which breaks case-sensitive systemd unit filenames (a "MyService" unit installed
// at /etc/systemd/system/MyService.service would be queried as "myservice.service"
// and never found). Callers pass the exact OS service identifier — build it with
// managedControllerUnitName so the query name matches the on-disk unit name.
var (
	controllerHandle      = managedControllerHandle
	controllerReload      = controller.Reload
	controllerCheckExist  = managedControllerCheckExist
	controllerCheckActive = managedControllerCheckActive
)

// managedControllerUnitName maps a managed-service Name to the exact identifier the
// OS service manager expects, without LoadServiceName's lowercasing/suffix rewrite.
// On systemd it appends ".service" (matching linuxSystemdUnitPath's on-disk unit
// filename, case preserved); on Windows (SCM) and non-systemd managers it returns
// the raw name. Service names ending in ".service" are rejected at validation on
// Linux (validateWindowsServiceName), so this never produces a "foo.service.service".
func managedControllerUnitName(name string) string {
	name = strings.TrimSpace(name)
	if platform.Current() != platform.OSLinux {
		return name
	}
	// Registration already guarantees systemctl exists; only append the suffix for
	// systemd. An openrc/sysvinit host uses the bare name.
	if client, err := controller.New(); err == nil && client.Name() != "systemd" {
		return name
	}
	return name + ".service"
}

func managedControllerHandle(operate, serviceName string) error {
	client, err := controller.New()
	if err != nil {
		return err
	}
	return client.Operate(operate, serviceName)
}

func managedControllerCheckExist(serviceName string) (bool, error) {
	client, err := controller.New()
	if err != nil {
		return false, err
	}
	return client.IsExist(serviceName)
}

func managedControllerCheckActive(serviceName string) (bool, error) {
	client, err := controller.New()
	if err != nil {
		return false, err
	}
	return client.IsActive(serviceName)
}

const (
	windowsServiceRemoveRetryAttempts = 6
	windowsServiceRemoveRetryDelay    = 200 * time.Millisecond
)

type IWindowsServiceService interface {
	List() ([]response.WindowsServiceInfo, error)
	GetConfigTemplate(serviceType string) (*response.WindowsServiceConfigTemplate, error)
	GetConfigFile(id uint, fileType string) (*response.WindowsServiceConfigFile, error)
	UpdateConfigFile(id uint, req request.WindowsServiceConfigFileUpdate) error
	GetLogFile(id uint) (*response.WindowsServiceLogFile, error)
	Create(req request.WindowsServiceCreate) error
	UploadJar(name, serviceType, fileName string, content io.Reader) (*response.WindowsServiceJarUploadResult, error)
	UploadPackage(name, serviceType, fileName string, content io.Reader) (*response.WindowsServiceJarUploadResult, error)
	PreviewPackage(serviceType, workDir string) (*response.WindowsServiceJarUploadResult, error)
	Update(req request.WindowsServiceUpdate) error
	Operate(req request.WindowsServiceOperate) (*response.WindowsServiceInfo, error)
	Delete(id uint) error
}

func NewIWindowsServiceService() IWindowsServiceService {
	return &WindowsServiceService{}
}

var windowsServiceRepo = repo.NewIWindowsServiceRepo()

func parseRegisterService(enabled *bool) bool {
	if enabled == nil {
		return true
	}
	return *enabled
}

func (w *WindowsServiceService) List() ([]response.WindowsServiceInfo, error) {
	if err := ensureManagedServicePlatform(); err != nil {
		return []response.WindowsServiceInfo{}, nil
	}
	items, err := windowsServiceRepo.List()
	if err != nil {
		return nil, err
	}
	result := make([]response.WindowsServiceInfo, 0, len(items))
	for _, item := range items {
		w.refreshRuntimeStatus(&item)
		result = append(result, toWindowsServiceInfo(item))
	}
	return result, nil
}

func (w *WindowsServiceService) GetConfigTemplate(serviceType string) (*response.WindowsServiceConfigTemplate, error) {
	switch strings.TrimSpace(serviceType) {
	case "java", "package":
		return defaultManagedWindowsServiceConfigResponse(), nil
	case "dll":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported service type %s", serviceType)
	}
}

func (w *WindowsServiceService) GetConfigFile(id uint, fileType string) (*response.WindowsServiceConfigFile, error) {
	if err := ensureManagedServicePlatform(); err != nil {
		return nil, err
	}
	item, err := windowsServiceRepo.Get(repo.WithByID(id))
	if err != nil {
		return nil, err
	}
	return w.getServiceConfigFile(item, fileType)
}

func (w *WindowsServiceService) UpdateConfigFile(id uint, req request.WindowsServiceConfigFileUpdate) error {
	if err := ensureManagedServicePlatform(); err != nil {
		return err
	}
	item, err := windowsServiceRepo.Get(repo.WithByID(id))
	if err != nil {
		return err
	}
	return w.updateServiceConfigFile(item, req.Type, req.Content)
}

func (w *WindowsServiceService) GetLogFile(id uint) (*response.WindowsServiceLogFile, error) {
	if err := ensureManagedServicePlatform(); err != nil {
		return nil, err
	}
	item, err := windowsServiceRepo.Get(repo.WithByID(id))
	if err != nil {
		return nil, err
	}
	return w.getServiceLogFile(item)
}

func (w *WindowsServiceService) Create(req request.WindowsServiceCreate) error {
	if err := ensureManagedServicePlatform(); err != nil {
		return err
	}
	if err := validateWindowsServiceCreateRequest(req); err != nil {
		return err
	}
	registerService := parseRegisterService(req.RegisterService)
	items := []model.WindowsService{{
		Name:            strings.TrimSpace(req.Name),
		DisplayName:     strings.TrimSpace(req.DisplayName),
		ServiceType:     req.ServiceType,
		ExecPath:        strings.TrimSpace(req.ExecPath),
		Args:            strings.TrimSpace(req.Args),
		WorkDir:         strings.TrimSpace(req.WorkDir),
		DLLDir:          strings.TrimSpace(req.DLLDir),
		ConfigPath:      strings.TrimSpace(req.ConfigPath),
		JarPath:         strings.TrimSpace(req.JarPath),
		WinSWPath:       strings.TrimSpace(req.WinSWPath),
		Status:          "Waiting",
		Message:         "",
		RegisterService: registerService,
		AutoStart:       req.AutoStart,
	}}
	if isDeliveryPackageTemplateCreateRequest(req) {
		deliveryPackageItems, err := buildDeliveryPackageCreateItems(req, common.GetDockerComposeCommand(), registerService)
		if err != nil {
			return err
		}
		items = deliveryPackageItems
	}
	for index := range items {
		if err := w.normalizeManagedFields(&items[index]); err != nil {
			return err
		}
	}
	for index := range items {
		configTemplate := req.ConfigTemplate
		if isDeliveryPackageComposeItem(items[index]) {
			configTemplate = nil
		}
		if err := w.prepareArtifacts(&items[index], strings.TrimSpace(req.EnvFilePath), configTemplate); err != nil {
			w.cleanupCreatedWindowsServices(items[:index+1])
			return err
		}
	}
	return w.createPreparedWindowsServices(items)
}

func isDeliveryPackageTemplateCreateRequest(req request.WindowsServiceCreate) bool {
	return strings.TrimSpace(req.ServiceType) == "package" &&
		strings.TrimSpace(req.WorkDir) != "" &&
		strings.TrimSpace(req.JarPath) == ""
}

func (w *WindowsServiceService) Update(req request.WindowsServiceUpdate) error {
	if err := ensureManagedServicePlatform(); err != nil {
		return err
	}
	item, err := windowsServiceRepo.Get(repo.WithByID(req.ID))
	if err != nil {
		return err
	}
	previousRegistered := item.RegisterService
	// Snapshot the pre-update record: if the service is being renamed we must tear
	// down the OLD registration under its OLD name, otherwise the old unit is left
	// orphaned (still enabled, ExecStart pointing at the now-defunct old service
	// dir) and would be relaunched on the next boot.
	previous := item
	registerService := parseRegisterService(req.RegisterService)
	item.Name = strings.TrimSpace(req.Name)
	item.DisplayName = strings.TrimSpace(req.DisplayName)
	item.ServiceType = req.ServiceType
	item.ExecPath = strings.TrimSpace(req.ExecPath)
	item.Args = strings.TrimSpace(req.Args)
	item.WorkDir = strings.TrimSpace(req.WorkDir)
	item.DLLDir = strings.TrimSpace(req.DLLDir)
	item.ConfigPath = strings.TrimSpace(req.ConfigPath)
	item.JarPath = strings.TrimSpace(req.JarPath)
	item.WinSWPath = strings.TrimSpace(req.WinSWPath)
	item.RegisterService = registerService
	item.AutoStart = req.AutoStart
	if err := w.normalizeManagedFields(&item); err != nil {
		return err
	}
	renamed := strings.TrimSpace(previous.Name) != "" && previous.Name != item.Name
	configTemplate := req.ConfigTemplate
	if isDeliveryPackageComposeItem(item) {
		configTemplate = nil
	}
	if err := w.prepareArtifacts(&item, strings.TrimSpace(req.EnvFilePath), configTemplate); err != nil {
		return err
	}
	// On a rename the OLD registration is torn down (by its OLD name) only AFTER
	// the new service is fully prepared, registered, and saved: the new name/dir
	// cannot collide with the old, a brief coexistence is harmless, and the old
	// service stays intact if any step for the new one fails — never destroy the
	// working registration before its replacement exists. syncRegistration must
	// therefore not tear down under the new name on a rename.
	teardownPrevious := previousRegistered && !renamed
	if err := w.syncRegistration(&item, teardownPrevious); err != nil {
		return err
	}
	if err := windowsServiceRepo.Save(context.Background(), &item); err != nil {
		return err
	}
	if renamed && previous.RegisterService {
		w.unregisterManagedService(previous)
		if previous.ServicePath != "" {
			oldServiceDir := filepath.Dir(previous.ServicePath)
			if pathWithin(loadWindowsServiceRoot(), oldServiceDir) {
				_ = os.RemoveAll(oldServiceDir)
			}
		}
	}
	return nil
}

func (w *WindowsServiceService) UploadJar(name, serviceType, fileName string, content io.Reader) (*response.WindowsServiceJarUploadResult, error) {
	// Storing the uploaded jar artifact is platform-neutral; the Windows gate
	// belongs on registration (Create/Operate/Delete), not on the upload itself.
	return w.uploadJarPackage(name, serviceType, fileName, content)
}

func (w *WindowsServiceService) UploadPackage(name, serviceType, fileName string, content io.Reader) (*response.WindowsServiceJarUploadResult, error) {
	// Archive extraction + config preview is platform-neutral (no Windows APIs);
	// the platform gate belongs on registration (Create/Operate/Delete), not here,
	// so a non-Windows core node can still parse and preview an uploaded package.
	name = strings.TrimSpace(name)
	serviceType = strings.TrimSpace(serviceType)

	safeFileName := sanitizeUploadFileName(fileName)
	if safeFileName == "" {
		return nil, errors.New("package file name is required")
	}
	if strings.EqualFold(filepath.Ext(safeFileName), ".jar") {
		return w.uploadJarPackage(name, serviceType, safeFileName, content)
	}
	if serviceType == "" {
		serviceType = "package"
	}
	if name == "" && serviceType == "package" && isSupportedDeliveryPackageArchive(safeFileName) {
		name = buildWindowsServicePackageKey(safeFileName)
	}
	if name == "" {
		return nil, errors.New("service name is required")
	}
	if serviceType != "java" && serviceType != "package" {
		return nil, errors.New("package upload is only supported for java and delivery package services")
	}
	if serviceType != "package" || !isSupportedDeliveryPackageArchive(safeFileName) {
		return nil, errors.New("delivery package only supports .jar, .zip, .tar.gz, .tgz, .tar.xz, and .txz files")
	}

	templateDir := buildWindowsServiceTemplateUploadDir(name, safeFileName)
	packageDir := filepath.Join(buildWindowsServiceDir(name), "package", filepath.Base(templateDir))
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		return nil, err
	}
	packagePath := filepath.Join(packageDir, safeFileName)
	if err := writeUploadedFile(packagePath, content); err != nil {
		return nil, err
	}
	if err := extractDeliveryPackageArchive(packagePath, templateDir); err != nil {
		return nil, err
	}
	configTemplate, _, err := discoverManagedWindowsServiceConfig(templateDir)
	if err != nil {
		return nil, err
	}
	var configTemplateResp *response.WindowsServiceConfigTemplate
	if configTemplate != nil {
		configTemplateResp = configTemplate.toResponse("")
	}
	services, _ := buildDeliveryPackagePackageServicePreviews(request.WindowsServiceCreate{
		ServiceType: "package",
		WorkDir:     templateDir,
		WinSWPath:   defaultManagedServiceWinSWPath(),
	}, common.GetDockerComposeCommand())
	return &response.WindowsServiceJarUploadResult{
		WorkDir:        templateDir,
		WinSWPath:      defaultManagedServiceWinSWPath(),
		FileName:       safeFileName,
		InstallDir:     loadWindowsInstallDir(),
		ConfigTemplate: configTemplateResp,
		Services:       services,
	}, nil
}

func (w *WindowsServiceService) PreviewPackage(serviceType, workDir string) (*response.WindowsServiceJarUploadResult, error) {
	if err := ensureManagedServicePlatform(); err != nil {
		return nil, err
	}
	serviceType = strings.TrimSpace(serviceType)
	if serviceType == "" {
		serviceType = "package"
	}
	if serviceType != "package" {
		return nil, errors.New("package preview is only supported for delivery package services")
	}
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return nil, errors.New("delivery package template directory is required")
	}
	// The only legitimate workDir is an upload template dir under the service
	// root; reject anything else so a caller cannot walk and disclose arbitrary
	// host directories via the preview response.
	if !pathWithin(loadWindowsServiceRoot(), workDir) {
		return nil, errors.New("delivery package template directory is outside the managed service root")
	}
	configTemplate, _, err := discoverManagedWindowsServiceConfig(workDir)
	if err != nil {
		return nil, err
	}
	var configTemplateResp *response.WindowsServiceConfigTemplate
	if configTemplate != nil {
		configTemplateResp = configTemplate.toResponse("")
	}
	services, err := buildDeliveryPackagePackageServicePreviews(request.WindowsServiceCreate{
		ServiceType: "package",
		WorkDir:     workDir,
		WinSWPath:   defaultManagedServiceWinSWPath(),
	}, common.GetDockerComposeCommand())
	if err != nil {
		return nil, err
	}
	return &response.WindowsServiceJarUploadResult{
		WorkDir:        workDir,
		WinSWPath:      defaultManagedServiceWinSWPath(),
		InstallDir:     loadWindowsInstallDir(),
		ConfigTemplate: configTemplateResp,
		Services:       services,
	}, nil
}

func (w *WindowsServiceService) uploadJarPackage(name, serviceType, fileName string, content io.Reader) (*response.WindowsServiceJarUploadResult, error) {
	name = strings.TrimSpace(name)
	serviceType = strings.TrimSpace(serviceType)
	if name == "" {
		return nil, errors.New("service name is required")
	}
	if serviceType != "java" && serviceType != "package" {
		return nil, errors.New("jar upload is only supported for java and delivery package services")
	}

	safeFileName := sanitizeUploadFileName(fileName)
	if safeFileName == "" || !strings.EqualFold(filepath.Ext(safeFileName), ".jar") {
		return nil, errors.New("only .jar files are supported")
	}

	appDir := buildWindowsServiceAppDir(name)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, err
	}
	if err := removeExistingJarFiles(appDir, safeFileName); err != nil {
		return nil, err
	}

	targetPath := filepath.Join(appDir, safeFileName)
	tmpPath := targetPath + ".uploading"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(tmpFile, content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return nil, err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	javaPath, err := resolveBundledJavaExecutable()
	if err != nil {
		return nil, err
	}

	return &response.WindowsServiceJarUploadResult{
		JarPath:    targetPath,
		WorkDir:    appDir,
		ExecPath:   javaPath,
		WinSWPath:  defaultManagedServiceWinSWPath(),
		FileName:   safeFileName,
		InstallDir: loadWindowsInstallDir(),
	}, nil
}

func (w *WindowsServiceService) Operate(req request.WindowsServiceOperate) (*response.WindowsServiceInfo, error) {
	if err := ensureManagedServicePlatform(); err != nil {
		return nil, err
	}
	item, err := windowsServiceRepo.Get(repo.WithByID(req.ID))
	if err != nil {
		return nil, err
	}
	if !item.RegisterService {
		return nil, errors.New(windowsServiceNotRegisteredMessage)
	}
	// "status" is a read-only query: never drive it through the controller (on
	// systemd `systemctl status` exits non-zero for any inactive-but-registered
	// unit, which would otherwise persist a bogus UnHealthy). Refresh the true
	// runtime state and return it without turning a read into a written error.
	if req.Operate == "status" {
		w.refreshRuntimeStatus(&item)
		if err := windowsServiceRepo.Save(context.Background(), &item); err != nil {
			return nil, err
		}
		info := toWindowsServiceInfo(item)
		return &info, nil
	}
	// Both windows (WinSW/SCM) and linux (systemd) drive the real service through
	// the controller; there is no fake start/stop on either platform anymore.
	if err := controllerHandle(req.Operate, managedControllerUnitName(item.Name)); err != nil {
		item.Status = "UnHealthy"
		item.Message = err.Error()
		_ = windowsServiceRepo.Save(context.Background(), &item)
		info := toWindowsServiceInfo(item)
		return &info, err
	}
	switch req.Operate {
	case "enable", "disable":
		item.AutoStart = req.Operate == "enable"
		// enable/disable only flip the autostart flag — they do not start or stop the
		// process — so statusByOperate's enable→Healthy mapping would persist a state
		// the service is not actually in. Read the real runtime state instead.
		w.refreshRuntimeStatus(&item)
	default:
		item.Status = statusByOperate(req.Operate)
		item.Message = ""
	}
	if err := windowsServiceRepo.Save(context.Background(), &item); err != nil {
		return nil, err
	}
	info := toWindowsServiceInfo(item)
	return &info, nil
}

func (w *WindowsServiceService) Delete(id uint) error {
	if err := ensureManagedServicePlatform(); err != nil {
		return err
	}
	item, err := windowsServiceRepo.Get(repo.WithByID(id))
	if err != nil {
		return err
	}
	if item.RegisterService {
		w.unregisterManagedService(item)
	}
	if item.ServicePath != "" {
		serviceDir := filepath.Dir(item.ServicePath)
		if pathWithin(loadWindowsServiceRoot(), serviceDir) {
			_ = os.RemoveAll(serviceDir)
		}
	}
	return windowsServiceRepo.Delete(context.Background(), repo.WithByID(id))
}

func buildWindowsServicePath(name string) string {
	return filepath.Join(loadWindowsServiceRoot(), name, name+".xml")
}

func buildWindowsServiceAppDir(name string) string {
	return filepath.Join(buildWindowsServiceDir(name), "app")
}

func buildWindowsServiceTemplateDir(name string) string {
	return filepath.Join(buildWindowsServiceDir(name), "template")
}

func buildWindowsServiceTemplateUploadDir(name string, fileName string) string {
	baseName := buildWindowsServicePackageKey(fileName)
	if baseName == "" {
		baseName = "package"
	}
	uploadID := fmt.Sprintf("%s-%s", baseName, time.Now().UTC().Format("20060102150405.000000000"))
	return filepath.Join(buildWindowsServiceTemplateDir(name), uploadID)
}

func buildWindowsServiceUninstallCmdPath(name string) string {
	return filepath.Join(buildWindowsServiceDir(name), name+"-uninstall.cmd")
}

// buildManagedServiceUninstallScriptPath is the platform-appropriate uninstall
// helper surfaced in the API response: a WinSW-driven .cmd on Windows, a systemd
// teardown .sh on Linux.
func buildManagedServiceUninstallScriptPath(name string) string {
	if platform.Current() == platform.OSLinux {
		return filepath.Join(buildWindowsServiceDir(name), name+"-uninstall.sh")
	}
	return buildWindowsServiceUninstallCmdPath(name)
}

func buildWindowsServiceUninstallPS1Path(name string) string {
	return filepath.Join(buildWindowsServiceDir(name), name+"-uninstall.ps1")
}

// statusByOperate maps a process-affecting operate to the state it leaves the
// service in. enable/disable/status never reach here (Operate refreshes the real
// runtime state for those instead of assuming one).
func statusByOperate(operate string) string {
	switch operate {
	case "start", "restart":
		return "Healthy"
	case "stop":
		return "Stopped"
	default:
		return "Waiting"
	}
}

func toWindowsServiceInfo(item model.WindowsService) response.WindowsServiceInfo {
	return response.WindowsServiceInfo{
		ID:                  item.ID,
		Name:                item.Name,
		DisplayName:         item.DisplayName,
		ServiceType:         item.ServiceType,
		ExecPath:            item.ExecPath,
		Args:                item.Args,
		WorkDir:             item.WorkDir,
		EnvFilePath:         item.EnvFilePath,
		DLLDir:              item.DLLDir,
		ConfigPath:          item.ConfigPath,
		JarPath:             item.JarPath,
		WinSWPath:           item.WinSWPath,
		ServicePath:         item.ServicePath,
		UninstallScriptPath: buildManagedServiceUninstallScriptPath(item.Name),
		Status:              item.Status,
		Message:             item.Message,
		ConfigTemplate:      loadManagedWindowsServiceConfig(item),
		RegisterService:     item.RegisterService,
		AutoStart:           item.AutoStart,
	}
}

func (w *WindowsServiceService) prepareArtifacts(item *model.WindowsService, envFilePath string, configReq *request.WindowsServiceConfigTemplate) error {
	managedConfig := buildManagedWindowsServiceConfig(configReq)
	if managedConfig != nil {
		item.ConfigPath = managedConfig.generatedPath(item.WorkDir)
		item.Args = ensureSpringConfigArgument(item.Args, item.ConfigPath)
	}
	serviceDir := buildWindowsServiceDir(item.Name)
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		return err
	}
	if managedConfig != nil {
		if err := writeManagedWindowsServiceConfig(item.ConfigPath, managedConfig.Content); err != nil {
			return err
		}
	}
	if platform.Current() == platform.OSLinux {
		return w.prepareLinuxArtifacts(item, serviceDir, envFilePath, managedConfig)
	}
	item.ServicePath = filepath.Join(serviceDir, item.Name+".xml")
	if envFilePath == "" {
		envFilePath = filepath.Join(serviceDir, item.Name+".env")
	}
	item.EnvFilePath = envFilePath
	if err := writeUTF8File(item.EnvFilePath, w.buildEnvFile(item, managedConfig)); err != nil {
		return err
	}
	scriptPath := filepath.Join(serviceDir, item.Name+".cmd")
	if err := os.WriteFile(scriptPath, []byte(w.buildStartScript(item)), 0o644); err != nil {
		return err
	}
	if err := writeUTF8File(filepath.Join(serviceDir, item.Name+".ps1"), w.buildPowerShellScript(item)); err != nil {
		return err
	}
	if err := os.WriteFile(buildWindowsServiceUninstallCmdPath(item.Name), []byte(w.buildUninstallScript(item)), 0o644); err != nil {
		return err
	}
	if err := writeUTF8File(buildWindowsServiceUninstallPS1Path(item.Name), w.buildServiceUninstallPowerShellScript(item)); err != nil {
		return err
	}
	if err := os.WriteFile(item.ServicePath, []byte(w.buildWinSWXML(item, scriptPath)), 0o644); err != nil {
		return err
	}
	return nil
}

func (w *WindowsServiceService) normalizeManagedFields(item *model.WindowsService) error {
	item.Name = strings.TrimSpace(item.Name)
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	item.ExecPath = strings.TrimSpace(item.ExecPath)
	item.WorkDir = strings.TrimSpace(item.WorkDir)
	item.EnvFilePath = strings.TrimSpace(item.EnvFilePath)
	item.DLLDir = strings.TrimSpace(item.DLLDir)
	item.ConfigPath = strings.TrimSpace(item.ConfigPath)
	item.JarPath = strings.TrimSpace(item.JarPath)
	item.WinSWPath = strings.TrimSpace(item.WinSWPath)

	if err := validateWindowsServiceName(item.Name); err != nil {
		return err
	}
	if item.DisplayName == "" {
		return errors.New("display name is required")
	}
	if item.WinSWPath == "" {
		item.WinSWPath = defaultManagedServiceWinSWPath()
	}

	switch item.ServiceType {
	case "package":
		if isDeliveryPackageComposeItem(*item) {
			if item.WorkDir == "" {
				item.WorkDir = filepath.Dir(item.ConfigPath)
			}
			// Defense in depth: the compose service name(s) in Args are embedded
			// verbatim into the generated launch command, so reject anything outside
			// the docker-compose legal charset ([A-Za-z0-9._-]) on the direct
			// Create/Update path too (the package-discovery path already validates).
			// Empty Args (=> `docker compose up`, i.e. every service) is allowed;
			// multiple space-separated names are each validated.
			composeServices := strings.Fields(item.Args)
			for _, serviceName := range composeServices {
				if !isValidComposeServiceName(serviceName) {
					return fmt.Errorf("invalid compose service name %q", serviceName)
				}
			}
			// Re-join with single spaces: Fields tolerates any whitespace (incl.
			// newlines), but the launch scripts re-emit Args verbatim, so "svc1\nsvc2"
			// must not survive into a multi-line exec command.
			item.Args = strings.Join(composeServices, " ")
			return nil
		}
		fallthrough
	case "java":
		javaPath, err := resolveBundledJavaExecutable()
		if err != nil {
			return err
		}
		item.ExecPath = javaPath
		if item.JarPath == "" {
			return errors.New("jar package is required for java and delivery package services")
		}
		if item.WorkDir == "" {
			item.WorkDir = buildWindowsServiceAppDir(item.Name)
		}
	case "dll":
		if platform.Current() != platform.OSWindows {
			return errors.New("dll services are only supported on windows")
		}
		if item.ExecPath == "" {
			return errors.New("executable path is required for dll services")
		}
		if item.WorkDir == "" {
			item.WorkDir = filepath.Dir(item.ExecPath)
		}
	default:
		return fmt.Errorf("unsupported service type %s", item.ServiceType)
	}
	return nil
}

func validateWindowsServiceCreateRequest(req request.WindowsServiceCreate) error {
	serviceType := strings.TrimSpace(req.ServiceType)
	if serviceType == "" {
		return errors.New("service type is required")
	}
	if serviceType != "java" && serviceType != "dll" && serviceType != "package" {
		return fmt.Errorf("unsupported service type %s", serviceType)
	}
	if isDeliveryPackageTemplateCreateRequest(req) {
		return nil
	}
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("service name is required")
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		return errors.New("display name is required")
	}
	return nil
}

func (w *WindowsServiceService) refreshRuntimeStatus(item *model.WindowsService) {
	if err := ensureManagedServicePlatform(); err != nil {
		return
	}
	if !item.RegisterService {
		item.Status = "NotRegistered"
		item.Message = windowsServiceNotRegisteredMessage
		return
	}
	unitName := managedControllerUnitName(item.Name)
	// Existence is authoritative: systemd's IsExist returns (true, exit-1 error) for a
	// disabled-but-present unit, so an accompanying error when exist==true is expected
	// and must NOT be surfaced as UnHealthy — fall through to the active check. Only
	// exist==false maps to Waiting; a real system error there (permission/timeout) is
	// logged rather than silently swallowed, but the UI state stays Waiting.
	exist, err := controllerCheckExist(unitName)
	if !exist {
		if err != nil {
			global.LOG.Debugf("managed service %s existence check returned error, treating as not installed: %v", item.Name, err)
		}
		item.Status = "Waiting"
		item.Message = "service not installed"
		return
	}
	active, err := controllerCheckActive(unitName)
	if err != nil {
		item.Status = "UnHealthy"
		item.Message = err.Error()
		return
	}
	if active {
		item.Status = "Healthy"
		item.Message = ""
		return
	}
	item.Status = "Stopped"
	item.Message = ""
}

func (w *WindowsServiceService) getServiceConfigFile(item model.WindowsService, fileType string) (*response.WindowsServiceConfigFile, error) {
	configPath, err := resolveWindowsServiceConfigFilePath(item, fileType)
	if err != nil {
		return nil, err
	}
	content, err := readFileWithLimit(configPath, maxWindowsServiceConfigFileSize)
	if err != nil {
		return nil, err
	}
	return &response.WindowsServiceConfigFile{
		Type:    strings.TrimSpace(fileType),
		Path:    configPath,
		Content: string(content),
	}, nil
}

func (w *WindowsServiceService) updateServiceConfigFile(item model.WindowsService, fileType, content string) error {
	configPath, err := resolveWindowsServiceConfigFilePath(item, fileType)
	if err != nil {
		return err
	}
	info, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s file does not exist: %s", strings.TrimSpace(fileType), configPath)
		}
		return err
	}
	if err := atomicWriteFile(configPath, []byte(content), info.Mode().Perm()); err != nil {
		return err
	}
	return nil
}

func (w *WindowsServiceService) getServiceLogFile(item model.WindowsService) (*response.WindowsServiceLogFile, error) {
	logPath, exists := resolveWindowsServicePrimaryLogPath(item.Name)
	logFile := &response.WindowsServiceLogFile{
		Name:   filepath.Base(logPath),
		Path:   logPath,
		Exists: exists,
	}
	if !exists {
		return logFile, nil
	}
	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return logFile, nil
		}
		return nil, err
	}
	logFile.Size = info.Size()
	logFile.ModifiedAt = info.ModTime()
	return logFile, nil
}

func (w *WindowsServiceService) buildEnvFile(item *model.WindowsService, managedConfig *managedWindowsServiceConfig) string {
	var lines []string
	if item.DLLDir != "" {
		lines = append(lines, "DLL_DIR="+item.DLLDir)
	}
	if item.ConfigPath != "" {
		lines = append(lines, "CONFIG_PATH="+item.ConfigPath)
	}
	if item.JarPath != "" {
		lines = append(lines, "JAR_PATH="+item.JarPath)
	}
	lines = appendManagedWindowsServiceConfigEnv(lines, managedConfig)
	// systemd EnvironmentFile requires LF and no BOM; the Windows PowerShell loader
	// reads the file as CRLF. The writer picks BOM vs no-BOM per platform.
	sep := "\r\n"
	if platform.Current() == platform.OSLinux {
		sep = "\n"
	}
	return strings.Join(lines, sep) + sep
}

// unregisterManagedService tears down an already-registered service from its OS
// service manager, best-effort, dispatching by platform.
func (w *WindowsServiceService) unregisterManagedService(item model.WindowsService) {
	switch platform.Current() {
	case platform.OSWindows:
		_ = w.operateWinSW(item, "stop")
		_ = w.operateWinSW(item, "uninstall")
	case platform.OSLinux:
		w.unregisterLinuxService(item)
	}
}

func (w *WindowsServiceService) syncRegistration(item *model.WindowsService, previousRegistered bool) error {
	if platform.Current() == platform.OSLinux {
		return w.syncLinuxRegistration(item, previousRegistered)
	}
	if platform.Current() != platform.OSWindows {
		return nil
	}

	if previousRegistered {
		_ = w.operateWinSW(*item, "stop")
		_ = w.operateWinSW(*item, "uninstall")
	}

	if !item.RegisterService {
		item.Status = "NotRegistered"
		item.Message = windowsServiceNotRegisteredMessage
		return nil
	}
	if strings.TrimSpace(item.WinSWPath) == "" {
		item.Status = "UnHealthy"
		item.Message = "winsw path is required when registering windows service"
		return errors.New(item.Message)
	}
	if err := w.ensureWinSWExecutable(*item); err != nil {
		item.Status = "UnHealthy"
		item.Message = err.Error()
		return err
	}
	if err := w.operateWinSW(*item, "install"); err != nil {
		item.Status = "UnHealthy"
		item.Message = err.Error()
		return err
	}
	if item.AutoStart {
		if err := controllerHandle("enable", managedControllerUnitName(item.Name)); err != nil {
			item.Status = "UnHealthy"
			item.Message = err.Error()
			return err
		}
	} else {
		if err := controllerHandle("disable", managedControllerUnitName(item.Name)); err != nil {
			item.Status = "UnHealthy"
			item.Message = err.Error()
			return err
		}
	}
	item.Status = "Stopped"
	item.Message = ""
	return nil
}

func (w *WindowsServiceService) createPreparedWindowsServices(items []model.WindowsService) error {
	created := make([]model.WindowsService, 0, len(items))
	for index := range items {
		if err := w.syncRegistration(&items[index], false); err != nil {
			w.cleanupCreatedWindowsServices(append(created, items[index]))
			return err
		}
		if err := windowsServiceRepo.Create(context.Background(), &items[index]); err != nil {
			w.cleanupCreatedWindowsServices(append(created, items[index]))
			return err
		}
		created = append(created, items[index])
	}
	return nil
}

func (w *WindowsServiceService) cleanupCreatedWindowsServices(items []model.WindowsService) {
	for _, item := range items {
		if item.RegisterService {
			w.unregisterManagedService(item)
		}
		if item.ServicePath != "" {
			_ = os.RemoveAll(filepath.Dir(item.ServicePath))
		}
		if item.ID != 0 {
			_ = windowsServiceRepo.Delete(context.Background(), repo.WithByID(item.ID))
		}
	}
}

func (w *WindowsServiceService) buildStartScript(item *model.WindowsService) string {
	return fmt.Sprintf(`@echo off
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%%~dp0%s.ps1"
`, item.Name)
}

func (w *WindowsServiceService) buildUninstallScript(item *model.WindowsService) string {
	return fmt.Sprintf(`@echo off
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%%~dp0%s-uninstall.ps1"
`, item.Name)
}

func (w *WindowsServiceService) buildPowerShellScript(item *model.WindowsService) string {
	workDir := item.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(item.ExecPath)
	}
	envFilePath := item.EnvFilePath
	if envFilePath == "" {
		envFilePath = filepath.Join(buildWindowsServiceDir(item.Name), item.Name+".env")
	}
	commandLine := fmt.Sprintf("& \"%s\"", item.ExecPath)
	args := strings.TrimSpace(item.Args)
	if isDeliveryPackageComposeItem(*item) {
		composeCmd := strings.TrimSpace(item.ExecPath)
		serviceName := strings.TrimSpace(item.Args)
		commandLine = composeCmd + fmt.Sprintf(" -f \"%s\" up", item.ConfigPath)
		if serviceName != "" {
			commandLine += " " + serviceName
		}
	} else if (item.ServiceType == "java" || item.ServiceType == "package") && item.JarPath != "" && !strings.Contains(args, "-jar") {
		appArgs := ""
		args, appArgs = detachManagedSpringConfigArgument(args, item.ConfigPath)
		if args != "" {
			commandLine += " " + args
		}
		commandLine += fmt.Sprintf(" -jar \"%s\"", item.JarPath)
		if appArgs != "" {
			commandLine += " " + appArgs
		}
	} else if args != "" {
		commandLine += " " + args
	}

	// The generated script must run under Windows PowerShell 2.0, the version that
	// ships with a stock Windows 7 SP1 / Server 2008 R2. PowerShell 2.0 executes on
	// the .NET 2.0 CLR, so .NET 4.0+ APIs (e.g. [string]::IsNullOrWhiteSpace) and the
	// String.Split(char, int) overload are unavailable and abort the script (with
	// $ErrorActionPreference='Stop') before the service ever starts. Everything here
	// is limited to PowerShell 2.0 / .NET 2.0 constructs: .Trim(), IndexOf/Substring,
	// StartsWith, and null-guarded property access.
	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$envFile = '%s'
if (Test-Path $envFile) {
  foreach ($line in [System.IO.File]::ReadAllLines($envFile, [System.Text.Encoding]::UTF8)) {
    if ($line -eq $null -or $line.Trim() -eq '' -or $line.StartsWith('#')) {
      continue
    }
    $idx = $line.IndexOf('=')
    if ($idx -gt 0) {
      $envKey = $line.Substring(0, $idx)
      $envValue = $line.Substring($idx + 1)
      Set-Item -Path ('Env:' + $envKey) -Value $envValue
    }
  }
}
if ($env:DLL_DIR -and $env:DLL_DIR.Trim() -ne '') {
  $env:Path = $env:DLL_DIR + ';' + $env:Path
}
Set-Location '%s'
%s
`, escapePowerShellSingleQuoted(envFilePath), escapePowerShellSingleQuoted(workDir), commandLine)
}

func (w *WindowsServiceService) buildServiceUninstallPowerShellScript(item *model.WindowsService) string {
	serviceDir := buildWindowsServiceDir(item.Name)
	wrapperPath := filepath.Join(serviceDir, item.Name+".exe")
	cmdPath := buildWindowsServiceUninstallCmdPath(item.Name)
	ps1Path := buildWindowsServiceUninstallPS1Path(item.Name)

	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$wrapperPath = '%s'
if (Test-Path $wrapperPath) {
  try {
    & $wrapperPath stop | Out-Null
  } catch {}
  try {
    & $wrapperPath uninstall | Out-Null
  } catch {}
}
$cleanupScript = Join-Path $env:TEMP ('1panel-service-cleanup-%s.cmd')
$cleanupContent = @"
@echo off
timeout /t 2 /nobreak >nul
del /f /q "%s" >nul 2>nul
del /f /q "%s" >nul 2>nul
rmdir /s /q "%s"
del /f /q "%%~f0"
"@
[System.IO.File]::WriteAllText($cleanupScript, $cleanupContent, [System.Text.Encoding]::ASCII)
Start-Process -FilePath 'cmd.exe' -ArgumentList @('/c', $cleanupScript) -WindowStyle Hidden | Out-Null
`, escapePowerShellSingleQuoted(wrapperPath), escapePowerShellSingleQuoted(item.Name), cmdPath, ps1Path, serviceDir)
}

func writeUTF8File(path, content string) error {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte(content)...)
	return atomicWriteFile(path, data, 0o644)
}

// atomicWriteFile writes data to a sibling temp file and renames it over path,
// so a crash or full disk mid-write cannot leave a truncated/corrupt file for a
// managed service to load. os.Rename is atomic within the same directory.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if _, statErr := os.Stat(tmpName); statErr == nil {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (w *WindowsServiceService) buildWinSWXML(item *model.WindowsService, scriptPath string) string {
	logDir := buildWindowsServiceLogDir()
	_ = os.MkdirAll(logDir, 0o755)
	return fmt.Sprintf(`<service>
  <id>%s</id>
  <name>%s</name>
  <description>%s</description>
  <executable>cmd.exe</executable>
  <arguments>/c "%s"</arguments>
  <workingdirectory>%s</workingdirectory>
  <logpath>%s</logpath>
  <log mode="roll" />
  <stoptimeout>15sec</stoptimeout>
  <onfailure action="restart" delay="5 sec" />
</service>
`, escapeXML(item.Name), escapeXML(item.DisplayName), escapeXML(item.ServiceType+" service"), escapeXML(scriptPath), escapeXML(filepath.Dir(scriptPath)), escapeXML(logDir))
}

func (w *WindowsServiceService) ensureWinSWExecutable(item model.WindowsService) error {
	source := strings.TrimSpace(item.WinSWPath)
	if source == "" {
		return nil
	}
	target := filepath.Join(filepath.Dir(item.ServicePath), item.Name+".exe")
	srcFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (w *WindowsServiceService) operateWinSW(item model.WindowsService, operate string) error {
	wrapperPath := filepath.Join(filepath.Dir(item.ServicePath), item.Name+".exe")
	if _, err := os.Stat(wrapperPath); err != nil {
		return err
	}
	cmdArgs := []string{operate}
	if operate == "refresh" {
		cmdArgs = []string{"refresh"}
	}
	cmdItem := exec.Command(wrapperPath, cmdArgs...)
	cmdItem.Dir = filepath.Dir(item.ServicePath)
	output, err := cmdItem.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(manager.DecodeWindowsOutput(output))
		if isCLRRuntimeLoadFailure(err) {
			return fmt.Errorf("WinSW.exe requires .NET Framework 4.x which is not installed on this system (exit code 0x80131700); install .NET Framework 4.6.1 or later and retry")
		}
		return fmt.Errorf("%s: %s", err.Error(), detail)
	}
	return nil
}

// isCLRRuntimeLoadFailure reports whether the command failed with
// CLR_E_SHIM_RUNTIMELOAD (0x80131700): the process is a .NET Framework
// executable (e.g. WinSW.exe) but no matching .NET runtime is installed —
// the stock state of Windows 7 SP1, which ships without .NET 4.x.
func isCLRRuntimeLoadFailure(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	return uint32(exitErr.ExitCode()) == 0x80131700
}

func loadWindowsServiceRoot() string {
	// Linux keeps managed services inside the 1panel data tree; Windows preserves
	// the legacy <InstallDir>/service layout so already-deployed services still resolve.
	if platform.Current() == platform.OSLinux {
		return filepath.Join(loadWindowsInstallDir(), "1panel", "services")
	}
	return filepath.Join(loadWindowsInstallDir(), "service")
}

func loadWindowsLogRoot() string {
	return filepath.Join(loadWindowsInstallDir(), "1panel", "log")
}

func resolveWindowsServiceConfigFilePath(item model.WindowsService, fileType string) (string, error) {
	switch strings.TrimSpace(fileType) {
	case "config":
		if strings.TrimSpace(item.ConfigPath) == "" {
			return "", errors.New("config file path is empty")
		}
		return strings.TrimSpace(item.ConfigPath), nil
	case "env":
		if strings.TrimSpace(item.EnvFilePath) == "" {
			return "", errors.New("env file path is empty")
		}
		return strings.TrimSpace(item.EnvFilePath), nil
	default:
		return "", fmt.Errorf("invalid config file type %s", fileType)
	}
}

func buildWindowsServiceLogDir() string {
	segment := "windows-services"
	if platform.Current() == platform.OSLinux {
		segment = "services"
	}
	return filepath.Join(loadWindowsLogRoot(), segment)
}

func buildWindowsServiceLogCandidates(name string) []string {
	basePath := filepath.Join(buildWindowsServiceLogDir(), strings.TrimSpace(name))
	return []string{
		basePath + ".out.log",
		basePath + ".err.log",
		basePath + ".wrapper.log",
	}
}

func resolveWindowsServicePrimaryLogPath(name string) (string, bool) {
	candidates := buildWindowsServiceLogCandidates(name)
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return candidates[0], false
}

func buildWindowsServiceDir(name string) string {
	return filepath.Join(loadWindowsServiceRoot(), name)
}

func loadWindowsInstallDir() string {
	baseDir := strings.TrimSpace(global.CONF.Base.InstallDir)
	if baseDir == "" {
		baseDir = platform.DefaultInstallDir()
	}
	return baseDir
}

func loadDefaultWinSWPath() string {
	return filepath.Join(loadWindowsInstallDir(), "tools", "WinSW.exe")
}

func resolveBundledJavaExecutable() (string, error) {
	baseDir := loadWindowsInstallDir()
	javaBinName := "java.exe"
	if platform.Current() == platform.OSLinux {
		javaBinName = "java"
	}
	patterns := []string{
		filepath.Join(baseDir, "zulu*"),
		filepath.Join(baseDir, "*jdk*"),
		filepath.Join(baseDir, "*java*"),
	}
	var candidates []string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			javaPath := filepath.Join(match, "bin", javaBinName)
			if _, err := os.Stat(javaPath); err == nil {
				candidates = append(candidates, javaPath)
			}
		}
	}
	sort.Strings(candidates)
	if len(candidates) > 0 {
		return candidates[0], nil
	}
	// Linux hosts commonly rely on a system JRE rather than a bundled one.
	if platform.Current() == platform.OSLinux {
		if javaPath, err := exec.LookPath("java"); err == nil {
			return javaPath, nil
		}
		return "", fmt.Errorf("bundled java runtime not found under %s and 'java' is not in PATH; install a JRE/JDK", baseDir)
	}
	return "", fmt.Errorf("bundled java runtime not found under %s", baseDir)
}

func sanitizeUploadFileName(fileName string) string {
	baseName := strings.TrimSpace(filepath.Base(fileName))
	baseName = strings.ReplaceAll(baseName, "/", "")
	baseName = strings.ReplaceAll(baseName, "\\", "")
	return baseName
}

func removeExistingJarFiles(dir, keepFileName string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".jar") {
			continue
		}
		if entry.Name() == keepFileName {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func isSupportedDeliveryPackageArchive(fileName string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(fileName))
	return strings.HasSuffix(lowerName, ".zip") ||
		strings.HasSuffix(lowerName, ".tar.gz") ||
		strings.HasSuffix(lowerName, ".tgz") ||
		strings.HasSuffix(lowerName, ".tar.xz") ||
		strings.HasSuffix(lowerName, ".txz")
}

func buildWindowsServicePackageKey(fileName string) string {
	name := sanitizeUploadFileName(fileName)
	lowerName := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lowerName, ".tar.gz"):
		name = name[:len(name)-len(".tar.gz")]
	case strings.HasSuffix(lowerName, ".tar.xz"):
		name = name[:len(name)-len(".tar.xz")]
	case strings.HasSuffix(lowerName, ".tgz"):
		name = name[:len(name)-len(".tgz")]
	case strings.HasSuffix(lowerName, ".txz"):
		name = name[:len(name)-len(".txz")]
	default:
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	return sanitizeServiceName(name)
}

func writeUploadedFile(targetPath string, content io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	tmpPath := targetPath + ".uploading"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmpFile, content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func extractDeliveryPackageArchive(packagePath, targetDir string) error {
	lowerName := strings.ToLower(packagePath)
	switch {
	case strings.HasSuffix(lowerName, ".zip"):
		return extractZipArchive(packagePath, targetDir)
	case strings.HasSuffix(lowerName, ".tar.gz"), strings.HasSuffix(lowerName, ".tgz"):
		return extractTarGzArchive(packagePath, targetDir)
	case strings.HasSuffix(lowerName, ".tar.xz"), strings.HasSuffix(lowerName, ".txz"):
		return extractTarXZArchive(packagePath, targetDir)
	default:
		return fmt.Errorf("unsupported delivery package file %s", filepath.Base(packagePath))
	}
}

func extractZipArchive(packagePath, targetDir string) error {
	archiveReader, err := zip.OpenReader(packagePath)
	if err != nil {
		return err
	}
	defer archiveReader.Close()

	for _, file := range archiveReader.File {
		targetPath, err := safeExtractPath(targetDir, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		if err := writeExtractedFile(targetPath, source, file.FileInfo().Mode()); err != nil {
			_ = source.Close()
			return err
		}
		if err := source.Close(); err != nil {
			return err
		}
	}
	return nil
}

func extractTarXZArchive(packagePath, targetDir string) error {
	packageFile, err := os.Open(packagePath)
	if err != nil {
		return err
	}
	defer packageFile.Close()

	xzReader, err := xz.NewReader(packageFile)
	if err != nil {
		return err
	}
	return extractTarStream(xzReader, targetDir)
}

func extractTarGzArchive(packagePath, targetDir string) error {
	packageFile, err := os.Open(packagePath)
	if err != nil {
		return err
	}
	defer packageFile.Close()

	gzipReader, err := gzip.NewReader(packageFile)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	return extractTarStream(gzipReader, targetDir)
}

func extractTarStream(source io.Reader, targetDir string) error {
	tarReader := tar.NewReader(source)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		targetPath, err := safeExtractPath(targetDir, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			if err := writeExtractedFile(targetPath, tarReader, os.FileMode(header.Mode)); err != nil {
				return err
			}
		}
	}
}

func safeExtractPath(targetDir, archiveName string) (string, error) {
	cleanName := filepath.Clean(strings.ReplaceAll(archiveName, "\\", "/"))
	if cleanName == "." || strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
		return "", fmt.Errorf("invalid archive entry path %s", archiveName)
	}
	targetPath := filepath.Join(targetDir, cleanName)
	relPath, err := filepath.Rel(targetDir, targetPath)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry path escapes target directory: %s", archiveName)
	}
	return targetPath, nil
}

func writeExtractedFile(targetPath string, source io.Reader, mode os.FileMode) error {
	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(targetFile, source); err != nil {
		_ = targetFile.Close()
		return err
	}
	return targetFile.Close()
}

func removePathWithRetry(target string) error {
	return removePathWithRetryFns(target, os.RemoveAll, time.Sleep)
}

func removePathWithRetryFns(target string, removeAll func(string) error, sleep func(time.Duration)) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}

	var lastErr error
	for attempt := 0; attempt < windowsServiceRemoveRetryAttempts; attempt++ {
		err := removeAll(target)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if _, statErr := os.Stat(target); os.IsNotExist(statErr) {
			return nil
		}
		lastErr = err
		if !isRetryableRemovePathError(err) || attempt == windowsServiceRemoveRetryAttempts-1 {
			return err
		}
		sleep(time.Duration(attempt+1) * windowsServiceRemoveRetryDelay)
	}
	return lastErr
}

func isRetryableRemovePathError(err error) bool {
	if err == nil {
		return false
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}

	if errno, ok := err.(syscall.Errno); ok {
		switch errno {
		case syscall.Errno(5), syscall.Errno(16), syscall.Errno(32):
			return true
		}
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	retryableMarkers := []string{
		"used by another process",
		"being used by another process",
		"access is denied",
		"resource busy",
		"device or resource busy",
		"text file busy",
	}
	for _, marker := range retryableMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func escapeXML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func escapePowerShellSingleQuoted(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func ensureManagedServicePlatform() error {
	switch platform.Current() {
	case platform.OSWindows, platform.OSLinux:
		return nil
	default:
		return errors.New("service management is only supported on windows and linux")
	}
}

// defaultManagedServiceWinSWPath returns the bundled WinSW path on Windows and an
// empty string on Linux, where systemd (not WinSW) drives registration. Callers use
// it wherever a WinSW default was previously injected so Linux artifacts/responses
// leave the field blank.
func defaultManagedServiceWinSWPath() string {
	if platform.Current() == platform.OSWindows {
		return loadDefaultWinSWPath()
	}
	return ""
}

// maxWindowsServiceConfigFileSize caps how much data a single config/env read may
// pull into memory, preventing a maliciously large ConfigPath from exhausting memory.
const maxWindowsServiceConfigFileSize = 5 << 20 // 5 MiB

// validateWindowsServiceName rejects service names that could escape the managed
// service root via path traversal (`..`, path separators) or smuggle shell /
// PowerShell metacharacters into generated scripts. The name is used directly in
// filepath.Join(root, name) and in Delete's os.RemoveAll, so it must be strict.
func validateWindowsServiceName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("service name is required")
	}
	if len(name) > 128 {
		return errors.New("service name is too long")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("service name %q contains an illegal path sequence", name)
	}
	// On Linux the managed unit is installed at <name>.service and queried with the
	// same suffix appended (managedControllerUnitName); a name that already ends in
	// ".service" would produce a mismatched "foo.service.service" unit, so reject it
	// outright instead of double-suffixing.
	if platform.Current() == platform.OSLinux && strings.HasSuffix(strings.ToLower(name), ".service") {
		return fmt.Errorf("service name %q must not end with .service on linux", name)
	}
	for i, r := range name {
		valid := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
		if !valid {
			return fmt.Errorf("service name %q contains illegal characters; only letters, digits, '.', '_' and '-' are allowed", name)
		}
		if i == 0 && (r == '.' || r == '-') {
			return fmt.Errorf("service name %q must start with a letter or digit", name)
		}
	}
	return nil
}

// pathWithin reports whether target resides within root (inclusive), guarding
// os.RemoveAll / read / write operations against traversal outside a trusted root.
func pathWithin(root, target string) bool {
	root = strings.TrimSpace(root)
	target = strings.TrimSpace(target)
	if root == "" || target == "" {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// readFileWithLimit reads a file only if it does not exceed limit bytes, so a
// caller-controlled path cannot force an unbounded allocation.
func readFileWithLimit(path string, limit int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file %s is too large (%d bytes, limit %d)", path, info.Size(), limit)
	}
	return os.ReadFile(path)
}
