package jobs

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// Stop reasons returned by the retention helpers' batch gate. The empty
// string means "keep running". A run stopped for any reason is graceful and
// resumable: the next run deletes the remainder.
const (
	stopReasonTimeout     = "timeout"
	stopReasonControlStop = "control_stop"
	stopReasonSignal      = "signal"
	stopReasonMaxBatches  = "max_batches"
)

// defaultControlPollInterval is how often a paused job re-checks the
// job_runtime_controls table. Tests override it via ControlPollInterval.
const defaultControlPollInterval = 5 * time.Second

// maxConsecutiveControlReadErrors is how many consecutive failed control-table
// reads a PAUSED job tolerates before stopping gracefully (fail-closed)
// instead of resuming deletion on its own.
const maxConsecutiveControlReadErrors = 3

// retentionRunOptions carries the optional runtime controls shared by all
// four retention cleanup helpers. A nil *retentionRunOptions means "no
// controls": no control-table checks, no signal handling, no batch limit, no
// incremental vacuum — exactly the pre-controls behavior.
type retentionRunOptions struct {
	// MaxBatches stops the run gracefully after N delete batches (0 = off).
	MaxBatches int
	// VacuumEveryBatches runs VACUUM (ANALYZE) on VacuumTables every N
	// batches (0 = off).
	VacuumEveryBatches int
	VacuumTables       []string
	// StopCh aborts the run gracefully at the next batch boundary when
	// signaled (e.g. SIGTERM/SIGINT; k8s pod deletion sends SIGTERM).
	StopCh <-chan os.Signal
	// ControlJobName is the job's key in job_runtime_controls. Empty
	// disables control-table checks entirely.
	ControlJobName string
	// ControlPollInterval is how often a paused job re-checks the control
	// table (0 = defaultControlPollInterval).
	ControlPollInterval time.Duration
}

// initRuntimeControl registers the job in job_runtime_controls (keeping any
// pre-existing row, so an operator can pause/stop a job before it starts).
// A missing table (older schema) only disables controls for the run.
func initRuntimeControl(db *gorm.DB, logHelper *log.Helper, opts *retentionRunOptions) {
	if opts == nil || opts.ControlJobName == "" {
		return
	}
	err := db.Exec(`
		INSERT INTO job_runtime_controls (job_name, desired_state, updated_by, note)
		VALUES (?, 'running', 'job', 'row registered automatically at job start')
		ON CONFLICT (job_name) DO NOTHING
	`, opts.ControlJobName).Error
	if err != nil {
		// Deploy-ordering safe: an older schema without the table must not
		// break the job — controls are simply disabled for this run.
		logHelper.Warnf("job_runtime_controls unavailable, runtime controls disabled for this run: %v", err)
		opts.ControlJobName = ""
	}
}

// checkBatchGate is called at the top of every delete batch. It returns a
// non-empty stop reason when the run should stop gracefully, and blocks
// (polling the control table) while the job is paused.
func checkBatchGate(db *gorm.DB, logHelper *log.Helper, opts *retentionRunOptions, batchCount int) string {
	if opts == nil {
		return ""
	}

	// Deterministic batch limit.
	if opts.MaxBatches > 0 && batchCount >= opts.MaxBatches {
		logHelper.Infof("Stopping after %d batches (--max-batches); rerun the job to delete the remainder", batchCount)
		return stopReasonMaxBatches
	}

	// OS signal (non-blocking; k8s pod deletion sends SIGTERM).
	if opts.StopCh != nil {
		select {
		case <-opts.StopCh:
			logHelper.Info("Received stop signal — stopping gracefully at batch boundary (resumable)")
			return stopReasonSignal
		default:
		}
	}

	// Control table.
	if opts.ControlJobName != "" {
		var state string
		err := db.Raw("SELECT desired_state FROM job_runtime_controls WHERE job_name = ?", opts.ControlJobName).Scan(&state).Error
		if err != nil {
			if isMissingControlTable(err) {
				// Deploy-ordering safe: an older schema without the table must
				// not break the job — controls are simply disabled for this run.
				logHelper.Warnf("job_runtime_controls unavailable, runtime controls disabled for this run: %v", err)
				opts.ControlJobName = ""
			} else {
				// Fail-closed on any other read error: continuing could ignore
				// a pending 'stop'. A spurious stop only costs a rerun; an
				// ignored stop costs unwanted deletions.
				logHelper.Warnf("Failed to read job_runtime_controls — stopping gracefully at batch boundary (fail-closed, resumable): %v", err)
				return stopReasonControlStop
			}
		} else if state == "paused" {
			return waitForResume(db, logHelper, opts)
		} else if state == "stop" {
			logHelper.Info("Runtime control requested stop — stopping gracefully at batch boundary (resumable)")
			return stopReasonControlStop
		}
	}

	return ""
}

// isMissingControlTable reports whether err means the job_runtime_controls
// table itself is absent (Postgres SQLSTATE 42P01 undefined_table, or SQLite's
// "no such table"). Any OTHER error — e.g. a missing column — is not treated
// as "controls unavailable", so it cannot silently disable controls.
func isMissingControlTable(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "42P01" // SQLSTATE undefined_table
	}
	// Non-Postgres engines (SQLite unit-test lane).
	return strings.Contains(err.Error(), "no such table")
}

// waitForResume blocks while the job is paused, re-checking the control table
// every ControlPollInterval. It returns a stop reason if the pause turned
// into a stop (or a signal arrived) while waiting; "" to continue running.
// Control-table read errors are FAIL-CLOSED while paused: an operator pauses
// the job during an incident, so a transient read error must not auto-resume
// deletion — the job stays paused and, if the table stays unreadable, stops
// gracefully (resumable) instead of resuming on its own.
func waitForResume(db *gorm.DB, logHelper *log.Helper, opts *retentionRunOptions) string {
	interval := opts.ControlPollInterval
	if interval <= 0 {
		interval = defaultControlPollInterval
	}
	consecutiveReadErrors := 0
	logHelper.Info("Paused via job_runtime_controls — worklist preserved, waiting for resume")
	for {
		// A signal while paused must still abort promptly.
		if opts.StopCh != nil {
			select {
			case <-opts.StopCh:
				logHelper.Info("Received stop signal while paused — stopping gracefully (resumable)")
				return stopReasonSignal
			default:
			}
		}

		time.Sleep(interval)

		var state string
		err := db.Raw("SELECT desired_state FROM job_runtime_controls WHERE job_name = ?", opts.ControlJobName).Scan(&state).Error
		if err != nil {
			consecutiveReadErrors++
			if consecutiveReadErrors >= maxConsecutiveControlReadErrors {
				logHelper.Warnf("job_runtime_controls unreadable for %d consecutive polls while paused — stopping gracefully (fail-closed, resumable): %v",
					consecutiveReadErrors, err)
				return stopReasonControlStop
			}
			logHelper.Warnf("Failed to read job_runtime_controls while paused — staying paused (attempt %d): %v", consecutiveReadErrors, err)
			continue
		}
		consecutiveReadErrors = 0
		switch state {
		case "paused":
			continue
		case "stop":
			logHelper.Info("Runtime control requested stop while paused — stopping gracefully (resumable)")
			return stopReasonControlStop
		default: // "running" or the row was deleted
			logHelper.Info("Resumed via job_runtime_controls")
			return ""
		}
	}
}

// maybeVacuum runs VACUUM (ANALYZE) on the configured tables every
// VacuumEveryBatches batches, reclaiming reuse-space and refreshing planner
// statistics mid-run. This is ONLINE (SHARE UPDATE EXCLUSIVE — no DML
// blocking) but NEVER returns disk to the OS: the final pg_repack stage in
// the production runbook remains required. Failures are logged, non-fatal.
func maybeVacuum(db *gorm.DB, logHelper *log.Helper, opts *retentionRunOptions, batchCount int) {
	if opts == nil || opts.VacuumEveryBatches <= 0 || len(opts.VacuumTables) == 0 {
		return
	}
	if batchCount%opts.VacuumEveryBatches != 0 {
		return
	}
	for _, table := range opts.VacuumTables {
		start := time.Now()
		// Table names come from the job files' hardcoded lists, never user
		// input — Sprintf is safe here, but they must never become dynamic.
		if err := db.Exec(fmt.Sprintf("VACUUM (ANALYZE) %s", table)).Error; err != nil {
			logHelper.Warnf("VACUUM (ANALYZE) %s failed (non-fatal): %v", table, err)
		} else {
			logHelper.Infof("VACUUM (ANALYZE) %s completed in %v", table, time.Since(start))
		}
	}
}

// newRetentionRunOptions builds the options for a cleanup job run. In
// non-dry-run mode it also wires SIGTERM/SIGINT to a graceful stop, so a k8s
// pod deletion (or Ctrl-C) stops the job at the next batch boundary instead
// of killing it mid-batch.
func newRetentionRunOptions(controlJobName string, maxBatches, vacuumEveryBatches int, vacuumTables []string, dryRun bool) *retentionRunOptions {
	opts := &retentionRunOptions{
		MaxBatches:         maxBatches,
		VacuumEveryBatches: vacuumEveryBatches,
		VacuumTables:       vacuumTables,
		ControlJobName:     controlJobName,
	}
	if !dryRun {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		opts.StopCh = sigCh
	}
	return opts
}
