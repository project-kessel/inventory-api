package model_legacy

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/project-kessel/inventory-api/internal"
)

type Resource struct {
	ID               uuid.UUID           `gorm:"type:uuid;primarykey"`
	InventoryId      *uuid.UUID          `gorm:"index"`
	OrgId            string              `gorm:"index"`
	ResourceData     internal.JsonObject `json:"resource_data" gorm:"type:jsonb"`
	ResourceType     string              `json:"resource_type" gorm:"type:text;index:resource_type_id_index"`
	WorkspaceId      string              `json:"workspace_id" gorm:"type:text;index:workspace_id_resource_type_index,priority:2"`
	ConsoleHref      string
	ApiHref          string
	Labels           Labels
	CreatedAt        *time.Time
	UpdatedAt        *time.Time
	ConsistencyToken string
	// Reporter Fields
	ReporterResourceId string `json:"reporter_resource_id"`
	ReporterType       string `json:"reporter_type"`
	ReporterInstanceId string `json:"reporter_instance_id"`
	ReporterVersion    string `json:"reporter_version"`
	//Unique Indexes
	ReporterResourceUniqueIndex
	// Reporter Principal
	ReporterId string `json:"reporter_id"`
	// Deprecated: Use Reporter Fields instead(ReporterId, ReporterResourceId)
	Reporter ResourceReporter
}

type ReporterResourceUniqueIndex struct {
	ResourceType       string `gorm:"uniqueIndex:reporter_resource_unique_index"`
	ReporterResourceId string `gorm:"uniqueIndex:reporter_resource_unique_index"`
	ReporterType       string `gorm:"uniqueIndex:reporter_resource_unique_index"`
	ReporterInstanceId string `gorm:"uniqueIndex:reporter_resource_unique_index"`
}

func ReporterResourceIdv1beta2FromResource(resource *Resource) ReporterResourceUniqueIndex {
	return ReporterResourceUniqueIndex{
		ReporterResourceId: resource.ReporterResourceId,
		ResourceType:       resource.ResourceType,
		ReporterInstanceId: resource.ReporterInstanceId,
		ReporterType:       resource.ReporterType,
	}
}

type ResourceReporter struct {
	Reporter
	LocalResourceId string `json:"local_resource_id"`
}

func (r *Resource) GormDbAfterMigration(db *gorm.DB, s *schema.Schema) error {
	switch db.Name() {
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
