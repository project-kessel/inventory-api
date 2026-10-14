package jobs

import (
	"database/sql"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Runtime-control gate tests (requirement: pause / resume / stop a running
// production job, then verify data). Table-driven across all four cleanup
// helpers. Each fixture creates 6 deletable rows + keepers; runs use
// batchSize 2 so a full run is 3 batches.
//
// The control table (job_runtime_controls) is manipulated through the
// OBSERVER handle: temp tables are session-local, so the job's connection
// pool must never be touched from outside while it runs.

const controlsTestPollInterval = 10 * time.Millisecond

type controlsFixture struct {
	name      string
	job       string
	setup     func(t *testing.T, db *gorm.DB, now time.Time)
	run       func(db *gorm.DB, logger *log.Helper, opts *retentionRunOptions) (int64, string, error)
	remaining func(t *testing.T, db *gorm.DB, now time.Time) int64
}

func TestPostgresRuntimeControls(t *testing.T) {
	fixtures := newControlsFixtures()

	// G1 — stop requested BEFORE the job starts: nothing is deleted.
	t.Run("stop_before_start", func(t *testing.T) {
		for _, f := range fixtures {
			t.Run(f.name, func(t *testing.T) {
				db, observer := setupPostgresTestDBPair(t)
				logger := testLogger()
				now := time.Now().UTC()
				f.setup(t, db, now)
				require.NoError(t, observer.Exec(
					"INSERT INTO job_runtime_controls (job_name, desired_state, updated_by, note) VALUES (?, 'stop', 'test', 'stop before start')",
					f.job).Error)

				deleted, reason, err := f.run(db, logger, &retentionRunOptions{
					ControlJobName:      f.job,
					ControlPollInterval: controlsTestPollInterval,
				})

				require.NoError(t, err)
				assert.Equal(t, stopReasonControlStop, reason)
				assert.Equal(t, int64(0), deleted)
				assert.Equal(t, int64(6), f.remaining(t, db, now),
					"nothing may be deleted when the job is stopped before it starts")
			})
		}
	})

	// G2 — pause before start, then resume: the run completes exactly like an
	// uncontrolled one.
	t.Run("pause_then_resume", func(t *testing.T) {
		for _, f := range fixtures {
			t.Run(f.name, func(t *testing.T) {
				db, observer := setupPostgresTestDBPair(t)
				logger := testLogger()
				now := time.Now().UTC()
				f.setup(t, db, now)
				require.NoError(t, observer.Exec(
					"INSERT INTO job_runtime_controls (job_name, desired_state, updated_by, note) VALUES (?, 'paused', 'test', 'pause at start')",
					f.job).Error)

				opts := &retentionRunOptions{
					ControlJobName:      f.job,
					ControlPollInterval: controlsTestPollInterval,
				}
				var deleted int64
				var reason string
				var err error
				done := make(chan struct{})
				go func() {
					defer close(done)
					deleted, reason, err = f.run(db, logger, opts)
				}()

				// The job must be parked in the pause loop, not finished.
				time.Sleep(150 * time.Millisecond)
				select {
				case <-done:
					t.Fatal("job completed while it should be paused")
				default:
				}

				require.NoError(t, observer.Exec(
					"UPDATE job_runtime_controls SET desired_state = 'running', updated_by = 'test' WHERE job_name = ?",
					f.job).Error)
				<-done

				require.NoError(t, err)
				assert.Empty(t, reason)
				assert.Equal(t, int64(6), deleted)
				assert.Equal(t, int64(0), f.remaining(t, db, now),
					"a paused-then-resumed run must delete exactly what an uncontrolled run deletes")
			})
		}
	})

	// G3 — pause, then stop while paused: graceful exit, nothing deleted.
	t.Run("pause_then_stop", func(t *testing.T) {
		for _, f := range fixtures {
			t.Run(f.name, func(t *testing.T) {
				db, observer := setupPostgresTestDBPair(t)
				logger := testLogger()
				now := time.Now().UTC()
				f.setup(t, db, now)
				require.NoError(t, observer.Exec(
					"INSERT INTO job_runtime_controls (job_name, desired_state, updated_by, note) VALUES (?, 'paused', 'test', 'pause at start')",
					f.job).Error)

				opts := &retentionRunOptions{
					ControlJobName:      f.job,
					ControlPollInterval: controlsTestPollInterval,
				}
				var deleted int64
				var reason string
				var err error
				done := make(chan struct{})
				go func() {
					defer close(done)
					deleted, reason, err = f.run(db, logger, opts)
				}()

				time.Sleep(150 * time.Millisecond)
				select {
				case <-done:
					t.Fatal("job completed while it should be paused")
				default:
				}

				require.NoError(t, observer.Exec(
					"UPDATE job_runtime_controls SET desired_state = 'stop', updated_by = 'test' WHERE job_name = ?",
					f.job).Error)
				<-done

				require.NoError(t, err)
				assert.Equal(t, stopReasonControlStop, reason)
				assert.Equal(t, int64(0), deleted)
				assert.Equal(t, int64(6), f.remaining(t, db, now))
			})
		}
	})

	// G4 — a signal (SIGTERM, as k8s pod deletion sends) stops the run
	// gracefully before the first batch.
	t.Run("signal", func(t *testing.T) {
		for _, f := range fixtures {
			t.Run(f.name, func(t *testing.T) {
				db, _ := setupPostgresTestDBPair(t)
				logger := testLogger()
				now := time.Now().UTC()
				f.setup(t, db, now)

				sigCh := make(chan os.Signal, 1)
				sigCh <- syscall.SIGTERM

				deleted, reason, err := f.run(db, logger, &retentionRunOptions{StopCh: sigCh})

				require.NoError(t, err)
				assert.Equal(t, stopReasonSignal, reason)
				assert.Equal(t, int64(0), deleted)
				assert.Equal(t, int64(6), f.remaining(t, db, now))
			})
		}
	})

	// G5 — --max-batches stops after exactly N batches.
	t.Run("max_batches", func(t *testing.T) {
		for _, f := range fixtures {
			t.Run(f.name, func(t *testing.T) {
				db, _ := setupPostgresTestDBPair(t)
				logger := testLogger()
				now := time.Now().UTC()
				f.setup(t, db, now)

				deleted, reason, err := f.run(db, logger, &retentionRunOptions{MaxBatches: 1})

				require.NoError(t, err)
				assert.Equal(t, stopReasonMaxBatches, reason)
				assert.Equal(t, int64(2), deleted, "one batch of batchSize 2")
				assert.Equal(t, int64(4), f.remaining(t, db, now))
			})
		}
	})

	// G7 — missing control table (older schema / deploy ordering): the job
	// warns, disables controls, and completes the full run.
	t.Run("missing_control_table", func(t *testing.T) {
		for _, f := range fixtures {
			t.Run(f.name, func(t *testing.T) {
				db, observer := setupPostgresTestDBPair(t)
				logger := testLogger()
				now := time.Now().UTC()
				f.setup(t, db, now)
				require.NoError(t, observer.Exec("DROP TABLE job_runtime_controls").Error)

				deleted, reason, err := f.run(db, logger, &retentionRunOptions{
					ControlJobName:      f.job,
					ControlPollInterval: controlsTestPollInterval,
				})

				require.NoError(t, err)
				assert.Empty(t, reason)
				assert.Equal(t, int64(6), deleted,
					"a missing control table must not prevent the run from completing")
				assert.Equal(t, int64(0), f.remaining(t, db, now))
			})
		}
	})
}

// controlsFixtureByName returns the fixture with the given name ("reporter_reps",
// "common_reps", "orphan", "tombstoned", "cascade") — safer than indexing the slice.
func controlsFixtureByName(name string) controlsFixture {
	for _, f := range newControlsFixtures() {
		if f.name == name {
			return f
		}
	}
	panic("unknown controls fixture: " + name)
}

// A signal that arrives while the job is PAUSED must also stop it (it must
// not be stuck in the pause poll loop until someone resumes it).
func TestPostgresRuntimeControls_SignalWhilePaused(t *testing.T) {
	db, _ := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()
	f := controlsFixtureByName("orphan")

	sigCh := make(chan os.Signal, 1)
	opts := &retentionRunOptions{
		ControlJobName:      f.job,
		ControlPollInterval: controlsTestPollInterval,
		StopCh:              sigCh,
	}
	f.setup(t, db, now)
	require.NoError(t, db.Exec(
		"INSERT INTO job_runtime_controls (job_name, desired_state, updated_by, note) VALUES (?, 'paused', 'test', 'pause at start')",
		f.job).Error)

	var deleted int64
	var reason string
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		deleted, reason, err = f.run(db, logger, opts)
	}()

	time.Sleep(150 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("job completed while it should be paused")
	default:
	}
	sigCh <- syscall.SIGTERM
	<-done

	require.NoError(t, err)
	assert.Equal(t, stopReasonSignal, reason)
	assert.Equal(t, int64(0), deleted)
}

// --vacuum-every-batches: VACUUM (ANALYZE) must actually run on the target
// tables mid-run (observable via pg_stat_user_tables.last_vacuum).
func TestPostgresRuntimeControls_VacuumEveryBatches(t *testing.T) {
	t.Run("orphan_job_vacuums_common_representations", func(t *testing.T) {
		db, observer := setupPostgresTestDBPair(t)
		logger := testLogger()
		now := time.Now().UTC()
		f := controlsFixtureByName("orphan")
		f.setup(t, db, now)

		deleted, reason, err := f.run(db, logger, &retentionRunOptions{
			VacuumEveryBatches: 1,
			VacuumTables:       []string{"common_representations"},
		})

		require.NoError(t, err)
		assert.Empty(t, reason)
		assert.Equal(t, int64(6), deleted)
		var lastVacuum sql.NullTime
		require.NoError(t, observer.Raw(
			"SELECT last_vacuum FROM pg_stat_user_tables WHERE relname = 'common_representations'").Scan(&lastVacuum).Error)
		assert.True(t, lastVacuum.Valid, "VACUUM must have run on common_representations")
	})

	t.Run("tombstone_job_vacuums_both_tables", func(t *testing.T) {
		db, observer := setupPostgresTestDBPair(t)
		logger := testLogger()
		now := time.Now().UTC()
		f := controlsFixtureByName("tombstoned")
		f.setup(t, db, now)

		deleted, reason, err := f.run(db, logger, &retentionRunOptions{
			VacuumEveryBatches: 1,
			VacuumTables:       []string{"reporter_resources", "reporter_representations"},
		})

		require.NoError(t, err)
		assert.Empty(t, reason)
		assert.Equal(t, int64(6), deleted)
		for _, table := range []string{"reporter_resources", "reporter_representations"} {
			var lastVacuum sql.NullTime
			require.NoError(t, observer.Raw(
				"SELECT last_vacuum FROM pg_stat_user_tables WHERE relname = ?", table).Scan(&lastVacuum).Error)
			assert.True(t, lastVacuum.Valid, "VACUUM must have run on %s", table)
		}
	})
}

// isMissingControlTable classification — the fail-closed vs disable decision
// hinges on only the undefined-table SQLSTATE being "controls unavailable".
func TestIsMissingControlTable(t *testing.T) {
	assert.False(t, isMissingControlTable(nil))
	assert.True(t, isMissingControlTable(&pgconn.PgError{Code: "42P01", Message: `relation "job_runtime_controls" does not exist`}),
		"undefined_table (42P01) means the table is absent")
	assert.False(t, isMissingControlTable(&pgconn.PgError{Code: "42703", Message: `column "desired_state" does not exist`}),
		"undefined_column is NOT a missing table — must not silently disable controls")
	assert.False(t, isMissingControlTable(errors.New("connection refused")))
	assert.True(t, isMissingControlTable(errors.New("no such table: job_runtime_controls")),
		"SQLite lane: no such table")
}

// A gate read error that is NOT "table missing" must stop the run gracefully
// (fail-closed) rather than disabling controls and continuing — continuing
// could ignore a pending 'stop'. A missing table, by contrast, only disables
// controls (deploy-ordering safe, pinned end-to-end by missing_control_table).
func TestPostgresRuntimeControls_ReadErrorStopsFailClosed(t *testing.T) {
	db, observer := setupPostgresTestDBPair(t)
	logger := testLogger()

	t.Run("non_missing_table_error_stops", func(t *testing.T) {
		// Break the table in a way that is not "table absent": dropping the
		// desired_state column makes the gate's SELECT fail with an
		// undefined-column error.
		require.NoError(t, observer.Exec(
			"ALTER TABLE job_runtime_controls DROP COLUMN desired_state CASCADE").Error)

		opts := &retentionRunOptions{ControlJobName: "controls_test_read_error"}
		reason := checkBatchGate(db, logger, opts, 0)
		assert.Equal(t, stopReasonControlStop, reason,
			"a non-missing-table read error must stop the run, not disable controls")
	})

	t.Run("missing_table_disables_controls", func(t *testing.T) {
		require.NoError(t, observer.Exec("DROP TABLE job_runtime_controls").Error)

		opts := &retentionRunOptions{ControlJobName: "controls_test_read_error"}
		reason := checkBatchGate(db, logger, opts, 0)
		assert.Empty(t, reason, "a missing control table must not stop the run")
		assert.Empty(t, opts.ControlJobName, "controls are disabled for the rest of the run")
	})
}

// A PAUSED job whose control table becomes unreadable must not resume
// deletion on its own (fail-closed): after a few failed polls it stops
// gracefully, resumably, with nothing deleted by that run.
func TestPostgresRuntimeControls_PausedThenTableUnreadable(t *testing.T) {
	db, observer := setupPostgresTestDBPair(t)
	logger := testLogger()
	now := time.Now().UTC()
	f := controlsFixtureByName("orphan")
	f.setup(t, db, now)
	require.NoError(t, db.Exec(
		"INSERT INTO job_runtime_controls (job_name, desired_state, updated_by, note) VALUES (?, 'paused', 'test', 'pause at start')",
		f.job).Error)

	opts := &retentionRunOptions{
		ControlJobName:      f.job,
		ControlPollInterval: controlsTestPollInterval,
	}
	var deleted int64
	var reason string
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		deleted, reason, err = f.run(db, logger, opts)
	}()

	time.Sleep(150 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("job completed while it should be paused")
	default:
	}
	require.NoError(t, observer.Exec("DROP TABLE job_runtime_controls").Error)
	<-done

	require.NoError(t, err)
	assert.Equal(t, stopReasonControlStop, reason,
		"a paused job with an unreadable control table must stop gracefully, not resume on its own")
	assert.Equal(t, int64(0), deleted)
}

// newControlsFixtures returns one fixture per cleanup helper, in the order
// reporter_reps, common_reps, orphan, tombstoned, cascade. Every fixture
// creates exactly 6 deletable rows (plus keepers) and runs with batchSize 2
// and a short batch delay.
func newControlsFixtures() []controlsFixture {
	return []controlsFixture{
		{
			name: "reporter_reps",
			job:  "controls_test_reporter_reps",
			setup: func(t *testing.T, db *gorm.DB, now time.Time) {
				reporterResourceID := uuid.New()
				resourceID := uuid.New()
				createPGResource(t, db, resourceID)
				createPGReporterResource(t, db, reporterResourceID, resourceID, "hbi", false, daysAgo(now, 60), daysAgo(now, 60))
				for i := 0; i < 6; i++ {
					createPGReporterRepresentation(t, db, reporterResourceID, uint(i), 1, daysAgo(now, 25+i))
				}
				createPGReporterRepresentation(t, db, reporterResourceID, 6, 1, daysAgo(now, 2))
			},
			run: func(db *gorm.DB, logger *log.Helper, opts *retentionRunOptions) (int64, string, error) {
				return deleteOldReporterRepresentations(db, logger, false, 7, 30, "", 2, 30, 0, opts)
			},
			remaining: func(t *testing.T, db *gorm.DB, now time.Time) int64 {
				return countPGRows(t, db, &model.ReporterRepresentation{}, "created_at < ?", daysAgo(now, 9))
			},
		},
		{
			name: "common_reps",
			job:  "controls_test_common_reps",
			setup: func(t *testing.T, db *gorm.DB, now time.Time) {
				resourceID := uuid.New()
				createPGResource(t, db, resourceID)
				for i := 0; i < 6; i++ {
					createPGCommonRepresentation(t, db, resourceID, uint(i), "hbi", daysAgo(now, 25+i))
				}
				createPGCommonRepresentation(t, db, resourceID, 6, "hbi", daysAgo(now, 2))
			},
			run: func(db *gorm.DB, logger *log.Helper, opts *retentionRunOptions) (int64, string, error) {
				return deleteOldCommonRepresentations(db, logger, false, 7, 30, "", 2, 30, 0, opts)
			},
			remaining: func(t *testing.T, db *gorm.DB, now time.Time) int64 {
				return countPGRows(t, db, &model.CommonRepresentation{}, "created_at < ?", daysAgo(now, 9))
			},
		},
		{
			name: "orphan",
			job:  "controls_test_orphan",
			setup: func(t *testing.T, db *gorm.DB, now time.Time) {
				orphanedResource := uuid.New()
				createPGResource(t, db, orphanedResource)
				for i := 0; i < 6; i++ {
					createPGCommonRepresentation(t, db, orphanedResource, uint(i), "hbi", daysAgo(now, 25+i))
				}
				keeperResource := uuid.New()
				createPGResource(t, db, keeperResource)
				reporterResourceID := uuid.New()
				createPGReporterResource(t, db, reporterResourceID, keeperResource, "hbi", false, daysAgo(now, 60), daysAgo(now, 60))
				createPGCommonRepresentation(t, db, keeperResource, 0, "hbi", daysAgo(now, 30))
			},
			run: func(db *gorm.DB, logger *log.Helper, opts *retentionRunOptions) (int64, string, error) {
				return deleteOrphanedCommonRepresentations(db, logger, false, "", 2, 30, 0, opts)
			},
			remaining: func(t *testing.T, db *gorm.DB, now time.Time) int64 {
				var n int64
				require.NoError(t, db.Raw(
					"SELECT COUNT(*) FROM common_representations cr WHERE NOT EXISTS (SELECT 1 FROM reporter_resources rr WHERE rr.resource_id = cr.resource_id)").Scan(&n).Error)
				return n
			},
		},
		{
			name: "tombstoned",
			job:  "controls_test_tombstoned",
			setup: func(t *testing.T, db *gorm.DB, now time.Time) {
				for i := 0; i < 6; i++ {
					id := uuid.New()
					resourceID := uuid.New()
					createPGResource(t, db, resourceID)
					createPGReporterResource(t, db, id, resourceID, "hbi", true, daysAgo(now, 60), daysAgo(now, 31+i))
				}
				recentTombstone := uuid.New()
				recentResource := uuid.New()
				createPGResource(t, db, recentResource)
				createPGReporterResource(t, db, recentTombstone, recentResource, "hbi", true, daysAgo(now, 60), daysAgo(now, 5))
			},
			run: func(db *gorm.DB, logger *log.Helper, opts *retentionRunOptions) (int64, string, error) {
				return deleteOldTombstonedResources(db, logger, false, 30, "", 2, 30, 0, opts)
			},
			remaining: func(t *testing.T, db *gorm.DB, now time.Time) int64 {
				return countPGRows(t, db, &model.ReporterResource{}, "tombstone = ? AND updated_at < ?", true, daysAgo(now, 30))
			},
		},
		{
			// The cascade helper's deleted count is its reporter_resources
			// count; its common reps die in the interleaved orphan pass and
			// the final drain (skipped after a partial stop, like the
			// standalone orphan job).
			name: "cascade",
			job:  "controls_test_cascade",
			setup: func(t *testing.T, db *gorm.DB, now time.Time) {
				for i := 0; i < 6; i++ {
					id := uuid.New()
					resourceID := uuid.New()
					createPGResource(t, db, resourceID)
					createPGReporterResource(t, db, id, resourceID, "hbi", true, daysAgo(now, 60), daysAgo(now, 31+i))
					createPGCommonRepresentation(t, db, resourceID, 0, "hbi", daysAgo(now, 40))
				}
				recentTombstone := uuid.New()
				recentResource := uuid.New()
				createPGResource(t, db, recentResource)
				createPGReporterResource(t, db, recentTombstone, recentResource, "hbi", true, daysAgo(now, 60), daysAgo(now, 5))
				createPGCommonRepresentation(t, db, recentResource, 0, "hbi", daysAgo(now, 4))
			},
			run: func(db *gorm.DB, logger *log.Helper, opts *retentionRunOptions) (int64, string, error) {
				result, err := deleteOldTombstonedResourcesCascade(db, logger, false, 30, DeletionOrderOldestFirst, "", 2, 10, 30, 0, opts)
				return result.ReporterResourcesDeleted, result.StopReason, err
			},
			remaining: func(t *testing.T, db *gorm.DB, now time.Time) int64 {
				return countPGRows(t, db, &model.ReporterResource{}, "tombstone = ? AND updated_at < ?", true, daysAgo(now, 30))
			},
		},
	}
}
