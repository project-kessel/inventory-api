package jobs

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
)

func NewTombstonedResourcesCleanupJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var dryRun bool
	var tombstoneDays int
	var batchSize int
	var batchDelayMs int
	var reporterType string

	cmd := &cobra.Command{
		Use:   "tombstoned-resources-cleanup-job",
		Short: "Delete tombstoned resources completely (Phase 2 - medium production impact)",
		Long: `Delete tombstoned resources and all their representations completely.
This is Phase 2 of the retention cleanup process with medium production impact.

Deletion includes:
  - reporter_resources records (tombstoned >M days)
  - All associated reporter_representations (cascade delete)
  - Orphaned resource records (no reporter_resources reference them)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanupTombstonedResources(storageOptions, loggerOptions, dryRun, tombstoneDays, reporterType, batchSize, batchDelayMs)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deletion counts without executing any deletes")
	cmd.Flags().IntVar(&tombstoneDays, "tombstone-days", DefaultRetentionCleanupTombstoneDays, "Delete tombstoned resources older than this many days")
	cmd.Flags().StringVar(&reporterType, "reporter-type", "", "Optional: Only delete for specific reporter type (e.g., 'hbi')")
	cmd.Flags().IntVar(&batchSize, "batch-size", DefaultTombstoneCleanupBatchSize, "Number of resources to delete per batch")
	cmd.Flags().IntVar(&batchDelayMs, "batch-delay-ms", DefaultRetentionCleanupBatchDelay, "Delay between batches in milliseconds")

	return cmd
}

func cleanupTombstonedResources(storageOptions *storage.Options, loggerOptions common.LoggerOptions, dryRun bool, tombstoneDays int, reporterType string, batchSize int, batchDelayMs int) error {
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "tombstoned_resources_cleanup"))

	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.New(storageConfig, logHelper)
	if err != nil {
		return err
	}

	if dryRun {
		logHelper.Infof("Dry-run: %t", dryRun)
	}

	if reporterType != "" {
		logHelper.Infof("Starting tombstoned resources cleanup for reporter_type=%s", reporterType)
	} else {
		logHelper.Infof("Starting tombstoned resources cleanup for ALL reporters")
	}
	logHelper.Infof("Tombstone policy: Delete resources tombstoned >%d days", tombstoneDays)

	if !dryRun {
		logHelper.Infof("Using batch size: %d resources, delay between batches: %dms", batchSize, batchDelayMs)
	}

	totalDeleted, err := deleteOldTombstonedResources(db, logHelper, dryRun, tombstoneDays, reporterType, batchSize, batchDelayMs)
	if err != nil {
		logHelper.Warnw("msg", "Tombstoned resources cleanup failed",
			"action", "TOMBSTONED_RESOURCES_CLEANUP",
			"tombstone_days", tombstoneDays,
			"reporter_type", reporterType,
			"outcome", "failure",
			"error", err.Error(),
		)
		return err
	}

	if dryRun {
		logHelper.Infof("[DRY-RUN] Would delete %d tombstoned resources (and their representations via cascade)", totalDeleted)
		logHelper.Info("[DRY-RUN] No data was modified")
	} else {
		logHelper.Infof("Tombstoned resources cleanup completed: Deleted %d resources", totalDeleted)

		logHelper.Infow("msg", "Tombstoned resources cleanup completed",
			"action", "TOMBSTONED_RESOURCES_CLEANUP",
			"tombstone_days", tombstoneDays,
			"reporter_type", reporterType,
			"deleted_count", totalDeleted,
			"outcome", "success",
		)
	}

	return nil
}
