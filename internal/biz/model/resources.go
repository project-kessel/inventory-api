package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"time"
)

type Resource struct {
	ID           uint64 `gorm:"primarykey"`
	OrgId        string `gorm:"index"`
	ResourceData JsonObject
	ResourceType string
	WorkspaceId  string
	Reporter     ResourceReporter
	ConsoleHref  string
	ApiHref      string
	Labels       Labels
	CreatedAt    *time.Time
	UpdatedAt    *time.Time
	DeletedAt    *time.Time `gorm:"-:all"`
}

type ResourceReporter struct {
	Reporter
	LocalResourceId string `json:"local_resource_id"`
}

func (r *Resource) GormDbAfterMigration(db *gorm.DB, s *schema.Schema) error {
	switch db.Dialector.Name() {
	case "sqlite":
		break
	case "postgres":
		const labelsIdx = "idx_resource_labels"
		if !db.Migrator().HasIndex(r, labelsIdx) {
			statement := fmt.Sprintf("CREATE INDEX %s on %s USING gin ( (%s) jsonb_path_ops );", labelsIdx, s.Table, s.LookUpField("Labels").DBName)
			db.Exec(statement)
		}
	}
	return nil
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
