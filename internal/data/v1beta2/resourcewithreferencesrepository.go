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
func (r *ResourceWithReferencesRepository) CreateWithTx(ctx context.Context, db *gorm.DB, resourceWithReferences *v1beta2.ResourceWithReferences) (*v1beta2.ResourceWithReferences, error) {
	if resourceWithReferences == nil {
		return nil, fmt.Errorf("resourceWithReferences cannot be nil")
	}
	if resourceWithReferences.Resource == nil {
		return nil, fmt.Errorf("resource cannot be nil")
	}

	err := WithTx(ctx, db, func(tx *gorm.DB) error {
		// Generate ID for resource if not set
		if resourceWithReferences.Resource.ID == uuid.Nil {
			var err error
			resourceWithReferences.Resource.ID, err = uuid.NewV7()
			if err != nil {
				return fmt.Errorf("failed to generate uuid for resource: %w", err)
			}
		}

		// Create the resource
		if err := tx.Create(resourceWithReferences.Resource).Error; err != nil {
			return fmt.Errorf("failed to create resource: %w", err)
		}

		// Create representation references if they exist
		for _, ref := range resourceWithReferences.RepresentationReferences {
			if ref != nil {
				// Set the resource ID reference
				ref.ResourceID = resourceWithReferences.Resource.ID
				if err := tx.Create(ref).Error; err != nil {
					return fmt.Errorf("failed to create representation reference: %w", err)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resourceWithReferences, nil
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

// UpdateConsistencyToken updates the consistency_token field for a Resource by ID
func (r *ResourceWithReferencesRepository) UpdateConsistencyToken(ctx context.Context, resourceID uuid.UUID, token string) error {
	result := r.DB.WithContext(ctx).Model(&v1beta2.Resource{}).Where("id = ?", resourceID).Update("consistency_token", token)
	if result.Error != nil {
		return fmt.Errorf("failed to update consistency token: %w", result.Error)
	}
	return nil
}

// UpdateRepresentationVersion updates the representation_version field for RepresentationReferences
// based on the provided filter criteria. Returns the number of rows affected.
func (r *ResourceWithReferencesRepository) UpdateRepresentationVersion(ctx context.Context, filter v1beta2.RepresentationVersionUpdateFilter, newVersion int) (int64, error) {
	query := r.DB.WithContext(ctx).Model(&v1beta2.RepresentationReference{}).
		Where("resource_id = ?", filter.ResourceID)

	// Add optional reporter_type filter
	if filter.ReporterType != nil {
		query = query.Where("reporter_type = ?", *filter.ReporterType)
	}

	// Add optional local_resource_id filter
	if filter.LocalResourceID != nil {
		query = query.Where("local_resource_id = ?", *filter.LocalResourceID)
	}

	result := query.Update("representation_version", newVersion)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to update representation version: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// UpdateCommonRepresentationVersion updates the representation version for "inventory" reporter type references
// This is a convenience method for the common case of updating inventory (common) representations
func (r *ResourceWithReferencesRepository) UpdateCommonRepresentationVersion(ctx context.Context, resourceID uuid.UUID, newVersion int) (int64, error) {
	inventoryReporter := "inventory"
	filter := v1beta2.RepresentationVersionUpdateFilter{
		ResourceID:   resourceID,
		ReporterType: &inventoryReporter,
		// LocalResourceID is nil, so it updates all inventory references for the resource
	}
	return r.UpdateRepresentationVersion(ctx, filter, newVersion)
}

// UpdateReporterRepresentationVersion updates the representation version for a specific reporter and local resource
// This is a convenience method for updating a specific reporter representation
func (r *ResourceWithReferencesRepository) UpdateReporterRepresentationVersion(ctx context.Context, resourceID uuid.UUID, reporterType string, localResourceID string, newVersion int) (int64, error) {
	filter := v1beta2.RepresentationVersionUpdateFilter{
		ResourceID:      resourceID,
		ReporterType:    &reporterType,
		LocalResourceID: &localResourceID,
	}
	return r.UpdateRepresentationVersion(ctx, filter, newVersion)
}
