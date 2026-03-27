package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
)

type MetricsSummary struct {
	ID          uuid.UUID           `gorm:"type:uuid;primaryKey;not null"`
	CollectedAt time.Time           `gorm:"not null;index:idx_metrics_summary_collected_at"`
	Metrics     internal.JsonObject `gorm:"type:jsonb;not null"`
}
