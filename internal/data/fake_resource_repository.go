package data

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase"
)

type fakeResourceRepository struct {
	mu        sync.RWMutex
	resources map[string]*storedResource
	// Optional test overrides for workspace IDs returned by FindCommonRepresentationsByVersion
	overrideCurrent  string
	overridePrevious string
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

// NewFakeResourceRepositoryWithWorkspaceOverrides allows tests to control the
// workspace IDs returned for current and previous versions.
func NewFakeResourceRepositoryWithWorkspaceOverrides(current, previous string) ResourceRepository {
	return &fakeResourceRepository{
		resources:        make(map[string]*storedResource),
		overrideCurrent:  current,
		overridePrevious: previous,
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
		if strings.EqualFold(stored.localResourceID, key.LocalResourceId().Serialize()) &&
			strings.EqualFold(stored.resourceType, key.ResourceType().Serialize()) &&
			strings.EqualFold(stored.reporterType, key.ReporterType().Serialize()) &&
			!stored.tombstone {

			// If search key has empty reporterInstanceId, match any stored resource
			// If search key has reporterInstanceId, it must match exactly
			if searchReporterInstanceId == "" || strings.EqualFold(stored.reporterInstanceID, searchReporterInstanceId) {
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

func (f *fakeResourceRepository) FindVersionedRepresentationsByVersion(tx *gorm.DB, key bizmodel.ReporterResourceKey, currentVersion uint) ([]VersionedRepresentation, error) {
	// This is a fake implementation for testing
	// In a real test, you would mock this based on your test data needs
	var results []VersionedRepresentation

	// Prefer explicit overrides when provided by tests
	if f.overrideCurrent != "" {
		results = append(results, VersionedRepresentation{Data: map[string]interface{}{"workspace_id": f.overrideCurrent}, Version: currentVersion, Kind: RepresentationKindCommon})
		if f.overridePrevious != "" && currentVersion > 0 {
			results = append(results, VersionedRepresentation{Data: map[string]interface{}{"workspace_id": f.overridePrevious}, Version: currentVersion - 1, Kind: RepresentationKindCommon})
		}
		return results, nil
	}

	// For testing purposes, we'll return mock data based on the version
	// In a real implementation, this would query the database for common_representations

	// Mock data for testing - you can customize this based on your test needs
	switch currentVersion {
	case 0:
		// Version 0 - initial creation
		results = append(results, VersionedRepresentation{Data: map[string]interface{}{"workspace_id": "test-workspace-initial"}, Version: currentVersion, Kind: RepresentationKindCommon})
	case 1:
		// Version 1 - first update
		results = append(results, VersionedRepresentation{Data: map[string]interface{}{"workspace_id": "test-workspace-v1"}, Version: currentVersion, Kind: RepresentationKindCommon})
		// Also include previous (version 0) for contract parity with real repo
		results = append(results, VersionedRepresentation{Data: map[string]interface{}{"workspace_id": "test-workspace-previous"}, Version: 0, Kind: RepresentationKindCommon})
	case 2:
		// Version 2 - workspace change scenario
		results = append(results, VersionedRepresentation{Data: map[string]interface{}{"workspace_id": "test-workspace-v2"}, Version: currentVersion, Kind: RepresentationKindCommon})

		// Add previous (current-1) version if requested
		if currentVersion > 0 {
			results = append(results, VersionedRepresentation{Data: map[string]interface{}{"workspace_id": "test-workspace-previous"}, Version: currentVersion - 1, Kind: RepresentationKindCommon})
		}
	}

	return results, nil
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
