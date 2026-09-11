package data

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

type fakeResourceRepository struct {
	mu                              sync.RWMutex
	resourcesByPrimaryKey           map[uuid.UUID]*storedResource                                  // keyed by primary key (ResourceID) - simulates database primary storage
	resourcesByCompositeKey         map[string]uuid.UUID                                           // composite key -> primary key mapping for unique constraint
	resources                       map[string]*storedResource                                     // legacy field for backward compatibility
	commonRepresentationsByResource map[uuid.UUID]map[uint]*storedCommonRepresentation             // keyed by resource_id -> version (simulates common_representations table)
	reporterRepsByReporterResource  map[uuid.UUID]map[uint]map[uint]*storedReporterRepresentation // keyed by reporter_resource_id -> version -> generation (simulates reporter_representations table)
	processedTransactionIds         map[string]bool                                                // track processed transaction IDs for idempotency testing
	maxCommonVersionByResourceID    map[uuid.UUID]*uint                                            // mirrors MAX(version) FROM common_representations WHERE resource_id = ?
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

type storedCommonRepresentation struct {
	data    internal.JsonObject
	version uint
}

type storedReporterRepresentation struct {
	data       internal.JsonObject
	version    uint
	generation uint
}

func NewFakeResourceRepository() bizmodel.ResourceRepository {
	return &fakeResourceRepository{
		resourcesByPrimaryKey:           make(map[uuid.UUID]*storedResource),
		resourcesByCompositeKey:         make(map[string]uuid.UUID),
		resources:                       make(map[string]*storedResource),
		commonRepresentationsByResource: make(map[uuid.UUID]map[uint]*storedCommonRepresentation),
		reporterRepsByReporterResource:  make(map[uuid.UUID]map[uint]map[uint]*storedReporterRepresentation),
		processedTransactionIds:         make(map[string]bool),
		maxCommonVersionByResourceID:    make(map[uuid.UUID]*uint),
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

func (f *fakeResourceRepository) Save(tx *gorm.DB, resource bizmodel.Resource, operationType bizmodel.EventOperationType, txid bizmodel.TransactionId) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	resourceSnapshot, reporterResourceSnapshot, reporterRepresentationSnapshot, commonRepresentationSnapshot, err := resource.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	compositeKey := f.makeCompositeKey(
		reporterResourceSnapshot.ReporterResourceKey.LocalResourceID,
		reporterResourceSnapshot.ReporterResourceKey.ReporterType,
		reporterResourceSnapshot.ReporterResourceKey.ResourceType,
		reporterResourceSnapshot.ReporterResourceKey.ReporterInstanceID,
		reporterResourceSnapshot.RepresentationVersion,
		reporterResourceSnapshot.Generation,
	)

	reporterResourcePrimaryKey := reporterResourceSnapshot.ID

	if existingResource, exists := f.resourcesByPrimaryKey[reporterResourcePrimaryKey]; exists {
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
		if existingPrimaryKey, exists := f.resourcesByCompositeKey[compositeKey]; exists {
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

	// Store common representation if present (simulates common_representations table)
	if commonRepresentationSnapshot != nil {
		resourceID := resourceSnapshot.ID
		if _, ok := f.commonRepresentationsByResource[resourceID]; !ok {
			f.commonRepresentationsByResource[resourceID] = make(map[uint]*storedCommonRepresentation)
		}
		f.commonRepresentationsByResource[resourceID][commonVersion] = &storedCommonRepresentation{
			data:    cloneJsonObject(commonRepresentationSnapshot.Representation.Data),
			version: commonVersion,
		}
	}

	// Store reporter representation if present (simulates reporter_representations table)
	if reporterRepresentationSnapshot != nil {
		reporterResourceID := reporterResourceSnapshot.ID
		if _, ok := f.reporterRepsByReporterResource[reporterResourceID]; !ok {
			f.reporterRepsByReporterResource[reporterResourceID] = make(map[uint]map[uint]*storedReporterRepresentation)
		}
		reporterVersion := reporterResourceSnapshot.RepresentationVersion
		if _, ok := f.reporterRepsByReporterResource[reporterResourceID][reporterVersion]; !ok {
			f.reporterRepsByReporterResource[reporterResourceID][reporterVersion] = make(map[uint]*storedReporterRepresentation)
		}
		f.reporterRepsByReporterResource[reporterResourceID][reporterVersion][reporterResourceSnapshot.Generation] = &storedReporterRepresentation{
			data:       cloneJsonObject(reporterRepresentationSnapshot.Representation.Data),
			version:    reporterVersion,
			generation: reporterResourceSnapshot.Generation,
		}
	}

	if reporterRepresentationSnapshot != nil && reporterRepresentationSnapshot.TransactionId != "" {
		f.markTransactionIdAsProcessed(reporterRepresentationSnapshot.TransactionId)
	}
	if commonRepresentationSnapshot != nil && commonRepresentationSnapshot.TransactionId != "" {
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

	// Find the latest version for the given natural key.
	// Prefer non-tombstoned resources (a live resource from a newer lifecycle always
	// beats a tombstoned one from an older lifecycle), then break ties by highest
	// representation version + generation — matching the real repository's behaviour.
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
				// A live resource always wins over a tombstoned one.
				if !stored.tombstone && latestResource.tombstone {
					latestResource = stored
					continue
				}
				if stored.tombstone && !latestResource.tombstone {
					continue
				}
				// Both have the same tombstone state — pick highest version/generation.
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
			ConsoleHref:           nil, // optional
			RepresentationVersion: latestResource.representationVersion,
			Generation:            latestResource.generation,
			Tombstone:             latestResource.tombstone,
			CreatedAt:             latestResource.createdAt,
			UpdatedAt:             latestResource.updatedAt,
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

// inMemoryRepresentationFetcher implements the representationFetcher interface using in-memory maps.
// findResourceIDs locates the resource_id and reporter_resource_id for a given ReporterResourceKey.
// This mimics the JOIN on reporter_resources table in the real SQL implementation.
func (f *fakeResourceRepository) findResourceIDs(key bizmodel.ReporterResourceKey) (resourceID, reporterResourceID uuid.UUID, found bool) {
	for _, stored := range f.resourcesByPrimaryKey {
		if strings.EqualFold(stored.localResourceID, key.LocalResourceId().Serialize()) &&
			strings.EqualFold(stored.resourceType, key.ResourceType().Serialize()) &&
			strings.EqualFold(stored.reporterType, key.ReporterType().Serialize()) {
			searchReporterInstanceId := key.ReporterInstanceId().Serialize()
			if searchReporterInstanceId == "" || strings.EqualFold(stored.reporterInstanceID, searchReporterInstanceId) {
				return stored.resourceID, stored.reporterResourceID, true
			}
		}
	}
	return uuid.UUID{}, uuid.UUID{}, false
}

type inMemoryRepresentationFetcher struct {
	resourceID         uuid.UUID
	reporterResourceID uuid.UUID
	commonReps         map[uint]*storedCommonRepresentation
	reporterReps       map[uint]map[uint]*storedReporterRepresentation
}

func (m *inMemoryRepresentationFetcher) fetchCommon(version uint) (bizmodel.Representation, *bizmodel.Version) {
	if m.commonReps == nil {
		return nil, nil
	}
	if entry, ok := m.commonReps[version]; ok {
		v := bizmodel.NewVersion(entry.version)
		return bizmodel.Representation(cloneJsonObject(entry.data)), &v
	}
	return nil, nil
}

func (m *inMemoryRepresentationFetcher) fetchReporter(version uint) (bizmodel.Representation, *bizmodel.Version) {
	if m.reporterReps == nil {
		return nil, nil
	}
	// Find current reporter representation (highest generation at this version)
	if generations, ok := m.reporterReps[version]; ok {
		var maxGen uint
		var maxEntry *storedReporterRepresentation
		for gen, entry := range generations {
			if maxEntry == nil || gen > maxGen {
				maxGen = gen
				maxEntry = entry
			}
		}
		if maxEntry != nil {
			v := bizmodel.NewVersion(maxEntry.version)
			return bizmodel.Representation(cloneJsonObject(maxEntry.data)), &v
		}
	}
	return nil, nil
}

func (m *inMemoryRepresentationFetcher) fetchPreviousReporter(currentVersion uint) (bizmodel.Representation, *bizmodel.Version) {
	if m.reporterReps == nil {
		return nil, nil
	}
	// Find previous reporter representation (immediately before current version/generation)
	type versionGen struct {
		version    uint
		generation uint
		entry      *storedReporterRepresentation
	}
	var allReps []versionGen
	for v, generations := range m.reporterReps {
		for g, entry := range generations {
			allReps = append(allReps, versionGen{v, g, entry})
		}
	}
	// Find max version/generation that is less than currentVersion
	var maxRep *versionGen
	for i := range allReps {
		rep := &allReps[i]
		if rep.version < currentVersion {
			if maxRep == nil || rep.version > maxRep.version || (rep.version == maxRep.version && rep.generation > maxRep.generation) {
				maxRep = rep
			}
		}
	}
	if maxRep != nil {
		v := bizmodel.NewVersion(maxRep.entry.version)
		return bizmodel.Representation(cloneJsonObject(maxRep.entry.data)), &v
	}
	return nil, nil
}

func (f *fakeResourceRepository) FindCurrentAndPreviousVersionedRepresentations(
	tx *gorm.DB,
	key bizmodel.ReporterResourceKey,
	currentCommonVersion *bizmodel.Version,
	currentReporterVersion *bizmodel.Version,
	operationType bizmodel.EventOperationType,
) (*bizmodel.Representations, *bizmodel.Representations, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	resourceID, reporterResourceID, found := f.findResourceIDs(key)
	if !found {
		return nil, nil, fmt.Errorf("resource not found for key")
	}

	fetcher := &inMemoryRepresentationFetcher{
		resourceID:         resourceID,
		reporterResourceID: reporterResourceID,
		commonReps:         f.commonRepresentationsByResource[resourceID],
		reporterReps:       f.reporterRepsByReporterResource[reporterResourceID],
	}

	return fetchCurrentAndPreviousRepresentations(fetcher, currentCommonVersion, currentReporterVersion, operationType)
}

func (f *fakeResourceRepository) FindLatestRepresentations(tx *gorm.DB, key bizmodel.ReporterResourceKey) (*bizmodel.Representations, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	resourceID, reporterResourceID, found := f.findResourceIDs(key)
	if !found {
		return nil, fmt.Errorf("resource not found for key")
	}

	// Find latest common representation
	var latestCommon bizmodel.Representation
	var latestCommonVer *bizmodel.Version

	if commonVersions, ok := f.commonRepresentationsByResource[resourceID]; ok {
		var maxVersion uint
		var maxEntry *storedCommonRepresentation
		for version, entry := range commonVersions {
			if maxEntry == nil || version > maxVersion {
				maxVersion = version
				maxEntry = entry
			}
		}
		if maxEntry != nil {
			v := bizmodel.NewVersion(maxEntry.version)
			latestCommon = bizmodel.Representation(cloneJsonObject(maxEntry.data))
			latestCommonVer = &v
		}
	}

	// Find latest reporter representation
	var latestReporter bizmodel.Representation
	var latestReporterVer *bizmodel.Version

	if reporterVersions, ok := f.reporterRepsByReporterResource[reporterResourceID]; ok {
		var maxVersion, maxGen uint
		var maxEntry *storedReporterRepresentation
		for version, generations := range reporterVersions {
			for gen, entry := range generations {
				if maxEntry == nil || version > maxVersion || (version == maxVersion && gen > maxGen) {
					maxVersion = version
					maxGen = gen
					maxEntry = entry
				}
			}
		}
		if maxEntry != nil {
			v := bizmodel.NewVersion(maxEntry.version)
			latestReporter = bizmodel.Representation(cloneJsonObject(maxEntry.data))
			latestReporterVer = &v
		}
	}

	// Must have at least one stream
	if len(latestCommon) == 0 && len(latestReporter) == 0 {
		return nil, fmt.Errorf("no representations found for key")
	}

	return bizmodel.NewRepresentations(latestCommon, latestCommonVer, latestReporter, latestReporterVer)
}

func (f *fakeResourceRepository) GetDB() *gorm.DB {
	// Fake repository doesn't use a real database
	return nil
}

func (f *fakeResourceRepository) GetTransactionManager() bizmodel.TransactionManager {
	// Return a fake transaction manager for testing
	return NewFakeTransactionManager(3) // Default retry count
}

func (f *fakeResourceRepository) makeCompositeKey(localResourceID, reporterType, resourceType, reporterInstanceID string, representationVersion, generation uint) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%d", localResourceID, reporterType, resourceType, reporterInstanceID, representationVersion, generation)
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
func (f *fakeResourceRepository) HasTransactionIdBeenProcessed(tx *gorm.DB, transactionId bizmodel.TransactionId) (bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	_, exists := f.processedTransactionIds[transactionId.String()]
	return exists, nil
}
