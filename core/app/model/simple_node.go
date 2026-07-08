package model

import "time"

type SimpleNode struct {
	BaseModel
	Name             string     `json:"name" gorm:"type:varchar(64);not null;uniqueIndex"`
	Addr             string     `json:"addr" gorm:"type:varchar(256);not null"`
	SecurityEntrance string     `json:"securityEntrance" gorm:"type:varchar(256)"`
	APIKey           string     `json:"apiKey" gorm:"type:text"`
	Description      string     `json:"description" gorm:"type:varchar(256)"`
	Status           string     `json:"status" gorm:"type:varchar(64)"`
	Message          string     `json:"message" gorm:"type:text"`
	LastCheckAt      *time.Time `json:"lastCheckAt"`
}
