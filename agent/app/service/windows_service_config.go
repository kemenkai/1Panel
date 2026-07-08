package service

import (
	"bufio"
	_ "embed"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/1Panel-dev/1Panel/agent/app/dto/request"
	"github.com/1Panel-dev/1Panel/agent/app/dto/response"
	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/utils/platform"
)

const (
	windowsServiceConfigTemplateDefault = "spring-boot-application-dev"
	windowsServiceConfigFileName        = "application-dev.yml"
)

const (
	envAppServerPort        = "APP_SERVER_PORT"
	envAppName              = "APP_NAME"
	envRedisHost            = "REDIS_HOST"
	envPlatformBaseURL      = "ANZHENER_PLATFORM_BASE_URL"
	envXxlJobAdminAddresses = "XXL_JOB_ADMIN_ADDRESSES"
	envHisDBHost            = "HIS_DB_HOST"
	envHisDBPort            = "HIS_DB_PORT"
	envHisDBServerName      = "HIS_DB_SERVER_NAME"
	envHisDBSchema          = "HIS_DB_SCHEMA"
	envHisDBUser            = "HIS_DB_USER"
	envHisDBPassword        = "HIS_DB_PASSWORD"
	envOrgCode              = "ANZHENER_ORG_CODE"
	envHospitalCode         = "ANZHENER_HOSPITAL_CODE"
	envPlatformCode         = "ANZHENER_PLATFORM_CODE"
)

var configPlaceholderPattern = regexp.MustCompile(`\$\{([A-Za-z0-9_]+):([^}]*)\}`)

//go:embed templates/application-dev.yml
var windowsServiceConfigTemplateContent string

type managedWindowsServiceConfigField struct {
	EnvKey       string
	DefaultValue string
	Value        string
	ConfigPath   string
	InputType    string
	Sensitive    bool
}

type managedWindowsServiceConfig struct {
	Enabled  bool
	Template string
	FileName string
	Content  string
	Fields   []managedWindowsServiceConfigField
}

func defaultManagedWindowsServiceConfig() managedWindowsServiceConfig {
	return newManagedWindowsServiceConfigFromContent(
		windowsServiceConfigTemplateContent,
		windowsServiceConfigTemplateDefault,
		windowsServiceConfigFileName,
	)
}

func newManagedWindowsServiceConfigFromContent(templateContent, templateName, fileName string) managedWindowsServiceConfig {
	normalizedContent := normalizeManagedWindowsServiceConfigContent(templateContent)
	if normalizedContent == "" {
		normalizedContent = normalizeManagedWindowsServiceConfigContent(windowsServiceConfigTemplateContent)
	}
	if strings.TrimSpace(templateName) == "" {
		templateName = windowsServiceConfigTemplateDefault
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = windowsServiceConfigFileName
	}
	return managedWindowsServiceConfig{
		Enabled:  true,
		Template: templateName,
		FileName: fileName,
		Content:  normalizedContent,
		Fields:   parseManagedWindowsServiceConfigTemplateFields(normalizedContent),
	}
}

func normalizeManagedWindowsServiceConfigContent(content string) string {
	return strings.TrimPrefix(content, "\uFEFF")
}

func parseManagedWindowsServiceConfigTemplateFields(templateContent string) []managedWindowsServiceConfigField {
	type yamlLevel struct {
		indent int
		key    string
	}

	var (
		fields []managedWindowsServiceConfigField
		stack  []yamlLevel
		seen   = make(map[string]struct{})
	)

	scanner := bufio.NewScanner(strings.NewReader(templateContent))
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		content := strings.TrimLeft(line, " ")
		if strings.HasPrefix(content, "- ") {
			content = strings.TrimSpace(strings.TrimPrefix(content, "- "))
		}

		colonIndex := strings.Index(content, ":")
		if colonIndex <= 0 {
			continue
		}

		key := strings.TrimSpace(content[:colonIndex])
		valuePart := strings.TrimSpace(content[colonIndex+1:])

		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}

		pathParts := make([]string, 0, len(stack)+1)
		for _, level := range stack {
			pathParts = append(pathParts, level.key)
		}
		pathParts = append(pathParts, key)
		configPath := strings.Join(pathParts, ".")

		matches := configPlaceholderPattern.FindAllStringSubmatch(valuePart, -1)
		for _, match := range matches {
			envKey := strings.TrimSpace(match[1])
			if envKey == "" {
				continue
			}
			if _, ok := seen[envKey]; ok {
				continue
			}
			defaultValue := match[2]
			sensitive := isSensitiveConfigEnvKey(envKey)
			fields = append(fields, managedWindowsServiceConfigField{
				EnvKey:       envKey,
				DefaultValue: defaultValue,
				Value:        defaultValue,
				ConfigPath:   configPath,
				InputType:    detectConfigFieldInputType(envKey, configPath, defaultValue, sensitive),
				Sensitive:    sensitive,
			})
			seen[envKey] = struct{}{}
		}

		if valuePart == "" || valuePart == "|" || valuePart == ">" {
			stack = append(stack, yamlLevel{indent: indent, key: key})
		}
	}

	return fields
}

func isSensitiveConfigEnvKey(envKey string) bool {
	upperKey := strings.ToUpper(strings.TrimSpace(envKey))
	sensitiveMarkers := []string{"PASSWORD", "PASSWD", "SECRET", "TOKEN", "PRIVATE_KEY", "PRIVATEKEY"}
	for _, marker := range sensitiveMarkers {
		if strings.Contains(upperKey, marker) {
			return true
		}
	}
	return false
}

func detectConfigFieldInputType(envKey, configPath, defaultValue string, sensitive bool) string {
	if sensitive {
		return "password"
	}
	upperKey := strings.ToUpper(strings.TrimSpace(envKey))
	if isIntegerString(defaultValue) && (strings.Contains(upperKey, "PORT") || strings.HasSuffix(strings.ToLower(configPath), ".port")) {
		return "number"
	}
	return "text"
}

func isIntegerString(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func buildManagedWindowsServiceConfig(req *request.WindowsServiceConfigTemplate) *managedWindowsServiceConfig {
	if req == nil || !req.Enabled {
		return nil
	}

	cfg := defaultManagedWindowsServiceConfig()
	if content := normalizeManagedWindowsServiceConfigContent(req.Content); strings.TrimSpace(content) != "" {
		cfg = newManagedWindowsServiceConfigFromContent(content, req.Template, req.FileName)
	}
	if value := strings.TrimSpace(req.Template); value != "" {
		cfg.Template = value
	}
	if value := strings.TrimSpace(req.FileName); value != "" {
		cfg.FileName = value
	}

	fieldIndexes := make(map[string]int, len(cfg.Fields))
	for index := range cfg.Fields {
		fieldIndexes[cfg.Fields[index].EnvKey] = index
	}

	for _, field := range req.DynamicFields {
		envKey := strings.TrimSpace(field.EnvKey)
		if envKey == "" {
			continue
		}
		value := field.Value
		if index, ok := fieldIndexes[envKey]; ok {
			cfg.Fields[index].Value = value
			if cfg.Fields[index].ConfigPath == "" && strings.TrimSpace(field.ConfigPath) != "" {
				cfg.Fields[index].ConfigPath = strings.TrimSpace(field.ConfigPath)
			}
			continue
		}
		sensitive := field.Sensitive || isSensitiveConfigEnvKey(envKey)
		defaultValue := field.DefaultValue
		cfg.Fields = append(cfg.Fields, managedWindowsServiceConfigField{
			EnvKey:       envKey,
			DefaultValue: defaultValue,
			Value:        value,
			ConfigPath:   strings.TrimSpace(field.ConfigPath),
			InputType:    normalizeConfigFieldInputType(field.InputType, envKey, field.ConfigPath, defaultValue, sensitive),
			Sensitive:    sensitive,
		})
	}

	return &cfg
}

func normalizeConfigFieldInputType(inputType, envKey, configPath, defaultValue string, sensitive bool) string {
	switch strings.TrimSpace(inputType) {
	case "password", "number", "text":
		return strings.TrimSpace(inputType)
	default:
		return detectConfigFieldInputType(envKey, configPath, defaultValue, sensitive)
	}
}

func (c managedWindowsServiceConfig) generatedPath(workDir string) string {
	fileName := safeConfigFileName(c.FileName)
	if fileName == "" {
		fileName = windowsServiceConfigFileName
	}
	return filepath.Join(workDir, "config", fileName)
}

// safeConfigFileName reduces a caller-supplied config file name to a bare base
// name, stripping any directory components so it cannot traverse out of the
// managed config directory (e.g. `..\..\Windows\x.yml`).
func safeConfigFileName(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if name == "." || name == ".." {
		return ""
	}
	return name
}

func (c managedWindowsServiceConfig) toResponse(generatedPath string) *response.WindowsServiceConfigTemplate {
	fields := make([]response.WindowsServiceConfigTemplateField, 0, len(c.Fields))
	for _, field := range c.Fields {
		fields = append(fields, response.WindowsServiceConfigTemplateField{
			EnvKey:       field.EnvKey,
			DefaultValue: field.DefaultValue,
			Value:        field.Value,
			ConfigPath:   field.ConfigPath,
			InputType:    field.InputType,
			Sensitive:    field.Sensitive,
		})
	}
	return &response.WindowsServiceConfigTemplate{
		Enabled:       true,
		Template:      c.Template,
		FileName:      c.FileName,
		GeneratedPath: generatedPath,
		Content:       c.Content,
		DynamicFields: fields,
	}
}

func writeManagedWindowsServiceConfig(configPath, content string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}
	normalizedContent := normalizeManagedWindowsServiceConfigContent(content)
	if strings.TrimSpace(normalizedContent) == "" {
		normalizedContent = normalizeManagedWindowsServiceConfigContent(windowsServiceConfigTemplateContent)
	}
	// Linux Spring Boot loaders reject a UTF-8 BOM at the head of application.yml;
	// Windows keeps the BOM for consistency with the other UTF-8 artifacts there.
	// The config can hold plaintext secrets and is read by the root-run service, so
	// it is written 0600 (never world-readable under the 0755 service tree). NOTE:
	// if a future unit drops privileges via User=, this must become 0640 with the
	// group set to the service user, or the process cannot read its own config.
	if platform.Current() == platform.OSLinux {
		return atomicWriteFile(configPath, []byte(normalizedContent), 0o600)
	}
	return writeUTF8File(configPath, normalizedContent)
}

func appendManagedWindowsServiceConfigEnv(lines []string, cfg *managedWindowsServiceConfig) []string {
	if cfg == nil {
		return lines
	}
	for _, field := range cfg.Fields {
		if strings.TrimSpace(field.EnvKey) == "" {
			continue
		}
		lines = append(lines, field.EnvKey+"="+field.Value)
	}
	return lines
}

func ensureSpringConfigArgument(args, configPath string) string {
	trimmed := strings.TrimSpace(args)
	if configPath == "" {
		return trimmed
	}
	lowerArgs := strings.ToLower(trimmed)
	if strings.Contains(lowerArgs, "--spring.config.location") || strings.Contains(lowerArgs, "--spring.config.additional-location") {
		return trimmed
	}
	if trimmed != "" {
		trimmed += " "
	}
	trimmed += "--spring.config.location=\"" + configPath + "\""
	return strings.TrimSpace(trimmed)
}

func detachManagedSpringConfigArgument(args, configPath string) (string, string) {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" || configPath == "" {
		return trimmed, ""
	}

	springArg := "--spring.config.location=\"" + configPath + "\""
	if !strings.Contains(trimmed, springArg) {
		return trimmed, ""
	}

	remaining := strings.Replace(trimmed, springArg, "", 1)
	for strings.Contains(remaining, "  ") {
		remaining = strings.ReplaceAll(remaining, "  ", " ")
	}
	return strings.TrimSpace(remaining), springArg
}

func defaultManagedWindowsServiceConfigResponse() *response.WindowsServiceConfigTemplate {
	cfg := defaultManagedWindowsServiceConfig()
	return cfg.toResponse("")
}

func loadManagedWindowsServiceConfig(item model.WindowsService) *response.WindowsServiceConfigTemplate {
	if item.ServiceType != "java" && item.ServiceType != "package" {
		return nil
	}
	envMap := loadWindowsServiceEnvMap(item.EnvFilePath)
	if cfg, ok := loadManagedWindowsServiceConfigFromPath(item.ConfigPath); ok {
		for index := range cfg.Fields {
			if value, exists := envMap[cfg.Fields[index].EnvKey]; exists {
				cfg.Fields[index].Value = value
			}
		}
		return cfg.toResponse(item.ConfigPath)
	}
	if !isManagedWindowsServiceConfig(item.ConfigPath, envMap) {
		return nil
	}

	cfg := defaultManagedWindowsServiceConfig()
	for index := range cfg.Fields {
		if value, ok := envMap[cfg.Fields[index].EnvKey]; ok {
			cfg.Fields[index].Value = value
		}
	}
	return cfg.toResponse(item.ConfigPath)
}

func isManagedWindowsServiceConfig(configPath string, envMap map[string]string) bool {
	if isManagedWindowsServiceConfigFileName(filepath.Base(strings.TrimSpace(configPath))) {
		return true
	}
	for _, field := range defaultManagedWindowsServiceConfig().Fields {
		if _, ok := envMap[field.EnvKey]; ok {
			return true
		}
	}
	return false
}

func loadWindowsServiceEnvMap(envFilePath string) map[string]string {
	values := make(map[string]string)
	if strings.TrimSpace(envFilePath) == "" {
		return values
	}
	file, err := os.Open(envFilePath)
	if err != nil {
		return values
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimPrefix(strings.TrimSpace(scanner.Text()), "\uFEFF")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return values
}

func loadManagedWindowsServiceConfigFromPath(configPath string) (*managedWindowsServiceConfig, bool) {
	content, err := readFileWithLimit(strings.TrimSpace(configPath), maxWindowsServiceConfigFileSize)
	if err != nil {
		return nil, false
	}
	normalizedContent := normalizeManagedWindowsServiceConfigContent(string(content))
	if strings.TrimSpace(normalizedContent) == "" {
		return nil, false
	}
	cfg := newManagedWindowsServiceConfigFromContent(normalizedContent, windowsServiceConfigTemplateDefault, filepath.Base(configPath))
	if len(cfg.Fields) == 0 && !isManagedWindowsServiceConfigFileName(filepath.Base(configPath)) {
		return nil, false
	}
	return &cfg, true
}

func isManagedWindowsServiceConfigFileName(fileName string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(fileName))
	if strings.HasPrefix(lowerName, "application") || strings.HasPrefix(lowerName, "bootstrap") {
		return strings.HasSuffix(lowerName, ".yml") || strings.HasSuffix(lowerName, ".yaml")
	}
	return false
}

func discoverManagedWindowsServiceConfig(rootDir string) (*managedWindowsServiceConfig, string, error) {
	if strings.TrimSpace(rootDir) == "" {
		return nil, "", nil
	}
	candidates := make([]string, 0, 4)
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if isManagedWindowsServiceConfigFileName(d.Name()) {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return managedWindowsServiceConfigCandidateScore(candidates[i]) < managedWindowsServiceConfigCandidateScore(candidates[j])
	})
	for _, candidate := range candidates {
		cfg, ok := loadManagedWindowsServiceConfigFromPath(candidate)
		if !ok {
			continue
		}
		return cfg, candidate, nil
	}
	return nil, "", nil
}

func managedWindowsServiceConfigCandidateScore(path string) int {
	lowerPath := strings.ToLower(filepath.ToSlash(path))
	switch {
	case strings.HasSuffix(lowerPath, "/config/application-dev.yml"):
		return 0
	case strings.HasSuffix(lowerPath, "/config/application.yml"):
		return 1
	case strings.HasSuffix(lowerPath, "/config/bootstrap.yml"):
		return 2
	case strings.HasSuffix(lowerPath, "/config/bootstrap-dev.yml"):
		return 3
	case strings.HasSuffix(lowerPath, "application-dev.yml"):
		return 4
	case strings.HasSuffix(lowerPath, "application.yml"):
		return 5
	case strings.HasSuffix(lowerPath, "bootstrap.yml"):
		return 6
	default:
		return 10
	}
}
