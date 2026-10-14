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
	var batchDelayMs int
	var reporterType string
	var timeout time.Duration
	var maxBatches int
	var vacuumEveryBatches int

	cmd := &cobra.Command{
		Use:   "tombstone-cascade-cleanup-job",
		Short: "Delete old-tombstoned reporter_resources in a selectable order",
		Long: `Delete old-tombstoned reporter_resources rows; the database cascade
removes their reporter_representations. This is the batched, order-selectable
counterpart of the tombstoned-resources-cleanup-job.

Common representations are NOT deleted: removing a reporter row orphans that
resource's common_representations (when no other reporter row remains), and
those orphans are cleaned by the orphaned-common-representations-cleanup-job.
The parent resource row is NOT deleted (same known limitation as the
tombstone job).

Deletion order (--deletion-order):
  - oldest-first (default): the longest-tombstoned rows die first
  - largest-history-first: rows whose resources have the most common
    representations die first (history size is snapshotted once, at worklist
    collection)

--reporter-type scopes the worklist to that reporter's rows; each row is
judged independently (a resource with one old-tombstoned and one active
reporter row loses only the tombstoned one). --batch-size bounds
reporter_resources rows per batch.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanupTombstoneCascade(storageOptions, loggerOptions, dryRun, tombstoneDays, deletionOrder, reporterType, batchSize, batchDelayMs, timeout, maxBatches, vacuumEveryBatches)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deletion counts without executing any deletes")
	cmd.Flags().IntVar(&tombstoneDays, "tombstone-days", DefaultRetentionCleanupTombstoneDays, "Delete tombstoned resources older than this many days")
	cmd.Flags().StringVar(&deletionOrder, "deletion-order", DeletionOrderOldestFirst, "Deletion order: 'oldest-first' (longest-tombstoned first) or 'largest-history-first' (most common representations first)")
	cmd.Flags().StringVar(&reporterType, "reporter-type", "", "Optional: only delete tombstoned resources for this reporter type (e.g., 'hbi')")
	cmd.Flags().IntVar(&batchSize, "batch-size", DefaultTombstoneCleanupBatchSize, "Number of tombstoned reporter_resources to delete per batch")
	cmd.Flags().IntVar(&batchDelayMs, "batch-delay-ms", DefaultRetentionCleanupBatchDelay, "Delay between batches in milliseconds")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Gracefully stop after this duration (e.g. '48h'); 0 = no timeout. Stops between batches and exits successfully — resumable, the next run deletes the remainder")
	cmd.Flags().IntVar(&maxBatches, "max-batches", 0, "Stop gracefully after this many tombstone delete batches (0 = no limit). Resumable: the next run deletes the remainder")
	cmd.Flags().IntVar(&vacuumEveryBatches, "vacuum-every-batches", 0, "Run VACUUM (ANALYZE) on this job's target tables every N batches (0 = off). Online; reclaims reuse-space only — disk is returned by the final pg_repack step")

	return cmd
}

func cleanupTombstoneCascade(storageOptions *storage.Options, loggerOptions common.LoggerOptions, dryRun bool, tombstoneDays int, deletionOrder string, reporterType string, batchSize int, batchDelayMs int, timeout time.Duration, maxBatches int, vacuumEveryBatches int) error {
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
		logHelper.Infof("Starting tombstone cascade cleanup for reporter_type=%s", reporterType)
	} else {
		logHelper.Infof("Starting tombstone cascade cleanup for ALL reporters")
	}
	logHelper.Infof("Tombstone policy: Delete reporter resources tombstoned >%d days", tombstoneDays)
	logHelper.Infof("Deletion order: %s", deletionOrder)

	if !dryRun {
		logHelper.Infof("Using batch size: %d reporter resources, delay between batches: %dms", batchSize, batchDelayMs)
	}

	opts := newRetentionRunOptions("tombstone_cascade_cleanup", maxBatches, vacuumEveryBatches, []string{"reporter_resources", "reporter_representations"}, dryRun)

	startTime := time.Now()
	result, err := deleteOldTombstonedResourcesCascade(db, logHelper, dryRun, tombstoneDays, deletionOrder, reporterType, batchSize, batchDelayMs, timeout, opts)
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
		logHelper.Infof("[DRY-RUN] Would delete %d tombstoned reporter resources (their reporter_representations via cascade; their common representations are NOT deleted — they become the orphan job's work)", result.ReporterResourcesDeleted)
		logHelper.Info("[DRY-RUN] No data was modified")
	} else {
		logHelper.Infof("Tombstone cascade cleanup completed: Deleted %d reporter resources (elapsed: %v)",
			result.ReporterResourcesDeleted, time.Since(startTime))

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
			logHelper.Warnf("Run stopped early (reason: %s) after deleting %d reporter resources; rerun the job to delete the remainder",
				result.StopReason, result.ReporterResourcesDeleted)
		}

		logHelper.Infow("msg", "Tombstone cascade cleanup completed",
			"action", "TOMBSTONE_CASCADE_CLEANUP",
			"tombstone_days", tombstoneDays,
			"deletion_order", deletionOrder,
			"reporter_type", reporterType,
			"deleted_count", result.ReporterResourcesDeleted,
			"reporter_resources_deleted", result.ReporterResourcesDeleted,
			"outcome", outcome,
			"stopped_reason", result.StopReason,
		)
	}

	return nil
}
