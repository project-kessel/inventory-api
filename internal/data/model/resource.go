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

func NewResource(
	id uuid.UUID,
	resourceType string,
	commonVersion uint,
) (*Resource, error) {
	r := &Resource{
		ID:            id,
		Type:          resourceType,
		CommonVersion: commonVersion,
	}

	if err := validateResource(r); err != nil {
		return nil, err
	}

	return r, nil
}

func validateResource(r *Resource) error {
	return aggregateErrors(
		validateUUIDRequired("ID", r.ID),
		validateStringRequired("Type", r.Type),
		validateMaxLength("Type", r.Type, MaxResourceTypeLength),
		validateMinValueUint("CommonVersion", r.CommonVersion, 0),
	)
}

func (Resource) TableName() string {
	return "resource"
}
