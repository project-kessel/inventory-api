package data

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"gorm.io/gorm"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
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

func (f *fakeResourceRepository) MustGetNextTransactionID() string {
	txid, err := f.GetNextTransactionID()
	if err != nil {
		panic(err)
	}
	return txid
}

func (f *fakeResourceRepository) SaveWithAutoTxID(tx *gorm.DB, resource bizmodel.Resource, operationType model_legacy.EventOperationType) error {
	txid, err := f.GetNextTransactionID()
	if err != nil {
		return err
	}
	return f.Save(tx, resource, operationType, txid)
}

func (f *fakeResourceRepository) SaveWithTransaction(resource bizmodel.Resource, operationType model_legacy.EventOperationType, txid string) error {
	// For the fake repository, we don't need actual transaction management
	// Just call Save with nil transaction since the fake implementation doesn't use it
	return f.Save(nil, resource, operationType, txid)
}

func (f *fakeResourceRepository) Save(tx *gorm.DB, resource bizmodel.Resource, operationType model_legacy.EventOperationType, txid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	dataResource, dataReporterResource, _, _, err := resource.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
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

func (f *fakeResourceRepository) makeKey(localResourceID, resourceType, reporterType, reporterInstanceID string) string {
	return fmt.Sprintf("%s|%s|%s|%s", localResourceID, resourceType, reporterType, reporterInstanceID)
}
