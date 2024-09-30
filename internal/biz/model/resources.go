package model

import (
	"database/sql/driver"
	"encoding/json"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"time"
)

type Resource struct {
	ID           uint64 `gorm:"primarykey"`
	ResourceData JsonObject
	ResourceType string
	Workspace    string
	Reporter     ResourceReporter
	ConsoleHref  string
	ApiHref      string
	Labels       Labels
	// Todo: Should we use pointers here to let the database handle them for us?
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

type ResourceReporter struct {
	Reporter
	LocalResourceId string
}

func (ResourceReporter) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	return GormDBDataType(db, field)
}

func (data ResourceReporter) Value() (driver.Value, error) {
	return json.Marshal(data)
}

func (data *ResourceReporter) Scan(value interface{}) error {
	return Scan(value, data)
}
