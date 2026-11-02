package data

import (
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

// The two-stream read layer must fetch each representation with a predicate that
// PostgreSQL can turn into an index lookup. common_representations_pkey is
// (resource_id, version), so version has to be compared for equality: a range,
// OR, or IN predicate on version degrades into a filter over every version of the
// resource (see RHCLOUD-52565 / PR #1538, where the OR form cost ~3344 versus ~22).
const (
	commonRepresentationFetchSQL = "SELECT cr.data, cr.version " +
		"FROM reporter_resources rr " +
		"JOIN common_representations cr ON rr.resource_id = cr.resource_id " +
		"WHERE rr.local_resource_id = $1 " +
		"AND rr.resource_type = $2 " +
		"AND rr.reporter_type = $3 " +
		"AND rr.reporter_instance_id = $4 " +
		"AND cr.version = $5 " +
		"LIMIT $6"

	reporterRepresentationFetchSQL = "SELECT rrep.data, rrep.version " +
		"FROM reporter_resources rr " +
		"JOIN reporter_representations rrep ON rr.id = rrep.reporter_resource_id " +
		"WHERE rr.local_resource_id = $1 " +
		"AND rr.resource_type = $2 " +
		"AND rr.reporter_type = $3 " +
		"AND rr.reporter_instance_id = $4 " +
		"AND (rrep.version = $5 AND rrep.generation = $6) " +
		"LIMIT $7"
)

// newQueryShapeRepository returns a repository backed by a sqlmock connection so
// tests can assert the exact SQL emitted for a lookup.
func newQueryShapeRepository(t *testing.T) (bizmodel.ResourceRepository, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)

	return NewResourceRepository(db, nil, nil), mock
}

func queryShapeResourceKey(t *testing.T) bizmodel.ReporterResourceKey {
	t.Helper()

	key, err := bizmodel.NewReporterResourceKey("resource-1", "host", "hbi", "instance-1")
	require.NoError(t, err)

	return key
}

// lateralReporterHistorySQL matches the generation-ordered history lookup. The
// ordering and limit have to be applied inside the lateral subquery: applying
// them after the join makes PostgreSQL scan and sort the resource's whole
// reporter history, which is what got the original implementation reverted.
func lateralReporterHistorySQL(fragments ...string) string {
	escaped := make([]string, 0, len(fragments))
	for _, fragment := range fragments {
		escaped = append(escaped, regexp.QuoteMeta(fragment))
	}

	return "(?s)" + strings.Join(escaped, `.*`)
}

// TestFindCurrentAndPreviousVersionedRepresentationsUsesIndexableCommonLookup
// guards against reintroducing a version predicate that defeats
// common_representations_pkey. An update advances only the common stream, so the
// fetch must be two equality lookups (current version and the one before it) and
// nothing else.
func TestFindCurrentAndPreviousVersionedRepresentationsUsesIndexableCommonLookup(t *testing.T) {
	repository, mock := newQueryShapeRepository(t)
	key := queryShapeResourceKey(t)

	mock.ExpectQuery(regexp.QuoteMeta(commonRepresentationFetchSQL)).
		WithArgs("resource-1", "host", "hbi", "instance-1", uint(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"data", "version"}).
			AddRow([]byte(`{"workspace_id":"workspace-v42"}`), 42))
	mock.ExpectQuery(regexp.QuoteMeta(commonRepresentationFetchSQL)).
		WithArgs("resource-1", "host", "hbi", "instance-1", uint(41), 1).
		WillReturnRows(sqlmock.NewRows([]string{"data", "version"}).
			AddRow([]byte(`{"workspace_id":"workspace-v41"}`), 41))

	commonVersion := bizmodel.NewVersion(42)
	versions := bizmodel.NewRepresentationVersions(&commonVersion, nil, nil)

	current, previous, err := repository.FindCurrentAndPreviousVersionedRepresentations(
		nil, key, versions, bizmodel.OperationTypeUpdated)
	require.NoError(t, err)

	require.NotNil(t, current)
	require.Equal(t, uint(42), current.CommonVersion().Uint())
	require.Equal(t, "workspace-v42", current.WorkspaceID())
	require.Nil(t, current.ReporterData(), "an unadvanced reporter stream must not be loaded")

	require.NotNil(t, previous)
	require.Equal(t, uint(41), previous.CommonVersion().Uint())
	require.Equal(t, "workspace-v41", previous.WorkspaceID())
}

// TestFindCurrentAndPreviousVersionedRepresentationsReporterOnlyUpdate verifies
// that a reporter-only event loads the reporter stream by generation and version
// and skips the common stream entirely.
func TestFindCurrentAndPreviousVersionedRepresentationsReporterOnlyUpdate(t *testing.T) {
	repository, mock := newQueryShapeRepository(t)
	key := queryShapeResourceKey(t)

	mock.ExpectQuery(regexp.QuoteMeta(reporterRepresentationFetchSQL)).
		WithArgs("resource-1", "host", "hbi", "instance-1", uint(7), uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"data", "version"}).
			AddRow([]byte(`{"payload":"v7"}`), 7))
	mock.ExpectQuery(lateralReporterHistorySQL(
		"JOIN LATERAL (",
		"(candidate.generation, candidate.version) < ($1, $2)",
		"ORDER BY candidate.generation DESC, candidate.version DESC",
	)).
		WithArgs(uint(1), uint(7), "resource-1", "host", "hbi", "instance-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"data", "version"}).
			AddRow([]byte(`{"payload":"v6"}`), 6))

	reporterVersion := bizmodel.NewVersion(7)
	generation := bizmodel.NewGeneration(1)
	versions := bizmodel.NewRepresentationVersions(nil, &reporterVersion, &generation)

	current, previous, err := repository.FindCurrentAndPreviousVersionedRepresentations(
		nil, key, versions, bizmodel.OperationTypeUpdated)
	require.NoError(t, err)

	require.NotNil(t, current)
	require.Nil(t, current.CommonVersion(), "an unadvanced common stream must not be loaded")
	require.Equal(t, uint(7), current.ReporterVersion().Uint())

	require.NotNil(t, previous)
	require.Equal(t, uint(6), previous.ReporterVersion().Uint())
}

// TestFindCurrentAndPreviousVersionedRepresentationsDeleteLoadsLastLiveReporter
// verifies the delete path: the common representation is read at the event's
// (unchanged) version, and the reporter stream is read with the gap-tolerant,
// tombstone-filtered lateral lookup bounded by the tombstone's coordinates.
func TestFindCurrentAndPreviousVersionedRepresentationsDeleteLoadsLastLiveReporter(t *testing.T) {
	repository, mock := newQueryShapeRepository(t)
	key := queryShapeResourceKey(t)

	mock.ExpectQuery(regexp.QuoteMeta(commonRepresentationFetchSQL)).
		WithArgs("resource-1", "host", "hbi", "instance-1", uint(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"data", "version"}).
			AddRow([]byte(`{"workspace_id":"workspace-v42"}`), 42))
	mock.ExpectQuery(lateralReporterHistorySQL(
		"JOIN LATERAL (",
		"(candidate.generation, candidate.version) < ($1, $2)",
		"AND candidate.tombstone = false",
		"ORDER BY candidate.generation DESC, candidate.version DESC",
	)).
		WithArgs(uint(1), uint(8), "resource-1", "host", "hbi", "instance-1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"data", "version"}).
			AddRow([]byte(`{"payload":"last-live"}`), 6))

	commonVersion := bizmodel.NewVersion(42)
	// The delete event carries the tombstone version, so the last live reporter
	// data sits at an earlier version.
	tombstoneVersion := bizmodel.NewVersion(8)
	generation := bizmodel.NewGeneration(1)
	versions := bizmodel.NewRepresentationVersions(&commonVersion, &tombstoneVersion, &generation)

	current, previous, err := repository.FindCurrentAndPreviousVersionedRepresentations(
		nil, key, versions, bizmodel.OperationTypeDeleted)
	require.NoError(t, err)

	require.NotNil(t, current)
	require.Equal(t, uint(42), current.CommonVersion().Uint())
	require.Equal(t, uint(6), current.ReporterVersion().Uint(), "delete must read the last live reporter row")
	require.Nil(t, previous, "a delete only needs the last live state")
}
