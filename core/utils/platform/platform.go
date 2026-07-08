package platform

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	OSLinux   = "linux"
	OSWindows = "windows"
	OSDarwin  = "darwin"

	LocalAgentSockPath = "/etc/1panel/agent.sock"
	LocalAgentHTTPAddr = "127.0.0.1:26789"

	FeatureDocker         = "docker"
	FeatureToolbox        = "toolbox-management"
	FeatureJavaService    = "java-service"
	FeatureCustomService  = "custom-service"
	FeatureDeliveryPackageService = "package-service"
	FeatureDLLService     = "dll-service"
	FeatureWindowsLite    = "windows-lite"
)

func Current() string {
	return runtime.GOOS
}

func UseLocalAgentSocket() bool {
	return Current() != OSWindows
}

func DefaultInstallDir() string {
	if Current() == OSWindows {
		return `C:\1Panel`
	}
	return "/opt"
}

func InstallDir() string {
	if Current() == OSWindows {
		if dir := detectWindowsInstallDir(); dir != "" {
			return dir
		}
		return DefaultInstallDir()
	}
	if dir := loadLinuxInstallDirFromParamFile(); dir != "" {
		return dir
	}
	return DefaultInstallDir()
}

func ConfigDir() string {
	return filepath.Join(InstallDir(), "1panel", "conf")
}

func ParamFilePath() string {
	if Current() == OSWindows {
		return filepath.Join(ConfigDir(), "1pctl.env")
	}
	return "/usr/local/bin/1pctl"
}

func ReadParam(name string) string {
	file, err := os.Open(ParamFilePath())
	if err != nil {
		return ""
	}
	defer file.Close()

	prefix := name + "="
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, prefix) {
			return strings.Trim(strings.TrimPrefix(line, prefix), `"`)
		}
	}
	return ""
}

func SupportedFeatures() []string {
	switch Current() {
	case OSWindows:
		return []string{
			FeatureToolbox,
			FeatureDocker,
			FeatureJavaService,
			FeatureCustomService,
			FeatureDeliveryPackageService,
			FeatureDLLService,
			FeatureWindowsLite,
		}
	default:
		return []string{
			FeatureToolbox,
			FeatureDocker,
			FeatureJavaService,
			FeatureCustomService,
			FeatureDeliveryPackageService,
		}
	}
}

func loadLinuxInstallDirFromParamFile() string {
	file, err := os.Open("/usr/local/bin/1pctl")
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "BASE_DIR=") {
			return strings.Trim(strings.TrimPrefix(line, "BASE_DIR="), `"`)
		}
	}
	return ""
}

func detectWindowsInstallDir() string {
	candidates := []string{}
	if envDir := strings.TrimSpace(os.Getenv("PANEL_BASE_DIR")); envDir != "" {
		candidates = append(candidates, envDir)
	}
	if workDir, err := os.Getwd(); err == nil {
		candidates = append(candidates, workDir)
	}
	if executable, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(executable)
		candidates = append(candidates, exeDir)
		if parent := normalizeWindowsInstallDir(exeDir); parent != exeDir {
			candidates = append(candidates, parent)
		}
	}

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		normalized := normalizeWindowsInstallDir(candidate)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		if _, err := os.Stat(filepath.Join(normalized, "1panel", "conf", "1pctl.env")); err == nil {
			return normalized
		}
	}
	return ""
}

func normalizeWindowsInstallDir(candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	cleaned := filepath.Clean(candidate)
	switch strings.ToLower(filepath.Base(cleaned)) {
	case "bin", "service", "tools":
		return filepath.Dir(cleaned)
	case "conf":
		parent := filepath.Dir(cleaned)
		if strings.EqualFold(filepath.Base(parent), "1panel") {
			return filepath.Dir(parent)
		}
	}
	return cleaned
}
