package data

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
)

// newRepresentationHistoryRepository seeds deliberately sparse history, including
// earlier and later generations, so the database must select the right rows.
func newRepresentationHistoryRepository(t *testing.T, deleted bool) (bizmodel.ResourceRepository, bizmodel.ReporterResourceKey) {
	t.Helper()
	repository, db := setupResourceRepository(t)
	resource := createTestResourceWithLocalId(t, "query-history")
	require.NoError(t, repository.Save(db, resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("query-history-seed")))
	key := createContractReporterResourceKey(t, "query-history", "k8s_cluster", "ocm", "ocm-instance-1")

	resourceSnapshot, reporterResourceSnapshot, _, _, err := resource.Serialize()
	require.NoError(t, err)
	for _, version := range []uint{41, 42} {
		representation, err := datamodel.NewCommonRepresentation(resourceSnapshot.ID,
			internal.JsonObject{"workspace_id": fmt.Sprintf("workspace-v%d", version)},
			version, "ocm", "ocm-instance-1", "")
		require.NoError(t, err)
		require.NoError(t, db.Create(representation).Error)
	}
	require.NoError(t, db.Where("reporter_resource_id = ?", reporterResourceSnapshot.ID).
		Delete(&datamodel.ReporterRepresentation{}).Error)
	for _, row := range []struct {
		generation, version uint
		tombstone           bool
		data                string
	}{
		{0, 99, false, "earlier-generation"},
		{1, 6, false, "v6"},
		{1, 7, deleted, "v7"},
		{1, 8, true, ""},
		{2, 0, false, "later-generation"},
	} {
		data := internal.JsonObject{}
		if !row.tombstone {
			data["payload"] = row.data
		}
		representation, err := datamodel.NewReporterRepresentation(data, reporterResourceSnapshot.ID,
			row.version, row.generation, nil, "", row.tombstone, nil)
		require.NoError(t, err)
		require.NoError(t, db.Create(representation).Error)
	}
	return repository, key
}

// TestFindCurrentAndPreviousVersionedRepresentationsCommonOnlyUpdate verifies
// that the database selects the requested and previous common versions without
// returning an unadvanced reporter stream.
func TestFindCurrentAndPreviousVersionedRepresentationsCommonOnlyUpdate(t *testing.T) {
	repository, key := newRepresentationHistoryRepository(t, false)
	versions := bizmodel.NewRepresentationVersions(ptrVersion(42), nil, nil)

	current, previous, err := repository.FindCurrentAndPreviousVersionedRepresentations(
		nil, key, versions, bizmodel.OperationTypeUpdated)
	require.NoError(t, err)

	require.NotNil(t, current)
	require.NotNil(t, current.CommonVersion())
	require.Equal(t, uint(42), current.CommonVersion().Uint())
	require.Equal(t, "workspace-v42", current.WorkspaceID())
	require.Nil(t, current.ReporterVersion())
	require.Nil(t, current.ReporterData(), "an unadvanced reporter stream must not be loaded")

	require.NotNil(t, previous)
	require.NotNil(t, previous.CommonVersion())
	require.Equal(t, uint(41), previous.CommonVersion().Uint())
	require.Equal(t, "workspace-v41", previous.WorkspaceID())
	require.Nil(t, previous.ReporterVersion())
	require.Nil(t, previous.ReporterData())
}

// TestFindCurrentAndPreviousVersionedRepresentationsReporterOnlyUpdate verifies
// that the database selects the requested reporter generation and its predecessor
// without returning an unadvanced common stream.
func TestFindCurrentAndPreviousVersionedRepresentationsReporterOnlyUpdate(t *testing.T) {
	repository, key := newRepresentationHistoryRepository(t, false)
	versions := bizmodel.NewRepresentationVersions(nil, ptrVersion(7), ptrGeneration(1))

	current, previous, err := repository.FindCurrentAndPreviousVersionedRepresentations(
		nil, key, versions, bizmodel.OperationTypeUpdated)
	require.NoError(t, err)

	require.NotNil(t, current)
	require.Nil(t, current.CommonVersion(), "an unadvanced common stream must not be loaded")
	require.Nil(t, current.CommonData())
	require.NotNil(t, current.ReporterVersion())
	require.Equal(t, uint(7), current.ReporterVersion().Uint())
	require.Equal(t, "v7", current.ReporterData()["payload"])

	require.NotNil(t, previous)
	require.Nil(t, previous.CommonVersion())
	require.Nil(t, previous.CommonData())
	require.NotNil(t, previous.ReporterVersion())
	require.Equal(t, uint(6), previous.ReporterVersion().Uint())
	require.Equal(t, "v6", previous.ReporterData()["payload"])
}

// TestFindCurrentAndPreviousVersionedRepresentationsDeleteLoadsLastLiveReporter
// verifies that the database skips tombstones across a version gap and excludes
// later generations when selecting the last live state for a delete.
func TestFindCurrentAndPreviousVersionedRepresentationsDeleteLoadsLastLiveReporter(t *testing.T) {
	repository, key := newRepresentationHistoryRepository(t, true)
	versions := bizmodel.NewRepresentationVersions(ptrVersion(42), ptrVersion(8), ptrGeneration(1))

	current, previous, err := repository.FindCurrentAndPreviousVersionedRepresentations(
		nil, key, versions, bizmodel.OperationTypeDeleted)
	require.NoError(t, err)

	require.NotNil(t, current)
	require.NotNil(t, current.CommonVersion())
	require.Equal(t, uint(42), current.CommonVersion().Uint())
	require.Equal(t, "workspace-v42", current.WorkspaceID())
	require.NotNil(t, current.ReporterVersion())
	require.Equal(t, uint(6), current.ReporterVersion().Uint(), "delete must read the last live reporter row")
	require.Equal(t, "v6", current.ReporterData()["payload"])
	require.Nil(t, previous, "a delete only needs the last live state")
}
