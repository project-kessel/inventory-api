package model

import "time"

type LocalInventoryToResource struct {
	// Our local resource id
	ResourceId uint64     `gorm:"primarykey"`
	CreatedAt  *time.Time `gorm:"index"`

	ReporterResourceId
}

type ReporterResourceId struct {
	// Id in reporter's side
	LocalResourceId string `gorm:"primarykey"`
	ResourceType    string `gorm:"primarykey"`

	// Reporter identification
	ReporterId string `gorm:"primarykey"`
}
