package model

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test for CodeRabbit finding: delete after reporter-only update must preserve lastCommonVersion
// This test demonstrates the bug where r.commonVersion is nil after a reporter-only update,
// but the delete event should use r.lastCommonVersion to enable tuple cleanup.
func TestResource_Delete_PreservesLastCommonVersionAfterReporterOnlyUpdate(t *testing.T) {
	t.Parallel()
	fixture := NewResourceTestFixture()

	t.Run("delete after reporter-only update should include last common version", func(t *testing.T) {
		t.Parallel()

		// Step 1: Create resource with both reporter and common representations
		reporterRep := fixture.ValidReporterRepresentationType()
		commonRep := fixture.ValidCommonRepresentationType()
		resource, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			NewTransactionId("tx-create"),
			fixture.ValidReporterResourceIdType(),
			fixture.ValidApiHrefType(),
			fixture.ValidConsoleHrefType(),
			&reporterRep,
			&commonRep,
			nil,
		)
		require.NoError(t, err)

		// Verify initial state: commonVersion and lastCommonVersion both = 0
		assert.NotNil(t, resource.commonVersion, "commonVersion should be set after creation")
		assert.NotNil(t, resource.lastCommonVersion, "lastCommonVersion should be set after creation")
		assert.Equal(t, uint(0), resource.commonVersion.Uint(), "initial commonVersion should be 0")
		assert.Equal(t, uint(0), resource.lastCommonVersion.Uint(), "initial lastCommonVersion should be 0")

		// Step 2: Perform reporter-only update (no common representation)
		// This should clear r.commonVersion but preserve r.lastCommonVersion
		key, err := NewReporterResourceKey(
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)
		require.NoError(t, err)

		reporterData := internal.JsonObject{"reporter_field": "reporter-only-value"}
		reporterRep2, err := NewRepresentation(reporterData)
		require.NoError(t, err)

		apiHref := fixture.ValidApiHrefType()
		txID := NewTransactionId("tx-reporter-only")
		err = resource.Update(
			key,
			apiHref,
			nil, // no console href
			nil, // no reporter version
			&reporterRep2,
			nil, // nil common representation - reporter-only update
			txID,
		)
		require.NoError(t, err)

		// Verify state after reporter-only update:
		// - commonVersion should be nil (cleared because no common representation)
		// - lastCommonVersion should still be 0 (preserved from initial state)
		assert.Nil(t, resource.commonVersion, "commonVersion should be nil after reporter-only update")
		assert.NotNil(t, resource.lastCommonVersion, "lastCommonVersion should be preserved after reporter-only update")
		assert.Equal(t, uint(0), resource.lastCommonVersion.Uint(), "lastCommonVersion should still be 0")

		// Step 3: Delete the resource
		err = resource.Delete(key)
		require.NoError(t, err)

		// Step 4: Verify the delete event contains the last common version (not nil)
		deleteEvents := resource.ResourceDeleteEvents()
		require.Len(t, deleteEvents, 1, "should have one delete event")

		deleteEvent := deleteEvents[0]
		commonVersion := deleteEvent.CurrentCommonVersion()

		// BUG REPRODUCTION: This assertion will FAIL with the current code
		// because Delete() uses r.commonVersion (nil) instead of r.lastCommonVersion (0)
		assert.NotNil(t, commonVersion, "delete event should preserve last common version, not nil")
		if commonVersion != nil {
			assert.Equal(t, uint(0), commonVersion.Uint(), "delete event should have last common version = 0")
		}
	})

	t.Run("delete after common+reporter update should include current common version", func(t *testing.T) {
		t.Parallel()

		// Create resource
		reporterRep := fixture.ValidReporterRepresentationType()
		commonRep := fixture.ValidCommonRepresentationType()
		resource, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			NewTransactionId("tx-create"),
			fixture.ValidReporterResourceIdType(),
			fixture.ValidApiHrefType(),
			fixture.ValidConsoleHrefType(),
			&reporterRep,
			&commonRep,
			nil,
		)
		require.NoError(t, err)

		// Perform update with both common and reporter representations
		key, err := NewReporterResourceKey(
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)
		require.NoError(t, err)

		reporterData2 := internal.JsonObject{"reporter_field": "updated"}
		reporterRep2, err := NewRepresentation(reporterData2)
		require.NoError(t, err)

		commonData2 := internal.JsonObject{"workspace_id": "updated-workspace"}
		commonRep2, err := NewRepresentation(commonData2)
		require.NoError(t, err)

		apiHref2 := fixture.ValidApiHrefType()
		txID2 := NewTransactionId("tx-both-update")
		err = resource.Update(
			key,
			apiHref2,
			nil,
			nil,
			&reporterRep2,
			&commonRep2, // common representation provided
			txID2,
		)
		require.NoError(t, err)

		// After update with common representation, both should be version 1
		assert.NotNil(t, resource.commonVersion, "commonVersion should be set after update")
		assert.NotNil(t, resource.lastCommonVersion, "lastCommonVersion should be set after update")
		assert.Equal(t, uint(1), resource.commonVersion.Uint(), "commonVersion should be 1")
		assert.Equal(t, uint(1), resource.lastCommonVersion.Uint(), "lastCommonVersion should be 1")

		// Delete
		err = resource.Delete(key)
		require.NoError(t, err)

		// Verify delete event has common version 1
		deleteEvents := resource.ResourceDeleteEvents()
		require.Len(t, deleteEvents, 1, "should have one delete event")

		deleteEvent := deleteEvents[0]
		commonVersion := deleteEvent.CurrentCommonVersion()

		assert.NotNil(t, commonVersion, "delete event should have common version")
		if commonVersion != nil {
			assert.Equal(t, uint(1), commonVersion.Uint(), "delete event should have common version = 1")
		}
	})
}

// Test 1.3: Characterization test for common-only update asymmetry
// Documents that representationVersion increments even when no reporter representation is written.
// This creates version gaps that the delete path must tolerate.
func TestResource_CommonOnlyUpdate_IncrementVersionWithoutRepresentation(t *testing.T) {
	t.Parallel()
	fixture := NewResourceTestFixture()

	t.Run("common-only update increments representationVersion but event has no reporter representation", func(t *testing.T) {
		t.Parallel()

		// Create resource with reporter data
		reporterRep := fixture.ValidReporterRepresentationType()
		commonRep := fixture.ValidCommonRepresentationType()
		resource, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			NewTransactionId("tx-create"),
			fixture.ValidReporterResourceIdType(),
			fixture.ValidApiHrefType(),
			fixture.ValidConsoleHrefType(),
			&reporterRep,
			&commonRep,
			nil,
		)
		require.NoError(t, err)

		// Initial state: representationVersion should be 0
		_, reporterResourceSnap, _, _, err := resource.Serialize()
		require.NoError(t, err)
		initialVersion := reporterResourceSnap.RepresentationVersion
		assert.Equal(t, uint(0), initialVersion, "initial representation version should be 0")

		// Create event should have reporter representation
		createEvents := resource.ResourceReportEvents()
		require.Len(t, createEvents, 1, "should have one create event")
		assert.NotNil(t, createEvents[0].reporterRepresentation, "create event should have reporter representation")

		// Perform common-only update (nil reporter data)
		key, err := NewReporterResourceKey(
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
		)
		require.NoError(t, err)

		commonData := internal.JsonObject{"workspace_id": "updated-workspace"}
		commonRep2, err := NewRepresentation(commonData)
		require.NoError(t, err)

		apiHref := fixture.ValidApiHrefType()
		txID := NewTransactionId("tx-update-common")
		err = resource.Update(
			key,
			apiHref,
			nil, // no console href
			nil, // no reporter version
			nil, // nil reporter data - this is the key: common-only update
			&commonRep2,
			txID,
		)
		require.NoError(t, err)

		// representationVersion should have incremented to 1
		_, reporterResourceSnap2, _, _, err := resource.Serialize()
		require.NoError(t, err)
		afterUpdateVersion := reporterResourceSnap2.RepresentationVersion
		assert.Equal(t, uint(1), afterUpdateVersion, "representation version should increment to 1")

		// But the event should NOT have a reporter representation
		updateEvents := resource.ResourceReportEvents()
		require.Len(t, updateEvents, 1, "should have one update event")
		assert.Nil(t, updateEvents[0].reporterRepresentation, "common-only update event should have nil reporter representation")
		assert.NotNil(t, updateEvents[0].commonRepresentation, "common-only update event should have common representation")

		// This is the asymmetry: version advanced from 0 to 1, but no v1 row will be written.
		// The delete path must tolerate this gap.
	})
}
