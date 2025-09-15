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

	key := f.makeKey(
		reporterResourceSnapshot.ReporterResourceKey.LocalResourceID,
		reporterResourceSnapshot.ReporterResourceKey.ResourceType,
		reporterResourceSnapshot.ReporterResourceKey.ReporterType,
		reporterResourceSnapshot.ReporterResourceKey.ReporterInstanceID,
	)

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

	f.resources[key] = stored
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

	for _, stored := range f.resources {
		if stored.localResourceID == key.LocalResourceId().Serialize() &&
			stored.resourceType == key.ResourceType().Serialize() &&
			stored.reporterType == key.ReporterType().Serialize() {

			// If search key has empty reporterInstanceId, match any stored resource
			// If search key has reporterInstanceId, it must match exactly
			if searchReporterInstanceId == "" || stored.reporterInstanceID == searchReporterInstanceId {
				placeholderData := map[string]interface{}{"_placeholder": true}
				resource, err := bizmodel.NewResource(bizmodel.ResourceId(stored.resourceID), bizmodel.LocalResourceId(stored.localResourceID), bizmodel.ResourceType(stored.resourceType), bizmodel.ReporterType(stored.reporterType), bizmodel.ReporterInstanceId(stored.reporterInstanceID), bizmodel.ReporterResourceId(stored.reporterResourceID), bizmodel.ApiHref(""), bizmodel.ConsoleHref(""), bizmodel.Representation(placeholderData), bizmodel.Representation(placeholderData), nil)
				if err != nil {
					return nil, fmt.Errorf("failed to create resource: %w", err)
				}
				return &resource, nil
			}
		}
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
