package service

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/1Panel-dev/1Panel/agent/app/dto/request"
	"github.com/1Panel-dev/1Panel/agent/app/dto/response"
	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/app/repo"
	"github.com/1Panel-dev/1Panel/agent/constant"
	"github.com/1Panel-dev/1Panel/agent/global"
)

type testWindowsServiceRepo struct {
	item  model.WindowsService
	items []model.WindowsService
	err   error
}

func assertPathUnder(t *testing.T, path string, parent string) {
	t.Helper()
	rel, err := filepath.Rel(parent, path)
	if err != nil {
		t.Fatalf("failed to resolve %s under %s: %v", path, parent, err)
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		t.Fatalf("expected %s to be under %s", path, parent)
	}
}

func (t testWindowsServiceRepo) List(opts ...repo.DBOption) ([]model.WindowsService, error) {
	return t.items, t.err
}

// managedServiceRootIsWindows reports whether the current platform uses the legacy
// Windows service layout. The managed-service backend keys directory/log/artifact
// layout off platform.Current()==runtime.GOOS, so tests derive expectations the same
// way and stay correct under both `go test` (linux here) and GOOS=windows compiles.
func managedServiceRootIsWindows() bool {
	return runtime.GOOS == "windows"
}

// testManagedServiceRoot mirrors loadWindowsServiceRoot: the legacy <dir>/service on
// Windows, <dir>/1panel/services inside the 1Panel data tree on Linux.
func testManagedServiceRoot(installDir string) string {
	if managedServiceRootIsWindows() {
		return filepath.Join(installDir, "service")
	}
	return filepath.Join(installDir, "1panel", "services")
}

func testManagedServiceDir(installDir, name string) string {
	return filepath.Join(testManagedServiceRoot(installDir), name)
}

// testManagedLogDir mirrors buildWindowsServiceLogDir's platform-aware segment.
func testManagedLogDir(installDir string) string {
	segment := "windows-services"
	if !managedServiceRootIsWindows() {
		segment = "services"
	}
	return filepath.Join(installDir, "1panel", "log", segment)
}

// fakeBundledJava writes a fake bundled JRE so resolveBundledJavaExecutable resolves
// deterministically without a host java: bin/java.exe on Windows, bin/java on Linux
// (glob picks the zulu* candidate before falling back to PATH).
func fakeBundledJava(t *testing.T, installDir string) string {
	t.Helper()
	javaBinName := "java.exe"
	if !managedServiceRootIsWindows() {
		javaBinName = "java"
	}
	javaPath := filepath.Join(installDir, "zulu21-test", "bin", javaBinName)
	if err := os.MkdirAll(filepath.Dir(javaPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(javaPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	return javaPath
}

// fakeBundledWinSW writes a fake WinSW binary. It is only consumed on Windows; on
// Linux the managed backend leaves WinSWPath empty, so use expectedManagedWinSWPath
// to derive the field value a normalized item should carry.
func fakeBundledWinSW(t *testing.T, installDir string) string {
	t.Helper()
	winSWPath := filepath.Join(installDir, "tools", "WinSW.exe")
	if err := os.MkdirAll(filepath.Dir(winSWPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(winSWPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	return winSWPath
}

// expectedManagedWinSWPath is the WinSWPath value a normalized item/response carries:
// the bundled path on Windows, empty on Linux (systemd drives registration there).
func expectedManagedWinSWPath(winSWPath string) string {
	if managedServiceRootIsWindows() {
		return winSWPath
	}
	return ""
}

func (t testWindowsServiceRepo) Get(opts ...repo.DBOption) (model.WindowsService, error) {
	return t.item, t.err
}

func (t testWindowsServiceRepo) Create(ctx context.Context, service *model.WindowsService) error {
	return t.err
}

func (t testWindowsServiceRepo) Save(ctx context.Context, service *model.WindowsService) error {
	return t.err
}

func (t testWindowsServiceRepo) Delete(ctx context.Context, opts ...repo.DBOption) error {
	return t.err
}

func TestParseManagedWindowsServiceConfigTemplateFields(t *testing.T) {
	templateContent := `
server:
  port: ${APP_SERVER_PORT:6084}
spring:
  application:
    name: ${APP_NAME:anzhener-biz}
  datasource:
    password: ${DB_PASSWORD:secret}
`

	fields := parseManagedWindowsServiceConfigTemplateFields(templateContent)
	if len(fields) != 3 {
		t.Fatalf("expected 3 dynamic fields, got %d", len(fields))
	}

	if fields[0].EnvKey != "APP_SERVER_PORT" || fields[0].DefaultValue != "6084" || fields[0].ConfigPath != "server.port" || fields[0].InputType != "number" {
		t.Fatalf("unexpected first field: %#v", fields[0])
	}
	if fields[1].EnvKey != "APP_NAME" || fields[1].DefaultValue != "anzhener-biz" || fields[1].ConfigPath != "spring.application.name" || fields[1].InputType != "text" {
		t.Fatalf("unexpected second field: %#v", fields[1])
	}
	if fields[2].EnvKey != "DB_PASSWORD" || fields[2].DefaultValue != "secret" || fields[2].ConfigPath != "spring.datasource.password" || !fields[2].Sensitive || fields[2].InputType != "password" {
		t.Fatalf("unexpected third field: %#v", fields[2])
	}
}

func TestNormalizeManagedFieldsUsesBundledJavaAndWinSW(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	javaPath := fakeBundledJava(t, installDir)
	winSWPath := fakeBundledWinSW(t, installDir)

	item := model.WindowsService{
		Name:        "demo-java",
		DisplayName: "Demo Java",
		ServiceType: "java",
		JarPath:     filepath.Join(testManagedServiceDir(installDir, "demo-java"), "app", "demo.jar"),
	}

	svc := &WindowsServiceService{}
	if err := svc.normalizeManagedFields(&item); err != nil {
		t.Fatalf("normalizeManagedFields returned error: %v", err)
	}

	if item.ExecPath != javaPath {
		t.Fatalf("expected execPath %s, got %s", javaPath, item.ExecPath)
	}
	expectedWorkDir := filepath.Join(testManagedServiceDir(installDir, "demo-java"), "app")
	if item.WorkDir != expectedWorkDir {
		t.Fatalf("expected workDir %s, got %s", expectedWorkDir, item.WorkDir)
	}
	if want := expectedManagedWinSWPath(winSWPath); item.WinSWPath != want {
		t.Fatalf("expected winswPath %s, got %s", want, item.WinSWPath)
	}
}

func TestUploadJarStoresArtifactAndRemovesOldJar(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	fakeBundledJava(t, installDir)
	fakeBundledWinSW(t, installDir)

	appDir := filepath.Join(testManagedServiceDir(installDir, "demo-java"), "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldJar := filepath.Join(appDir, "old.jar")
	if err := os.WriteFile(oldJar, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := &WindowsServiceService{}
	res, err := svc.UploadJar("demo-java", "java", "demo.jar", bytes.NewBufferString("new-jar"))
	if err != nil {
		t.Fatalf("UploadJar returned error: %v", err)
	}

	expectedJar := filepath.Join(appDir, "demo.jar")
	if res.JarPath != expectedJar {
		t.Fatalf("expected jarPath %s, got %s", expectedJar, res.JarPath)
	}
	data, err := os.ReadFile(expectedJar)
	if err != nil {
		t.Fatalf("failed to read uploaded jar: %v", err)
	}
	if string(data) != "new-jar" {
		t.Fatalf("expected uploaded jar content %q, got %q", "new-jar", string(data))
	}
	if _, err := os.Stat(oldJar); !os.IsNotExist(err) {
		t.Fatalf("expected old jar to be removed, stat err: %v", err)
	}
}

func TestUploadPackageExtractsDeliveryPackageZipToTemplateDir(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	fakeBundledJava(t, installDir)
	winSWPath := fakeBundledWinSW(t, installDir)

	var packageContent bytes.Buffer
	zipWriter := zip.NewWriter(&packageContent)
	composeFile, err := zipWriter.Create("delivery/docker-compose-files/docker-compose-medical-api.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := composeFile.Write([]byte("services:\n  medical-api:\n    image: example/medical-api:latest\n")); err != nil {
		t.Fatal(err)
	}
	jarFile, err := zipWriter.Create("delivery/medical/data/api/medical-biz.jar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jarFile.Write([]byte("jar")); err != nil {
		t.Fatal(err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}

	svc := &WindowsServiceService{}
	res, err := svc.UploadPackage("delivery-medical", "package", "delivery-medical.zip", bytes.NewReader(packageContent.Bytes()))
	if err != nil {
		t.Fatalf("UploadPackage returned error: %v", err)
	}

	templateRoot := filepath.Join(testManagedServiceDir(installDir, "delivery-medical"), "template")
	assertPathUnder(t, res.WorkDir, templateRoot)
	if res.JarPath != "" {
		t.Fatalf("expected empty jarPath for delivery package, got %s", res.JarPath)
	}
	if want := expectedManagedWinSWPath(winSWPath); res.WinSWPath != want {
		t.Fatalf("expected winswPath %s, got %s", want, res.WinSWPath)
	}
	if _, err := os.Stat(filepath.Join(res.WorkDir, "delivery", "docker-compose-files", "docker-compose-medical-api.yml")); err != nil {
		t.Fatalf("expected compose file to be extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(res.WorkDir, "delivery", "medical", "data", "api", "medical-biz.jar")); err != nil {
		t.Fatalf("expected jar file to be extracted: %v", err)
	}
}

func TestUploadPackageExtractsDeliveryPackageTarGzToTemplateDir(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	fakeBundledJava(t, installDir)
	winSWPath := fakeBundledWinSW(t, installDir)

	var packageContent bytes.Buffer
	gzipWriter := gzip.NewWriter(&packageContent)
	tarWriter := tar.NewWriter(gzipWriter)
	composeContent := []byte("services:\n  medical-api:\n    image: example/medical-api:latest\n")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "delivery/docker-compose-files/docker-compose-medical-api.yml",
		Mode: 0o644,
		Size: int64(len(composeContent)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(composeContent); err != nil {
		t.Fatal(err)
	}
	jarContent := []byte("jar")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "delivery/medical/data/api/medical-biz.jar",
		Mode: 0o644,
		Size: int64(len(jarContent)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(jarContent); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}

	svc := &WindowsServiceService{}
	res, err := svc.UploadPackage("delivery-medical", "package", "delivery-medical.tar.gz", bytes.NewReader(packageContent.Bytes()))
	if err != nil {
		t.Fatalf("UploadPackage returned error: %v", err)
	}

	templateRoot := filepath.Join(testManagedServiceDir(installDir, "delivery-medical"), "template")
	assertPathUnder(t, res.WorkDir, templateRoot)
	if res.JarPath != "" {
		t.Fatalf("expected empty jarPath for delivery package, got %s", res.JarPath)
	}
	if want := expectedManagedWinSWPath(winSWPath); res.WinSWPath != want {
		t.Fatalf("expected winswPath %s, got %s", want, res.WinSWPath)
	}
	if _, err := os.Stat(filepath.Join(res.WorkDir, "delivery", "docker-compose-files", "docker-compose-medical-api.yml")); err != nil {
		t.Fatalf("expected compose file to be extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(res.WorkDir, "delivery", "medical", "data", "api", "medical-biz.jar")); err != nil {
		t.Fatalf("expected jar file to be extracted: %v", err)
	}
}

func TestUploadPackageKeepsDeliveryPackageTemplateUploadsIsolated(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	fakeBundledJava(t, installDir)
	fakeBundledWinSW(t, installDir)

	buildPackage := func(files map[string]string) []byte {
		var packageContent bytes.Buffer
		gzipWriter := gzip.NewWriter(&packageContent)
		tarWriter := tar.NewWriter(gzipWriter)
		for name, content := range files {
			fileContent := []byte(content)
			if err := tarWriter.WriteHeader(&tar.Header{
				Name: name,
				Mode: 0o644,
				Size: int64(len(fileContent)),
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := tarWriter.Write(fileContent); err != nil {
				t.Fatal(err)
			}
		}
		if err := tarWriter.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gzipWriter.Close(); err != nil {
			t.Fatal(err)
		}
		return packageContent.Bytes()
	}

	firstPackage := buildPackage(map[string]string{
		"delivery/docker-compose-files/docker-compose-old.yml": "services:\n  old-api:\n    image: example/old-api:latest\n",
		"delivery/old/data/old-service.jar":                    "old-jar",
	})
	secondPackage := buildPackage(map[string]string{
		"delivery/docker-compose-files/docker-compose-new.yml": "services:\n  new-api:\n    image: example/new-api:latest\n",
		"delivery/new/data/new-service.jar":                    "new-jar",
	})

	svc := &WindowsServiceService{}
	first, err := svc.UploadPackage("delivery-medical", "package", "delivery-medical.tar.gz", bytes.NewReader(firstPackage))
	if err != nil {
		t.Fatalf("first UploadPackage returned error: %v", err)
	}
	res, err := svc.UploadPackage("delivery-medical", "package", "delivery-medical.tar.gz", bytes.NewReader(secondPackage))
	if err != nil {
		t.Fatalf("second UploadPackage returned error: %v", err)
	}

	templateRoot := filepath.Join(testManagedServiceDir(installDir, "delivery-medical"), "template")
	assertPathUnder(t, first.WorkDir, templateRoot)
	assertPathUnder(t, res.WorkDir, templateRoot)
	if first.WorkDir == res.WorkDir {
		t.Fatalf("expected each upload to use an isolated template directory, got %s", res.WorkDir)
	}
	if _, err := os.Stat(filepath.Join(res.WorkDir, "delivery", "docker-compose-files", "docker-compose-new.yml")); err != nil {
		t.Fatalf("expected new compose file to be extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(res.WorkDir, "delivery", "new", "data", "new-service.jar")); err != nil {
		t.Fatalf("expected new jar file to be extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(first.WorkDir, "delivery", "docker-compose-files", "docker-compose-old.yml")); err != nil {
		t.Fatalf("expected old compose file to remain in first isolated upload: %v", err)
	}
	if _, err := os.Stat(filepath.Join(first.WorkDir, "delivery", "old", "data", "old-service.jar")); err != nil {
		t.Fatalf("expected old jar file to remain in first isolated upload: %v", err)
	}
}

func TestRemovePathWithRetryFnsRetriesTransientLockError(t *testing.T) {
	targetDir := t.TempDir()
	attempts := 0
	sleepCalls := 0
	err := removePathWithRetryFns(
		targetDir,
		func(_ string) error {
			attempts++
			if attempts < 3 {
				return &os.PathError{
					Op:   "unlinkat",
					Path: `D:\1Panel\service\DeliveryPackage-Service\template\delivery_1.0.0.0\images\x86_64\mysql_8_4_2.tar.gz`,
					Err:  errors.New("The process cannot access the file because it is being used by another process."),
				}
			}
			return nil
		},
		func(_ time.Duration) {
			sleepCalls++
		},
	)
	if err != nil {
		t.Fatalf("expected removePathWithRetryFns to recover from transient lock, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 remove attempts, got %d", attempts)
	}
	if sleepCalls != 2 {
		t.Fatalf("expected 2 retry sleeps, got %d", sleepCalls)
	}
}

func TestPrepareArtifactsGeneratesManagedConfigAndEnv(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	item := model.WindowsService{
		Name:        "demo-java",
		DisplayName: "Demo Java",
		ServiceType: "java",
		ExecPath:    filepath.Join(installDir, "zulu21-test", "bin", "java.exe"),
		WorkDir:     filepath.Join(installDir, "service", "demo-java", "app"),
		JarPath:     filepath.Join(installDir, "service", "demo-java", "app", "demo.jar"),
		Args:        "-Xms512m -Xmx512m",
	}

	svc := &WindowsServiceService{}
	configReq := &request.WindowsServiceConfigTemplate{
		Enabled:  true,
		FileName: windowsServiceConfigFileName,
		DynamicFields: []request.WindowsServiceConfigTemplateField{
			{EnvKey: envAppServerPort, Value: "6099"},
			{EnvKey: envAppName, Value: "delivery-medical"},
			{EnvKey: envRedisHost, Value: "192.168.10.20"},
			{EnvKey: envPlatformBaseURL, Value: "https://example.org:28171"},
			{EnvKey: envXxlJobAdminAddresses, Value: "http://192.168.10.30:9080/xxl-job-admin"},
			{EnvKey: envHisDBHost, Value: "192.168.10.40"},
			{EnvKey: envHisDBPort, Value: "1522"},
			{EnvKey: envHisDBServerName, Value: "orcl1"},
			{EnvKey: envHisDBSchema, Value: "his_schema"},
			{EnvKey: envHisDBUser, Value: "his_user"},
			{EnvKey: envHisDBPassword, Value: "his_password"},
			{EnvKey: envOrgCode, Value: "ORG001"},
			{EnvKey: envHospitalCode, Value: "H001"},
			{EnvKey: envPlatformCode, Value: "PLATFORM001"},
		},
	}
	if err := svc.prepareArtifacts(&item, "", configReq); err != nil {
		t.Fatalf("prepareArtifacts returned error: %v", err)
	}

	expectedConfigPath := filepath.Join(item.WorkDir, "config", windowsServiceConfigFileName)
	if item.ConfigPath != expectedConfigPath {
		t.Fatalf("expected configPath %s, got %s", expectedConfigPath, item.ConfigPath)
	}
	if !strings.Contains(item.Args, "--spring.config.location=\""+expectedConfigPath+"\"") {
		t.Fatalf("expected args to contain generated config location, got %s", item.Args)
	}

	configData, err := os.ReadFile(expectedConfigPath)
	if err != nil {
		t.Fatalf("failed to read generated config: %v", err)
	}
	if !strings.Contains(string(configData), "${APP_SERVER_PORT:6084}") {
		t.Fatalf("expected generated config to contain managed template placeholders")
	}

	envData, err := os.ReadFile(item.EnvFilePath)
	if err != nil {
		t.Fatalf("failed to read generated env file: %v", err)
	}
	envContent := string(envData)
	expectedEnvLines := []string{
		"CONFIG_PATH=" + expectedConfigPath,
		"JAR_PATH=" + item.JarPath,
		"APP_SERVER_PORT=6099",
		"APP_NAME=delivery-medical",
		"REDIS_HOST=192.168.10.20",
		"ANZHENER_PLATFORM_BASE_URL=https://example.org:28171",
		"XXL_JOB_ADMIN_ADDRESSES=http://192.168.10.30:9080/xxl-job-admin",
		"HIS_DB_HOST=192.168.10.40",
		"HIS_DB_PORT=1522",
		"HIS_DB_SERVER_NAME=orcl1",
		"HIS_DB_SCHEMA=his_schema",
		"HIS_DB_USER=his_user",
		"HIS_DB_PASSWORD=his_password",
		"ANZHENER_ORG_CODE=ORG001",
		"ANZHENER_HOSPITAL_CODE=H001",
		"ANZHENER_PLATFORM_CODE=PLATFORM001",
	}
	for _, line := range expectedEnvLines {
		if !strings.Contains(envContent, line) {
			t.Fatalf("expected env file to contain %q, got %s", line, envContent)
		}
	}
	if !strings.Contains(envContent, "HIS_DB_SCHEMA=his_schema") {
		t.Fatalf("expected env file to contain overridden schema, got %s", envContent)
	}
}

func TestPrepareArtifactsGeneratesServiceUninstallScripts(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	appDir := filepath.Join(testManagedServiceDir(installDir, "demo-java"), "app")
	item := model.WindowsService{
		Name:        "demo-java",
		DisplayName: "Demo Java",
		ServiceType: "java",
		ExecPath:    fakeBundledJava(t, installDir),
		WorkDir:     appDir,
		JarPath:     filepath.Join(appDir, "demo.jar"),
	}

	svc := &WindowsServiceService{}
	if err := svc.prepareArtifacts(&item, "", nil); err != nil {
		t.Fatalf("prepareArtifacts returned error: %v", err)
	}

	if runtime.GOOS != "windows" {
		// Linux teardown is a self-contained systemd uninstall script.
		uninstallSh := filepath.Join(buildWindowsServiceDir(item.Name), item.Name+"-uninstall.sh")
		data, err := os.ReadFile(uninstallSh)
		if err != nil {
			t.Fatalf("failed to read uninstall shell script: %v", err)
		}
		script := string(data)
		for _, want := range []string{
			"systemctl stop",
			"systemctl disable",
			linuxSystemdUnitPath(item.Name),
			"systemctl daemon-reload",
			"rm -rf",
			buildWindowsServiceDir(item.Name),
		} {
			if !strings.Contains(script, want) {
				t.Fatalf("expected uninstall shell script to contain %q, got:\n%s", want, script)
			}
		}
		return
	}

	uninstallCmdPath := buildWindowsServiceUninstallCmdPath(item.Name)
	uninstallPS1Path := buildWindowsServiceUninstallPS1Path(item.Name)
	uninstallCmdData, err := os.ReadFile(uninstallCmdPath)
	if err != nil {
		t.Fatalf("failed to read uninstall cmd script: %v", err)
	}
	if !strings.Contains(string(uninstallCmdData), item.Name+"-uninstall.ps1") {
		t.Fatalf("expected uninstall cmd to call uninstall ps1, got %s", string(uninstallCmdData))
	}

	uninstallPS1Data, err := os.ReadFile(uninstallPS1Path)
	if err != nil {
		t.Fatalf("failed to read uninstall powershell script: %v", err)
	}
	uninstallPS1 := string(uninstallPS1Data)
	if !strings.Contains(uninstallPS1, item.Name+".exe") {
		t.Fatalf("expected uninstall powershell script to reference wrapper exe, got %s", uninstallPS1)
	}
	if !strings.Contains(uninstallPS1, "uninstall | Out-Null") {
		t.Fatalf("expected uninstall powershell script to uninstall winsw service, got %s", uninstallPS1)
	}
	if !strings.Contains(uninstallPS1, "rmdir /s /q") {
		t.Fatalf("expected uninstall powershell script to clean service directory, got %s", uninstallPS1)
	}
}

func TestToWindowsServiceInfoIncludesUninstallScriptPath(t *testing.T) {
	item := model.WindowsService{
		Name:        "demo-java",
		DisplayName: "Demo Java",
		ServiceType: "java",
	}

	info := toWindowsServiceInfo(item)
	expectedName := item.Name + "-uninstall.cmd"
	if runtime.GOOS != "windows" {
		expectedName = item.Name + "-uninstall.sh"
	}
	expectedPath := filepath.Join(buildWindowsServiceDir(item.Name), expectedName)
	if info.UninstallScriptPath != expectedPath {
		t.Fatalf("expected uninstallScriptPath %s, got %s", expectedPath, info.UninstallScriptPath)
	}
}

func TestLoadManagedWindowsServiceConfigUsesDynamicFields(t *testing.T) {
	installDir := t.TempDir()
	configPath := filepath.Join(installDir, "application-dev.yml")
	envPath := filepath.Join(installDir, "demo.env")
	if err := os.WriteFile(envPath, []byte(strings.Join([]string{
		envAppServerPort + "=6101",
		envAppName + "=loaded-service",
		envHisDBPassword + "=secret-pass",
	}, "\r\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := loadManagedWindowsServiceConfig(model.WindowsService{
		ServiceType: "java",
		ConfigPath:  configPath,
		EnvFilePath: envPath,
	})
	if cfg == nil {
		t.Fatal("expected config template to be loaded")
	}
	if len(cfg.DynamicFields) == 0 {
		t.Fatal("expected dynamic fields to be returned")
	}

	fieldMap := make(map[string]response.WindowsServiceConfigTemplateField)
	for _, field := range cfg.DynamicFields {
		fieldMap[field.EnvKey] = field
	}

	if fieldMap[envAppServerPort].Value != "6101" {
		t.Fatalf("expected APP_SERVER_PORT to use env override, got %#v", fieldMap[envAppServerPort])
	}
	if fieldMap[envAppName].Value != "loaded-service" {
		t.Fatalf("expected APP_NAME to use env override, got %#v", fieldMap[envAppName])
	}
	passwordField := fieldMap[envHisDBPassword]
	if passwordField.Value != "secret-pass" || !passwordField.Sensitive || passwordField.InputType != "password" {
		t.Fatalf("expected password field to be sensitive with env override, got %#v", passwordField)
	}
}

func TestLoadManagedWindowsServiceConfigRefreshesDynamicFieldsFromConfigFile(t *testing.T) {
	installDir := t.TempDir()
	configPath := filepath.Join(installDir, "application-dev.yml")
	envPath := filepath.Join(installDir, "demo.env")

	firstConfig := strings.TrimSpace(`
server:
  port: ${APP_SERVER_PORT:7001}
integration:
  endpoint: ${FIRST_ENDPOINT:https://first.example}
`)
	if err := os.WriteFile(configPath, []byte(firstConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("FIRST_ENDPOINT=https://env-first.example"), 0o644); err != nil {
		t.Fatal(err)
	}

	first := loadManagedWindowsServiceConfig(model.WindowsService{
		ServiceType: "java",
		ConfigPath:  configPath,
		EnvFilePath: envPath,
	})
	if first == nil {
		t.Fatal("expected first config template to be loaded")
	}
	firstFields := make(map[string]response.WindowsServiceConfigTemplateField, len(first.DynamicFields))
	for _, field := range first.DynamicFields {
		firstFields[field.EnvKey] = field
	}
	if _, ok := firstFields["FIRST_ENDPOINT"]; !ok {
		t.Fatalf("expected FIRST_ENDPOINT in first field set, got %#v", first.DynamicFields)
	}
	if firstFields[envAppServerPort].DefaultValue != "7001" {
		t.Fatalf("expected APP_SERVER_PORT default 7001, got %#v", firstFields[envAppServerPort])
	}

	secondConfig := strings.TrimSpace(`
server:
  port: ${APP_SERVER_PORT:8123}
security:
  token: ${SECOND_TOKEN:second-default-token}
retry:
  count: ${RETRY_COUNT:5}
`)
	if err := os.WriteFile(configPath, []byte(secondConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte(strings.Join([]string{
		"SECOND_TOKEN=env-second-token",
		"RETRY_COUNT=9",
	}, "\r\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	second := loadManagedWindowsServiceConfig(model.WindowsService{
		ServiceType: "java",
		ConfigPath:  configPath,
		EnvFilePath: envPath,
	})
	if second == nil {
		t.Fatal("expected second config template to be loaded")
	}
	secondFields := make(map[string]response.WindowsServiceConfigTemplateField, len(second.DynamicFields))
	for _, field := range second.DynamicFields {
		secondFields[field.EnvKey] = field
	}
	if _, ok := secondFields["FIRST_ENDPOINT"]; ok {
		t.Fatalf("expected FIRST_ENDPOINT to disappear after config file changed, got %#v", second.DynamicFields)
	}
	if secondFields[envAppServerPort].DefaultValue != "8123" || secondFields[envAppServerPort].Value != "8123" {
		t.Fatalf("expected APP_SERVER_PORT to refresh from second config file, got %#v", secondFields[envAppServerPort])
	}
	if secondFields["SECOND_TOKEN"].Value != "env-second-token" || !secondFields["SECOND_TOKEN"].Sensitive {
		t.Fatalf("expected SECOND_TOKEN to load env override as sensitive field, got %#v", secondFields["SECOND_TOKEN"])
	}
	if secondFields["RETRY_COUNT"].DefaultValue != "5" || secondFields["RETRY_COUNT"].Value != "9" {
		t.Fatalf("expected RETRY_COUNT defaults and env override from second config, got %#v", secondFields["RETRY_COUNT"])
	}
}

func TestUploadPackageReturnsConfigTemplateFromExtractedConfigFile(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	javaPath := filepath.Join(installDir, "zulu21-test", "bin", "java.exe")
	if err := os.MkdirAll(filepath.Dir(javaPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(javaPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	winSWPath := filepath.Join(installDir, "tools", "WinSW.exe")
	if err := os.MkdirAll(filepath.Dir(winSWPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(winSWPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	var packageContent bytes.Buffer
	zipWriter := zip.NewWriter(&packageContent)
	configFile, err := zipWriter.Create("delivery/pda/config/application-dev.yml")
	if err != nil {
		t.Fatal(err)
	}
	configContent := strings.TrimSpace(`
server:
  port: ${APP_SERVER_PORT:6900}
spring:
  redis:
    host: ${REDIS_HOST:base-redis}
security:
  token: ${SECOND_TOKEN:abc123}
`)
	if _, err := configFile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}

	svc := &WindowsServiceService{}
	res, err := svc.UploadPackage("delivery-medical", "package", "delivery-medical.zip", bytes.NewReader(packageContent.Bytes()))
	if err != nil {
		t.Fatalf("UploadPackage returned error: %v", err)
	}
	if res.ConfigTemplate == nil {
		t.Fatal("expected upload response to include configTemplate")
	}
	fieldMap := make(map[string]response.WindowsServiceConfigTemplateField, len(res.ConfigTemplate.DynamicFields))
	for _, field := range res.ConfigTemplate.DynamicFields {
		fieldMap[field.EnvKey] = field
	}
	if fieldMap[envAppServerPort].DefaultValue != "6900" {
		t.Fatalf("expected APP_SERVER_PORT default 6900, got %#v", fieldMap[envAppServerPort])
	}
	if fieldMap[envRedisHost].DefaultValue != "base-redis" {
		t.Fatalf("expected REDIS_HOST default base-redis, got %#v", fieldMap[envRedisHost])
	}
	if fieldMap["SECOND_TOKEN"].DefaultValue != "abc123" || !fieldMap["SECOND_TOKEN"].Sensitive {
		t.Fatalf("expected SECOND_TOKEN to be included as sensitive field, got %#v", fieldMap["SECOND_TOKEN"])
	}
}

func TestBuildPowerShellScriptPlacesManagedSpringConfigAfterJar(t *testing.T) {
	installDir := t.TempDir()
	item := model.WindowsService{
		Name:        "demo-java",
		DisplayName: "Demo Java",
		ServiceType: "java",
		ExecPath:    filepath.Join(installDir, "zulu21-test", "bin", "java.exe"),
		WorkDir:     filepath.Join(installDir, "service", "demo-java", "app"),
		JarPath:     filepath.Join(installDir, "service", "demo-java", "app", "demo.jar"),
		ConfigPath:  filepath.Join(installDir, "service", "demo-java", "app", "config", windowsServiceConfigFileName),
		Args:        "-Xms512m -Xmx512m --spring.config.location=\"" + filepath.Join(installDir, "service", "demo-java", "app", "config", windowsServiceConfigFileName) + "\"",
	}

	svc := &WindowsServiceService{}
	script := svc.buildPowerShellScript(&item)

	expected := "-Xms512m -Xmx512m -jar \"" + item.JarPath + "\" --spring.config.location=\"" + item.ConfigPath + "\""
	if !strings.Contains(script, expected) {
		t.Fatalf("expected command line order %q, got script:\n%s", expected, script)
	}
}

func TestBuildPowerShellScriptStaysPowerShell2Compatible(t *testing.T) {
	installDir := t.TempDir()
	item := model.WindowsService{
		Name:        "demo-ps2",
		DisplayName: "Demo PS2",
		ServiceType: "dll",
		ExecPath:    filepath.Join(installDir, "app", "demo.exe"),
		WorkDir:     filepath.Join(installDir, "app"),
		DLLDir:      filepath.Join(installDir, "app", "dll"),
		EnvFilePath: filepath.Join(installDir, "app", "demo.env"),
	}

	svc := &WindowsServiceService{}
	script := svc.buildPowerShellScript(&item)

	// Windows 7 SP1 ships PowerShell 2.0 on the .NET 2.0 CLR. These APIs only exist
	// in .NET 4.0+/newer runtimes and abort the service-start script before it runs.
	forbidden := []string{
		"IsNullOrWhiteSpace", // [string]::IsNullOrWhiteSpace — .NET 4.0+
		".Split('=', 2)",     // String.Split(char, int) — ambiguous/unavailable on .NET 2.0
	}
	for _, token := range forbidden {
		if strings.Contains(script, token) {
			t.Fatalf("generated start script uses PowerShell 2.0-incompatible construct %q:\n%s", token, script)
		}
	}
}

func TestBuildDeliveryPackageCreateItemsUsesComposeWhenDockerComposeAvailable(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	winSWPath := fakeBundledWinSW(t, installDir)

	templateDir := filepath.Join(installDir, "delivery-template")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	composePath := filepath.Join(templateDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(`
services:
  base-auth:
    image: example/auth:latest
  base-gateway:
    image: example/gateway:latest
`), 0o644); err != nil {
		t.Fatal(err)
	}

	req := request.WindowsServiceCreate{
		Name:        "delivery-base",
		DisplayName: "DeliveryPackage Base",
		ServiceType: "package",
		WorkDir:     templateDir,
	}
	registerService := true
	items, err := buildDeliveryPackageCreateItems(req, "docker compose", registerService)
	if err != nil {
		t.Fatalf("buildDeliveryPackageCreateItems returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 compose service items, got %d: %#v", len(items), items)
	}

	if items[0].Name != "base-auth" || items[0].ExecPath != "docker compose" || items[0].ConfigPath != composePath || items[0].JarPath != "" {
		t.Fatalf("unexpected first compose item: %#v", items[0])
	}
	if items[1].Name != "base-gateway" || items[1].ExecPath != "docker compose" || items[1].ConfigPath != composePath || items[1].JarPath != "" {
		t.Fatalf("unexpected second compose item: %#v", items[1])
	}
	if want := expectedManagedWinSWPath(winSWPath); items[0].WinSWPath != want || items[1].WinSWPath != want {
		t.Fatalf("expected default WinSW path %s, got %#v", want, items)
	}
}

func TestBuildDeliveryPackageCreateItemsFallsBackToAllJarsWhenDockerComposeUnavailable(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	javaPath := fakeBundledJava(t, installDir)
	fakeBundledWinSW(t, installDir)

	templateDir := filepath.Join(installDir, "delivery-template")
	authJar := filepath.Join(templateDir, "data", "auth", "delivery-auth.jar")
	gatewayJar := filepath.Join(templateDir, "data", "gateway", "delivery-gateway.jar")
	for _, jarPath := range []string{authJar, gatewayJar} {
		if err := os.MkdirAll(filepath.Dir(jarPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(jarPath, []byte("jar"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	req := request.WindowsServiceCreate{
		Name:        "delivery-base",
		DisplayName: "DeliveryPackage Base",
		ServiceType: "package",
		WorkDir:     templateDir,
	}
	registerService := true
	items, err := buildDeliveryPackageCreateItems(req, "", registerService)
	if err != nil {
		t.Fatalf("buildDeliveryPackageCreateItems returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 jar service items, got %d: %#v", len(items), items)
	}

	if items[0].Name != "delivery-auth" || items[0].ExecPath != javaPath || items[0].JarPath != authJar || items[0].WorkDir != filepath.Dir(authJar) {
		t.Fatalf("unexpected first jar item: %#v", items[0])
	}
	if items[1].Name != "delivery-gateway" || items[1].ExecPath != javaPath || items[1].JarPath != gatewayJar || items[1].WorkDir != filepath.Dir(gatewayJar) {
		t.Fatalf("unexpected second jar item: %#v", items[1])
	}
}

func TestIsDeliveryPackageTemplateCreateRequestKeepsUploadedJarModeSeparate(t *testing.T) {
	templateReq := request.WindowsServiceCreate{
		ServiceType: "package",
		WorkDir:     `D:\1Panel\delivery-template`,
	}
	if !isDeliveryPackageTemplateCreateRequest(templateReq) {
		t.Fatal("expected delivery request with only workDir to use template mode")
	}

	uploadedJarReq := request.WindowsServiceCreate{
		ServiceType: "package",
		WorkDir:     `D:\1Panel\service\delivery-medical\app`,
		JarPath:     `D:\1Panel\service\delivery-medical\app\delivery-medical.jar`,
	}
	if isDeliveryPackageTemplateCreateRequest(uploadedJarReq) {
		t.Fatal("expected delivery request with jarPath to stay in uploaded jar mode")
	}
}

func TestBuildPowerShellScriptUsesDockerComposeForDeliveryPackageComposeItem(t *testing.T) {
	installDir := t.TempDir()
	composePath := filepath.Join(installDir, "docker-compose.yml")
	item := model.WindowsService{
		Name:        "base-auth",
		DisplayName: "Base Auth",
		ServiceType: "package",
		ExecPath:    "docker compose",
		WorkDir:     filepath.Dir(composePath),
		ConfigPath:  composePath,
		Args:        "base-auth",
	}

	svc := &WindowsServiceService{}
	script := svc.buildPowerShellScript(&item)

	expected := "docker compose -f \"" + composePath + "\" up base-auth"
	if !strings.Contains(script, expected) {
		t.Fatalf("expected compose command %q, got script:\n%s", expected, script)
	}
}

func TestBuildDeliveryPackageCreateItemsKeepsOriginalComposeServiceNameInCommand(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	winSWPath := filepath.Join(installDir, "tools", "WinSW.exe")
	if err := os.MkdirAll(filepath.Dir(winSWPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(winSWPath, []byte("winsw"), 0o644); err != nil {
		t.Fatal(err)
	}

	templateDir := filepath.Join(installDir, "delivery-template")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "docker-compose.yml"), []byte(`
services:
  base_auth:
    image: example/auth:latest
`), 0o644); err != nil {
		t.Fatal(err)
	}

	registerService := true
	items, err := buildDeliveryPackageCreateItems(request.WindowsServiceCreate{
		Name:        "delivery-base",
		DisplayName: "DeliveryPackage Base",
		ServiceType: "package",
		WorkDir:     templateDir,
	}, "docker compose", registerService)
	if err != nil {
		t.Fatalf("buildDeliveryPackageCreateItems returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 compose service item, got %d", len(items))
	}
	if items[0].Name != "base-auth" {
		t.Fatalf("expected sanitized Windows service name base-auth, got %s", items[0].Name)
	}
	if items[0].Args != "base_auth" {
		t.Fatalf("expected original compose service name base_auth in args, got %s", items[0].Args)
	}
}

func TestBuildDeliveryPackageCreateItemsAcceptsDottedComposeFileNames(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	winSWPath := filepath.Join(installDir, "tools", "WinSW.exe")
	if err := os.MkdirAll(filepath.Dir(winSWPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(winSWPath, []byte("winsw"), 0o644); err != nil {
		t.Fatal(err)
	}

	templateDir := filepath.Join(installDir, "delivery-template")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "docker-compose.prod.yml"), []byte(`
services:
  base-auth:
    image: example/auth:latest
`), 0o644); err != nil {
		t.Fatal(err)
	}

	registerService := true
	items, err := buildDeliveryPackageCreateItems(request.WindowsServiceCreate{
		Name:        "delivery-base",
		DisplayName: "DeliveryPackage Base",
		ServiceType: "package",
		WorkDir:     templateDir,
	}, "docker compose", registerService)
	if err != nil {
		t.Fatalf("buildDeliveryPackageCreateItems returned error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "base-auth" {
		t.Fatalf("expected dotted compose file to produce base-auth item, got %#v", items)
	}
}

func TestUploadPackageDeliveryPackageArchiveWithoutNameOrServiceTypeReturnsServices(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	fakeBundledJava(t, installDir)
	fakeBundledWinSW(t, installDir)

	var packageContent bytes.Buffer
	zipWriter := zip.NewWriter(&packageContent)
	firstJar, err := zipWriter.Create("delivery/auth/delivery-auth.jar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := firstJar.Write([]byte("auth")); err != nil {
		t.Fatal(err)
	}
	secondJar, err := zipWriter.Create("delivery/gateway/delivery-gateway.jar")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secondJar.Write([]byte("gateway")); err != nil {
		t.Fatal(err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}

	svc := &WindowsServiceService{}
	res, err := svc.UploadPackage("", "", "DeliveryPackage Medical.zip", bytes.NewReader(packageContent.Bytes()))
	if err != nil {
		t.Fatalf("UploadPackage returned error: %v", err)
	}

	templateRoot := filepath.Join(testManagedServiceDir(installDir, "DeliveryPackage-Medical"), "template")
	assertPathUnder(t, res.WorkDir, templateRoot)
	if len(res.Services) != 2 {
		t.Fatalf("expected 2 service previews, got %d: %#v", len(res.Services), res.Services)
	}
	if res.Services[0].Name != "delivery-auth" || res.Services[0].RuntimeType != "java" || res.Services[0].JarPath == "" {
		t.Fatalf("unexpected first service preview: %#v", res.Services[0])
	}
	if res.Services[1].Name != "delivery-gateway" || res.Services[1].RuntimeType != "java" || res.Services[1].JarPath == "" {
		t.Fatalf("unexpected second service preview: %#v", res.Services[1])
	}
}

func TestBuildDeliveryPackageCreateItemsAllowsEmptyNameAndDisplayName(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	fakeBundledJava(t, installDir)

	templateDir := filepath.Join(installDir, "delivery-template")
	jarPath := filepath.Join(templateDir, "data", "pda-service.jar")
	if err := os.MkdirAll(filepath.Dir(jarPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jarPath, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := buildDeliveryPackageCreateItems(request.WindowsServiceCreate{
		ServiceType: "package",
		WorkDir:     templateDir,
	}, "", true)
	if err != nil {
		t.Fatalf("buildDeliveryPackageCreateItems returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 jar service item, got %d: %#v", len(items), items)
	}
	if items[0].Name != "pda-service" || items[0].DisplayName != "pda-service" {
		t.Fatalf("expected name/displayName from jar, got %#v", items[0])
	}
}

func TestBuildDeliveryPackageCreateItemsMakesDuplicateComposeServiceNamesUnique(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	templateDir := filepath.Join(installDir, "delivery-template")
	firstCompose := filepath.Join(templateDir, "one", "docker-compose-web.yml")
	secondCompose := filepath.Join(templateDir, "two", "docker-compose-web.yml")
	for _, composePath := range []string{firstCompose, secondCompose} {
		if err := os.MkdirAll(filepath.Dir(composePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(composePath, []byte("services:\n  web:\n    image: example/web:latest\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	items, err := buildDeliveryPackageCreateItems(request.WindowsServiceCreate{
		ServiceType: "package",
		WorkDir:     templateDir,
	}, "docker compose", true)
	if err != nil {
		t.Fatalf("buildDeliveryPackageCreateItems returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 compose service items, got %d: %#v", len(items), items)
	}
	if items[0].Name == items[1].Name {
		t.Fatalf("expected unique Windows service names, got %#v", items)
	}
	for _, item := range items {
		if item.Args != "web" {
			t.Fatalf("expected original compose service key in Args, got %#v", item)
		}
	}
}

func TestUploadPackageJavaStillRequiresName(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	svc := &WindowsServiceService{}
	_, err := svc.UploadPackage("", "java", "demo.jar", bytes.NewBufferString("jar"))
	if err == nil || !strings.Contains(err.Error(), "service name is required") {
		t.Fatalf("expected java upload without name to fail, got %v", err)
	}
}

func TestGetServiceConfigFileReadsBoundPaths(t *testing.T) {
	rootDir := t.TempDir()
	configPath := filepath.Join(rootDir, "application.yml")
	envPath := filepath.Join(rootDir, "service.env")
	if err := os.WriteFile(configPath, []byte("server:\n  port: 8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("APP_NAME=demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	item := model.WindowsService{
		Name:        "demo-service",
		ConfigPath:  configPath,
		EnvFilePath: envPath,
	}
	svc := &WindowsServiceService{}

	configFile, err := svc.getServiceConfigFile(item, "config")
	if err != nil {
		t.Fatalf("getServiceConfigFile config returned error: %v", err)
	}
	if configFile.Type != "config" || configFile.Path != configPath || configFile.Content != "server:\n  port: 8080\n" {
		t.Fatalf("unexpected config response: %#v", configFile)
	}

	envFile, err := svc.getServiceConfigFile(item, "env")
	if err != nil {
		t.Fatalf("getServiceConfigFile env returned error: %v", err)
	}
	if envFile.Type != "env" || envFile.Path != envPath || envFile.Content != "APP_NAME=demo\n" {
		t.Fatalf("unexpected env response: %#v", envFile)
	}
}

func TestGetServiceConfigFileRejectsInvalidType(t *testing.T) {
	svc := &WindowsServiceService{}
	_, err := svc.getServiceConfigFile(model.WindowsService{Name: "demo-service"}, "other")
	if err == nil || !strings.Contains(err.Error(), "invalid config file type") {
		t.Fatalf("expected invalid type error, got %v", err)
	}
}

func TestGetServiceConfigFileErrorsWhenPathEmpty(t *testing.T) {
	svc := &WindowsServiceService{}
	_, err := svc.getServiceConfigFile(model.WindowsService{Name: "demo-service"}, "config")
	if err == nil || !strings.Contains(err.Error(), "config file path is empty") {
		t.Fatalf("expected empty config path error, got %v", err)
	}
}

func TestUpdateServiceConfigFileWritesExistingBoundFile(t *testing.T) {
	rootDir := t.TempDir()
	configPath := filepath.Join(rootDir, "application.yml")
	if err := os.WriteFile(configPath, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	item := model.WindowsService{
		Name:       "demo-service",
		ConfigPath: configPath,
	}
	svc := &WindowsServiceService{}
	if err := svc.updateServiceConfigFile(item, "config", "new content"); err != nil {
		t.Fatalf("updateServiceConfigFile returned error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read updated config file: %v", err)
	}
	if string(data) != "new content" {
		t.Fatalf("expected updated file content %q, got %q", "new content", string(data))
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("failed to stat updated config file: %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("expected file mode 0640, got %v", info.Mode().Perm())
	}
}

func TestUpdateServiceConfigFileErrorsWhenFileDoesNotExist(t *testing.T) {
	rootDir := t.TempDir()
	item := model.WindowsService{
		Name:       "demo-service",
		ConfigPath: filepath.Join(rootDir, "missing.yml"),
	}
	svc := &WindowsServiceService{}
	err := svc.updateServiceConfigFile(item, "config", "new content")
	if err == nil || !strings.Contains(err.Error(), "config file does not exist") {
		t.Fatalf("expected missing file error, got %v", err)
	}
}

func TestGetServiceLogFileDerivesMainLogMetadata(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	logPath := filepath.Join(testManagedLogDir(installDir), "demo-service.out.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("hello log"), 0o644); err != nil {
		t.Fatal(err)
	}
	modifiedAt := time.Now().Add(-2 * time.Minute).Round(time.Second)
	if err := os.Chtimes(logPath, modifiedAt, modifiedAt); err != nil {
		t.Fatal(err)
	}

	svc := &WindowsServiceService{}
	logFile, err := svc.getServiceLogFile(model.WindowsService{Name: "demo-service"})
	if err != nil {
		t.Fatalf("getServiceLogFile returned error: %v", err)
	}
	if !logFile.Exists || logFile.Path != logPath || logFile.Name != "demo-service.out.log" || logFile.Size != int64(len("hello log")) {
		t.Fatalf("unexpected log metadata: %#v", logFile)
	}
	if logFile.ModifiedAt.IsZero() || !logFile.ModifiedAt.Equal(modifiedAt) {
		t.Fatalf("expected modified time %v, got %#v", modifiedAt, logFile.ModifiedAt)
	}
}

func TestGetServiceLogFileReturnsEmptyStateWhenMissing(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	svc := &WindowsServiceService{}
	logFile, err := svc.getServiceLogFile(model.WindowsService{Name: "demo-service"})
	if err != nil {
		t.Fatalf("getServiceLogFile returned error: %v", err)
	}
	expectedPath := filepath.Join(testManagedLogDir(installDir), "demo-service.out.log")
	if logFile.Path != expectedPath || logFile.Name != "demo-service.out.log" {
		t.Fatalf("unexpected missing log metadata: %#v", logFile)
	}
	if logFile.Exists {
		t.Fatalf("expected missing log file state, got %#v", logFile)
	}
	if logFile.Size != 0 {
		t.Fatalf("expected missing log size 0, got %#v", logFile)
	}
	if !logFile.ModifiedAt.IsZero() {
		t.Fatalf("expected zero modified time for missing log, got %#v", logFile.ModifiedAt)
	}
}

func TestReadLogByLineSupportsWindowsServiceType(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() {
		global.CONF.Base.InstallDir = restoreInstallDir
	}()

	logPath := filepath.Join(testManagedLogDir(installDir), "demo-service.out.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	restoreRepo := windowsServiceRepo
	windowsServiceRepo = testWindowsServiceRepo{
		item: model.WindowsService{
			BaseModel: model.BaseModel{ID: 1},
			Name:      "demo-service",
		},
	}
	defer func() {
		windowsServiceRepo = restoreRepo
	}()

	res, err := (&FileService{}).ReadLogByLine(request.FileReadByLineReq{
		Page:     1,
		PageSize: 20,
		Type:     constant.TypeWindowsService,
		ID:       1,
		Name:     "../ignored.log",
	})
	if err != nil {
		t.Fatalf("ReadLogByLine returned error: %v", err)
	}
	if res.Path != logPath {
		t.Fatalf("expected log path %s, got %s", logPath, res.Path)
	}
	if len(res.Lines) != 2 || res.Lines[0] != "line1" || res.Lines[1] != "line2" {
		t.Fatalf("unexpected log lines: %#v", res.Lines)
	}
}
