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

// FakeResourceRepository implements model.Store for testing with in-memory storage.
type FakeResourceRepository struct {
	repo *fakeResourceRepo
}

// NewFakeResourceRepository creates a new FakeResourceRepository for testing.
func NewFakeResourceRepository() *FakeResourceRepository {
	return &FakeResourceRepository{
		repo: newFakeResourceRepo(),
	}
}

var _ bizmodel.Store = (*FakeResourceRepository)(nil)

// Begin starts a new fake transaction.
func (s *FakeResourceRepository) Begin() (bizmodel.Tx, error) {
	return &fakeResourceRepositoryTx{
		repo: s.repo,
	}, nil
}

// RunSerializable executes fn within a fake transaction. No retry logic
// is applied since the in-memory implementation never produces serialization failures.
func (s *FakeResourceRepository) RunSerializable(_ string, fn func(tx bizmodel.Tx) error) error {
	tx, err := s.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// fakeResourceRepositoryTx implements model.Tx for testing.
type fakeResourceRepositoryTx struct {
	repo *fakeResourceRepo
	done bool
}

var _ bizmodel.Tx = (*fakeResourceRepositoryTx)(nil)

func (tx *fakeResourceRepositoryTx) ResourceRepository() bizmodel.ResourceRepository {
	return tx.repo
}

func (tx *fakeResourceRepositoryTx) Commit() error {
	if tx.done {
		return nil
	}
	tx.done = true
	return nil
}

func (tx *fakeResourceRepositoryTx) Rollback() error {
	if tx.done {
		return nil
	}
	tx.done = true
	return nil
}

// fakeResourceRepo implements the new model.ResourceRepository (no gorm params).
type fakeResourceRepo struct {
	mu                           sync.RWMutex
	resourcesByPrimaryKey        map[uuid.UUID]*storedResource
	resourcesByCompositeKey      map[string]uuid.UUID
	representationsByVersion     map[string]map[uint]*storedRepresentation
	processedTransactionIds      map[string]bool
	maxCommonVersionByResourceID map[uuid.UUID]*uint
}

func newFakeResourceRepo() *fakeResourceRepo {
	return &fakeResourceRepo{
		resourcesByPrimaryKey:        make(map[uuid.UUID]*storedResource),
		resourcesByCompositeKey:      make(map[string]uuid.UUID),
		representationsByVersion:     make(map[string]map[uint]*storedRepresentation),
		processedTransactionIds:      make(map[string]bool),
		maxCommonVersionByResourceID: make(map[uuid.UUID]*uint),
	}
}

var _ bizmodel.ResourceRepository = (*fakeResourceRepo)(nil)

func (f *fakeResourceRepo) NextResourceId() (bizmodel.ResourceId, error) {
	uuidV7, err := uuid.NewV7()
	if err != nil {
		return bizmodel.ResourceId{}, err
	}
	return bizmodel.NewResourceId(uuidV7)
}

func (f *fakeResourceRepo) NextReporterResourceId() (bizmodel.ReporterResourceId, error) {
	uuidV7, err := uuid.NewV7()
	if err != nil {
		return bizmodel.ReporterResourceId{}, err
	}
	return bizmodel.NewReporterResourceId(uuidV7)
}

func (f *fakeResourceRepo) Save(resource bizmodel.Resource, operationType bizmodel.EventOperationType, txid bizmodel.TransactionId) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	resourceSnapshot, reporterResourceSnapshot, reporterRepresentationSnapshot, commonRepresentationSnapshot, err := resource.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	compositeKey := makeCompositeKey(
		reporterResourceSnapshot.ReporterResourceKey.LocalResourceID,
		reporterResourceSnapshot.ReporterResourceKey.ReporterType,
		reporterResourceSnapshot.ReporterResourceKey.ResourceType,
		reporterResourceSnapshot.ReporterResourceKey.ReporterInstanceID,
		reporterResourceSnapshot.RepresentationVersion,
		reporterResourceSnapshot.Generation,
	)

	reporterResourcePrimaryKey := reporterResourceSnapshot.ID

	if existingResource, exists := f.resourcesByPrimaryKey[reporterResourcePrimaryKey]; exists {
		oldCompositeKey := makeCompositeKey(
			existingResource.localResourceID,
			existingResource.reporterType,
			existingResource.resourceType,
			existingResource.reporterInstanceID,
			existingResource.representationVersion,
			existingResource.generation,
		)
		delete(f.resourcesByCompositeKey, oldCompositeKey)
	} else {
		if existingPrimaryKey, exists := f.resourcesByCompositeKey[compositeKey]; exists {
			return fmt.Errorf("duplicate key violation: reporter_resource_key_idx unique constraint failed for key: %s (conflicts with existing resource: %s)", compositeKey, existingPrimaryKey)
		}
	}

	var commonData internal.JsonObject
	var commonVersion uint
	if commonRepresentationSnapshot != nil {
		commonData = commonRepresentationSnapshot.Representation.Data
		commonVersion = commonRepresentationSnapshot.Version

		resourceID := resourceSnapshot.ID
		if prev := f.maxCommonVersionByResourceID[resourceID]; prev == nil || commonVersion > *prev {
			v := commonVersion
			f.maxCommonVersionByResourceID[resourceID] = &v
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

	f.resourcesByPrimaryKey[reporterResourcePrimaryKey] = stored
	f.resourcesByCompositeKey[compositeKey] = reporterResourcePrimaryKey

	historyKey := makeHistoryKey(
		stored.localResourceID,
		stored.reporterType,
		stored.resourceType,
		stored.reporterInstanceID,
	)
	if _, ok := f.representationsByVersion[historyKey]; !ok {
		f.representationsByVersion[historyKey] = make(map[uint]*storedRepresentation)
	}
	f.representationsByVersion[historyKey][stored.representationVersion] = &storedRepresentation{
		commonData:    cloneJsonObject(stored.commonData),
		commonVersion: commonVersion,
	}

	if reporterRepresentationSnapshot != nil && reporterRepresentationSnapshot.TransactionId != "" {
		f.processedTransactionIds[reporterRepresentationSnapshot.TransactionId] = true
	}
	if commonRepresentationSnapshot != nil && commonRepresentationSnapshot.TransactionId != "" {
		f.processedTransactionIds[commonRepresentationSnapshot.TransactionId] = true
	}

	return nil
}

func (f *fakeResourceRepo) FindResourceByKeys(key bizmodel.ReporterResourceKey) (*bizmodel.Resource, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	searchReporterInstanceId := key.ReporterInstanceId().Serialize()

	var latestResource *storedResource
	for _, stored := range f.resourcesByPrimaryKey {
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
			LastCommonVersion: f.maxCommonVersionByResourceID[latestResource.resourceID],
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

func (f *fakeResourceRepo) FindCurrentAndPreviousVersionedRepresentations(key bizmodel.ReporterResourceKey, currentVersion *bizmodel.Version, operationType bizmodel.EventOperationType) (*bizmodel.Representations, *bizmodel.Representations, error) {
	if currentVersion == nil {
		return nil, nil, nil
	}

	historyKey := makeHistoryKey(
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

	if cv > 0 {
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

func (f *fakeResourceRepo) FindLatestRepresentations(key bizmodel.ReporterResourceKey) (*bizmodel.Representations, error) {
	historyKey := makeHistoryKey(
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

	v := bizmodel.NewVersion(latest.commonVersion)
	return bizmodel.NewRepresentations(
		bizmodel.Representation(cloneJsonObject(latest.commonData)),
		&v,
		nil,
		nil,
	)
}

func (f *fakeResourceRepo) HasTransactionIdBeenProcessed(transactionId bizmodel.TransactionId) (bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	_, exists := f.processedTransactionIds[transactionId.String()]
	return exists, nil
}

// markTransactionIdAsProcessed is a test helper for marking transaction IDs as processed.
func (f *fakeResourceRepo) markTransactionIdAsProcessed(transactionId string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.processedTransactionIds[transactionId] = true
}

// --- shared helpers (also used by the storedResource types in fake_resource_repository.go) ---

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

func makeCompositeKey(localResourceID, reporterType, resourceType, reporterInstanceID string, representationVersion, generation uint) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%d", localResourceID, reporterType, resourceType, reporterInstanceID, representationVersion, generation)
}

func makeHistoryKey(localResourceID, reporterType, resourceType, reporterInstanceID string) string {
	return strings.ToLower(fmt.Sprintf("%s|%s|%s|%s", localResourceID, reporterType, resourceType, reporterInstanceID))
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
