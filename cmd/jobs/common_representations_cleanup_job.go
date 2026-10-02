package jobs

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
)

func NewCommonRepresentationsCleanupJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var dryRun bool
	var retentionDays int
	var tombstoneDays int
	var batchSize int
	var batchDelayMs int
	var reporterType string

	cmd := &cobra.Command{
		Use:   "common-representations-cleanup-job",
		Short: "Clean up old common_representations (Phase 3 - highest production impact)",
		Long: `Delete old common_representations based on retention policy.
This is Phase 3 of the retention cleanup process with the HIGHEST production impact.
Consumer queries read from this table, so deletions may affect query performance during execution.

Retention policy:
  - Active resources: Delete representations older than N days from latest
  - Tombstoned ≤M days: Delete representations older than N days from latest
  - Tombstoned >M days: Excluded (already deleted by tombstoned-resources-cleanup-job)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanupCommonRepresentations(storageOptions, loggerOptions, dryRun, retentionDays, tombstoneDays, reporterType, batchSize, batchDelayMs)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deletion counts without executing any deletes")
	cmd.Flags().IntVar(&retentionDays, "retention-days", DefaultRetentionCleanupDays, "Delete representations older than this many days from latest")
	cmd.Flags().IntVar(&tombstoneDays, "tombstone-days", DefaultRetentionCleanupTombstoneDays, "Exclude tombstoned resources older than this many days (already deleted)")
	cmd.Flags().StringVar(&reporterType, "reporter-type", "", "Optional: Only delete for specific reporter type (e.g., 'hbi')")
	cmd.Flags().IntVar(&batchSize, "batch-size", DefaultRetentionCleanupBatchSize, "Number of records to delete per batch")
	cmd.Flags().IntVar(&batchDelayMs, "batch-delay-ms", DefaultRetentionCleanupBatchDelay, "Delay between batches in milliseconds")

	return cmd
}

func cleanupCommonRepresentations(storageOptions *storage.Options, loggerOptions common.LoggerOptions, dryRun bool, retentionDays int, tombstoneDays int, reporterType string, batchSize int, batchDelayMs int) error {
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "common_representations_cleanup"))

	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.New(storageConfig, logHelper)
	if err != nil {
		return err
	}

	if dryRun {
		logHelper.Infof("Dry-run: %t", dryRun)
	}

	if reporterType != "" {
		logHelper.Infof("Starting common_representations cleanup for reporter_type=%s", reporterType)
	} else {
		logHelper.Infof("Starting common_representations cleanup for ALL reporters")
	}
	logHelper.Infof("Retention policy: %d days from latest representation", retentionDays)
	logHelper.Infof("Tombstone exclusion: Resources tombstoned >%d days (already deleted)", tombstoneDays)

	if !dryRun {
		logHelper.Infof("Using batch size: %d rows, delay between batches: %dms", batchSize, batchDelayMs)
		logHelper.Warn("⚠️  High production impact: Consumer queries read from this table")
	}

	totalDeleted, err := deleteOldCommonRepresentations(db, logHelper, dryRun, retentionDays, tombstoneDays, reporterType, batchSize, batchDelayMs)
	if err != nil {
		logHelper.Warnw("msg", "Common representations cleanup failed",
			"action", "COMMON_REPRESENTATIONS_CLEANUP",
			"retention_days", retentionDays,
			"tombstone_days", tombstoneDays,
			"reporter_type", reporterType,
			"outcome", "failure",
			"error", err.Error(),
		)
		return err
	}

	if dryRun {
		logHelper.Infof("[DRY-RUN] Would delete %d common_representations", totalDeleted)
		logHelper.Info("[DRY-RUN] No data was modified")
	} else {
		logHelper.Infof("Common representations cleanup completed: Deleted %d records", totalDeleted)

		logHelper.Infow("msg", "Common representations cleanup completed",
			"action", "COMMON_REPRESENTATIONS_CLEANUP",
			"retention_days", retentionDays,
			"tombstone_days", tombstoneDays,
			"reporter_type", reporterType,
			"deleted_count", totalDeleted,
			"outcome", "success",
		)
	}

	return nil
}
