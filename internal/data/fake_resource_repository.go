package data

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

type fakeResourceRepository struct {
	mu                           sync.RWMutex
	resourcesByPrimaryKey        map[uuid.UUID]*storedResource
	resourcesByCompositeKey      map[string]uuid.UUID
	representationsByVersion     map[string]map[uint]*storedRepresentation
	processedTransactionIds      map[string]bool
	maxCommonVersionByResourceID map[uuid.UUID]*uint
}

type storedResource struct {
	resourceID            uuid.UUID
	resourceType          string
	commonVersion         *uint
	commonData            internal.JsonObject
	consistencyToken      string
	reporterResourceID    uuid.UUID
	localResourceID       string
	reporterType          string
	reporterInstanceID    string
	representationVersion uint
	generation            uint
	tombstone             bool
	createdAt             time.Time
	updatedAt             time.Time
}

type storedRepresentation struct {
	commonData    internal.JsonObject
	commonVersion uint
}

func NewFakeResourceRepository() bizmodel.ResourceRepository {
	return &fakeResourceRepository{
		resourcesByPrimaryKey:        make(map[uuid.UUID]*storedResource),
		resourcesByCompositeKey:      make(map[string]uuid.UUID),
		representationsByVersion:     make(map[string]map[uint]*storedRepresentation),
		processedTransactionIds:      make(map[string]bool),
		maxCommonVersionByResourceID: make(map[uuid.UUID]*uint),
	}
}

var _ bizmodel.ResourceRepository = (*fakeResourceRepository)(nil)

func (f *fakeResourceRepository) Begin(_ string) (bizmodel.ResourceTx, error) {
	f.mu.RLock()
	tx := &fakeResourceTx{
		repo:                         f,
		resourcesByPrimaryKey:        cloneResourcesByPrimaryKey(f.resourcesByPrimaryKey),
		resourcesByCompositeKey:      cloneStringUUIDMap(f.resourcesByCompositeKey),
		representationsByVersion:     cloneRepresentationsByVersion(f.representationsByVersion),
		processedTransactionIds:      cloneBoolMap(f.processedTransactionIds),
		maxCommonVersionByResourceID: cloneUintPtrMap(f.maxCommonVersionByResourceID),
	}
	f.mu.RUnlock()
	return tx, nil
}

func (f *fakeResourceRepository) Transact(_ string, fn func(bizmodel.ResourceTx) error) error {
	tx, err := f.Begin("")
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// --- fakeResourceTx implements model.ResourceTx ---
//
// Each transaction operates on a snapshot of the repository state taken at
// Begin time. Writes mutate only the snapshot. Commit atomically replaces
// the live repository state; Rollback discards the snapshot.

type fakeResourceTx struct {
	repo                         *fakeResourceRepository
	resourcesByPrimaryKey        map[uuid.UUID]*storedResource
	resourcesByCompositeKey      map[string]uuid.UUID
	representationsByVersion     map[string]map[uint]*storedRepresentation
	processedTransactionIds      map[string]bool
	maxCommonVersionByResourceID map[uuid.UUID]*uint
	done                         bool
}

var _ bizmodel.ResourceTx = (*fakeResourceTx)(nil)

func (tx *fakeResourceTx) Commit() error {
	if tx.done {
		return nil
	}
	tx.repo.mu.Lock()
	tx.repo.resourcesByPrimaryKey = tx.resourcesByPrimaryKey
	tx.repo.resourcesByCompositeKey = tx.resourcesByCompositeKey
	tx.repo.representationsByVersion = tx.representationsByVersion
	tx.repo.processedTransactionIds = tx.processedTransactionIds
	tx.repo.maxCommonVersionByResourceID = tx.maxCommonVersionByResourceID
	tx.repo.mu.Unlock()
	tx.done = true
	return nil
}

func (tx *fakeResourceTx) Rollback() error {
	tx.done = true
	return nil
}

func (tx *fakeResourceTx) NextResourceId() (bizmodel.ResourceId, error) {
	uuidV7, err := uuid.NewV7()
	if err != nil {
		return bizmodel.ResourceId{}, err
	}
	return bizmodel.NewResourceId(uuidV7)
}

func (tx *fakeResourceTx) NextReporterResourceId() (bizmodel.ReporterResourceId, error) {
	uuidV7, err := uuid.NewV7()
	if err != nil {
		return bizmodel.ReporterResourceId{}, err
	}
	return bizmodel.NewReporterResourceId(uuidV7)
}

func (tx *fakeResourceTx) Save(resource bizmodel.Resource, operationType bizmodel.EventOperationType, txid bizmodel.TransactionId) error {
	resourceSnapshot, reporterResourceSnapshot, reporterRepresentationSnapshot, commonRepresentationSnapshot, err := resource.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	compositeKey := tx.repo.makeCompositeKey(
		reporterResourceSnapshot.ReporterResourceKey.LocalResourceID,
		reporterResourceSnapshot.ReporterResourceKey.ReporterType,
		reporterResourceSnapshot.ReporterResourceKey.ResourceType,
		reporterResourceSnapshot.ReporterResourceKey.ReporterInstanceID,
		reporterResourceSnapshot.RepresentationVersion,
		reporterResourceSnapshot.Generation,
	)

	reporterResourcePrimaryKey := reporterResourceSnapshot.ID

	if existingResource, exists := tx.resourcesByPrimaryKey[reporterResourcePrimaryKey]; exists {
		oldCompositeKey := tx.repo.makeCompositeKey(
			existingResource.localResourceID,
			existingResource.reporterType,
			existingResource.resourceType,
			existingResource.reporterInstanceID,
			existingResource.representationVersion,
			existingResource.generation,
		)
		delete(tx.resourcesByCompositeKey, oldCompositeKey)
	} else {
		if existingPrimaryKey, exists := tx.resourcesByCompositeKey[compositeKey]; exists {
			return fmt.Errorf("duplicate key violation: reporter_resource_key_idx unique constraint failed for key: %s (conflicts with existing resource: %s)", compositeKey, existingPrimaryKey)
		}
	}

	var commonData internal.JsonObject
	var commonVersion uint
	if commonRepresentationSnapshot != nil {
		commonData = commonRepresentationSnapshot.Representation.Data
		commonVersion = commonRepresentationSnapshot.Version

		// Track the max common version seen for this resource (mirrors the MAX subquery in the real repo).
		resourceID := resourceSnapshot.ID
		if prev := tx.maxCommonVersionByResourceID[resourceID]; prev == nil || commonVersion > *prev {
			v := commonVersion
			tx.maxCommonVersionByResourceID[resourceID] = &v
		}
	}

	stored := &storedResource{
		resourceID:            resourceSnapshot.ID,
		resourceType:          resourceSnapshot.Type,
		commonVersion:         resourceSnapshot.CommonVersion,
		commonData:            commonData,
		consistencyToken:      resourceSnapshot.ConsistencyToken,
		reporterResourceID:    reporterResourceSnapshot.ID,
		localResourceID:       reporterResourceSnapshot.ReporterResourceKey.LocalResourceID,
		reporterType:          reporterResourceSnapshot.ReporterResourceKey.ReporterType,
		reporterInstanceID:    reporterResourceSnapshot.ReporterResourceKey.ReporterInstanceID,
		representationVersion: reporterResourceSnapshot.RepresentationVersion,
		createdAt:             reporterResourceSnapshot.CreatedAt,
		updatedAt:             reporterResourceSnapshot.UpdatedAt,
		generation:            reporterResourceSnapshot.Generation,
		tombstone:             reporterResourceSnapshot.Tombstone,
	}

	tx.resourcesByPrimaryKey[reporterResourcePrimaryKey] = stored
	tx.resourcesByCompositeKey[compositeKey] = reporterResourcePrimaryKey

	historyKey := tx.repo.makeHistoryKey(
		stored.localResourceID,
		stored.reporterType,
		stored.resourceType,
		stored.reporterInstanceID,
	)
	if _, ok := tx.representationsByVersion[historyKey]; !ok {
		tx.representationsByVersion[historyKey] = make(map[uint]*storedRepresentation)
	}
	tx.representationsByVersion[historyKey][stored.representationVersion] = &storedRepresentation{
		commonData:    cloneJsonObject(stored.commonData),
		commonVersion: commonVersion,
	}

	if reporterRepresentationSnapshot != nil && reporterRepresentationSnapshot.TransactionId != "" {
		tx.processedTransactionIds[reporterRepresentationSnapshot.TransactionId] = true
	}
	if commonRepresentationSnapshot != nil && commonRepresentationSnapshot.TransactionId != "" {
		tx.processedTransactionIds[commonRepresentationSnapshot.TransactionId] = true
	}

	return nil
}

func (tx *fakeResourceTx) FindResourceByKeys(key bizmodel.ReporterResourceKey) (*bizmodel.Resource, error) {
	searchReporterInstanceId := key.ReporterInstanceId().Serialize()

	var latestResource *storedResource
	for _, stored := range tx.resourcesByPrimaryKey {
		if strings.EqualFold(stored.localResourceID, key.LocalResourceId().Serialize()) &&
			strings.EqualFold(stored.resourceType, key.ResourceType().Serialize()) &&
			strings.EqualFold(stored.reporterType, key.ReporterType().Serialize()) {

			if searchReporterInstanceId == "" || strings.EqualFold(stored.reporterInstanceID, searchReporterInstanceId) {
				if latestResource == nil {
					latestResource = stored
					continue
				}
				if !stored.tombstone && latestResource.tombstone {
					latestResource = stored
					continue
				}
				if stored.tombstone && !latestResource.tombstone {
					continue
				}
				if stored.representationVersion > latestResource.representationVersion ||
					(stored.representationVersion == latestResource.representationVersion && stored.generation > latestResource.generation) {
					latestResource = stored
				}
			}
		}
	}

	if latestResource != nil {
		resourceSnapshot := bizmodel.ResourceSnapshot{
			ID:                latestResource.resourceID,
			Type:              latestResource.resourceType,
			CommonVersion:     latestResource.commonVersion,
			LastCommonVersion: tx.maxCommonVersionByResourceID[latestResource.resourceID],
			ConsistencyToken:  latestResource.consistencyToken,
			CreatedAt:         latestResource.createdAt,
			UpdatedAt:         latestResource.updatedAt,
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
			ConsoleHref:           nil,
			RepresentationVersion: latestResource.representationVersion,
			Generation:            latestResource.generation,
			Tombstone:             latestResource.tombstone,
			CreatedAt:             latestResource.createdAt,
			UpdatedAt:             latestResource.updatedAt,
		}

		resource := bizmodel.DeserializeResource(&resourceSnapshot, []bizmodel.ReporterResourceSnapshot{reporterResourceSnapshot}, nil, nil)
		if resource == nil {
			return nil, fmt.Errorf("failed to deserialize resource")
		}
		return resource, nil
	}

	return nil, bizmodel.ErrResourceNotFound
}

func (tx *fakeResourceTx) FindCurrentAndPreviousVersionedRepresentations(key bizmodel.ReporterResourceKey, currentVersion *bizmodel.Version, operationType bizmodel.EventOperationType) (*bizmodel.Representations, *bizmodel.Representations, error) {
	if currentVersion == nil {
		return nil, nil, nil
	}

	historyKey := tx.repo.makeHistoryKey(
		key.LocalResourceId().Serialize(),
		key.ReporterType().Serialize(),
		key.ResourceType().Serialize(),
		key.ReporterInstanceId().Serialize(),
	)

	versionMap := tx.representationsByVersion[historyKey]
	if versionMap == nil {
		return nil, nil, fmt.Errorf("no representations found for key")
	}

	cv := currentVersion.Uint()
	var current *bizmodel.Representations
	var previous *bizmodel.Representations

	if entry, ok := versionMap[cv]; ok {
		v := bizmodel.NewVersion(entry.commonVersion)
		var err error
		current, err = bizmodel.NewRepresentations(
			bizmodel.Representation(cloneJsonObject(entry.commonData)),
			&v,
			nil,
			nil,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	// Skip previous-version lookup for create operations, matching the real
	// repository's WHERE clause which only fetches cv-1 for non-create ops.
	if operationType.OperationType() != bizmodel.OperationTypeCreated && cv > 0 {
		if entry, ok := versionMap[cv-1]; ok {
			v := bizmodel.NewVersion(entry.commonVersion)
			var err error
			previous, err = bizmodel.NewRepresentations(
				bizmodel.Representation(cloneJsonObject(entry.commonData)),
				&v,
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

func (tx *fakeResourceTx) FindLatestRepresentations(key bizmodel.ReporterResourceKey) (*bizmodel.Representations, error) {
	historyKey := tx.repo.makeHistoryKey(
		key.LocalResourceId().Serialize(),
		key.ReporterType().Serialize(),
		key.ResourceType().Serialize(),
		key.ReporterInstanceId().Serialize(),
	)

	versionMap := tx.representationsByVersion[historyKey]
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

	v := bizmodel.NewVersion(latest.commonVersion)
	return bizmodel.NewRepresentations(
		bizmodel.Representation(cloneJsonObject(latest.commonData)),
		&v,
		nil,
		nil,
	)
}

func (tx *fakeResourceTx) HasTransactionIdBeenProcessed(transactionId bizmodel.TransactionId) (bool, error) {
	_, exists := tx.processedTransactionIds[transactionId.String()]
	return exists, nil
}

// --- helpers ---

func (f *fakeResourceRepository) makeCompositeKey(localResourceID, reporterType, resourceType, reporterInstanceID string, representationVersion, generation uint) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%d", localResourceID, reporterType, resourceType, reporterInstanceID, representationVersion, generation)
}

func (f *fakeResourceRepository) makeHistoryKey(localResourceID, reporterType, resourceType, reporterInstanceID string) string {
	return strings.ToLower(fmt.Sprintf("%s|%s|%s|%s", localResourceID, reporterType, resourceType, reporterInstanceID))
}

// markTransactionIdAsProcessed marks a transaction ID as processed for idempotency testing.
func (f *fakeResourceRepository) markTransactionIdAsProcessed(transactionId string) {
	if transactionId == "" {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processedTransactionIds[transactionId] = true
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

// --- snapshot clone helpers ---

func cloneResourcesByPrimaryKey(src map[uuid.UUID]*storedResource) map[uuid.UUID]*storedResource {
	dst := make(map[uuid.UUID]*storedResource, len(src))
	for k, v := range src {
		copied := *v
		copied.commonData = cloneJsonObject(v.commonData)
		dst[k] = &copied
	}
	return dst
}

func cloneStringUUIDMap(src map[string]uuid.UUID) map[string]uuid.UUID {
	dst := make(map[string]uuid.UUID, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneRepresentationsByVersion(src map[string]map[uint]*storedRepresentation) map[string]map[uint]*storedRepresentation {
	dst := make(map[string]map[uint]*storedRepresentation, len(src))
	for k, innerMap := range src {
		dstInner := make(map[uint]*storedRepresentation, len(innerMap))
		for v, rep := range innerMap {
			copied := *rep
			copied.commonData = cloneJsonObject(rep.commonData)
			dstInner[v] = &copied
		}
		dst[k] = dstInner
	}
	return dst
}

func cloneBoolMap(src map[string]bool) map[string]bool {
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneUintPtrMap(src map[uuid.UUID]*uint) map[uuid.UUID]*uint {
	dst := make(map[uuid.UUID]*uint, len(src))
	for k, v := range src {
		if v != nil {
			val := *v
			dst[k] = &val
		}
	}
	return dst
}
