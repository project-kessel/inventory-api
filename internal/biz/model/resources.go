package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type Resource struct {
	ID               uuid.UUID  `gorm:"type:uuid;primarykey"`
	InventoryId      *uuid.UUID `gorm:"index"`
	OrgId            string     `gorm:"index"`
	ResourceData     JsonObject
	ResourceType     string
	WorkspaceId      string
	ConsoleHref      string
	ApiHref          string
	Labels           Labels
	CreatedAt        *time.Time
	UpdatedAt        *time.Time
	ConsistencyToken string
	// Reporter Fields
	ReporterResourceId string `json:"reporter_resource_id"`
	ReporterId         string `json:"reporter_id"`
	// Deprecated: Use Reporter Fields instead(ReporterId, ReporterResourceId)
	Reporter ResourceReporter
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

func (r *Resource) BeforeCreate(db *gorm.DB) error {
	var err error
	if r.ID == uuid.Nil {
		r.ID, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("failed to generate uuid: %w", err)
		}
	}
	return nil
}
