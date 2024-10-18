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
	ReporterId   string `gorm:"primarykey"`
	ReporterType string `gorm:"primarykey"`
}

func ReporterResourceIdFromResource(resource *Resource) ReporterResourceId {
	return ReporterResourceId{
		LocalResourceId: resource.Reporter.LocalResourceId,
		ResourceType:    resource.ResourceType,
		ReporterId:      resource.Reporter.ReporterId,
		ReporterType:    resource.Reporter.ReporterType,
	}
}
