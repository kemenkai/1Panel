package response

type WindowsServiceJarUploadResult struct {
	JarPath        string                                `json:"jarPath"`
	WorkDir        string                                `json:"workDir"`
	ExecPath       string                                `json:"execPath"`
	WinSWPath      string                                `json:"winswPath"`
	FileName       string                                `json:"fileName"`
	InstallDir     string                                `json:"installDir"`
	ConfigTemplate *WindowsServiceConfigTemplate         `json:"configTemplate,omitempty"`
	Services       []WindowsServicePackageServicePreview `json:"services,omitempty"`
}

type WindowsServicePackageServicePreview struct {
	Name               string `json:"name"`
	DisplayName        string `json:"displayName"`
	ServiceType        string `json:"serviceType"`
	RuntimeType        string `json:"runtimeType"`
	WorkDir            string `json:"workDir"`
	ConfigPath         string `json:"configPath"`
	JarPath            string `json:"jarPath"`
	ExecPath           string `json:"execPath"`
	Args               string `json:"args"`
	WinSWPath          string `json:"winswPath"`
	SourcePath         string `json:"sourcePath"`
	ComposeServiceName string `json:"composeServiceName"`
}
