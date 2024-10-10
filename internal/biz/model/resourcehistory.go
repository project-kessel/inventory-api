package model

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
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

	ResourceId uint64 `gorm:"index"`
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
		break
	}
	return nil
}

func (*ResourceHistory) TableName() string {
	return "resource_history"
}
