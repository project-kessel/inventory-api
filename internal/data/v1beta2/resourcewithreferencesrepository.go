package v1beta2

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
)

type ResourceWithReferencesRepository struct {
	DB *gorm.DB
}

func NewResourceWithReferencesRepository(db *gorm.DB) *ResourceWithReferencesRepository {
	return &ResourceWithReferencesRepository{
		DB: db,
	}
}

// Create creates a new Resource with its RepresentationReferences in a transaction
func (r *ResourceWithReferencesRepository) Create(ctx context.Context, aggregate *v1beta2.ResourceWithReferences) (*v1beta2.ResourceWithReferences, error) {
	return r.CreateWithTx(ctx, r.DB, aggregate)
}

// CreateWithTx creates a new Resource with its RepresentationReferences using the provided database connection
// If the provided db is already a transaction, it will use it; otherwise it will create a new transaction
func (r *ResourceWithReferencesRepository) CreateWithTx(ctx context.Context, db *gorm.DB, aggregate *v1beta2.ResourceWithReferences) (*v1beta2.ResourceWithReferences, error) {
	if aggregate == nil {
		return nil, fmt.Errorf("aggregate cannot be nil")
	}
	if aggregate.Resource == nil {
		return nil, fmt.Errorf("resource cannot be nil")
	}

	// Check if we're already in a transaction by checking the CommitOrRollback method
	// If db is already a transaction, use it directly; otherwise start a new transaction
	var result *v1beta2.ResourceWithReferences

	createFunc := func(tx *gorm.DB) error {
		// Generate ID for resource if not set
		if aggregate.Resource.ID == uuid.Nil {
			var err error
			aggregate.Resource.ID, err = uuid.NewV7()
			if err != nil {
				return fmt.Errorf("failed to generate uuid for resource: %w", err)
			}
		}

		// Create the resource
		if err := tx.Create(aggregate.Resource).Error; err != nil {
			return fmt.Errorf("failed to create resource: %w", err)
		}

		// Create representation references if they exist
		for _, ref := range aggregate.RepresentationReferences {
			if ref != nil {
				// Set the resource ID reference
				ref.ResourceID = aggregate.Resource.ID
				if err := tx.Create(ref).Error; err != nil {
					return fmt.Errorf("failed to create representation reference: %w", err)
				}
			}
		}

		result = aggregate
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

// FindAllReferencesByReporterRepresentationId finds all representation references for the same resource_id
// based on the reporter's representation identifier
func (r *ResourceWithReferencesRepository) FindAllReferencesByReporterRepresentationId(ctx context.Context, reporterId v1beta2.ReporterRepresentationId) ([]*v1beta2.RepresentationReference, error) {
	var refs []*v1beta2.RepresentationReference

	query := r.DB.WithContext(ctx).
		Table("representation_references AS r1").
		Joins("JOIN representation_references AS r2 ON r1.resource_id = r2.resource_id").
		Where("r1.local_resource_id = ? AND r1.reporter_type = ? AND r1.resource_type = ? AND r1.reporter_instance_id = ?",
			reporterId.LocalResourceID, reporterId.ReporterType, reporterId.ResourceType, reporterId.ReporterInstanceID).
		Select("r2.resource_id, r2.local_resource_id, r2.reporter_type, r2.resource_type, r2.reporter_instance_id, r2.representation_version, r2.generation, r2.tombstone")

	err := query.Find(&refs).Error
	return refs, err
}
