package data

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"gorm.io/gorm"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/biz/usecase"
)

type fakeResourceRepository struct {
	mu                      sync.RWMutex
	resourcesByPrimaryKey   map[uuid.UUID]*storedResource // keyed by primary key (ResourceID) - simulates database primary storage
	resourcesByCompositeKey map[string]uuid.UUID          // composite key -> primary key mapping for unique constraint
}

type storedResource struct {
	resourceID            uuid.UUID
	resourceType          string
	commonVersion         uint
	reporterResourceID    uuid.UUID
	localResourceID       string
	reporterType          string
	reporterInstanceID    string
	representationVersion uint
	generation            uint
	tombstone             bool
}

func NewFakeResourceRepository() ResourceRepository {
	return &fakeResourceRepository{
		resourcesByPrimaryKey:   make(map[uuid.UUID]*storedResource),
		resourcesByCompositeKey: make(map[string]uuid.UUID),
	}
}

func (f *fakeResourceRepository) NextResourceId() (bizmodel.ResourceId, error) {
	uuidV7, err := uuid.NewV7()
	if err != nil {
		return bizmodel.ResourceId{}, err
	}

	return bizmodel.NewResourceId(uuidV7)
}

func (f *fakeResourceRepository) NextReporterResourceId() (bizmodel.ReporterResourceId, error) {
	uuidV7, err := uuid.NewV7()
	if err != nil {
		return bizmodel.ReporterResourceId{}, err
	}

	return bizmodel.NewReporterResourceId(uuidV7)
}

func (f *fakeResourceRepository) Save(tx *gorm.DB, resource bizmodel.Resource, operationType model_legacy.EventOperationType, txid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	resourceSnapshot, reporterResourceSnapshot, reporterRepresentationSnapshot, commonRepresentationSnapshot, err := resource.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	// In fake implementation, we don't actually store representations but we should acknowledge them
	_ = reporterRepresentationSnapshot
	_ = commonRepresentationSnapshot

	// Create the composite key that matches the unique constraint:
	// (LocalResourceID, ReporterType, ResourceType, ReporterInstanceID, RepresentationVersion, Generation)
	compositeKey := f.makeCompositeKey(
		reporterResourceSnapshot.ReporterResourceKey.LocalResourceID,
		reporterResourceSnapshot.ReporterResourceKey.ReporterType,
		reporterResourceSnapshot.ReporterResourceKey.ResourceType,
		reporterResourceSnapshot.ReporterResourceKey.ReporterInstanceID,
		reporterResourceSnapshot.RepresentationVersion,
		reporterResourceSnapshot.Generation,
	)

	// Simulate database Save() behavior: upsert by primary key (ReporterResourceID)
	reporterResourcePrimaryKey := reporterResourceSnapshot.ID

	// Check if this is an update to existing resource (same primary key)
	if existingResource, exists := f.resourcesByPrimaryKey[reporterResourcePrimaryKey]; exists {
		// This is an update - remove old composite key mapping
		oldCompositeKey := f.makeCompositeKey(
			existingResource.localResourceID,
			existingResource.reporterType,
			existingResource.resourceType,
			existingResource.reporterInstanceID,
			existingResource.representationVersion,
			existingResource.generation,
		)
		delete(f.resourcesByCompositeKey, oldCompositeKey)
	} else {
		// This is a new resource - check for unique constraint violation
		if existingPrimaryKey, exists := f.resourcesByCompositeKey[compositeKey]; exists {
			return fmt.Errorf("duplicate key violation: reporter_resource_key_idx unique constraint failed for key: %s (conflicts with existing resource: %s)", compositeKey, existingPrimaryKey)
		}
	}

	stored := &storedResource{
		resourceID:            resourceSnapshot.ID,
		resourceType:          resourceSnapshot.Type,
		commonVersion:         resourceSnapshot.CommonVersion,
		reporterResourceID:    reporterResourceSnapshot.ID,
		localResourceID:       reporterResourceSnapshot.ReporterResourceKey.LocalResourceID,
		reporterType:          reporterResourceSnapshot.ReporterResourceKey.ReporterType,
		reporterInstanceID:    reporterResourceSnapshot.ReporterResourceKey.ReporterInstanceID,
		representationVersion: reporterResourceSnapshot.RepresentationVersion,
		generation:            reporterResourceSnapshot.Generation,
		tombstone:             reporterResourceSnapshot.Tombstone,
	}

	// Store by primary key (simulates database primary storage)
	f.resourcesByPrimaryKey[reporterResourcePrimaryKey] = stored
	// Store composite key mapping (simulates unique constraint)
	f.resourcesByCompositeKey[compositeKey] = reporterResourcePrimaryKey
	return nil
}

func (f *fakeResourceRepository) FindResourceByKeys(tx *gorm.DB, key bizmodel.ReporterResourceKey) (*bizmodel.Resource, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Note: This fake implementation doesn't use the transaction parameter,
	// but we acknowledge it for consistency with the real implementation.
	// In a real scenario, tx would be used for database operations.
	_ = tx // Explicitly acknowledge the transaction parameter

	// Match the real repository's behavior: if reporterInstanceId is empty,
	// find any resource that matches the other key components
	searchReporterInstanceId := key.ReporterInstanceId().Serialize()

	// Find the latest version for the given natural key
	var latestResource *storedResource
	for _, stored := range f.resourcesByPrimaryKey {
		if strings.EqualFold(stored.localResourceID, key.LocalResourceId().Serialize()) &&
			strings.EqualFold(stored.resourceType, key.ResourceType().Serialize()) &&
			strings.EqualFold(stored.reporterType, key.ReporterType().Serialize()) {

			// If search key has empty reporterInstanceId, match any stored resource
			// If search key has reporterInstanceId, it must match exactly
			if searchReporterInstanceId == "" || strings.EqualFold(stored.reporterInstanceID, searchReporterInstanceId) {
				// Keep track of the resource with the highest representation version + generation
				if latestResource == nil ||
					stored.representationVersion > latestResource.representationVersion ||
					(stored.representationVersion == latestResource.representationVersion && stored.generation > latestResource.generation) {
					latestResource = stored
				}
			}
		}
	}

	if latestResource != nil {
		// Create snapshots that reflect the actual stored state
		resourceSnapshot := bizmodel.ResourceSnapshot{
			ID:               latestResource.resourceID,
			Type:             latestResource.resourceType,
			CommonVersion:    latestResource.commonVersion,
			ConsistencyToken: "",
		}

		reporterResourceSnapshot := bizmodel.ReporterResourceSnapshot{
			ID: latestResource.reporterResourceID,
			ReporterResourceKey: bizmodel.ReporterResourceKeySnapshot{
				LocalResourceID:    latestResource.localResourceID,
				ReporterType:       latestResource.reporterType,
				ResourceType:       latestResource.resourceType,
				ReporterInstanceID: latestResource.reporterInstanceID,
			},
			ResourceID:            latestResource.resourceID,
			APIHref:               "",
			ConsoleHref:           "",
			RepresentationVersion: latestResource.representationVersion,
			Generation:            latestResource.generation,
			Tombstone:             latestResource.tombstone,
		}

		// Use DeserializeResource to create a Resource that reflects the actual stored state
		resource := bizmodel.DeserializeResource(&resourceSnapshot, []bizmodel.ReporterResourceSnapshot{reporterResourceSnapshot}, nil, nil)
		if resource == nil {
			return nil, fmt.Errorf("failed to deserialize resource")
		}
		return resource, nil
	}

	return nil, gorm.ErrRecordNotFound
}

func (f *fakeResourceRepository) GetDB() *gorm.DB {
	// Fake repository doesn't use a real database
	return nil
}

func (f *fakeResourceRepository) GetTransactionManager() usecase.TransactionManager {
	// Return a fake transaction manager for testing
	return NewFakeTransactionManager(3) // Default retry count
}

func (f *fakeResourceRepository) makeKey(localResourceID, resourceType, reporterType, reporterInstanceID string) string {
	return fmt.Sprintf("%s|%s|%s|%s", localResourceID, resourceType, reporterType, reporterInstanceID)
}

func (f *fakeResourceRepository) makeCompositeKey(localResourceID, reporterType, resourceType, reporterInstanceID string, representationVersion, generation uint) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%d", localResourceID, reporterType, resourceType, reporterInstanceID, representationVersion, generation)
}
