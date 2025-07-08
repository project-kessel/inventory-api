package model

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CommonRepresentation struct {
	BaseRepresentation
	ID                         uuid.UUID `gorm:"type:text;column:id;primary_key"`
	ResourceType               string    `gorm:"size:128;column:resource_type"`
	Version                    uint      `gorm:"type:bigint;column:version;primary_key;check:version > 0"`
	ReportedByReporterType     string    `gorm:"size:128;column:reported_by_reporter_type"`
	ReportedByReporterInstance string    `gorm:"size:128;column:reported_by_reporter_instance"`
}

func (CommonRepresentation) TableName() string {
	return "common_representation"
}

// BeforeCreate hook generates UUID programmatically for cross-database compatibility.
// This approach is used instead of database-specific defaults (like PostgreSQL's gen_random_uuid())
// to ensure the model works correctly with both SQLite (used in tests) and PostgreSQL (production).
// SQLite doesn't support UUID type or gen_random_uuid() function, so we handle UUID generation
// in Go code using GORM hooks, following the same pattern as other models in the codebase.
func (cr *CommonRepresentation) BeforeCreate(db *gorm.DB) error {
	var err error
	if cr.ID == uuid.Nil {
		cr.ID, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("failed to generate uuid: %w", err)
		}
	}
	return nil
}
