package data

import (
	"testing"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/require"
)

func TestLastLiveReporterBeforeBoundaries(t *testing.T) {
	db := setupInMemoryDB(t)
	repo := NewResourceRepository(db, NewFakeTransactionManager(3), noopOutboxPublisher())
	resource := createTestResourceWithLocalId(t, "history-boundaries")
	require.NoError(t, repo.Save(db, resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("history-seed")))
	key := createContractReporterResourceKey(t, "history-boundaries", "k8s_cluster", "ocm", "ocm-instance-1")
	require.NoError(t, db.Exec("DELETE FROM reporter_representations").Error)
	// A high version in an earlier generation must sort before a low version
	// in the next generation. Tombstones and missing versions are intentional.
	for _, row := range []struct {
		generation, version uint
		tombstone           bool
	}{{0, 8, false}, {0, 9, true}, {1, 0, false}, {1, 2, true}, {1, 4, false}} {
		require.NoError(t, db.Exec(`INSERT INTO reporter_representations
			(reporter_resource_id, generation, version, tombstone, data)
			SELECT id, ?, ?, ?, ? FROM reporter_resources`, row.generation, row.version, row.tombstone, []byte(`{}`)).Error)
	}
	for _, tc := range []struct {
		name                string
		generation, version uint
		want                *bizmodel.Version
		wantPrevious        *bizmodel.Version
	}{
		{"before history", 0, 8, nil, nil},
		{"skip tombstone", 0, 10, ptrVersion(8), ptrVersion(9)},
		{"previous generation", 1, 0, ptrVersion(8), ptrVersion(9)},
		{"version gap", 1, 2, ptrVersion(0), ptrVersion(0)},
		{"strict boundary", 1, 4, ptrVersion(0), ptrVersion(2)},
		{"latest live", 1, 5, ptrVersion(4), ptrVersion(4)},
		{"future generation", 2, 0, ptrVersion(4), ptrVersion(4)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, version, err := repo.(*resourceRepository).fetchLastLiveReporterBefore(db, key, tc.version, tc.generation)
			require.NoError(t, err)
			require.Equal(t, tc.want, version)
			_, previousVersion, err := repo.(*resourceRepository).fetchPreviousReporterRepresentation(db, key, tc.version, tc.generation)
			require.NoError(t, err)
			require.Equal(t, tc.wantPrevious, previousVersion)
		})
	}
}
