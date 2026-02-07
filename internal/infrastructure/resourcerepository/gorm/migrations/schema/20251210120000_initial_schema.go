package schema

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Package-scoped types freeze the baseline schema for idempotent initial migration
type OutboxEvent struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;not null"`
	AggregateType string         `gorm:"column:aggregatetype;type:varchar(255);not null"`
	AggregateID   string         `gorm:"column:aggregateid;type:varchar(255);not null"`
	Operation     string         `gorm:"type:varchar(255);not null"`
	TxId          string         `gorm:"column:txid;type:varchar(255)"`
	Payload       map[string]any `gorm:"type:jsonb"`
}

type CommonRepresentation struct {
	Data                       map[string]any `gorm:"type:jsonb"`
	ResourceId                 uuid.UUID      `gorm:"type:uuid;primaryKey"`
	Version                    uint           `gorm:"type:bigint;primaryKey;check:version >= 0"`
	ReportedByReporterType     string         `gorm:"size:128"`
	ReportedByReporterInstance string         `gorm:"size:128"`
	TransactionId              string         `gorm:"size:128;index:ux_common_reps_txid_nn,where:transaction_id IS NOT NULL AND transaction_id != '',unique"`
	CreatedAt                  time.Time
}

type ReporterResource struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`

	LocalResourceID    string `gorm:"size:256;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:1;not null"`
	ReporterType       string `gorm:"size:128;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:2;not null"`
	ResourceType       string `gorm:"size:128;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:3;not null"`
	ReporterInstanceID string `gorm:"size:256;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:4;not null"`

	ResourceID  uuid.UUID `gorm:"index:reporter_resource_resource_id_idx;type:uuid;not null"`
	APIHref     string    `gorm:"size:512;not null"`
	ConsoleHref string    `gorm:"size:512"`

	RepresentationVersion uint `gorm:"index:reporter_resource_key_idx,unique;not null"`
	Generation            uint `gorm:"index:reporter_resource_key_idx,unique;not null"`
	Tombstone             bool `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ReporterRepresentation struct {
	Data               map[string]any `gorm:"type:jsonb"`
	ReporterResourceID uuid.UUID      `gorm:"size:128;primaryKey"`
	Version            uint           `gorm:"type:bigint;primaryKey;check:version >= 0"`
	Generation         uint           `gorm:"type:bigint;primaryKey;check:generation >= 0"`
	ReporterVersion    *string        `gorm:"size:128"`
	CommonVersion      uint           `gorm:"type:bigint;check:common_version >= 0"`
	TransactionId      string         `gorm:"size:128;index:ux_reporter_reps_txid_nn,where:transaction_id IS NOT NULL AND transaction_id != '',unique"`
	Tombstone          bool           `gorm:"not null"`
	CreatedAt          time.Time
	ReporterResource   ReporterResource `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:ReporterResourceID;references:ID"`
}

type Resource struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	Type             string    `gorm:"size:128;not null;"`
	CommonVersion    uint      `gorm:"type:bigint;check:common_version >= 0"`
	ConsistencyToken string    `gorm:"size:1024;column:ktn;"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (Resource) TableName() string { return "resource" }

func InitialSchema() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20251210120000",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&Resource{},
				&ReporterResource{},
				&ReporterRepresentation{},
				&CommonRepresentation{},
				&OutboxEvent{},
			)
		},
		Rollback: func(tx *gorm.DB) error {
			// No rollback
			return nil
		},
	}
}
