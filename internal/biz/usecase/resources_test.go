package usecase

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/project-kessel/inventory-api/internal/biz/usecase/testdata"
	"github.com/project-kessel/inventory-api/internal/data"
	datav1beta2 "github.com/project-kessel/inventory-api/internal/data/v1beta2"
)

// Test file for v1beta2 resource operations
// This test uses inmemory Fakes instead of mocks

/* Tests to migrate from resources_test.go
1. TestComputeReadAfterWrite
2. TestCheck_MissingResourc
3. TestCheck_ResourceExistsError
4. TestCheck_ErrorWithKessel
5. TestCheck_Allowed
6. TestCheckForUpdate_ResourceExistsError
7. TestCheckForUpdate_ErrorWithKessel
8. TestCheckForUpdate_WorkspaceAllowed
9. TestCheckForUpdate_MissingResource_Allowed
10. TestCheckForUpdate_Allowed
11. TestUpsertReturnsDbError
12. TestUpsertReturnsExistingUpdatedResource
13. TestUpsert_ReadAfterWrite
14. TestUpsert_ConsumerDisabled
15. TestUpsert_WaitCircuitBreaker
*/

/* Scenarios to add tests for with updated algorithm
1. No Resource exists, ReportResource with both representations
2. No Resource exists, ReportResource with only reporter representations
3. Resource exists, ReportResource with both representations
4. Resource exists, ReportResource with only reporter representation
5. Resource exists, ReportResource with only common representation
*/

// TestUpsertResource_NoResourceExists_WithBothRepresentations tests the scenario where:
// - No ResourceWithReferences exists for the given ReporterRepresentationId
// - UpsertResource is called with both CommonRepresentation and ReporterRepresentation
// - Expected: Creates new Resource, CommonRepresentation, ReporterRepresentation, and RepresentationReferences
func TestUpsertResource_NoResourceExists_WithBothRepresentations(t *testing.T) {
	// Arrange
	ctx := context.Background()
	request := testdata.HostReportResourceRequest()

	// Set up fake dependencies
	fakeCommonRepo := datav1beta2.NewFakeCommonRepresentationRepository()
	fakeReporterRepo := datav1beta2.NewFakeReporterRepresentationRepository()
	fakeResourceRepo := datav1beta2.NewFakeResourceWithReferencesRepository()
	fakeTransactionManager := data.NewFakeTransactionManager()

	// Create ResourceUsecase with fake dependencies
	usecase := NewResourceUsecase(
		fakeCommonRepo,
		fakeReporterRepo,
		fakeResourceRepo,
		nil, // DB not needed with fake transaction manager
		fakeTransactionManager,
		nil, // MetricsCollector not needed for this test
		"test-namespace",
		testLogger(),
	)

	// Act
	err := usecase.UpsertResource(ctx, request)

	// Assert
	require.NoError(t, err, "UpsertResource should succeed")

	// Verify that repositories were called and data was stored
	// Check that common representation was created
	commonReps := fakeCommonRepo.GetAll()
	assert.Len(t, commonReps, 1, "Should have created one common representation")
	if len(commonReps) > 0 {
		assert.Equal(t, "inventory", commonReps[0].ReporterType)
		assert.Equal(t, "host", commonReps[0].ResourceType)
		assert.Equal(t, 1, commonReps[0].Version)
		assert.NotNil(t, commonReps[0].Data)
	}

	// Check that reporter representation was created
	reporterReps := fakeReporterRepo.GetAll()
	assert.Len(t, reporterReps, 1, "Should have created one reporter representation")
	if len(reporterReps) > 0 {
		assert.Equal(t, "hbi", reporterReps[0].ReporterType)
		assert.Equal(t, "host", reporterReps[0].ResourceType)
		assert.Equal(t, 1, reporterReps[0].Version)
		assert.Equal(t, "3088be62-1c60-4884-b133-9200542d0b3f", reporterReps[0].ReporterInstanceID)
		assert.Equal(t, "dd1b73b9-3e33-4264-968c-e3ce55b9afec", reporterReps[0].LocalResourceID)
		assert.NotNil(t, reporterReps[0].Data)
	}

	// Check that resource with references was created
	resources := fakeResourceRepo.GetAllResources()
	assert.Len(t, resources, 1, "Should have created one resource")
	if len(resources) > 0 {
		assert.Equal(t, "host", resources[0].Type)
		assert.NotEqual(t, "", resources[0].ID.String())
	}

	// Verify test fixture data
	assert.Equal(t, "host", request.Type)
	assert.Equal(t, "hbi", request.ReporterType)
	assert.Equal(t, "3088be62-1c60-4884-b133-9200542d0b3f", request.ReporterInstanceId)
	assert.NotNil(t, request.Representations)
	assert.NotNil(t, request.Representations.Common)
	assert.NotNil(t, request.Representations.Reporter)
	assert.NotNil(t, request.Representations.Metadata)
}

// testLogger creates a test logger
func testLogger() log.Logger {
	return log.DefaultLogger
}
