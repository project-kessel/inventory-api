package data

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
)

// TestFindCurrentAndPreviousVersionedRepresentations_RHCLOUD49504 is a regression test
// for RHCLOUD-49504 where FindCurrentAndPreviousVersionedRepresentations was only fetching
// common representations and completely ignoring reporter representations.
//
// This caused features workspace tuples to never be replicated to SpiceDB because:
// 1. Features schema defines relations in reporter-specific data (direct_billing_account, direct_service_preferences)
// 2. Consumer called FindCurrentAndPreviousVersionedRepresentations which returned nil reporter data
// 3. CalculateTuplesForResource correctly returned empty tuples (nothing to calculate without reporter data)
// 4. Consumer checked IsEmpty() and skipped SpiceDB replication
//
// The fix added a LEFT JOIN to reporter_representations table so both common AND reporter data are fetched.
func TestFindCurrentAndPreviousVersionedRepresentations_RHCLOUD49504(t *testing.T) {
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, 3)
	repo := NewResourceRepository(db, tm, noopOutboxPublisher())

	t.Run("fetches both common and reporter representations for features workspace", func(t *testing.T) {
		// Create a features workspace resource with BOTH common and reporter data
		resource := createFeaturesWorkspaceResource(t, "features-workspace-123")

		// Save the resource (creates version 0)
		err := repo.Save(db, resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-features-create"))
		require.NoError(t, err)

		// Fetch the representations using the actual repository method
		key, err := bizmodel.NewReporterResourceKey("features-workspace-123", "workspace", "features", "features-instance-1")
		require.NoError(t, err)

		version := bizmodel.NewVersion(0)
		current, previous, err := repo.FindCurrentAndPreviousVersionedRepresentations(db, key, &version, bizmodel.OperationTypeCreated)
		require.NoError(t, err)
		require.NotNil(t, current, "current representation should not be nil")
		require.Nil(t, previous, "previous representation should be nil for version 0")

		// CRITICAL ASSERTIONS: Both common AND reporter data must be present
		// This would have FAILED before the fix (reporter data was always nil)
		assert.True(t, current.HasCommon(), "current representation MUST have common data")
		assert.True(t, current.HasReporter(), "BUG RHCLOUD-49504: current representation MUST have reporter data!")

		// Verify specific fields from common data
		assert.Equal(t, "workspace-uuid-123", current.WorkspaceID(), "workspace_id from common data")

		// Verify specific fields from reporter data
		billingAccount := current.ReporterStringField("direct_billing_account")
		assert.Equal(t, "billing-account-uuid-456", billingAccount, "direct_billing_account from reporter data")

		servicePrefs := current.ReporterStringSliceField("direct_service_preferences")
		require.Len(t, servicePrefs, 1, "direct_service_preferences array should have 1 element")
		assert.Equal(t, "service-uuid-789", servicePrefs[0], "first service preference")
	})

	t.Run("fetches reporter data across version updates", func(t *testing.T) {
		// Create initial resource
		resource := createFeaturesWorkspaceResource(t, "features-workspace-update")
		err := repo.Save(db, resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-features-create-2"))
		require.NoError(t, err)

		// Update with new reporter data
		key, err := bizmodel.NewReporterResourceKey("features-workspace-update", "workspace", "features", "features-instance-1")
		require.NoError(t, err)

		updatedCommon, err := bizmodel.NewRepresentation(map[string]interface{}{
			"workspace_id": "workspace-uuid-updated",
		})
		require.NoError(t, err)

		updatedReporter, err := bizmodel.NewRepresentation(map[string]interface{}{
			"direct_billing_account":     "billing-account-updated",
			"direct_service_preferences": []interface{}{"service-1", "service-2"},
		})
		require.NoError(t, err)

		apiHref, err := bizmodel.NewApiHref("https://api.example.com/workspace/updated")
		require.NoError(t, err)

		txid := bizmodel.NewTransactionId("tx-features-update")
		err = resource.Update(key, apiHref, nil, nil, &updatedReporter, &updatedCommon, txid)
		require.NoError(t, err)

		err = repo.Save(db, resource, bizmodel.OperationTypeUpdated, bizmodel.NewTransactionId("tx-features-update-save"))
		require.NoError(t, err)

		// Fetch current (v1) and previous (v0)
		version := bizmodel.NewVersion(1)
		current, previous, err := repo.FindCurrentAndPreviousVersionedRepresentations(db, key, &version, bizmodel.OperationTypeUpdated)
		require.NoError(t, err)
		require.NotNil(t, current, "current should not be nil")
		require.NotNil(t, previous, "previous should not be nil")

		// Both current and previous MUST have reporter data
		assert.True(t, current.HasCommon(), "current has common data")
		assert.True(t, current.HasReporter(), "current MUST have reporter data")
		assert.True(t, previous.HasCommon(), "previous has common data")
		assert.True(t, previous.HasReporter(), "previous MUST have reporter data")

		// Verify current version has updated values
		assert.Equal(t, "workspace-uuid-updated", current.WorkspaceID())
		assert.Equal(t, "billing-account-updated", current.ReporterStringField("direct_billing_account"))

		// Verify previous version has original values
		assert.Equal(t, "workspace-uuid-123", previous.WorkspaceID())
		assert.Equal(t, "billing-account-uuid-456", previous.ReporterStringField("direct_billing_account"))
	})

	t.Run("fetches both representations for regular resources (HBI host)", func(t *testing.T) {
		// Create a regular resource (e.g., HBI host) - domain model always creates both representations
		resource := createTestResourceWithLocalIdAndType(t, "host-regular", "host")
		err := repo.Save(db, resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-host-regular"))
		require.NoError(t, err)

		key, err := bizmodel.NewReporterResourceKey("host-regular", "host", "hbi", "hbi-instance-1")
		require.NoError(t, err)

		version := bizmodel.NewVersion(0)
		current, _, err := repo.FindCurrentAndPreviousVersionedRepresentations(db, key, &version, bizmodel.OperationTypeCreated)
		require.NoError(t, err)
		require.NotNil(t, current)

		// Regular resources have both common and reporter data
		assert.True(t, current.HasCommon(), "should have common data")
		assert.True(t, current.HasReporter(), "should have reporter data (domain model always creates both)")
	})
}

// TestFindLatestRepresentations_RHCLOUD49504 verifies that FindLatestRepresentations
// also fetches both common and reporter data (same bug as FindCurrentAndPreviousVersionedRepresentations).
func TestFindLatestRepresentations_RHCLOUD49504(t *testing.T) {
	db := setupInMemoryDB(t)
	mc := metricscollector.NewFakeMetricsCollector()
	tm := NewGormTransactionManager(mc, 3)
	repo := NewResourceRepository(db, tm, noopOutboxPublisher())

	t.Run("fetches both common and reporter representations", func(t *testing.T) {
		resource := createFeaturesWorkspaceResource(t, "features-latest-123")
		err := repo.Save(db, resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("tx-latest-create"))
		require.NoError(t, err)

		key, err := bizmodel.NewReporterResourceKey("features-latest-123", "workspace", "features", "features-instance-1")
		require.NoError(t, err)

		latest, err := repo.FindLatestRepresentations(db, key)
		require.NoError(t, err)
		require.NotNil(t, latest)

		// Both common and reporter data must be present
		assert.True(t, latest.HasCommon(), "latest MUST have common data")
		assert.True(t, latest.HasReporter(), "BUG RHCLOUD-49504: latest MUST have reporter data!")

		// Verify data is accessible
		assert.Equal(t, "workspace-uuid-123", latest.WorkspaceID())
		assert.Equal(t, "billing-account-uuid-456", latest.ReporterStringField("direct_billing_account"))
	})
}

// createFeaturesWorkspaceResource creates a test resource mimicking a features workspace
// with both common and reporter representations (as used in RHCLOUD-49504).
func createFeaturesWorkspaceResource(t *testing.T, localResourceId string) bizmodel.Resource {
	t.Helper()

	resourceId := uuid.New()
	reporterResourceId := uuid.New()

	// Reporter data: features-specific fields (direct_billing_account, direct_service_preferences)
	reporterData := internal.JsonObject{
		"direct_billing_account":     "billing-account-uuid-456",
		"direct_service_preferences": []interface{}{"service-uuid-789"},
	}

	// Common data: shared fields like workspace_id
	commonData := internal.JsonObject{
		"workspace_id": "workspace-uuid-123",
	}

	localResourceIdType, err := bizmodel.NewLocalResourceId(localResourceId)
	require.NoError(t, err)

	resourceType, err := bizmodel.NewResourceType("workspace")
	require.NoError(t, err)

	reporterType, err := bizmodel.NewReporterType("features")
	require.NoError(t, err)

	reporterInstanceId, err := bizmodel.NewReporterInstanceId("features-instance-1")
	require.NoError(t, err)

	apiHref, err := bizmodel.NewApiHref("https://api.example.com/workspace/" + localResourceId)
	require.NoError(t, err)

	consoleHref, err := bizmodel.NewConsoleHref("https://console.example.com/workspace/" + localResourceId)
	require.NoError(t, err)

	reporterRepresentation, err := bizmodel.NewRepresentation(reporterData)
	require.NoError(t, err)

	commonRepresentation, err := bizmodel.NewRepresentation(commonData)
	require.NoError(t, err)

	resourceIdType, err := bizmodel.NewResourceId(resourceId)
	require.NoError(t, err)

	reporterResourceIdType, err := bizmodel.NewReporterResourceId(reporterResourceId)
	require.NoError(t, err)

	txid := bizmodel.NewTransactionId("features-workspace-create-" + localResourceId)

	resource, err := bizmodel.NewResource(
		resourceIdType,
		localResourceIdType,
		resourceType,
		reporterType,
		reporterInstanceId,
		txid,
		reporterResourceIdType,
		apiHref,
		&consoleHref,
		&reporterRepresentation,
		&commonRepresentation,
		nil,
	)
	require.NoError(t, err)

	return resource
}
