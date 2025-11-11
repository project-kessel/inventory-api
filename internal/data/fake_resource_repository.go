package data

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal"
	"github.com/project-kessel/inventory-api/internal/biz"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase"
)

type fakeResourceRepository struct {
	mu                       sync.RWMutex
	resourcesByPrimaryKey    map[uuid.UUID]*storedResource // keyed by primary key (ResourceID) - simulates database primary storage
	resourcesByCompositeKey  map[string]uuid.UUID          // composite key -> primary key mapping for unique constraint
	resources                map[string]*storedResource    // legacy field for backward compatibility
	representationsByVersion map[string]map[uint]*storedRepresentation
	processedTransactionIds  map[string]bool // track processed transaction IDs for idempotency testing
}

type storedResource struct {
	resourceID            uuid.UUID
	resourceType          string
	commonVersion         uint
	commonData            internal.JsonObject
	reporterResourceID    uuid.UUID
	localResourceID       string
	reporterType          string
	reporterInstanceID    string
	representationVersion uint
	generation            uint
	tombstone             bool
}

type storedRepresentation struct {
	commonData    internal.JsonObject
	commonVersion uint
}

func NewFakeResourceRepository() ResourceRepository {
	return &fakeResourceRepository{
		resourcesByPrimaryKey:    make(map[uuid.UUID]*storedResource),
		resourcesByCompositeKey:  make(map[string]uuid.UUID),
		resources:                make(map[string]*storedResource),
		representationsByVersion: make(map[string]map[uint]*storedRepresentation),
		processedTransactionIds:  make(map[string]bool),
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

func (f *fakeResourceRepository) Save(tx *gorm.DB, resource bizmodel.Resource, operationType biz.EventOperationType, txid string) error {
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
		commonVersion:         *resourceSnapshot.CommonVersion,
		commonData:            commonRepresentationSnapshot.Representation.Data,
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

	historyKey := f.makeHistoryKey(
		stored.localResourceID,
		stored.reporterType,
		stored.resourceType,
		stored.reporterInstanceID,
	)
	if _, ok := f.representationsByVersion[historyKey]; !ok {
		f.representationsByVersion[historyKey] = make(map[uint]*storedRepresentation)
	}
	commonVersion := commonRepresentationSnapshot.Version
	f.representationsByVersion[historyKey][stored.representationVersion] = &storedRepresentation{
		commonData:    cloneJsonObject(stored.commonData),
		commonVersion: commonVersion,
	}

	// Mark transaction IDs as processed for idempotency testing
	if reporterRepresentationSnapshot.TransactionId != "" {
		f.markTransactionIdAsProcessed(reporterRepresentationSnapshot.TransactionId)
	}
	if commonRepresentationSnapshot.TransactionId != "" {
		f.markTransactionIdAsProcessed(commonRepresentationSnapshot.TransactionId)
	}

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
			CommonVersion:    &latestResource.commonVersion,
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

func (f *fakeResourceRepository) FindCurrentAndPreviousVersionedRepresentations(tx *gorm.DB, key bizmodel.ReporterResourceKey, currentVersion *uint, operationType biz.EventOperationType) (*bizmodel.Representations, *bizmodel.Representations, error) {
	if currentVersion == nil {
		return nil, nil, nil
	}

	historyKey := f.makeHistoryKey(
		key.LocalResourceId().Serialize(),
		key.ReporterType().Serialize(),
		key.ResourceType().Serialize(),
		key.ReporterInstanceId().Serialize(),
	)

	f.mu.RLock()
	defer f.mu.RUnlock()

	versionMap := f.representationsByVersion[historyKey]
	if versionMap == nil {
		return nil, nil, fmt.Errorf("no representations found for key")
	}

	var current *bizmodel.Representations
	var previous *bizmodel.Representations

	if entry, ok := versionMap[*currentVersion]; ok {
		var err error
		current, err = bizmodel.NewRepresentations(
			bizmodel.Representation(cloneJsonObject(entry.commonData)),
			uintPtr(entry.commonVersion),
			nil,
			nil,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	if *currentVersion > 0 {
		if entry, ok := versionMap[*currentVersion-1]; ok {
			var err error
			previous, err = bizmodel.NewRepresentations(
				bizmodel.Representation(cloneJsonObject(entry.commonData)),
				uintPtr(entry.commonVersion),
				nil,
				nil,
			)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return current, previous, nil
}

func (f *fakeResourceRepository) FindLatestRepresentations(tx *gorm.DB, key bizmodel.ReporterResourceKey) (*bizmodel.Representations, error) {
	historyKey := f.makeHistoryKey(
		key.LocalResourceId().Serialize(),
		key.ReporterType().Serialize(),
		key.ResourceType().Serialize(),
		key.ReporterInstanceId().Serialize(),
	)

	f.mu.RLock()
	defer f.mu.RUnlock()

	versionMap := f.representationsByVersion[historyKey]
	if len(versionMap) == 0 {
		return nil, fmt.Errorf("no representations found for key")
	}

	var maxVersion uint
	var latest *storedRepresentation
	for version, entry := range versionMap {
		if latest == nil || version > maxVersion {
			maxVersion = version
			latest = entry
		}
	}

	return bizmodel.NewRepresentations(
		bizmodel.Representation(cloneJsonObject(latest.commonData)),
		uintPtr(latest.commonVersion),
		nil,
		nil,
	)
}

func (f *fakeResourceRepository) GetDB() *gorm.DB {
	// Fake repository doesn't use a real database
	return nil
}

func (f *fakeResourceRepository) GetTransactionManager() usecase.TransactionManager {
	// Return a fake transaction manager for testing
	return NewFakeTransactionManager(3) // Default retry count
}

func (f *fakeResourceRepository) makeCompositeKey(localResourceID, reporterType, resourceType, reporterInstanceID string, representationVersion, generation uint) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%d", localResourceID, reporterType, resourceType, reporterInstanceID, representationVersion, generation)
}

func (f *fakeResourceRepository) makeHistoryKey(localResourceID, reporterType, resourceType, reporterInstanceID string) string {
	return strings.ToLower(fmt.Sprintf("%s|%s|%s|%s", localResourceID, reporterType, resourceType, reporterInstanceID))
}

// markTransactionIdAsProcessed marks a transaction ID as processed for idempotency testing
// Note: This method assumes the caller already holds the appropriate lock
func (f *fakeResourceRepository) markTransactionIdAsProcessed(transactionId string) {
	if transactionId == "" {
		return
	}

	// Don't acquire lock here since Save method already holds it
	f.processedTransactionIds[transactionId] = true
}

func uintPtr(v uint) *uint {
	value := v
	return &value
}

func cloneJsonObject(src internal.JsonObject) internal.JsonObject {
	if src == nil {
		return nil
	}
	clone := make(internal.JsonObject, len(src))
	for k, v := range src {
		if nested, ok := v.(map[string]interface{}); ok {
			nestedClone := make(map[string]interface{}, len(nested))
			for nk, nv := range nested {
				nestedClone[nk] = nv
			}
			clone[k] = nestedClone
		} else {
			clone[k] = v
		}
	}
	return clone
}

// HasTransactionIdBeenProcessed checks if a transaction ID has been processed before
// Returns true if the transaction has already been processed, false otherwise
func (f *fakeResourceRepository) HasTransactionIdBeenProcessed(tx *gorm.DB, transactionId string) (bool, error) {
	if transactionId == "" {
		return false, nil
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check if this transaction ID has been processed before
	_, exists := f.processedTransactionIds[transactionId]
	return exists, nil
}
