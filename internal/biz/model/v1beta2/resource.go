package v1beta2

import "github.com/google/uuid"

type ResourceOption1 struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Type string    `gorm:"size:128"`
}

func (ResourceOption1) TableName() string {
	return "resource"
}
