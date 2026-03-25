package schema

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MetricsSummary struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;not null"`
	CollectedAt time.Time      `gorm:"not null;index:idx_metrics_summary_collected_at"`
	Metrics     map[string]any `gorm:"type:jsonb;not null"`
}

func MetricsSummaryMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260325120000",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&MetricsSummary{})
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
