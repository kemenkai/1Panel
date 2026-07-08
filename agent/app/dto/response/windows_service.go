package response

import "time"

type WindowsServiceInfo struct {
	ID                  uint                          `json:"id"`
	Name                string                        `json:"name"`
	DisplayName         string                        `json:"displayName"`
	ServiceType         string                        `json:"serviceType"`
	ExecPath            string                        `json:"execPath"`
	Args                string                        `json:"args"`
	WorkDir             string                        `json:"workDir"`
	EnvFilePath         string                        `json:"envFilePath"`
	DLLDir              string                        `json:"dllDir"`
	ConfigPath          string                        `json:"configPath"`
	JarPath             string                        `json:"jarPath"`
	WinSWPath           string                        `json:"winswPath"`
	ServicePath         string                        `json:"servicePath"`
	UninstallScriptPath string                        `json:"uninstallScriptPath"`
	Status              string                        `json:"status"`
	Message             string                        `json:"message"`
	ConfigTemplate      *WindowsServiceConfigTemplate `json:"configTemplate,omitempty"`
	RegisterService     bool                          `json:"registerService"`
	AutoStart           bool                          `json:"autoStart"`
}

type WindowsServiceConfigTemplate struct {
	Enabled       bool                                `json:"enabled"`
	Template      string                              `json:"template"`
	FileName      string                              `json:"fileName"`
	GeneratedPath string                              `json:"generatedPath"`
	Content       string                              `json:"content"`
	DynamicFields []WindowsServiceConfigTemplateField `json:"dynamicFields"`
}

type WindowsServiceConfigTemplateField struct {
	EnvKey       string `json:"envKey"`
	DefaultValue string `json:"defaultValue"`
	Value        string `json:"value"`
	ConfigPath   string `json:"configPath"`
	InputType    string `json:"inputType"`
	Sensitive    bool   `json:"sensitive"`
}

type WindowsServiceConfigFile struct {
	Type    string `json:"type"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

type WindowsServiceLogFile struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Exists     bool      `json:"exists"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
}
