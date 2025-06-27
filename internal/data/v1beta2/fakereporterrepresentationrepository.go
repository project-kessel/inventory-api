package v1beta2

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
)

// FakeReporterRepresentationRepository is an in-memory fake implementation for testing
type FakeReporterRepresentationRepository struct {
	mu   sync.RWMutex
	data map[uuid.UUID]*v1beta2.ReporterRepresentation
}

// NewFakeReporterRepresentationRepository creates a new fake repository
func NewFakeReporterRepresentationRepository() *FakeReporterRepresentationRepository {
	return &FakeReporterRepresentationRepository{
		data: make(map[uuid.UUID]*v1beta2.ReporterRepresentation),
	}
}

// Create stores a new ReporterRepresentation
func (f *FakeReporterRepresentationRepository) Create(ctx context.Context, representation *v1beta2.ReporterRepresentation) (*v1beta2.ReporterRepresentation, error) {
	if representation == nil {
		return nil, fmt.Errorf("representation cannot be nil")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Create a composite key for identification
	key := fmt.Sprintf("%s_%s_%s_%d_%s_%d",
		representation.LocalResourceID,
		representation.ReporterType,
		representation.ResourceType,
		representation.Version,
		representation.ReporterInstanceID,
		representation.Generation)

	// Check if already exists
	for _, existing := range f.data {
		existingKey := fmt.Sprintf("%s_%s_%s_%d_%s_%d",
			existing.LocalResourceID,
			existing.ReporterType,
			existing.ResourceType,
			existing.Version,
			existing.ReporterInstanceID,
			existing.Generation)
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
func (f *FakeReporterRepresentationRepository) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data = make(map[uuid.UUID]*v1beta2.ReporterRepresentation)
}

// Count returns the number of stored representations
func (f *FakeReporterRepresentationRepository) Count() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.data)
}

// GetAll returns all stored representations (useful for testing)
func (f *FakeReporterRepresentationRepository) GetAll() []*v1beta2.ReporterRepresentation {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*v1beta2.ReporterRepresentation, 0, len(f.data))
	for _, rep := range f.data {
		copy := *rep
		result = append(result, &copy)
	}
	return result
}
