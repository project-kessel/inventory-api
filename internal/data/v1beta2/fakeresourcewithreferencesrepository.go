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
	mu         sync.RWMutex
	resources  map[uuid.UUID]*v1beta2.Resource
	references map[uuid.UUID][]*v1beta2.RepresentationReference // resourceID -> references
}

// NewFakeResourceWithReferencesRepository creates a new fake repository
func NewFakeResourceWithReferencesRepository() *FakeResourceWithReferencesRepository {
	return &FakeResourceWithReferencesRepository{
		resources:  make(map[uuid.UUID]*v1beta2.Resource),
		references: make(map[uuid.UUID][]*v1beta2.RepresentationReference),
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

	return result, nil
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

// Reset clears all data (useful for test cleanup)
func (f *FakeResourceWithReferencesRepository) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resources = make(map[uuid.UUID]*v1beta2.Resource)
	f.references = make(map[uuid.UUID][]*v1beta2.RepresentationReference)
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

// GetAllAggregates returns all stored resource aggregates (useful for testing)
func (f *FakeResourceWithReferencesRepository) GetAllAggregates() []*v1beta2.ResourceWithReferences {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*v1beta2.ResourceWithReferences, 0, len(f.resources))
	for resourceID, resource := range f.resources {
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
