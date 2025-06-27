package v1beta2

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
)

// FakeCommonRepresentationRepository is an in-memory fake implementation for testing
type FakeCommonRepresentationRepository struct {
	mu   sync.RWMutex
	data map[uuid.UUID]*v1beta2.CommonRepresentation
}

// NewFakeCommonRepresentationRepository creates a new fake repository
func NewFakeCommonRepresentationRepository() *FakeCommonRepresentationRepository {
	return &FakeCommonRepresentationRepository{
		data: make(map[uuid.UUID]*v1beta2.CommonRepresentation),
	}
}

// Create stores a new CommonRepresentation
func (f *FakeCommonRepresentationRepository) Create(ctx context.Context, representation *v1beta2.CommonRepresentation) (*v1beta2.CommonRepresentation, error) {
	if representation == nil {
		return nil, fmt.Errorf("representation cannot be nil")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Create a composite key for identification
	key := fmt.Sprintf("%s_%d", representation.LocalResourceID, representation.Version)

	// Check if already exists
	for _, existing := range f.data {
		existingKey := fmt.Sprintf("%s_%d", existing.LocalResourceID, existing.Version)
		if existingKey == key {
			return nil, fmt.Errorf("representation with key %s already exists", key)
		}
	}

	// Make a copy to avoid external modifications
	copy := *representation
	// Use the composite key as a pseudo-ID for storage
	pseudoID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(key))
	f.data[pseudoID] = &copy

	return &copy, nil
}

// Reset clears all data (useful for test cleanup)
func (f *FakeCommonRepresentationRepository) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data = make(map[uuid.UUID]*v1beta2.CommonRepresentation)
}

// Count returns the number of stored representations
func (f *FakeCommonRepresentationRepository) Count() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.data)
}

// GetAll returns all stored representations (useful for testing)
func (f *FakeCommonRepresentationRepository) GetAll() []*v1beta2.CommonRepresentation {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*v1beta2.CommonRepresentation, 0, len(f.data))
	for _, rep := range f.data {
		copy := *rep
		result = append(result, &copy)
	}
	return result
}
