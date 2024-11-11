package model

import (
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"time"
)

type ResourceHistory struct {
	ID           uuid.UUID `gorm:"type:uuid;primarykey"`
	OrgId        string    `gorm:"index"`
	ResourceData JsonObject
	ResourceType string
	WorkspaceId  string
	Reporter     ResourceReporter
	ConsoleHref  string
	ApiHref      string
	Labels       Labels
	Timestamp    *time.Time `gorm:"autoCreateTime"`

	ResourceId    uuid.UUID     `gorm:"type:uuid;index"`
	OperationType OperationType `gorm:"index"`
}

func (r *ResourceHistory) ResourceHistory(db *gorm.DB, s *schema.Schema) error {
	switch db.Dialector.Name() {
	case "sqlite":
		break
	case "postgres":
		const labelsIdx = "idx_resource_history_labels"
		if !db.Migrator().HasIndex(r, labelsIdx) {
			statement := fmt.Sprintf("CREATE INDEX %s on %s USING gin ( (%s) jsonb_path_ops );", labelsIdx, s.Table, s.LookUpField("Labels").DBName)
			db.Exec(statement)
		}
	}
	return nil
}

func (*ResourceHistory) TableName() string {
	return "resource_history"
}

func (r *ResourceHistory) BeforeCreate(db *gorm.DB) error {
	var err error
	if r.ID == uuid.Nil {
		r.ID, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("failed to generate uuid: %w", err)
		}
	}
	return nil
}
