package jobs

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

const (
	DefaultRetentionCleanupBatchSize  = 10000
	DefaultRetentionCleanupBatchDelay = 500
	DefaultRetentionCleanupDays       = 7
)

func NewRetentionCleanupJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var dryRun bool
	var retentionDays int
	var batchSize int
	var batchDelayMs int
	var reporterType string

	cmd := &cobra.Command{
		Use:   "retention-cleanup-job",
		Short: "Clean up old representation data based on retention policy",
		Long: `Delete reporter_representations and common_representations older than the specified retention period.
This job only deletes historical version data, NOT the resources or reporter_resources themselves.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanupRetentionData(storageOptions, loggerOptions, dryRun, retentionDays, reporterType, batchSize, batchDelayMs)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deletion counts without executing any deletes")
	cmd.Flags().IntVar(&retentionDays, "retention-days", DefaultRetentionCleanupDays, "Delete records older than this many days")
	cmd.Flags().StringVar(&reporterType, "reporter-type", "", "Optional: Only delete for specific reporter type (e.g., 'hbi'). If empty, deletes for all reporters.")
	cmd.Flags().IntVar(&batchSize, "batch-size", DefaultRetentionCleanupBatchSize, "Number of records to delete per batch")
	cmd.Flags().IntVar(&batchDelayMs, "batch-delay-ms", DefaultRetentionCleanupBatchDelay, "Delay between batches in milliseconds")

	return cmd
}

func cleanupRetentionData(storageOptions *storage.Options, loggerOptions common.LoggerOptions, dryRun bool, retentionDays int, reporterType string, batchSize int, batchDelayMs int) error {
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "retention_cleanup"))

	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.New(storageConfig, logHelper)
	if err != nil {
		return err
	}

	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	if dryRun {
		logHelper.Infof("Dry-run: %t", dryRun)
	}

	if reporterType != "" {
		logHelper.Infof("Starting retention cleanup job for reporter_type=%s, retention=%d days", reporterType, retentionDays)
	} else {
		logHelper.Infof("Starting retention cleanup job for ALL reporters, retention=%d days", retentionDays)
	}
	logHelper.Infof("Cutoff date: %s (will delete records created before this date)", cutoffDate.Format(time.RFC3339))

	if !dryRun {
		logHelper.Infof("Using batch size: %d rows, delay between batches: %dms", batchSize, batchDelayMs)
	}

	// Phase 1: Delete old ReporterRepresentation records
	totalReporterRepresentations, err := retentionDeleteBatchedReporterRepresentations(db, logHelper, dryRun, cutoffDate, reporterType, batchSize, batchDelayMs)
	if err != nil {
		logHelper.Warnw("msg", "Retention cleanup failed",
			"action", "RETENTION_CLEANUP",
			"phase", "reporter_representations",
			"retention_days", retentionDays,
			"reporter_type", reporterType,
			"principal", "system:cronjob:retention-cleanup-job",
			"outcome", "failure",
			"error", err.Error(),
		)
		return err
	}
	logDeleteResult(logHelper, dryRun, "ReporterRepresentation records", totalReporterRepresentations)

	// Phase 2: Delete old CommonRepresentation records
	totalCommonRepresentations, err := retentionDeleteBatchedCommonRepresentations(db, logHelper, dryRun, cutoffDate, reporterType, batchSize, batchDelayMs)
	if err != nil {
		logHelper.Warnw("msg", "Retention cleanup failed",
			"action", "RETENTION_CLEANUP",
			"phase", "common_representations",
			"retention_days", retentionDays,
			"reporter_type", reporterType,
			"principal", "system:cronjob:retention-cleanup-job",
			"outcome", "failure",
			"error", err.Error(),
		)
		return err
	}
	logDeleteResult(logHelper, dryRun, "CommonRepresentation records", totalCommonRepresentations)

	if dryRun {
		logHelper.Infof("[DRY-RUN] Summary: Would delete ReporterRepresentation=%d, CommonRepresentation=%d",
			totalReporterRepresentations, totalCommonRepresentations)
		logHelper.Info("[DRY-RUN] No data was modified")
	} else {
		logHelper.Infof("Retention cleanup job completed successfully. Total records deleted: ReporterRepresentation=%d, CommonRepresentation=%d",
			totalReporterRepresentations, totalCommonRepresentations)

		logHelper.Infow("msg", "Retention cleanup completed",
			"action", "RETENTION_CLEANUP",
			"retention_days", retentionDays,
			"reporter_type", reporterType,
			"principal", "system:cronjob:retention-cleanup-job",
			"deleted_count", totalReporterRepresentations+totalCommonRepresentations,
			"reporter_representations_deleted", totalReporterRepresentations,
			"common_representations_deleted", totalCommonRepresentations,
			"outcome", "success",
		)
	}

	return nil
}

func retentionDeleteBatchedReporterRepresentations(db *gorm.DB, logHelper *log.Helper, dryRun bool, cutoffDate time.Time, reporterType string, batchSize int, batchDelayMs int) (int64, error) {
	if dryRun {
		var count int64
		query := db.Model(&model.ReporterRepresentation{}).
			Where("created_at < ?", cutoffDate)

		// Optional filter by reporter type
		if reporterType != "" {
			// Need to join with reporter_resources to filter by reporter_type
			query = query.Joins("JOIN reporter_resources ON reporter_representations.reporter_resource_id = reporter_resources.id").
				Where("reporter_resources.reporter_type = ?", reporterType)
		}

		result := query.Count(&count)

		if result.Error != nil {
			logHelper.Errorf("Failed to count ReporterRepresentation records: %v", result.Error)
			return 0, result.Error
		}

		logDryRunEstimate(logHelper, "ReporterRepresentation (older than cutoff)", count, batchSize, batchDelayMs)
		return count, nil
	}

	// Actual deletion in batches
	var totalDeleted int64
	batchCount := 0

	logHelper.Info("Starting batched deletion of ReporterRepresentation records...")

	for {
		var deleteQuery string
		var args []interface{}

		// Build query based on whether we're filtering by reporter_type
		if reporterType != "" {
			deleteQuery = `
				DELETE FROM reporter_representations
				WHERE (reporter_resource_id, version, generation) IN (
					SELECT rr.reporter_resource_id, rr.version, rr.generation
					FROM reporter_representations rr
					JOIN reporter_resources res ON rr.reporter_resource_id = res.id
					WHERE rr.created_at < ?
					  AND res.reporter_type = ?
					LIMIT ?
				)
			`
			args = []interface{}{cutoffDate, reporterType, batchSize}
		} else {
			deleteQuery = `
				DELETE FROM reporter_representations
				WHERE (reporter_resource_id, version, generation) IN (
					SELECT reporter_resource_id, version, generation
					FROM reporter_representations
					WHERE created_at < ?
					LIMIT ?
				)
			`
			args = []interface{}{cutoffDate, batchSize}
		}

		result := db.Exec(deleteQuery, args...)

		if result.Error != nil {
			logHelper.Errorf("Failed to delete ReporterRepresentation batch %d: %v", batchCount+1, result.Error)
			return totalDeleted, result.Error
		}

		if result.RowsAffected == 0 {
			break
		}

		totalDeleted += result.RowsAffected
		batchCount++

		logHelper.Infof("Batch %d: Deleted %d ReporterRepresentation records (total so far: %d)", batchCount, result.RowsAffected, totalDeleted)

		if batchDelayMs > 0 {
			time.Sleep(time.Duration(batchDelayMs) * time.Millisecond)
		}
	}

	return totalDeleted, nil
}

func retentionDeleteBatchedCommonRepresentations(db *gorm.DB, logHelper *log.Helper, dryRun bool, cutoffDate time.Time, reporterType string, batchSize int, batchDelayMs int) (int64, error) {
	if dryRun {
		var count int64
		query := db.Model(&model.CommonRepresentation{}).
			Where("created_at < ?", cutoffDate)

		// Optional filter by reporter type
		if reporterType != "" {
			query = query.Where("reported_by_reporter_type = ?", reporterType)
		}

		result := query.Count(&count)

		if result.Error != nil {
			logHelper.Errorf("Failed to count CommonRepresentation records: %v", result.Error)
			return 0, result.Error
		}

		logDryRunEstimate(logHelper, "CommonRepresentation (older than cutoff)", count, batchSize, batchDelayMs)
		return count, nil
	}

	// Actual deletion in batches
	var totalDeleted int64
	batchCount := 0

	logHelper.Info("Starting batched deletion of CommonRepresentation records...")

	for {
		var deleteQuery string
		var args []interface{}

		// Build query based on whether we're filtering by reporter_type
		if reporterType != "" {
			deleteQuery = `
				DELETE FROM common_representations
				WHERE (resource_id, version) IN (
					SELECT resource_id, version
					FROM common_representations
					WHERE created_at < ?
					  AND reported_by_reporter_type = ?
					LIMIT ?
				)
			`
			args = []interface{}{cutoffDate, reporterType, batchSize}
		} else {
			deleteQuery = `
				DELETE FROM common_representations
				WHERE (resource_id, version) IN (
					SELECT resource_id, version
					FROM common_representations
					WHERE created_at < ?
					LIMIT ?
				)
			`
			args = []interface{}{cutoffDate, batchSize}
		}

		result := db.Exec(deleteQuery, args...)

		if result.Error != nil {
			logHelper.Errorf("Failed to delete CommonRepresentation batch %d: %v", batchCount+1, result.Error)
			return totalDeleted, result.Error
		}

		if result.RowsAffected == 0 {
			break
		}

		totalDeleted += result.RowsAffected
		batchCount++

		logHelper.Infof("Batch %d: Deleted %d CommonRepresentation records (total so far: %d)", batchCount, result.RowsAffected, totalDeleted)

		if batchDelayMs > 0 {
			time.Sleep(time.Duration(batchDelayMs) * time.Millisecond)
		}
	}

	return totalDeleted, nil
}
