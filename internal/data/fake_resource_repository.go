package data

import (
	"fmt"
	"sync"

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

func (f *fakeResourceRepository) GetNextTransactionID() (string, error) {
	txid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return txid.String(), nil
}

func (f *fakeResourceRepository) ReportRepresentations(tx *gorm.DB, reporterRepresentation interface{}, commonRepresentation interface{}) error {
	// For fake repository, we don't need to actually store representations
	// Just return nil to indicate success
	return nil
}

func (f *fakeResourceRepository) Save(tx *gorm.DB, resource bizmodel.Resource, operationType model_legacy.EventOperationType, txid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	dataResource, dataReporterResource, dataReporterRepresentation, dataCommonRepresentation, err := resource.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	// Use the new ReportRepresentations method
	if err := f.ReportRepresentations(tx, dataReporterRepresentation, dataCommonRepresentation); err != nil {
		return err
	}

	key := f.makeKey(
		dataReporterResource.LocalResourceID,
		dataReporterResource.ResourceType,
		dataReporterResource.ReporterType,
		dataReporterResource.ReporterInstanceID,
	)

	stored := &storedResource{
		resourceID:            dataResource.ID,
		resourceType:          dataResource.Type,
		commonVersion:         dataResource.CommonVersion,
		reporterResourceID:    dataReporterResource.ID,
		localResourceID:       dataReporterResource.LocalResourceID,
		reporterType:          dataReporterResource.ReporterType,
		reporterInstanceID:    dataReporterResource.ReporterInstanceID,
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

	resource, err := bizmodel.Deserialize(
		stored.resourceID,
		stored.resourceType,
		stored.commonVersion,
		stored.reporterResourceID,
		stored.localResourceID,
		stored.reporterType,
		stored.reporterInstanceID,
		stored.representationVersion,
		stored.generation,
		stored.tombstone,
		"redhat.com",
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize resource: %w", err)
	}

	return resource, nil
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
