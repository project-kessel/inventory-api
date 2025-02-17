package model

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InventoryResource struct {
	ID uuid.UUID `gorm:"type:uuid;primarykey"`
}

func (r *InventoryResource) BeforeCreate(db *gorm.DB) error {
	var err error
	if r.ID == uuid.Nil {
		r.ID, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("failed to generate uuid: %w", err)
		}
	}
	return nil
}
