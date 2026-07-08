package model

type WindowsService struct {
	BaseModel
	Name            string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"`
	DisplayName     string `json:"displayName" gorm:"type:varchar(128);not null"`
	ServiceType     string `json:"serviceType" gorm:"type:varchar(64);not null"`
	ExecPath        string `json:"execPath" gorm:"type:varchar(512);not null"`
	Args            string `json:"args" gorm:"type:text"`
	WorkDir         string `json:"workDir" gorm:"type:varchar(512)"`
	EnvFilePath     string `json:"envFilePath" gorm:"type:varchar(512)"`
	DLLDir          string `json:"dllDir" gorm:"type:varchar(512)"`
	ConfigPath      string `json:"configPath" gorm:"type:varchar(512)"`
	JarPath         string `json:"jarPath" gorm:"type:varchar(512)"`
	WinSWPath       string `json:"winswPath" gorm:"type:varchar(512)"`
	ServicePath     string `json:"servicePath" gorm:"type:varchar(512)"`
	Status          string `json:"status" gorm:"type:varchar(64)"`
	Message         string `json:"message" gorm:"type:text"`
	RegisterService bool   `json:"registerService"`
	AutoStart       bool   `json:"autoStart"`
}
