package jobs

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
)

func NewOrphanedCommonRepresentationsCleanupJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var dryRun bool
	var reporterType string
	var batchSize int
	var batchDelayMs int
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "orphaned-common-representations-cleanup-job",
		Short: "Clean up orphaned common_representations (resource fully deleted)",
		Long: `Delete ALL common_representations versions for resources that no longer have
any reporter_resources rows (e.g. after tombstoned-resources-cleanup-job completely
deleted the resource's reporter entries). Consumer queries read from this table, so
deletions may affect query performance during execution.

Orphan rule:
  - A representation is orphaned when no reporter_resources row references its resource
    (matched on reporter_resources.resource_id). A resource reported by multiple reporters
    only becomes orphaned once ALL of its reporter rows are gone.
  - The retention policy (keep last N days) is NOT applied here; every version of an
    orphaned resource is deleted. Use common-representations-cleanup-job for retention.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanupOrphanedCommonRepresentations(storageOptions, loggerOptions, dryRun, reporterType, batchSize, batchDelayMs, timeout)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deletion counts without executing any deletes")
	cmd.Flags().StringVar(&reporterType, "reporter-type", "", "Optional: Only delete for specific reporter type (e.g., 'hbi')")
	cmd.Flags().IntVar(&batchSize, "batch-size", DefaultRetentionCleanupBatchSize, "Number of records to delete per batch")
	cmd.Flags().IntVar(&batchDelayMs, "batch-delay-ms", DefaultRetentionCleanupBatchDelay, "Delay between batches in milliseconds")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Gracefully stop after this duration (e.g. '48h'); 0 = no timeout. Stops between batches and exits successfully — resumable, the next run deletes the remainder")

	return cmd
}

func cleanupOrphanedCommonRepresentations(storageOptions *storage.Options, loggerOptions common.LoggerOptions, dryRun bool, reporterType string, batchSize int, batchDelayMs int, timeout time.Duration) error {
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "orphaned_common_representations_cleanup"))

	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.New(storageConfig, logHelper)
	if err != nil {
		return err
	}

	if dryRun {
		logHelper.Infof("Dry-run: %t", dryRun)
	}

	if reporterType != "" {
		logHelper.Infof("Starting orphaned common_representations cleanup for reporter_type=%s", reporterType)
	} else {
		logHelper.Infof("Starting orphaned common_representations cleanup for ALL reporters")
	}

	if !dryRun {
		logHelper.Infof("Using batch size: %d rows, delay between batches: %dms", batchSize, batchDelayMs)
		logHelper.Warn("⚠️  High production impact: Consumer queries read from this table")
	}

	totalDeleted, timedOut, err := deleteOrphanedCommonRepresentations(db, logHelper, dryRun, reporterType, batchSize, batchDelayMs, timeout)
	if err != nil {
		logHelper.Warnw("msg", "Orphaned common representations cleanup failed",
			"action", "ORPHANED_COMMON_REPRESENTATIONS_CLEANUP",
			"reporter_type", reporterType,
			"outcome", "failure",
			"error", err.Error(),
		)
		return err
	}

	if dryRun {
		logHelper.Infof("[DRY-RUN] Would delete %d orphaned common_representations", totalDeleted)
		logHelper.Info("[DRY-RUN] No data was modified")
	} else {
		logHelper.Infof("Orphaned common representations cleanup completed: Deleted %d records", totalDeleted)

		outcome := "success"
		if timedOut {
			outcome = "partial_timeout"
			logHelper.Warnf("Run stopped at --timeout=%s after deleting %d records; rerun the job to delete the remainder", timeout, totalDeleted)
		}

		logHelper.Infow("msg", "Orphaned common representations cleanup completed",
			"action", "ORPHANED_COMMON_REPRESENTATIONS_CLEANUP",
			"reporter_type", reporterType,
			"deleted_count", totalDeleted,
			"outcome", outcome,
		)
	}

	return nil
}
