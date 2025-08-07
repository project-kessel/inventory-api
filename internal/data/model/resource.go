package model

import (
	"time"

	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

type Resource struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	Type             string    `gorm:"size:128;not null;"`
	CommonVersion    uint      `gorm:"type:bigint;check:common_version >= 0"`
	ConsistencyToken string    `gorm:"size:1024;column:ktn;"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (Resource) TableName() string {
	return "resource"
}

// SerializeToSnapshot converts GORM Resource to snapshot type - direct initialization without validation
func (r Resource) SerializeToSnapshot() bizmodel.ResourceSnapshot {
	return bizmodel.ResourceSnapshot{
		ID:               r.ID,
		Type:             r.Type,
		CommonVersion:    r.CommonVersion,
		ConsistencyToken: r.ConsistencyToken,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

// DeserializeFromSnapshot creates GORM Resource from snapshot - direct initialization without validation
func DeserializeResourceFromSnapshot(snapshot bizmodel.ResourceSnapshot) Resource {
	return Resource{
		ID:               snapshot.ID,
		Type:             snapshot.Type,
		CommonVersion:    snapshot.CommonVersion,
		ConsistencyToken: snapshot.ConsistencyToken,
		CreatedAt:        snapshot.CreatedAt,
		UpdatedAt:        snapshot.UpdatedAt,
	}
}
