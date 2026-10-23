//go:build integration

package reporterrepcleanup

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestReporterRepCleanupPostgres exercises the actual PostgreSQL selector and
// transactional batches, including generation changes and missing pointer rows.
func TestReporterRepCleanupPostgres(t *testing.T) {
	db, _ := setupReporterRepCleanupPostgres(t)
	options := reporterRepCleanupOptions{keepVersions: 2, batchSize: 3}
	type key struct{ Generation, Version int }
	seed := func(resourceID uuid.UUID, generation, version int, tombstone bool, keys ...key) uuid.UUID {
		t.Helper()
		require.NoError(t, db.Exec("INSERT INTO resource(id,type) VALUES (?, 'host') ON CONFLICT DO NOTHING", resourceID).Error)
		id := uuid.New()
		require.NoError(t, db.Exec(`INSERT INTO reporter_resources(id,resource_id,local_resource_id,reporter_type,resource_type,reporter_instance_id,api_href,generation,representation_version,tombstone)
   VALUES (?, ?, ?, 'hbi','host','cleanup-test','https://example.test',?,?,?)`, id, resourceID, id.String(), generation, version, tombstone).Error)
		for index, k := range keys {
			// Deliberately scramble timestamps; historical tombstones remain rankable.
			require.NoError(t, db.Exec(`INSERT INTO reporter_representations(reporter_resource_id,generation,version,data,transaction_id,tombstone,created_at)
    VALUES (?,?,?,'{}',?,?,now() - (? * INTERVAL '1 day'))`, id, k.Generation, k.Version, fmt.Sprintf("%s-%d", id, index), index%2 == 1, index%3*40).Error)
		}
		return id
	}
	retained := func(id uuid.UUID) []key {
		t.Helper()
		var keys []key
		require.NoError(t, db.Raw("SELECT generation,version FROM reporter_representations WHERE reporter_resource_id=? ORDER BY generation,version", id).Scan(&keys).Error)
		return keys
	}
	t.Run("per-reporter history across generations and gaps regardless of age", func(t *testing.T) {
		isolateReporterRepCleanupPostgresSubtest(t, db)
		resource := uuid.New()
		cases := []struct {
			id   uuid.UUID
			want []key
		}{
			{seed(resource, 0, 10, false, key{0, 0}, key{0, 4}, key{0, 8}), []key{{0, 4}, {0, 8}}}, // Missing exact pointer is legitimate.
			{seed(resource, 2, 0, false, key{0, 100}, key{1, 7}, key{1, 8}, key{2, 0}), []key{{1, 8}, {2, 0}}},
			{seed(uuid.New(), 1, 1, false, key{0, 9}, key{1, 0}, key{1, 1}, key{1, 2}, key{2, 0}), []key{{1, 0}, {1, 1}, {1, 2}, {2, 0}}},
			{seed(uuid.New(), 0, 3, true, key{0, 0}, key{0, 1}, key{0, 2}, key{0, 3}), []key{{0, 0}, {0, 1}, {0, 2}, {0, 3}}},
			{seed(uuid.New(), 0, 1, false, key{0, 0}, key{0, 1}), []key{{0, 0}, {0, 1}}},
			{seed(uuid.New(), 0, 0, false, key{0, 0}), []key{{0, 0}}},
			{seed(uuid.New(), 0, 0, false), nil},
		}
		preview, err := cleanupReporterRepresentations(context.Background(), db, options, false, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(4), preview.candidateRows)
		require.Zero(t, preview.deletedRows)
		require.Len(t, retained(cases[0].id), 3)
		capped := options
		capped.maxBatches = 1
		first, err := cleanupReporterRepresentations(context.Background(), db, capped, true, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(3), first.deletedRows)
		require.False(t, first.complete)
		second, err := cleanupReporterRepresentations(context.Background(), db, options, true, nil, func(_ int, rows int64, _ time.Duration) { require.LessOrEqual(t, rows, int64(3)) })
		require.NoError(t, err)
		require.Equal(t, int64(1), second.deletedRows)
		require.True(t, second.complete)
		for _, item := range cases {
			assert.Equal(t, item.want, retained(item.id))
		}
		final, err := cleanupReporterRepresentations(context.Background(), db, options, true, nil, nil)
		require.NoError(t, err)
		require.Zero(t, final.deletedRows)
		require.True(t, final.complete)
	})
	t.Run("keep three", func(t *testing.T) {
		isolateReporterRepCleanupPostgresSubtest(t, db)
		id := seed(uuid.New(), 0, 4, false, key{0, 0}, key{0, 1}, key{0, 2}, key{0, 3}, key{0, 4})
		keepThree := options
		keepThree.keepVersions = 3
		result, err := cleanupReporterRepresentations(context.Background(), db, keepThree, true, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(2), result.deletedRows)
		assert.Equal(t, []key{{0, 2}, {0, 3}, {0, 4}}, retained(id))
	})
	t.Run("cancellation after commit and fresh rerun", func(t *testing.T) {
		isolateReporterRepCleanupPostgresSubtest(t, db)
		id := seed(uuid.New(), 0, 6, false, key{0, 0}, key{0, 1}, key{0, 2}, key{0, 3}, key{0, 4}, key{0, 5}, key{0, 6})
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		defer sqlDB.SetMaxOpenConns(16)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result, err := cleanupReporterRepresentations(ctx, db, options, true, nil, func(_ int, _ int64, _ time.Duration) { cancel() })
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, int64(3), result.deletedRows)
		require.Len(t, retained(id), 4)
		result, err = cleanupReporterRepresentations(context.Background(), db, options, true, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(2), result.deletedRows)
	})
	t.Run("failure counts only earlier committed batches", func(t *testing.T) {
		isolateReporterRepCleanupPostgresSubtest(t, db)
		id := seed(uuid.New(), 0, 6, false, key{0, 0}, key{0, 1}, key{0, 2}, key{0, 3}, key{0, 4}, key{0, 5}, key{0, 6})
		require.NoError(t, db.Exec(`CREATE FUNCTION reporter_cleanup_fail() RETURNS trigger AS $$ BEGIN IF OLD.version=4 THEN RAISE EXCEPTION 'injected failure'; END IF; RETURN OLD; END; $$ LANGUAGE plpgsql`).Error)
		require.NoError(t, db.Exec(`CREATE TRIGGER reporter_cleanup_fail BEFORE DELETE ON reporter_representations FOR EACH ROW EXECUTE FUNCTION reporter_cleanup_fail()`).Error)
		t.Cleanup(func() {
			_ = db.Exec("DROP TRIGGER IF EXISTS reporter_cleanup_fail ON reporter_representations").Error
			_ = db.Exec("DROP FUNCTION IF EXISTS reporter_cleanup_fail()").Error
		})
		result, err := cleanupReporterRepresentations(context.Background(), db, options, true, nil, nil)
		require.ErrorContains(t, err, "injected failure")
		require.Equal(t, int64(3), result.deletedRows)
		require.Equal(t, 1, result.committedBatches)
		assert.Equal(t, []key{{0, 3}, {0, 4}, {0, 5}, {0, 6}}, retained(id))
	})
	t.Run("history and worklist roll back together including mismatch", func(t *testing.T) {
		isolateReporterRepCleanupPostgresSubtest(t, db)
		id := seed(uuid.New(), 0, 3, false, key{0, 0}, key{0, 1}, key{0, 2}, key{0, 3})
		require.NoError(t, db.Connection(func(connection *gorm.DB) error {
			require.NoError(t, connection.Exec(`CREATE TEMP TABLE reporter_rep_cleanup_candidates(reporter_resource_id uuid,generation bigint,version bigint,PRIMARY KEY(reporter_resource_id,generation,version))`).Error)
			defer connection.Exec("DROP TABLE pg_temp.reporter_rep_cleanup_candidates")
			query, args := reporterRepCleanupCandidateQuery(options)
			require.NoError(t, connection.Exec("INSERT INTO pg_temp.reporter_rep_cleanup_candidates ?", gorm.Expr(query, args...)).Error)
			rollback := errors.New("rollback batch")
			err := reporterRepCleanupTransaction(context.Background(), connection, false, func(tx *gorm.DB) error {
				deleted, err := deleteReporterRepCleanupBatch(tx, 1)
				require.NoError(t, err)
				require.Equal(t, int64(1), deleted)
				return rollback
			})
			require.ErrorIs(t, err, rollback)
			var count int64
			require.NoError(t, connection.Raw("SELECT count(*) FROM pg_temp.reporter_rep_cleanup_candidates").Scan(&count).Error)
			require.Equal(t, int64(2), count)
			require.Len(t, retained(id), 4)
			require.NoError(t, connection.Exec("INSERT INTO pg_temp.reporter_rep_cleanup_candidates VALUES (?,0,-1)", id).Error)
			err = reporterRepCleanupTransaction(context.Background(), connection, false, func(tx *gorm.DB) error { _, err := deleteReporterRepCleanupBatch(tx, 3); return err })
			require.ErrorContains(t, err, "worklist mismatch")
			require.NoError(t, connection.Raw("SELECT count(*) FROM pg_temp.reporter_rep_cleanup_candidates").Scan(&count).Error)
			require.Equal(t, int64(3), count)
			require.Len(t, retained(id), 4)
			return nil
		}))
	})
	t.Run("100000 row fixture including deep history", func(t *testing.T) {
		isolateReporterRepCleanupPostgresSubtest(t, db)
		id := seed(uuid.New(), 0, 49999, false)
		require.NoError(t, db.Exec(`INSERT INTO reporter_representations(reporter_resource_id,generation,version,data,tombstone)
   SELECT ?,0,n,'{}',FALSE FROM generate_series(0,49999)n`, id).Error)
		require.NoError(t, db.Exec(`INSERT INTO resource(id,type) SELECT ('00000000-0000-4000-8000-'||lpad(n::text,12,'0'))::uuid,'host' FROM generate_series(1,10000)n`).Error)
		require.NoError(t, db.Exec(`INSERT INTO reporter_resources(id,resource_id,local_resource_id,reporter_type,resource_type,reporter_instance_id,api_href,generation,representation_version,tombstone)
   SELECT ('00000000-0000-4000-9000-'||lpad(n::text,12,'0'))::uuid,('00000000-0000-4000-8000-'||lpad(n::text,12,'0'))::uuid,'perf-'||n,'hbi','host','cleanup-test','https://example.test',0,4,FALSE FROM generate_series(1,10000)n`).Error)
		require.NoError(t, db.Exec(`INSERT INTO reporter_representations(reporter_resource_id,generation,version,data,tombstone)
   SELECT ('00000000-0000-4000-9000-'||lpad(n::text,12,'0'))::uuid,0,v,'{}',FALSE FROM generate_series(1,10000)n CROSS JOIN generate_series(0,4)v`).Error)
		perf := options
		perf.batchSize = 1000
		perf.maxBatches = 1
		result, err := cleanupReporterRepresentations(context.Background(), db, perf, true, nil, func(_ int, _ int64, duration time.Duration) { t.Logf("100000 rows: first batch=%s", duration) })
		require.NoError(t, err)
		require.Equal(t, int64(79998), result.candidateRows)
		require.Equal(t, int64(1000), result.deletedRows)
		t.Logf("100000 rows: selection=%s", result.selectionDuration)
	})
}

// setupReporterRepCleanupPostgres starts an isolated database with the real schema
// and registers container and connection cleanup with the test.
func setupReporterRepCleanupPostgres(t *testing.T) (*gorm.DB, testcontainers.Container) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	password := uuid.NewString()
	postgresContainer, err := testcontainers.Run(
		ctx,
		"postgres:17.8",
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER":     "reporter_rep_cleanup",
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       "reporter_rep_cleanup",
		}),
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(3*time.Minute)),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		if err := postgresContainer.Terminate(cleanupContext); err != nil {
			t.Errorf("terminate reporter representation cleanup PostgreSQL test container: %v", err)
		}
	})

	host, err := postgresContainer.Host(ctx)
	require.NoError(t, err)
	port, err := postgresContainer.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	dsn := fmt.Sprintf("host=%s port=%s user=reporter_rep_cleanup password=%s dbname=reporter_rep_cleanup sslmode=disable", host, port.Port(), password)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(16)
	require.NoError(t, data.Migrate(db, nil))
	// Migrations may bind their GORM session to a dedicated advisory-lock connection.
	require.NoError(t, sqlDB.Close())
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	sqlDB, err = db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(16)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, postgresContainer
}

// isolateReporterRepCleanupPostgresSubtest clears resource fixtures between subtests.
func isolateReporterRepCleanupPostgresSubtest(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("TRUNCATE TABLE reporter_representations, reporter_resources, common_representations, resource CASCADE").Error)
}
