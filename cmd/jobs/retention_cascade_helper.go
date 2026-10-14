package jobs

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

// Deletion orders for the tombstone cascade cleanup job.
const (
	// DeletionOrderOldestFirst deletes the longest-tombstoned reporter rows
	// first (updated_at, id) — the same ordering as the standalone tombstone
	// job.
	DeletionOrderOldestFirst = "oldest-first"
	// DeletionOrderLargestHistoryFirst deletes reporter rows with the most
	// common representations first (history_size DESC, updated_at, id) —
	// biggest histories die first, so dead-tuple/bloat pressure is
	// front-loaded.
	DeletionOrderLargestHistoryFirst = "largest-history-first"
)

// cascadeCleanupResult reports what a tombstone cascade run deleted. On a
// partial run (StopReason != "") the counts are what was deleted SO FAR; the
// run is resumable and the next run deletes the remainder.
type cascadeCleanupResult struct {
	ReporterResourcesDeleted int64
	StopReason               string
}

// deleteOldTombstonedResourcesCascade deletes old-tombstoned
// reporter_resources rows in the selected deletion order; the database's
// foreign-key cascade removes their reporter_representations. It is the
// batched, order-selectable counterpart of the standalone tombstone job.
//
// It deliberately does NOT touch common_representations: deleting a
// reporter row orphans that resource's common reps (when no other reporter
// row remains), and those orphans are the orphaned-common-representations-
// cleanup-job's work. The parent resource row is also NOT deleted (same
// TODO as the standalone tombstone job).
//
// Eligibility is per reporter_resources row: tombstone = true AND updated_at
// older than the tombstoneDays cutoff. --reporter-type scopes the worklist to
// one reporter's rows; each is still judged independently.
//
// Deletion order is selected by deletionOrder (see the DeletionOrder*
// constants). The order key includes a history_size column — the number of
// the resource's common_representations SNAPSHOTTED AT COLLECT TIME.
// Per-batch recompute would rescan ~all of common_representations every
// batch; the snapshot gives a stable, deterministic order (and is what the
// ordering test pins). Deletion safety never depends on the snapshot: only
// worklist ids are deleted, and eligibility was fixed at collect time.
//
// Structure per batch: delete up to batchSize worklist rows from
// reporter_resources (cascade removes their reporter_representations), then
// drain the same rows from the worklist. If timeout > 0, deletion stops
// gracefully between batches once the deadline passes and the returned
// StopReason is "timeout". See retentionRunOptions for the other runtime
// controls.
func deleteOldTombstonedResourcesCascade(db *gorm.DB, logHelper *log.Helper, dryRun bool, tombstoneDays int, deletionOrder string, reporterType string, batchSize int, batchDelayMs int, timeout time.Duration, opts *retentionRunOptions) (cascadeCleanupResult, error) {
	var orderColumns string
	switch deletionOrder {
	case DeletionOrderOldestFirst:
		orderColumns = "updated_at, id"
	case DeletionOrderLargestHistoryFirst:
		orderColumns = "history_size DESC, updated_at, id"
	default:
		return cascadeCleanupResult{}, fmt.Errorf("invalid deletion order %q: must be %q or %q",
			deletionOrder, DeletionOrderOldestFirst, DeletionOrderLargestHistoryFirst)
	}

	daysStr := fmt.Sprintf("%d", tombstoneDays)

	if dryRun {
		var rrCount int64
		rrQuery := `
			SELECT COUNT(DISTINCT rr.id)
			FROM reporter_resources rr
			WHERE rr.tombstone = true
			  AND rr.updated_at < NOW() - (? || ' days')::INTERVAL
		`
		if reporterType != "" {
			rrQuery += " AND rr.reporter_type = ?"
			if err := db.Raw(rrQuery, daysStr, reporterType).Scan(&rrCount).Error; err != nil {
				return cascadeCleanupResult{}, err
			}
		} else {
			if err := db.Raw(rrQuery, daysStr).Scan(&rrCount).Error; err != nil {
				return cascadeCleanupResult{}, err
			}
		}

		logDryRunEstimate(logHelper, "Old tombstoned reporter resources (cascade)", rrCount, batchSize, batchDelayMs)
		return cascadeCleanupResult{ReporterResourcesDeleted: rrCount}, nil
	}

	initRuntimeControl(db, logHelper, opts)

	// Graceful timeout: stop between batches once the deadline passes. The job
	// is idempotent, so a partial run is a successful exit; rerun to continue.
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
		logHelper.Infof("Timeout: will stop gracefully after %s (resumable)", timeout)
	}

	// COLLECT-THEN-DELETE, same strategy as the other helpers: identify all
	// eligible rr rows ONCE into a temp worklist (with the history_size
	// snapshot for the largest-history-first order), then delete from the
	// static list in batches.
	logHelper.Infof("Collecting old-tombstoned reporter resources into the cascade worklist (one-time scan, deletion order: %s)...", deletionOrder)

	// Pre-drop so a crashed earlier run in the same session can never leave a
	// stale, partially-drained worklist behind.
	db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")

	createWorklist := `
		CREATE TEMP TABLE tombstone_cascade_worklist (
			id UUID PRIMARY KEY,
			resource_id UUID NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			history_size INT NOT NULL
		)
	`
	if err := db.Exec(createWorklist).Error; err != nil {
		logHelper.Errorf("Failed to create tombstone_cascade_worklist: %v", err)
		return cascadeCleanupResult{}, err
	}

	var populateQuery string
	var populateArgs []interface{}
	if reporterType != "" {
		populateQuery = `
			INSERT INTO tombstone_cascade_worklist (id, resource_id, updated_at, history_size)
			SELECT rr.id, rr.resource_id, rr.updated_at, COALESCE(hs.cnt, 0)
			FROM reporter_resources rr
			LEFT JOIN (
				SELECT resource_id, COUNT(*) AS cnt FROM common_representations GROUP BY resource_id
			) hs ON hs.resource_id = rr.resource_id
			WHERE rr.tombstone = true
			  AND rr.updated_at < NOW() - (? || ' days')::INTERVAL
			  AND rr.reporter_type = ?
		`
		populateArgs = []interface{}{daysStr, reporterType}
	} else {
		populateQuery = `
			INSERT INTO tombstone_cascade_worklist (id, resource_id, updated_at, history_size)
			SELECT rr.id, rr.resource_id, rr.updated_at, COALESCE(hs.cnt, 0)
			FROM reporter_resources rr
			LEFT JOIN (
				SELECT resource_id, COUNT(*) AS cnt FROM common_representations GROUP BY resource_id
			) hs ON hs.resource_id = rr.resource_id
			WHERE rr.tombstone = true
			  AND rr.updated_at < NOW() - (? || ' days')::INTERVAL
		`
		populateArgs = []interface{}{daysStr}
	}

	collectStart := time.Now()
	if err := db.Exec(populateQuery, populateArgs...).Error; err != nil {
		logHelper.Errorf("Failed to populate tombstone_cascade_worklist: %v", err)
		db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
		return cascadeCleanupResult{}, err
	}

	var worklistCount int64
	if err := db.Raw("SELECT COUNT(*) FROM tombstone_cascade_worklist").Scan(&worklistCount).Error; err != nil {
		logHelper.Warnf("Failed to count tombstone_cascade_worklist: %v", err)
	} else {
		logHelper.Infof("Identified %d old-tombstoned reporter resources in the cascade worklist in %v", worklistCount, time.Since(collectStart))
	}

	// ONE index matching the selected order, so every batch's ordered LIMIT
	// subselect is an index scan rather than a sort.
	if err := db.Exec(fmt.Sprintf("CREATE INDEX idx_tcw_order ON tombstone_cascade_worklist(%s)", orderColumns)).Error; err != nil {
		logHelper.Warnf("Failed to create worklist order index (continuing without it): %v", err)
	}

	// DRAIN INTEGRITY: the rr delete and the worklist drain below embed this
	// exact ordered subselect, so both provably target the same rows.
	batchSubselect := fmt.Sprintf(`
					SELECT id FROM tombstone_cascade_worklist
					ORDER BY %s
					LIMIT ?
				`, orderColumns)

	deleteRRQuery := "DELETE FROM reporter_resources WHERE id IN (" + batchSubselect + ")"
	drainWorklistQuery := "DELETE FROM tombstone_cascade_worklist WHERE id IN (" + batchSubselect + ")"

	result := cascadeCleanupResult{}
	batchCount := 0

	logHelper.Info("Deleting old-tombstoned reporter resources in batches (their common representations are orphaned for the orphan job)...")

	for {
		if !deadline.IsZero() && time.Now().After(deadline) {
			logHelper.Infof("Timeout reached after %d batches: %d reporter resources deleted so far — stopping gracefully, rerun to continue", batchCount, result.ReporterResourcesDeleted)
			db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
			result.StopReason = stopReasonTimeout
			return result, nil
		}

		if reason := checkBatchGate(db, logHelper, opts, batchCount); reason != "" {
			logHelper.Infof("Run stopping after %d batches: %d reporter resources deleted so far — reason: %s (rerun to continue)", batchCount, result.ReporterResourcesDeleted, reason)
			db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
			result.StopReason = reason
			return result, nil
		}

		deleteResult := db.Exec(deleteRRQuery, batchSize)
		if deleteResult.Error != nil {
			logHelper.Errorf("Failed to delete tombstoned resources batch %d: %v", batchCount+1, deleteResult.Error)
			db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
			return result, deleteResult.Error
		}

		if deleteResult.RowsAffected == 0 {
			break
		}

		// Drain the deleted rows from the worklist (identical ordered
		// subselect as the delete above — same rows, provably). A failed drain
		// must abort the run: already-deleted ids would stay at the head of
		// the worklist, the next delete batch would match 0 rows, and the run
		// would end as a silently incomplete "success".
		if err := db.Exec(drainWorklistQuery, batchSize).Error; err != nil {
			logHelper.Errorf("Failed to drain tombstone_cascade_worklist after batch %d: %v", batchCount, err)
			db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
			return result, err
		}

		result.ReporterResourcesDeleted += deleteResult.RowsAffected
		batchCount++

		logHelper.Infof("Batch %d: Deleted %d tombstoned reporter resources (total so far: %d)",
			batchCount, deleteResult.RowsAffected, result.ReporterResourcesDeleted)

		maybeVacuum(db, logHelper, opts, batchCount)

		if batchDelayMs > 0 {
			time.Sleep(time.Duration(batchDelayMs) * time.Millisecond)
		}
	}

	logHelper.Infof("Tombstone cascade completed in %v (reporter_resources deleted: %d; common_representations were NOT touched — orphaned rows are the orphan job's work)",
		time.Since(collectStart), result.ReporterResourcesDeleted)

	db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")

	return result, nil
}
