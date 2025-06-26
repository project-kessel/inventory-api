package v1beta2

import (
	"context"
	"fmt"

	"github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
	"gorm.io/gorm"
)

type ReporterRepresentationRepository struct {
	DB *gorm.DB
}

func NewReporterRepresentationRepository(db *gorm.DB) *ReporterRepresentationRepository {
	return &ReporterRepresentationRepository{
		DB: db,
	}
}

// Create creates a new ReporterRepresentation in the database
func (r *ReporterRepresentationRepository) Create(ctx context.Context, representation *v1beta2.ReporterRepresentation) (*v1beta2.ReporterRepresentation, error) {
	return r.CreateWithTx(ctx, r.DB, representation)
}

// CreateWithTx creates a new ReporterRepresentation using the provided database connection
// If the provided db is already a transaction, it will use it; otherwise it will create a new transaction
func (r *ReporterRepresentationRepository) CreateWithTx(ctx context.Context, db *gorm.DB, representation *v1beta2.ReporterRepresentation) (*v1beta2.ReporterRepresentation, error) {
	if representation == nil {
		return nil, fmt.Errorf("representation cannot be nil")
	}

	var result *v1beta2.ReporterRepresentation

	createFunc := func(tx *gorm.DB) error {
		if err := tx.Create(representation).Error; err != nil {
			return err
		}

		result = representation
		return nil
	}

	// Check if we're already in a transaction
	if isInTransaction(db) {
		// We're already in a transaction, use it directly
		err := createFunc(db.WithContext(ctx))
		if err != nil {
			return nil, err
		}
	} else {
		// Start a new transaction
		err := db.WithContext(ctx).Transaction(createFunc)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}
