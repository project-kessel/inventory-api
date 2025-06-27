package v1beta2

import (
	"context"
	"fmt"

	"github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
	"gorm.io/gorm"
)

type CommonRepresentationRepository struct {
	DB *gorm.DB
}

func NewCommonRepresentationRepository(db *gorm.DB) *CommonRepresentationRepository {
	return &CommonRepresentationRepository{
		DB: db,
	}
}

// Create creates a new CommonRepresentation in the database
func (r *CommonRepresentationRepository) Create(ctx context.Context, representation *v1beta2.CommonRepresentation) (*v1beta2.CommonRepresentation, error) {
	return r.CreateWithTx(ctx, r.DB, representation)
}

// CreateWithTx creates a new CommonRepresentation using the provided database connection
// If the provided db is already a transaction, it will use it; otherwise it will create a new transaction
func (r *CommonRepresentationRepository) CreateWithTx(ctx context.Context, db *gorm.DB, representation *v1beta2.CommonRepresentation) (*v1beta2.CommonRepresentation, error) {
	if representation == nil {
		return nil, fmt.Errorf("representation cannot be nil")
	}

	err := WithTx(ctx, db, func(tx *gorm.DB) error {
		return tx.Create(representation).Error
	})
	if err != nil {
		return nil, err
	}
	return representation, nil
}
