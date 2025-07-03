package v1beta2

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
)

// FakeResourceWithReferencesRepository is an in-memory fake implementation for testing
type FakeResourceWithReferencesRepository struct {
	mu            sync.RWMutex
	resources     map[uuid.UUID]*v1beta2.Resource
	references    map[uuid.UUID][]*v1beta2.RepresentationReference // resourceID -> references
	creationOrder []uuid.UUID                                      // Track creation order for consistent GetAllResourceAggregates
}

// NewFakeResourceWithReferencesRepository creates a new fake repository
func NewFakeResourceWithReferencesRepository() *FakeResourceWithReferencesRepository {
	return &FakeResourceWithReferencesRepository{
		resources:     make(map[uuid.UUID]*v1beta2.Resource),
		references:    make(map[uuid.UUID][]*v1beta2.RepresentationReference),
		creationOrder: []uuid.UUID{},
	}
}

// Create stores a new ResourceWithReferences
func (f *FakeResourceWithReferencesRepository) Create(ctx context.Context, aggregate *v1beta2.ResourceWithReferences) (*v1beta2.ResourceWithReferences, error) {
	if aggregate == nil {
		return nil, fmt.Errorf("aggregate cannot be nil")
	}
	if aggregate.Resource == nil {
		return nil, fmt.Errorf("resource cannot be nil")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Generate ID for resource if not set
	if aggregate.Resource.ID == uuid.Nil {
		var err error
		aggregate.Resource.ID, err = uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("failed to generate uuid for resource: %w", err)
		}
	}

	resourceID := aggregate.Resource.ID

	// Check if resource already exists
	if _, exists := f.resources[resourceID]; exists {
		return nil, fmt.Errorf("resource with ID %s already exists", resourceID)
	}

	// Make copies to avoid external modifications
	resourceCopy := *aggregate.Resource
	f.resources[resourceID] = &resourceCopy

	// Store references
	var refCopies []*v1beta2.RepresentationReference
	for _, ref := range aggregate.RepresentationReferences {
		if ref != nil {
			refCopy := *ref
			refCopy.ResourceID = resourceID // Ensure reference points to resource
			refCopies = append(refCopies, &refCopy)
		}
	}
	f.references[resourceID] = refCopies

	// Return the result
	result := &v1beta2.ResourceWithReferences{
		Resource:                 &resourceCopy,
		RepresentationReferences: refCopies,
	}

	f.creationOrder = append(f.creationOrder, resourceID)

	return result, nil
}

// CreateWithTx creates a new ResourceWithReferences (fake implementation ignores transaction)
func (f *FakeResourceWithReferencesRepository) CreateWithTx(ctx context.Context, db interface{}, aggregate *v1beta2.ResourceWithReferences) (*v1beta2.ResourceWithReferences, error) {
	// For fake implementation, just delegate to Create (ignore transaction)
	return f.Create(ctx, aggregate)
}

// FindAllReferencesByReporterRepresentationId finds all representation references for the same resource_id
// based on the reporter's representation identifier
func (f *FakeResourceWithReferencesRepository) FindAllReferencesByReporterRepresentationId(ctx context.Context, reporterId v1beta2.ReporterRepresentationId) ([]*v1beta2.RepresentationReference, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var matchingRefs []*v1beta2.RepresentationReference
	var resourceIDsFound []uuid.UUID

	// Find resource IDs that match the input criteria
	for resourceID, refs := range f.references {
		for _, ref := range refs {
			if ref.LocalResourceID == reporterId.LocalResourceID &&
				ref.ReporterType == reporterId.ReporterType &&
				ref.ResourceType == reporterId.ResourceType &&
				ref.ReporterInstanceID == reporterId.ReporterInstanceID {
				resourceIDsFound = append(resourceIDsFound, resourceID)
				break
			}
		}
	}

	// Collect all references for those resource IDs
	for _, resourceID := range resourceIDsFound {
		if refs, exists := f.references[resourceID]; exists {
			for _, ref := range refs {
				refCopy := *ref
				matchingRefs = append(matchingRefs, &refCopy)
			}
		}
	}

	return matchingRefs, nil
}

// UpdateConsistencyToken updates the consistency_token field for a Resource by ID
func (f *FakeResourceWithReferencesRepository) UpdateConsistencyToken(ctx context.Context, resourceID uuid.UUID, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	resource, exists := f.resources[resourceID]
	if !exists {
		return fmt.Errorf("resource with ID %s not found", resourceID)
	}

	// Update the consistency token
	resource.ConsistencyToken = token
	return nil
}

// UpdateRepresentationVersion updates the representation_version field for RepresentationReferences
// based on the provided filter criteria. Returns the number of rows affected.
func (f *FakeResourceWithReferencesRepository) UpdateRepresentationVersion(ctx context.Context, filter v1beta2.RepresentationVersionUpdateFilter, newVersion int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if resource exists
	if _, exists := f.resources[filter.ResourceID]; !exists {
		return 0, fmt.Errorf("resource with ID %s not found", filter.ResourceID)
	}

	refs, exists := f.references[filter.ResourceID]
	if !exists {
		// No references for this resource
		return 0, nil
	}

	var updatedCount int64
	for _, ref := range refs {
		// Check if this reference matches the filter criteria
		matches := true

		// Check reporter_type filter
		if filter.ReporterType != nil && ref.ReporterType != *filter.ReporterType {
			matches = false
		}

		// Check local_resource_id filter
		if filter.LocalResourceID != nil && ref.LocalResourceID != *filter.LocalResourceID {
			matches = false
		}

		// Update if it matches
		if matches {
			ref.RepresentationVersion = newVersion
			updatedCount++
		}
	}

	return updatedCount, nil
}

// UpdateCommonRepresentationVersion updates the representation version for "inventory" reporter type references
// This is a convenience method for the common case of updating inventory (common) representations
func (f *FakeResourceWithReferencesRepository) UpdateCommonRepresentationVersion(ctx context.Context, resourceID uuid.UUID, newVersion int) (int64, error) {
	inventoryReporter := "inventory"
	filter := v1beta2.RepresentationVersionUpdateFilter{
		ResourceID:   resourceID,
		ReporterType: &inventoryReporter,
		// LocalResourceID is nil, so it updates all inventory references for the resource
	}
	return f.UpdateRepresentationVersion(ctx, filter, newVersion)
}

// UpdateReporterRepresentationVersion updates the representation version for a specific reporter and local resource
// This is a convenience method for updating a specific reporter representation
func (f *FakeResourceWithReferencesRepository) UpdateReporterRepresentationVersion(ctx context.Context, resourceID uuid.UUID, reporterType string, localResourceID string, newVersion int) (int64, error) {
	filter := v1beta2.RepresentationVersionUpdateFilter{
		ResourceID:      resourceID,
		ReporterType:    &reporterType,
		LocalResourceID: &localResourceID,
	}
	return f.UpdateRepresentationVersion(ctx, filter, newVersion)
}

// UpdateCommonRepresentationVersionWithTx updates the representation version for "inventory" reporter type using the provided transaction
func (f *FakeResourceWithReferencesRepository) UpdateCommonRepresentationVersionWithTx(ctx context.Context, tx interface{}, resourceID uuid.UUID, newVersion int) (int64, error) {
	// For fake implementation, ignore transaction and delegate to non-tx method
	return f.UpdateCommonRepresentationVersion(ctx, resourceID, newVersion)
}

// UpdateReporterRepresentationVersionWithTx updates the representation version for a specific reporter using the provided transaction
func (f *FakeResourceWithReferencesRepository) UpdateReporterRepresentationVersionWithTx(ctx context.Context, tx interface{}, resourceID uuid.UUID, reporterType string, localResourceID string, newVersion int) (int64, error) {
	// For fake implementation, ignore transaction and delegate to non-tx method
	return f.UpdateReporterRepresentationVersion(ctx, resourceID, reporterType, localResourceID, newVersion)
}

// Reset clears all data (useful for test cleanup)
func (f *FakeResourceWithReferencesRepository) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resources = make(map[uuid.UUID]*v1beta2.Resource)
	f.references = make(map[uuid.UUID][]*v1beta2.RepresentationReference)
	f.creationOrder = []uuid.UUID{}
}

// Count returns the number of stored resources
func (f *FakeResourceWithReferencesRepository) Count() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.resources)
}

// GetAllResources returns all stored resources (useful for testing)
func (f *FakeResourceWithReferencesRepository) GetAllResources() []*v1beta2.Resource {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*v1beta2.Resource, 0, len(f.resources))
	for _, resource := range f.resources {
		copy := *resource
		result = append(result, &copy)
	}
	return result
}

// GetAllResourceAggregates returns all stored resource aggregates (useful for testing)
func (f *FakeResourceWithReferencesRepository) GetAllResourceAggregates() []*v1beta2.ResourceWithReferences {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*v1beta2.ResourceWithReferences, 0, len(f.resources))
	for _, resourceID := range f.creationOrder {
		resource := f.resources[resourceID]
		resourceCopy := *resource

		var refCopies []*v1beta2.RepresentationReference
		if refs, exists := f.references[resourceID]; exists {
			for _, ref := range refs {
				refCopy := *ref
				refCopies = append(refCopies, &refCopy)
			}
		}

		aggregate := &v1beta2.ResourceWithReferences{
			Resource:                 &resourceCopy,
			RepresentationReferences: refCopies,
		}
		result = append(result, aggregate)
	}
	return result
}
