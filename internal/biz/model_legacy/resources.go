package model_legacy

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
	ID               uuid.UUID `gorm:"type:uuid;primarykey"`
	ResourceType     string
	ConsistencyToken string

	// Representation References

	// Reporter Representation
	/*Representation

	LocalResourceID
	ReporterType
	ResourceType
	Version
	ReporterInstanceID
	Generation
	APIHref
	ConsoleHref
	CommonVersion
	Tombstone
	ReporterVersion */
	ConsoleHref        string
	ApiHref            string
	ReporterResourceId string     `json:"reporter_resource_id"` // LocalResourceID in ReporterRepresentation
	ReporterType       string     `json:"reporter_type"`
	ReporterInstanceId string     `json:"reporter_instance_id"`
	ReporterVersion    string     `json:"reporter_version"`
	ResourceData       JsonObject // Already part of Reporter Representation

	// Common Representation
	/*Representation
	ResourceId
	ResourceType
	Version
	ReportedByReporterType
	ReportedByReporterInstance*/
	WorkspaceId string //Representation in Common Representation

	// Metadata for operations
	CreatedAt *time.Time
	UpdatedAt *time.Time

	// Can be removed
	Labels Labels
	ReporterResourceUniqueIndex
	ReporterId  string `json:"reporter_id"`
	Reporter    ResourceReporter
	InventoryId *uuid.UUID `gorm:"index"`
	OrgId       string     `gorm:"index"`
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
