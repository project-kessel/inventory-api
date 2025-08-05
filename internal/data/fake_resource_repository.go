package data

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/biz/usecase"
)

type fakeResourceRepository struct {
	mu        sync.RWMutex
	resources map[string]*storedResource
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
		resources: make(map[string]*storedResource),
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

	dataResource, dataReporterResource, dataReporterRepresentation, dataCommonRepresentation, err := resource.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	// In fake implementation, we don't actually store representations but we should acknowledge them
	_ = dataReporterRepresentation
	_ = dataCommonRepresentation

	key := f.makeKey(
		dataReporterResource.ReporterResourceKey.LocalResourceID,
		dataReporterResource.ReporterResourceKey.ResourceType,
		dataReporterResource.ReporterResourceKey.ReporterType,
		dataReporterResource.ReporterResourceKey.ReporterInstanceID,
	)

	stored := &storedResource{
		resourceID:            dataResource.ID,
		resourceType:          dataResource.Type,
		commonVersion:         dataResource.CommonVersion,
		reporterResourceID:    dataReporterResource.ID,
		localResourceID:       dataReporterResource.ReporterResourceKey.LocalResourceID,
		reporterType:          dataReporterResource.ReporterResourceKey.ReporterType,
		reporterInstanceID:    dataReporterResource.ReporterResourceKey.ReporterInstanceID,
		representationVersion: dataReporterResource.RepresentationVersion,
		generation:            dataReporterResource.Generation,
		tombstone:             dataReporterResource.Tombstone,
	}

	f.resources[key] = stored
	return nil
}

func (f *fakeResourceRepository) FindResourceByKeys(tx *gorm.DB, key bizmodel.ReporterResourceKey) (*bizmodel.Resource, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	searchKey := f.makeKey(
		key.LocalResourceId(),
		key.ResourceType(),
		key.ReporterType(),
		key.ReporterInstanceId(),
	)

	stored, exists := f.resources[searchKey]
	if !exists {
		return nil, nil
	}

	// Create snapshots from stored data
	resourceSnapshot := bizmodel.ResourceSnapshot{
		ID:               stored.resourceID,
		Type:             stored.resourceType,
		CommonVersion:    stored.commonVersion,
		ConsistencyToken: "",
		CreatedAt:        time.Now(), // Placeholder
		UpdatedAt:        time.Now(), // Placeholder
	}

	reporterResourceSnapshot := bizmodel.ReporterResourceSnapshot{
		ID: stored.reporterResourceID,
		ReporterResourceKey: bizmodel.ReporterResourceKeySnapshot{
			LocalResourceID:    stored.localResourceID,
			ReporterType:       stored.reporterType,
			ResourceType:       stored.resourceType,
			ReporterInstanceID: stored.reporterInstanceID,
		},
		ResourceID:            stored.resourceID,
		APIHref:               "redhat.com", // Placeholder
		ConsoleHref:           "",           // Placeholder
		RepresentationVersion: stored.representationVersion,
		Generation:            stored.generation,
		Tombstone:             stored.tombstone,
		CreatedAt:             time.Now(), // Placeholder
		UpdatedAt:             time.Now(), // Placeholder
	}

	// Create placeholder representation snapshots
	reporterRepresentationSnapshot := bizmodel.ReporterRepresentationSnapshot{
		Representation: bizmodel.RepresentationSnapshot{
			Data: map[string]interface{}{}, // Placeholder
		},
		ReporterResourceID: stored.reporterResourceID.String(),
		Version:            stored.representationVersion,
		Generation:         stored.generation,
		CommonVersion:      stored.commonVersion,
		Tombstone:          stored.tombstone,
		CreatedAt:          time.Now(), // Placeholder
	}

	commonRepresentationSnapshot := bizmodel.CommonRepresentationSnapshot{
		Representation: bizmodel.RepresentationSnapshot{
			Data: map[string]interface{}{}, // Placeholder
		},
		ResourceId:                 stored.resourceID,
		Version:                    stored.commonVersion,
		ReportedByReporterType:     stored.reporterType,
		ReportedByReporterInstance: stored.reporterInstanceID,
		CreatedAt:                  time.Now(), // Placeholder
	}

	// Deserialize using the new snapshot-based approach
	resource := bizmodel.DeserializeResource(
		resourceSnapshot,
		reporterResourceSnapshot,
		reporterRepresentationSnapshot,
		commonRepresentationSnapshot,
	)

	return &resource, nil
}

func (f *fakeResourceRepository) GetDB() *gorm.DB {
	// Fake repository doesn't use a real database
	return nil
}

func (f *fakeResourceRepository) GetTransactionManager() usecase.TransactionManager {
	// Return a fake transaction manager for testing
	return NewFakeTransactionManager()
}

func (f *fakeResourceRepository) makeKey(localResourceID, resourceType, reporterType, reporterInstanceID string) string {
	return fmt.Sprintf("%s|%s|%s|%s", localResourceID, resourceType, reporterType, reporterInstanceID)
}
