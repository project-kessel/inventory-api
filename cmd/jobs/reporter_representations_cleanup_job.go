package jobs

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
)

func NewReporterRepresentationsCleanupJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var dryRun bool
	var retentionDays int
	var tombstoneDays int
	var batchSize int
	var batchDelayMs int
	var reporterType string

	cmd := &cobra.Command{
		Use:   "reporter-representations-cleanup-job",
		Short: "Clean up old reporter_representations (Phase 1 - lowest production impact)",
		Long: `Delete old reporter_representations based on retention policy.
This is Phase 1 of the retention cleanup process with the lowest production impact.

Retention policy:
  - Active resources: Delete representations older than N days from latest
  - Tombstoned ≤M days: Delete representations older than N days from latest
  - Tombstoned >M days: Excluded (handled by tombstoned-resources-cleanup-job)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanupReporterRepresentations(storageOptions, loggerOptions, dryRun, retentionDays, tombstoneDays, reporterType, batchSize, batchDelayMs)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deletion counts without executing any deletes")
	cmd.Flags().IntVar(&retentionDays, "retention-days", DefaultRetentionCleanupDays, "Delete representations older than this many days from latest")
	cmd.Flags().IntVar(&tombstoneDays, "tombstone-days", DefaultRetentionCleanupTombstoneDays, "Exclude tombstoned resources older than this many days (handled by separate job)")
	cmd.Flags().StringVar(&reporterType, "reporter-type", "", "Optional: Only delete for specific reporter type (e.g., 'hbi')")
	cmd.Flags().IntVar(&batchSize, "batch-size", DefaultRetentionCleanupBatchSize, "Number of records to delete per batch")
	cmd.Flags().IntVar(&batchDelayMs, "batch-delay-ms", DefaultRetentionCleanupBatchDelay, "Delay between batches in milliseconds")

	return cmd
}

func cleanupReporterRepresentations(storageOptions *storage.Options, loggerOptions common.LoggerOptions, dryRun bool, retentionDays int, tombstoneDays int, reporterType string, batchSize int, batchDelayMs int) error {
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "reporter_representations_cleanup"))

	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.New(storageConfig, logHelper)
	if err != nil {
		return err
	}

	if dryRun {
		logHelper.Infof("Dry-run: %t", dryRun)
	}

	if reporterType != "" {
		logHelper.Infof("Starting reporter_representations cleanup for reporter_type=%s", reporterType)
	} else {
		logHelper.Infof("Starting reporter_representations cleanup for ALL reporters")
	}
	logHelper.Infof("Retention policy: %d days from latest representation", retentionDays)
	logHelper.Infof("Tombstone exclusion: Resources tombstoned >%d days (handled by separate job)", tombstoneDays)

	if !dryRun {
		logHelper.Infof("Using batch size: %d rows, delay between batches: %dms", batchSize, batchDelayMs)
	}

	totalDeleted, err := deleteOldReporterRepresentations(db, logHelper, dryRun, retentionDays, tombstoneDays, reporterType, batchSize, batchDelayMs)
	if err != nil {
		logHelper.Warnw("msg", "Reporter representations cleanup failed",
			"action", "REPORTER_REPRESENTATIONS_CLEANUP",
			"retention_days", retentionDays,
			"tombstone_days", tombstoneDays,
			"reporter_type", reporterType,
			"outcome", "failure",
			"error", err.Error(),
		)
		return err
	}

	if dryRun {
		logHelper.Infof("[DRY-RUN] Would delete %d reporter_representations", totalDeleted)
		logHelper.Info("[DRY-RUN] No data was modified")
	} else {
		logHelper.Infof("Reporter representations cleanup completed: Deleted %d records", totalDeleted)

		logHelper.Infow("msg", "Reporter representations cleanup completed",
			"action", "REPORTER_REPRESENTATIONS_CLEANUP",
			"retention_days", retentionDays,
			"tombstone_days", tombstoneDays,
			"reporter_type", reporterType,
			"deleted_count", totalDeleted,
			"outcome", "success",
		)
	}

	return nil
}
