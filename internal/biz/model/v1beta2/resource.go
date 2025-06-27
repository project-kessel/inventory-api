package v1beta2

import "github.com/google/uuid"

type Resource struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Type string    `gorm:"size:128"`
}

func (Resource) TableName() string {
	return "resource"
}
