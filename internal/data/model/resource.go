package model

import (
	"time"

	"github.com/google/uuid"
)

type Resource struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	Type             string    `gorm:"size:128;not null;"`
	CommonVersion    uint      `gorm:"type:bigint;check:common_version >= 0"`
	ConsistencyToken string    `gorm:"size:1024;column:ktn;"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// TableName gives GORM the exact table name to use.Doing this for now until we can decouple from legacy_model.Resource and can reclaim the "resources" table name
func (Resource) TableName() string {
	return "resource"
}
