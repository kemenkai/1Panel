package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/1Panel-dev/1Panel/agent/app/dto/request"
	"github.com/1Panel-dev/1Panel/agent/app/dto/response"
	"github.com/1Panel-dev/1Panel/agent/app/model"
	"gopkg.in/yaml.v3"
)

type deliveryPackageComposeFile struct {
	Services map[string]any `yaml:"services"`
}

type deliveryPackageComposeService struct {
	ServiceName string
	WindowsName string
}

func buildDeliveryPackageCreateItems(req request.WindowsServiceCreate, dockerComposeCmd string, registerService bool) ([]model.WindowsService, error) {
	templateDir := strings.TrimSpace(req.WorkDir)
	if templateDir == "" {
		return nil, fmt.Errorf("delivery package template directory is required")
	}
	if dockerComposeCmd != "" {
		items, err := buildDeliveryPackageComposeItems(req, dockerComposeCmd, registerService)
		if err != nil {
			return nil, err
		}
		if len(items) > 0 {
			return items, nil
		}
	}
	return buildDeliveryPackageJarItems(req, registerService)
}

func buildDeliveryPackageComposeItems(req request.WindowsServiceCreate, dockerComposeCmd string, registerService bool) ([]model.WindowsService, error) {
	composeFiles, err := findDeliveryPackageComposeFiles(req.WorkDir)
	if err != nil {
		return nil, err
	}
	winSWPath := strings.TrimSpace(req.WinSWPath)
	if winSWPath == "" {
		winSWPath = defaultManagedServiceWinSWPath()
	}
	var items []model.WindowsService
	usedNames := map[string]int{}
	for _, composePath := range composeFiles {
		services, err := parseDeliveryPackageComposeServices(composePath)
		if err != nil {
			return nil, err
		}
		for _, service := range services {
			windowsName := uniqueDeliveryPackageComposeWindowsName(service.WindowsName, composePath, req.WorkDir, usedNames)
			items = append(items, model.WindowsService{
				Name:            windowsName,
				DisplayName:     buildDeliveryPackageDisplayName(req.DisplayName, service.ServiceName),
				ServiceType:     "package",
				ExecPath:        dockerComposeCmd,
				Args:            service.ServiceName,
				WorkDir:         filepath.Dir(composePath),
				DLLDir:          strings.TrimSpace(req.DLLDir),
				ConfigPath:      composePath,
				WinSWPath:       winSWPath,
				Status:          "Waiting",
				RegisterService: registerService,
				AutoStart:       req.AutoStart,
			})
		}
	}
	return items, nil
}

func buildDeliveryPackageJarItems(req request.WindowsServiceCreate, registerService bool) ([]model.WindowsService, error) {
	jarPaths, err := findDeliveryPackageJarFiles(req.WorkDir)
	if err != nil {
		return nil, err
	}
	if len(jarPaths) == 0 {
		return nil, fmt.Errorf("no delivery package compose file or jar package found under %s", req.WorkDir)
	}
	javaPath, err := resolveBundledJavaExecutable()
	if err != nil {
		return nil, err
	}
	winSWPath := strings.TrimSpace(req.WinSWPath)
	if winSWPath == "" {
		winSWPath = defaultManagedServiceWinSWPath()
	}
	var items []model.WindowsService
	for _, jarPath := range jarPaths {
		serviceName := sanitizeServiceName(strings.TrimSuffix(filepath.Base(jarPath), filepath.Ext(jarPath)))
		if serviceName == "" {
			continue
		}
		items = append(items, model.WindowsService{
			Name:            serviceName,
			DisplayName:     buildDeliveryPackageDisplayName(req.DisplayName, serviceName),
			ServiceType:     "package",
			ExecPath:        javaPath,
			Args:            strings.TrimSpace(req.Args),
			WorkDir:         filepath.Dir(jarPath),
			DLLDir:          strings.TrimSpace(req.DLLDir),
			ConfigPath:      strings.TrimSpace(req.ConfigPath),
			JarPath:         jarPath,
			WinSWPath:       winSWPath,
			Status:          "Waiting",
			RegisterService: registerService,
			AutoStart:       req.AutoStart,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no valid delivery package jar service found under %s", req.WorkDir)
	}
	return items, nil
}

func findDeliveryPackageComposeFiles(root string) ([]string, error) {
	var result []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		ext := filepath.Ext(name)
		if ext != ".yml" && ext != ".yaml" {
			return nil
		}
		if name == "docker-compose.yml" || name == "docker-compose.yaml" || name == "compose.yml" || name == "compose.yaml" || strings.HasPrefix(name, "docker-compose-") || strings.HasPrefix(name, "docker-compose.") {
			result = append(result, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

func parseDeliveryPackageComposeServices(composePath string) ([]deliveryPackageComposeService, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, err
	}
	var compose deliveryPackageComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, err
	}
	services := make([]deliveryPackageComposeService, 0, len(compose.Services))
	for name := range compose.Services {
		serviceName := strings.TrimSpace(name)
		// The raw service name is embedded verbatim into the generated launch
		// command, so reject anything outside the docker-compose legal charset
		// ([A-Za-z0-9._-]) to prevent command injection from an uploaded package.
		if !isValidComposeServiceName(serviceName) {
			continue
		}
		windowsName := sanitizeServiceName(serviceName)
		if windowsName != "" {
			services = append(services, deliveryPackageComposeService{
				ServiceName: serviceName,
				WindowsName: windowsName,
			})
		}
	}
	sort.Slice(services, func(i, j int) bool {
		return services[i].WindowsName < services[j].WindowsName
	})
	return services, nil
}

func buildDeliveryPackagePackageServicePreviews(req request.WindowsServiceCreate, dockerComposeCmd string) ([]response.WindowsServicePackageServicePreview, error) {
	items, err := buildDeliveryPackageCreateItems(req, dockerComposeCmd, true)
	if err != nil {
		return nil, err
	}
	previews := make([]response.WindowsServicePackageServicePreview, 0, len(items))
	for _, item := range items {
		runtimeType := "java"
		sourcePath := item.JarPath
		composeServiceName := ""
		if isDeliveryPackageComposeItem(item) {
			runtimeType = "compose"
			sourcePath = item.ConfigPath
			composeServiceName = item.Args
		}
		previews = append(previews, response.WindowsServicePackageServicePreview{
			Name:               item.Name,
			DisplayName:        item.DisplayName,
			ServiceType:        item.ServiceType,
			RuntimeType:        runtimeType,
			WorkDir:            item.WorkDir,
			ConfigPath:         item.ConfigPath,
			JarPath:            item.JarPath,
			ExecPath:           item.ExecPath,
			Args:               item.Args,
			WinSWPath:          item.WinSWPath,
			SourcePath:         sourcePath,
			ComposeServiceName: composeServiceName,
		})
	}
	return previews, nil
}

func uniqueDeliveryPackageComposeWindowsName(baseName, composePath, templateDir string, used map[string]int) string {
	baseName = sanitizeServiceName(baseName)
	if baseName == "" {
		return ""
	}
	if used[baseName] == 0 {
		used[baseName] = 1
		return baseName
	}
	contextName := buildDeliveryPackageComposeContextName(composePath, templateDir)
	candidate := baseName
	if contextName != "" {
		candidate = baseName + "-" + contextName
	}
	for used[candidate] > 0 {
		used[baseName]++
		candidate = fmt.Sprintf("%s-%d", baseName, used[baseName])
		if contextName != "" {
			candidate = fmt.Sprintf("%s-%s-%d", baseName, contextName, used[baseName])
		}
	}
	used[candidate] = 1
	return candidate
}

func buildDeliveryPackageComposeContextName(composePath, templateDir string) string {
	relPath, err := filepath.Rel(strings.TrimSpace(templateDir), strings.TrimSpace(composePath))
	if err != nil {
		relPath = composePath
	}
	context := strings.TrimSuffix(relPath, filepath.Ext(relPath))
	context = strings.TrimPrefix(context, "docker-compose")
	context = strings.TrimPrefix(context, "compose")
	return sanitizeServiceName(context)
}

func findDeliveryPackageJarFiles(root string) ([]string, error) {
	var result []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".jar") {
			return nil
		}
		result = append(result, path)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

func buildDeliveryPackageDisplayName(prefix, serviceName string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || strings.EqualFold(prefix, serviceName) {
		return serviceName
	}
	return prefix + " - " + serviceName
}

// isValidComposeServiceName matches the docker-compose service name grammar
// ([A-Za-z0-9._-]+). Anything else is untrusted input that must not reach the
// generated PowerShell launch command.
func isValidComposeServiceName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		valid := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
		if !valid {
			return false
		}
	}
	return true
}

func sanitizeServiceName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	name = b.String()
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	return strings.Trim(name, "-")
}

func isDeliveryPackageComposeItem(item model.WindowsService) bool {
	execPath := strings.TrimSpace(item.ExecPath)
	return item.ServiceType == "package" && item.JarPath == "" && strings.HasPrefix(execPath, "docker")
}
