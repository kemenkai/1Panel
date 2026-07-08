package request

type WindowsServiceCreate struct {
	Name            string                        `json:"name" validate:"max=128"`
	DisplayName     string                        `json:"displayName" validate:"max=128"`
	ServiceType     string                        `json:"serviceType" validate:"omitempty,oneof=java dll package"`
	ExecPath        string                        `json:"execPath" validate:"max=512"`
	Args            string                        `json:"args"`
	WorkDir         string                        `json:"workDir" validate:"max=512"`
	EnvFilePath     string                        `json:"envFilePath" validate:"max=512"`
	DLLDir          string                        `json:"dllDir" validate:"max=512"`
	ConfigPath      string                        `json:"configPath" validate:"max=512"`
	JarPath         string                        `json:"jarPath" validate:"max=512"`
	WinSWPath       string                        `json:"winswPath" validate:"max=512"`
	ConfigTemplate  *WindowsServiceConfigTemplate `json:"configTemplate"`
	RegisterService *bool                         `json:"registerService"`
	AutoStart       bool                          `json:"autoStart"`
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

type WindowsServiceUpdate struct {
	ID uint `json:"id" validate:"required"`
	WindowsServiceCreate
}

type WindowsServiceOperate struct {
	ID      uint   `json:"id" validate:"required"`
	Operate string `json:"operate" validate:"required,oneof=start stop restart enable disable status"`
}

type WindowsServiceConfigFileUpdate struct {
	Type    string `json:"type" validate:"required,oneof=config env"`
	Content string `json:"content"`
}
