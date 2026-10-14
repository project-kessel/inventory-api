package jobs

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
)

func NewTombstoneCascadeCleanupJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var dryRun bool
	var tombstoneDays int
	var deletionOrder string
	var batchSize int
	var orphanBatchSize int
	var batchDelayMs int
	var reporterType string
	var timeout time.Duration
	var maxBatches int
	var vacuumEveryBatches int

	cmd := &cobra.Command{
		Use:   "tombstone-cascade-cleanup-job",
		Short: "Delete old-tombstoned resources and their common representations in one run",
		Long: `Delete old-tombstoned reporter_resources AND their common_representations in
the same run, so a resource's data disappears all at once (the standalone
tombstone job instead orphans the common representations for the orphan job).

Deletion includes:
  - reporter_resources records (tombstoned >M days; cascade removes their
    reporter_representations)
  - ALL common_representations of resources whose reporter_resources rows are
    all gone (multi-reporter safe: a resource reported by an active reporter
    keeps its representations)

Deletion order (--deletion-order):
  - oldest-first (default): the longest-tombstoned resources die first
  - largest-history-first: resources with the most common representations die
    first (history size is snapshotted once, at worklist collection)

The parent resource row is NOT deleted (same known limitation as the
tombstone job). --reporter-type scopes only the tombstone worklist; orphan
deletion always runs for all reporters. --batch-size bounds reporter_resources
rows per batch, --orphan-batch-size bounds common_representations rows per
orphan pass; --max-batches bounds the tombstone loop only. The final orphan
drain runs only when the tombstone loop completes normally — after a partial
stop (timeout, control stop, signal, batch limit) the orphaned common
representations wait for the next run.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanupTombstoneCascade(storageOptions, loggerOptions, dryRun, tombstoneDays, deletionOrder, reporterType, batchSize, orphanBatchSize, batchDelayMs, timeout, maxBatches, vacuumEveryBatches)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deletion counts without executing any deletes")
	cmd.Flags().IntVar(&tombstoneDays, "tombstone-days", DefaultRetentionCleanupTombstoneDays, "Delete tombstoned resources older than this many days")
	cmd.Flags().StringVar(&deletionOrder, "deletion-order", DeletionOrderOldestFirst, "Deletion order: 'oldest-first' (longest-tombstoned first) or 'largest-history-first' (most common representations first)")
	cmd.Flags().StringVar(&reporterType, "reporter-type", "", "Optional: only delete tombstoned resources for this reporter type (e.g., 'hbi'). Scopes the tombstone worklist only — orphan deletion always runs for all reporters")
	cmd.Flags().IntVar(&batchSize, "batch-size", DefaultTombstoneCleanupBatchSize, "Number of tombstoned reporter_resources to delete per batch")
	cmd.Flags().IntVar(&orphanBatchSize, "orphan-batch-size", DefaultRetentionCleanupBatchSize, "Number of orphaned common_representations to delete per interleaved orphan batch")
	cmd.Flags().IntVar(&batchDelayMs, "batch-delay-ms", DefaultRetentionCleanupBatchDelay, "Delay between batches in milliseconds")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Gracefully stop after this duration (e.g. '48h'); 0 = no timeout. Stops between batches and exits successfully — resumable, the next run deletes the remainder")
	cmd.Flags().IntVar(&maxBatches, "max-batches", 0, "Stop gracefully after this many tombstone delete batches (0 = no limit). Resumable: the next run deletes the remainder")
	cmd.Flags().IntVar(&vacuumEveryBatches, "vacuum-every-batches", 0, "Run VACUUM (ANALYZE) on this job's target tables every N batches (0 = off). Online; reclaims reuse-space only — disk is returned by the final pg_repack step")

	return cmd
}

func cleanupTombstoneCascade(storageOptions *storage.Options, loggerOptions common.LoggerOptions, dryRun bool, tombstoneDays int, deletionOrder string, reporterType string, batchSize int, orphanBatchSize int, batchDelayMs int, timeout time.Duration, maxBatches int, vacuumEveryBatches int) error {
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "tombstone_cascade_cleanup"))

	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.New(storageConfig, logHelper)
	if err != nil {
		return err
	}

	if dryRun {
		logHelper.Infof("Dry-run: %t", dryRun)
	}

	if reporterType != "" {
		logHelper.Infof("Starting tombstone cascade cleanup for reporter_type=%s (worklist scope only)", reporterType)
	} else {
		logHelper.Infof("Starting tombstone cascade cleanup for ALL reporters")
	}
	logHelper.Infof("Tombstone policy: Delete resources tombstoned >%d days", tombstoneDays)
	logHelper.Infof("Deletion order: %s", deletionOrder)

	if !dryRun {
		logHelper.Infof("Using batch size: %d resources, orphan batch size: %d common representations, delay between batches: %dms", batchSize, orphanBatchSize, batchDelayMs)
	}

	opts := newRetentionRunOptions("tombstone_cascade_cleanup", maxBatches, vacuumEveryBatches, []string{"reporter_resources", "reporter_representations", "common_representations"}, dryRun)

	startTime := time.Now()
	result, err := deleteOldTombstonedResourcesCascade(db, logHelper, dryRun, tombstoneDays, deletionOrder, reporterType, batchSize, orphanBatchSize, batchDelayMs, timeout, opts)
	if err != nil {
		logHelper.Warnw("msg", "Tombstone cascade cleanup failed",
			"action", "TOMBSTONE_CASCADE_CLEANUP",
			"tombstone_days", tombstoneDays,
			"deletion_order", deletionOrder,
			"reporter_type", reporterType,
			"outcome", "failure",
			"error", err.Error(),
		)
		return err
	}

	if dryRun {
		logHelper.Infof("[DRY-RUN] Would delete %d tombstoned resources (and their representations via cascade)", result.ReporterResourcesDeleted)
		logHelper.Infof("[DRY-RUN] Would delete %d common representations of fully-deleted resources", result.CommonRepresentationsDeleted)
		logHelper.Info("[DRY-RUN] No data was modified")
	} else {
		logHelper.Infof("Tombstone cascade cleanup completed: Deleted %d resources, %d common representations (elapsed: %v)",
			result.ReporterResourcesDeleted, result.CommonRepresentationsDeleted, time.Since(startTime))

		outcome := "success"
		if result.StopReason != "" {
			switch result.StopReason {
			case stopReasonTimeout:
				outcome = "partial_timeout"
			case stopReasonControlStop:
				outcome = "partial_stopped"
			case stopReasonSignal:
				outcome = "partial_signal"
			case stopReasonMaxBatches:
				outcome = "partial_max_batches"
			}
			logHelper.Warnf("Run stopped early (reason: %s) after deleting %d resources and %d common representations; rerun the job to delete the remainder",
				result.StopReason, result.ReporterResourcesDeleted, result.CommonRepresentationsDeleted)
		}

		logHelper.Infow("msg", "Tombstone cascade cleanup completed",
			"action", "TOMBSTONE_CASCADE_CLEANUP",
			"tombstone_days", tombstoneDays,
			"deletion_order", deletionOrder,
			"reporter_type", reporterType,
			"deleted_count", result.ReporterResourcesDeleted+result.CommonRepresentationsDeleted,
			"reporter_resources_deleted", result.ReporterResourcesDeleted,
			"common_representations_deleted", result.CommonRepresentationsDeleted,
			"outcome", outcome,
			"stopped_reason", result.StopReason,
		)
	}

	return nil
}
