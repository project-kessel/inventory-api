package resources

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type Repo struct {
	DB *gorm.DB
}

func New(db *gorm.DB) *Repo {
	return &Repo{
		DB: db,
	}
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (*model.InventoryResource, error) {
	inventoryResource := model.InventoryResource{}
	if err := r.DB.Session(&gorm.Session{}).First(&inventoryResource, id).Error; err != nil {
		return nil, err
	}

	return &inventoryResource, nil
}
