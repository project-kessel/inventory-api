package usecase

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
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

// TestUpsertResource_NoResourceExists tests scenarios where no resource exists
func TestUpsertResource_NoResourceExists(t *testing.T) {
	tests := []struct {
		name                           string
		request                        *pb.ReportResourceRequest
		expectedResourceCount          int
		expectedCommonRepCount         int
		expectedReporterRepCount       int
		expectedRepresentationRefCount int
		expectedRefTypes               []string // Expected reporter types in representation references
	}{
		{
			name:                           "WithBothRepresentations",
			request:                        testdata.ReportResourceRequestBothRepresentations(),
			expectedResourceCount:          1,
			expectedCommonRepCount:         1,
			expectedReporterRepCount:       1,
			expectedRepresentationRefCount: 2,
			expectedRefTypes:               []string{"inventory", "hbi"},
		},
		{
			name:                           "WithOnlyReporterRepresentation",
			request:                        testdata.ReportResourceRequestReporterOnly(),
			expectedResourceCount:          1,
			expectedCommonRepCount:         0, // Should NOT create common representation when not provided
			expectedReporterRepCount:       1,
			expectedRepresentationRefCount: 1, // Should only create reporter reference
			expectedRefTypes:               []string{"hbi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := context.Background()

			// Set up dependencies
			commonRepo := datav1beta2.NewFakeCommonRepresentationRepository()
			reporterRepo := datav1beta2.NewFakeReporterRepresentationRepository()
			resourceRepo := datav1beta2.NewFakeResourceWithReferencesRepository()
			transactionManager := data.NewFakeTransactionManager()

			// Create ResourceUsecase with fake dependencies
			usecase := NewResourceUsecase(
				commonRepo,
				reporterRepo,
				resourceRepo,
				nil, // DB not needed with fake transaction manager
				transactionManager,
				nil, // MetricsCollector not needed for this test
				"test-namespace",
				testLogger(),
			)

			// Act
			err := usecase.ReportResource(ctx, tt.request)

			// Assert
			require.NoError(t, err, "ReportResource should succeed")

			// Assert all entities were created correctly
			assertResourceCreation(t, resourceRepo, tt.expectedResourceCount)
			assertReporterRepresentationCreation(t, reporterRepo, tt.expectedReporterRepCount, tt.expectedCommonRepCount > 0)
			assertCommonRepresentationCreation(t, commonRepo, tt.expectedCommonRepCount)
			assertRepresentationReferencesCreation(t, resourceRepo, tt.expectedResourceCount, tt.expectedRepresentationRefCount, tt.expectedRefTypes)
		})
	}
}

// assertResourceCreation verifies that the expected number of resources were created with correct structure
func assertResourceCreation(t *testing.T, resourceRepo *datav1beta2.FakeResourceWithReferencesRepository, expectedCount int) {
	t.Helper()

	resources := resourceRepo.GetAllResources()
	require.Len(t, resources, expectedCount, "Should have created expected number of resources")

	if expectedCount > 0 {
		expectedResource := testdata.ExpectedResource(resources[0].ID)
		assert.Equal(t, expectedResource, resources[0], "Resource should match expected structure")
		assert.NotEqual(t, uuid.Nil, resources[0].ID, "Resource ID should be generated")
	}
}

// assertReporterRepresentationCreation verifies that the expected number of reporter representations were created
func assertReporterRepresentationCreation(t *testing.T, reporterRepo *datav1beta2.FakeReporterRepresentationRepository, expectedCount int, hasCommonRep bool) {
	t.Helper()

	reporterReps := reporterRepo.GetAll()
	require.Len(t, reporterReps, expectedCount, "Should have created expected number of reporter representations")

	if expectedCount > 0 {
		var expectedReporterRep *v1beta2.ReporterRepresentation
		if hasCommonRep {
			expectedReporterRep = testdata.ExpectedReporterRepresentation(
				reporterReps[0].Data,      // Use actual serialized data
				reporterReps[0].CreatedAt, // Use actual timestamp
				reporterReps[0].UpdatedAt, // Use actual timestamp
			)
		} else {
			expectedReporterRep = testdata.ExpectedReporterRepresentationReporterOnly(
				reporterReps[0].Data,      // Use actual serialized data
				reporterReps[0].CreatedAt, // Use actual timestamp
				reporterReps[0].UpdatedAt, // Use actual timestamp
			)
		}
		assert.Equal(t, expectedReporterRep, reporterReps[0], "ReporterRepresentation should match expected structure")
		assert.NotNil(t, reporterReps[0].Data, "ReporterRepresentation data should not be nil")
	}
}

// assertCommonRepresentationCreation verifies that the expected number of common representations were created
func assertCommonRepresentationCreation(t *testing.T, commonRepo *datav1beta2.FakeCommonRepresentationRepository, expectedCount int) {
	t.Helper()

	commonReps := commonRepo.GetAll()
	require.Len(t, commonReps, expectedCount, "Should have created expected number of common representations")

	if expectedCount > 0 {
		expectedCommonRep := testdata.ExpectedCommonRepresentation(
			commonReps[0].Data,            // Use actual serialized data
			commonReps[0].CreatedAt,       // Use actual timestamp
			commonReps[0].UpdatedAt,       // Use actual timestamp
			commonReps[0].LocalResourceID, // Use actual generated ID
			commonReps[0].ReportedBy,      // Use actual reportedBy format
		)
		assert.Equal(t, expectedCommonRep, commonReps[0], "CommonRepresentation should match expected structure")
		assert.NotNil(t, commonReps[0].Data, "CommonRepresentation data should not be nil")
	}
}

// assertRepresentationReferencesCreation verifies that the expected representation references were created
func assertRepresentationReferencesCreation(t *testing.T, resourceRepo *datav1beta2.FakeResourceWithReferencesRepository, expectedResourceCount, expectedRefCount int, expectedRefTypes []string) {
	t.Helper()

	aggregates := resourceRepo.GetAllResourceAggregates()
	require.Len(t, aggregates, expectedResourceCount, "Should have created expected number of aggregates")

	if expectedResourceCount > 0 {
		resourceID := aggregates[0].Resource.ID
		refs := aggregates[0].RepresentationReferences
		require.Len(t, refs, expectedRefCount, "Should have created expected number of representation references")

		// Verify expected reference types
		actualRefTypes := make([]string, len(refs))
		for i, ref := range refs {
			actualRefTypes[i] = ref.ReporterType
		}
		assert.ElementsMatch(t, expectedRefTypes, actualRefTypes, "Should have expected reporter types in references")

		// Verify specific reference structures
		for _, ref := range refs {
			if ref.ReporterType == "inventory" {
				expectedInventoryRef := testdata.ExpectedCommonRepresentationReference(resourceID, ref.LocalResourceID)
				assert.Equal(t, expectedInventoryRef, ref, "Common representation reference should match expected structure")
			} else if ref.ReporterType == "hbi" {
				expectedReporterRef := testdata.ExpectedReporterRepresentationReference(resourceID)
				assert.Equal(t, expectedReporterRef, ref, "Reporter representation reference should match expected structure")
			}
		}
	}
}

// testLogger creates a test logger
func testLogger() log.Logger {
	return log.DefaultLogger
}
