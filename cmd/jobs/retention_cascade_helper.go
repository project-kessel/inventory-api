package jobs

import (
	"fmt"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

// Deletion orders for the tombstone cascade cleanup job.
const (
	// DeletionOrderOldestFirst deletes the longest-tombstoned resources first
	// (updated_at, id) — the same ordering as the standalone tombstone job.
	DeletionOrderOldestFirst = "oldest-first"
	// DeletionOrderLargestHistoryFirst deletes resources with the most common
	// representations first (history_size DESC, updated_at, id) — biggest
	// histories die first, so dead-tuple/bloat pressure is front-loaded.
	DeletionOrderLargestHistoryFirst = "largest-history-first"
)

// cascadeCleanupResult reports what a tombstone cascade run deleted. On a
// partial run (StopReason != "") the counts are what was deleted SO FAR; the
// run is resumable and the next run deletes the remainder.
type cascadeCleanupResult struct {
	ReporterResourcesDeleted     int64
	CommonRepresentationsDeleted int64 // interleaved per-batch pass + final orphan drain
	StopReason                   string
}

// deleteOldTombstonedResourcesCascade is the combined form of the tombstone and
// orphan jobs: it deletes old-tombstoned reporter_resources rows AND their
// common_representations in the SAME run, so a resource's data disappears all
// at once instead of orphaning common reps for a second job to clean up.
//
// Deletion order is selected by deletionOrder (see the DeletionOrder* constants).
// The order key includes a history_size column — the number of the resource's
// common_representations SNAPSHOTTED AT COLLECT TIME. Per-batch recompute would
// rescan ~all of common_representations every batch; the snapshot gives a
// stable, deterministic order (and is what the ordering test pins). Safety never
// depends on the snapshot: every batch re-checks the live anti-join before
// deleting any common representation.
//
// Multi-reporter safety: a resource's common_representations are deleted only
// when NO reporter_resources row remains for it (anti-join on resource_id — the
// canonical resource ID, NOT reporter_resources.id), re-evaluated per batch
// against live data. The parent resource row is NOT deleted (same TODO as the
// standalone tombstone job).
//
// --reporter-type scopes only the tombstone worklist; the orphan phases always
// run unscoped (a fully-deleted resource's reps are orphans regardless of which
// reporter reported them).
//
// Structure per batch: delete up to batchSize worklist rows from
// reporter_resources (cascade removes their reporter_representations), drain
// the same rows from the worklist, then delete up to orphanBatchSize
// common_representations orphaned by THIS batch (scoped to the batch's
// resource_ids — an unscoped per-batch anti-join would rescan the whole table
// every batch). Backlog beyond orphanBatchSize per batch is cleaned by the
// final drain, which reuses deleteOrphanedCommonRepresentations and therefore
// also picks up any pre-existing orphans.
//
// If timeout > 0, deletion stops gracefully between batches once the deadline
// passes and the returned StopReason is "timeout". The final orphan drain runs
// ONLY when the tombstone loop completes normally — after a partial stop the
// orphaned common reps simply wait for the next run (the drain could take
// hours at prod scale, which would defeat a stop request). See
// retentionRunOptions for the other runtime controls.
func deleteOldTombstonedResourcesCascade(db *gorm.DB, logHelper *log.Helper, dryRun bool, tombstoneDays int, deletionOrder string, reporterType string, batchSize int, orphanBatchSize int, batchDelayMs int, timeout time.Duration, opts *retentionRunOptions) (cascadeCleanupResult, error) {
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
		var crCount int64
		// A common rep is predicted deletable when its resource has at least
		// one eligible rr row and NO non-eligible rr row (any surviving
		// reporter row blocks the orphan pass).
		rrQuery := `
			SELECT COUNT(DISTINCT rr.id)
			FROM reporter_resources rr
			WHERE rr.tombstone = true
			  AND rr.updated_at < NOW() - (? || ' days')::INTERVAL
		`
		crQuery := `
			SELECT COUNT(*)
			FROM common_representations cr
			WHERE EXISTS (
				SELECT 1 FROM reporter_resources rr
				WHERE rr.resource_id = cr.resource_id
				  AND rr.tombstone = true
				  AND rr.updated_at < NOW() - (? || ' days')::INTERVAL
			)
			AND NOT EXISTS (
				SELECT 1 FROM reporter_resources rr2
				WHERE rr2.resource_id = cr.resource_id
				  AND NOT (
					rr2.tombstone = true
					AND rr2.updated_at < NOW() - (? || ' days')::INTERVAL
				  )
			)
		`
		if reporterType != "" {
			rrQuery += " AND rr.reporter_type = ?"
			if err := db.Raw(rrQuery, daysStr, reporterType).Scan(&rrCount).Error; err != nil {
				return cascadeCleanupResult{}, err
			}
			crQuery = `
				SELECT COUNT(*)
				FROM common_representations cr
				WHERE EXISTS (
					SELECT 1 FROM reporter_resources rr
					WHERE rr.resource_id = cr.resource_id
					  AND rr.tombstone = true
					  AND rr.updated_at < NOW() - (? || ' days')::INTERVAL
					  AND rr.reporter_type = ?
				)
				AND NOT EXISTS (
					SELECT 1 FROM reporter_resources rr2
					WHERE rr2.resource_id = cr.resource_id
					  AND NOT (
						rr2.tombstone = true
						AND rr2.updated_at < NOW() - (? || ' days')::INTERVAL
						AND rr2.reporter_type = ?
					  )
				)
			`
			if err := db.Raw(crQuery, daysStr, reporterType, daysStr, reporterType).Scan(&crCount).Error; err != nil {
				return cascadeCleanupResult{}, err
			}
		} else {
			if err := db.Raw(rrQuery, daysStr).Scan(&rrCount).Error; err != nil {
				return cascadeCleanupResult{}, err
			}
			if err := db.Raw(crQuery, daysStr, daysStr).Scan(&crCount).Error; err != nil {
				return cascadeCleanupResult{}, err
			}
		}

		// The final drain reuses the UNscoped orphan helper, so a real run also
		// deletes any pre-existing orphans (e.g. left by an earlier standalone
		// tombstone-job run). Those rows have no reporter_resources rows at all,
		// so they are disjoint from the eligible count above — the accurate
		// prediction is the sum.
		var preExistingOrphans int64
		if err := db.Raw(`
			SELECT COUNT(*)
			FROM common_representations cr
			WHERE NOT EXISTS (
				SELECT 1 FROM reporter_resources rr WHERE rr.resource_id = cr.resource_id
			)
		`).Scan(&preExistingOrphans).Error; err != nil {
			return cascadeCleanupResult{}, err
		}
		crCount += preExistingOrphans

		logDryRunEstimate(logHelper, "Old tombstoned resources (cascade, complete deletion)", rrCount, batchSize, batchDelayMs)
		logDryRunEstimate(logHelper, "CommonRepresentations (cascade, resources fully deleted + pre-existing orphans)", crCount, orphanBatchSize, batchDelayMs)
		return cascadeCleanupResult{ReporterResourcesDeleted: rrCount, CommonRepresentationsDeleted: crCount}, nil
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
	logHelper.Infof("Collecting old-tombstoned resources into the cascade worklist (one-time scan, deletion order: %s)...", deletionOrder)

	// Pre-drop so a crashed earlier run in the same session can never leave a
	// stale, partially-drained worklist behind.
	db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
	db.Exec("DROP TABLE IF EXISTS cascade_drained_batch")

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
		logHelper.Infof("Identified %d old-tombstoned resources in the cascade worklist in %v", worklistCount, time.Since(collectStart))
	}

	// ONE index matching the selected order, so every batch's ordered LIMIT
	// subselect is an index scan rather than a sort.
	if err := db.Exec(fmt.Sprintf("CREATE INDEX idx_tcw_order ON tombstone_cascade_worklist(%s)", orderColumns)).Error; err != nil {
		logHelper.Warnf("Failed to create worklist order index (continuing without it): %v", err)
	}

	createDrainedBatch := `
		CREATE TEMP TABLE cascade_drained_batch (
			resource_id UUID PRIMARY KEY
		)
	`
	if err := db.Exec(createDrainedBatch).Error; err != nil {
		logHelper.Errorf("Failed to create cascade_drained_batch: %v", err)
		db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
		return cascadeCleanupResult{}, err
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

	// One interleaved orphan batch per iteration, scoped to this batch's
	// resource_ids (index probes via the cr PK and
	// reporter_resource_resource_id_idx — no whole-table anti-join per batch).
	orphanBatchQuery := `
		DELETE FROM common_representations
		WHERE (resource_id, version) IN (
			SELECT cr.resource_id, cr.version
			FROM common_representations cr
			JOIN cascade_drained_batch d ON d.resource_id = cr.resource_id
			WHERE NOT EXISTS (
				SELECT 1 FROM reporter_resources rr WHERE rr.resource_id = cr.resource_id
			)
			ORDER BY cr.created_at, cr.resource_id, cr.version
			LIMIT ?
		)
	`

	result := cascadeCleanupResult{}
	batchCount := 0

	logHelper.Info("Deleting old-tombstoned resources (and their representations) in batches...")

	for {
		if !deadline.IsZero() && time.Now().After(deadline) {
			logHelper.Infof("Timeout reached after %d batches: %d resources deleted so far — stopping gracefully, rerun to continue", batchCount, result.ReporterResourcesDeleted)
			db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
			db.Exec("DROP TABLE IF EXISTS cascade_drained_batch")
			logHelper.Infof("Run stopped at timeout with %s orphaned common representations pending — the next run's final drain deletes them", pendingOrphanCount(db, logHelper))
			result.StopReason = stopReasonTimeout
			return result, nil
		}

		if reason := checkBatchGate(db, logHelper, opts, batchCount); reason != "" {
			logHelper.Infof("Run stopping after %d batches: %d resources deleted so far — reason: %s (rerun to continue)", batchCount, result.ReporterResourcesDeleted, reason)
			db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
			db.Exec("DROP TABLE IF EXISTS cascade_drained_batch")
			logHelper.Infof("Run stopped with orphaned common representations pending — the next run's final drain deletes them (current pending: %s)", pendingOrphanCount(db, logHelper))
			result.StopReason = reason
			return result, nil
		}

		// Record this batch's resource_ids BEFORE the delete, so the
		// interleaved orphan pass knows which resources to check.
		if err := db.Exec("TRUNCATE cascade_drained_batch").Error; err != nil {
			logHelper.Errorf("Failed to reset cascade_drained_batch at batch %d: %v", batchCount+1, err)
			db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
			db.Exec("DROP TABLE IF EXISTS cascade_drained_batch")
			return result, err
		}
		if err := db.Exec("INSERT INTO cascade_drained_batch (resource_id) SELECT DISTINCT resource_id FROM tombstone_cascade_worklist WHERE id IN ("+batchSubselect+")", batchSize).Error; err != nil {
			logHelper.Errorf("Failed to collect batch %d resource_ids: %v", batchCount+1, err)
			db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
			db.Exec("DROP TABLE IF EXISTS cascade_drained_batch")
			return result, err
		}

		deleteResult := db.Exec(deleteRRQuery, batchSize)
		if deleteResult.Error != nil {
			logHelper.Errorf("Failed to delete tombstoned resources batch %d: %v", batchCount+1, deleteResult.Error)
			db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
			db.Exec("DROP TABLE IF EXISTS cascade_drained_batch")
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
			db.Exec("DROP TABLE IF EXISTS cascade_drained_batch")
			return result, err
		}

		result.ReporterResourcesDeleted += deleteResult.RowsAffected
		batchCount++

		// Interleaved orphan pass for THIS batch's resources. RowsAffected may
		// be 0 (zero-history resources, or a multi-reporter resource whose
		// other reporter rows survive) — that must NOT exit the main loop;
		// only the rr delete's RowsAffected is the exit condition.
		orphanResult := db.Exec(orphanBatchQuery, orphanBatchSize)
		if orphanResult.Error != nil {
			logHelper.Errorf("Failed to delete orphaned CommonRepresentation batch %d: %v", batchCount, orphanResult.Error)
			db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
			db.Exec("DROP TABLE IF EXISTS cascade_drained_batch")
			return result, orphanResult.Error
		}
		result.CommonRepresentationsDeleted += orphanResult.RowsAffected

		logHelper.Infof("Batch %d: Deleted %d tombstoned resources (total so far: %d); orphan pass deleted %d common representations (total so far: %d)",
			batchCount, deleteResult.RowsAffected, result.ReporterResourcesDeleted, orphanResult.RowsAffected, result.CommonRepresentationsDeleted)

		maybeVacuum(db, logHelper, opts, batchCount)

		if batchDelayMs > 0 {
			time.Sleep(time.Duration(batchDelayMs) * time.Millisecond)
		}
	}

	cascadePhaseElapsed := time.Since(collectStart)
	logHelper.Infof("Tombstone cascade phase completed in %v (reporter_resources deleted: %d, common_representations deleted so far: %d)",
		cascadePhaseElapsed, result.ReporterResourcesDeleted, result.CommonRepresentationsDeleted)

	db.Exec("DROP TABLE IF EXISTS tombstone_cascade_worklist")
	db.Exec("DROP TABLE IF EXISTS cascade_drained_batch")

	// FINAL DRAIN: the per-batch orphan passes are bounded by orphanBatchSize,
	// so a backlog can remain (variant B front-loads the biggest histories).
	// Reuse the tested orphan helper — unscoped, so it also cleans any
	// pre-existing orphans. --max-batches bounds the tombstone loop ONLY; the
	// drain always runs to completion once the tombstone phase finished.
	var orphanOpts *retentionRunOptions
	if opts != nil {
		o := *opts
		o.MaxBatches = 0
		orphanOpts = &o
	}
	var drainTimeout time.Duration
	if !deadline.IsZero() {
		if !time.Now().Before(deadline) {
			logHelper.Infof("Timeout reached before the orphan drain: %s common representations still orphaned — stopping gracefully, rerun to continue", pendingOrphanCount(db, logHelper))
			result.StopReason = stopReasonTimeout
			return result, nil
		}
		drainTimeout = time.Until(deadline)
	}

	logHelper.Info("Draining remaining orphaned common representations...")
	drainStart := time.Now()
	drainDeleted, drainReason, err := deleteOrphanedCommonRepresentations(db, logHelper, false, "", orphanBatchSize, batchDelayMs, drainTimeout, orphanOpts)
	result.CommonRepresentationsDeleted += drainDeleted
	if err != nil {
		return result, err
	}
	if drainReason != "" {
		result.StopReason = drainReason
	}
	logHelper.Infof("Orphan drain phase completed in %v (common_representations deleted: %d)", time.Since(drainStart), drainDeleted)

	return result, nil
}

// pendingOrphanCount reports how many common_representations are currently
// orphaned (no reporter_resources row for their resource), as a display
// string. Used only for logging at partial-stop boundaries; on failure it
// returns "unknown" rather than a sentinel number that would read as a count.
func pendingOrphanCount(db *gorm.DB, logHelper *log.Helper) string {
	var n int64
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM common_representations cr
		WHERE NOT EXISTS (
			SELECT 1 FROM reporter_resources rr WHERE rr.resource_id = cr.resource_id
		)
	`).Scan(&n).Error; err != nil {
		logHelper.Warnf("Failed to count pending orphaned common representations: %v", err)
		return "unknown"
	}
	return strconv.FormatInt(n, 10)
}
