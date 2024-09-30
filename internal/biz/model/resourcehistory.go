package model

import (
	"time"
)

type ResourceHistory struct {
	ID           uint64 `gorm:"primarykey"`
	ResourceData JsonObject
	ResourceType string
	Workspace    string
	Reporter     ResourceReporter
	ConsoleHref  string
	ApiHref      string
	Labels       Labels
	CreatedAt    *time.Time
	// We don't need UpdatedAt in here. We won't update the history resource

	ResourceId       uint64
	OriginalResource Resource `gorm:"foreignKey:ResourceId" json:"-"`
}

func (*ResourceHistory) TableName() string {
	return "resource_history"
}
