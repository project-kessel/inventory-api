package resources

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
)

type Repo struct {
	DB *gorm.DB
}

func New(db *gorm.DB) *Repo {
	return &Repo{
		DB: db,
	}
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (*model_legacy.InventoryResource, error) {
	inventoryResource := model_legacy.InventoryResource{}
	if err := r.DB.Session(&gorm.Session{}).First(&inventoryResource, id).Error; err != nil {
		return nil, err
	}

	return &inventoryResource, nil
}
