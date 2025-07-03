package v1beta2

import "github.com/google/uuid"

type Resource struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	Type             string    `gorm:"size:128"`
	ConsistencyToken string    `gorm:"column:consistency_token"`
}

func (Resource) TableName() string {
	return "resource"
}
