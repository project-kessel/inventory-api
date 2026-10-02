package jobs

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

const (
	DefaultRetentionCleanupBatchSize     = 10000
	DefaultRetentionCleanupBatchDelay    = 500
	DefaultRetentionCleanupDays          = 7
	DefaultRetentionCleanupTombstoneDays = 30
	DefaultTombstoneCleanupBatchSize     = 1000  // Smaller batches for full deletion
)

func NewRetentionCleanupJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var dryRun bool
	var retentionDays int
	var tombstoneDays int
	var batchSize int
	var batchDelayMs int
	var reporterType string

	cmd := &cobra.Command{
		Use:   "retention-cleanup-job",
		Short: "Clean up old representation data based on retention policy",
		Long: `Delete reporter_representations and common_representations based on retention policy:
  - Active resources: Delete representations older than N days from latest
  - Tombstoned ≤M days: Delete representations older than N days from latest
  - Tombstoned >M days: Delete resource and all representations completely`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanupRetentionData(storageOptions, loggerOptions, dryRun, retentionDays, tombstoneDays, reporterType, batchSize, batchDelayMs)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deletion counts without executing any deletes")
	cmd.Flags().IntVar(&retentionDays, "retention-days", DefaultRetentionCleanupDays, "Delete representations older than this many days from latest")
	cmd.Flags().IntVar(&tombstoneDays, "tombstone-days", DefaultRetentionCleanupTombstoneDays, "Delete tombstoned resources completely after this many days")
	cmd.Flags().StringVar(&reporterType, "reporter-type", "", "Optional: Only delete for specific reporter type (e.g., 'hbi')")
	cmd.Flags().IntVar(&batchSize, "batch-size", DefaultRetentionCleanupBatchSize, "Number of records to delete per batch")
	cmd.Flags().IntVar(&batchDelayMs, "batch-delay-ms", DefaultRetentionCleanupBatchDelay, "Delay between batches in milliseconds")

	return cmd
}

func cleanupRetentionData(storageOptions *storage.Options, loggerOptions common.LoggerOptions, dryRun bool, retentionDays int, tombstoneDays int, reporterType string, batchSize int, batchDelayMs int) error {
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "retention_cleanup"))

	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.New(storageConfig, logHelper)
	if err != nil {
		return err
	}

	if dryRun {
		logHelper.Infof("Dry-run: %t", dryRun)
	}

	if reporterType != "" {
		logHelper.Infof("Starting retention cleanup job for reporter_type=%s", reporterType)
	} else {
		logHelper.Infof("Starting retention cleanup job for ALL reporters")
	}
	logHelper.Infof("Retention policy: %d days from latest representation", retentionDays)
	logHelper.Infof("Tombstone policy: Delete completely after %d days", tombstoneDays)

	if !dryRun {
		logHelper.Infof("Using batch size: %d rows, delay between batches: %dms", batchSize, batchDelayMs)
	}

	// Phase 1: Delete old representations (active + recently tombstoned)
	logHelper.Info("=== Phase 1: Cleaning up old representations ===")

	totalReporterRepresentations, err := deleteOldReporterRepresentations(db, logHelper, dryRun, retentionDays, tombstoneDays, reporterType, batchSize, batchDelayMs)
	if err != nil {
		logHelper.Warnw("msg", "Retention cleanup failed",
			"action", "RETENTION_CLEANUP",
			"phase", "reporter_representations",
			"retention_days", retentionDays,
			"tombstone_days", tombstoneDays,
			"reporter_type", reporterType,
			"outcome", "failure",
			"error", err.Error(),
		)
		return err
	}
	logDeleteResult(logHelper, dryRun, "ReporterRepresentation records (Phase 1)", totalReporterRepresentations)

	totalCommonRepresentations, err := deleteOldCommonRepresentations(db, logHelper, dryRun, retentionDays, tombstoneDays, reporterType, batchSize, batchDelayMs)
	if err != nil {
		logHelper.Warnw("msg", "Retention cleanup failed",
			"action", "RETENTION_CLEANUP",
			"phase", "common_representations",
			"retention_days", retentionDays,
			"tombstone_days", tombstoneDays,
			"reporter_type", reporterType,
			"outcome", "failure",
			"error", err.Error(),
		)
		return err
	}
	logDeleteResult(logHelper, dryRun, "CommonRepresentation records (Phase 1)", totalCommonRepresentations)

	// Phase 2: Delete old tombstoned resources completely
	logHelper.Info("=== Phase 2: Deleting old tombstoned resources ===")

	tombstoneCleanupBatchSize := DefaultTombstoneCleanupBatchSize

	deletedResources, err := deleteOldTombstonedResources(db, logHelper, dryRun, tombstoneDays, reporterType, tombstoneCleanupBatchSize, batchDelayMs)
	if err != nil {
		logHelper.Warnw("msg", "Tombstone cleanup failed",
			"action", "RETENTION_CLEANUP",
			"phase", "tombstoned_resources",
			"tombstone_days", tombstoneDays,
			"reporter_type", reporterType,
			"outcome", "failure",
			"error", err.Error(),
		)
		return err
	}
	logDeleteResult(logHelper, dryRun, "Tombstoned resources completely deleted (Phase 2)", deletedResources)

	if dryRun {
		logHelper.Infof("[DRY-RUN] Summary:")
		logHelper.Infof("  Phase 1 - Old representations: ReporterRepresentation=%d, CommonRepresentation=%d",
			totalReporterRepresentations, totalCommonRepresentations)
		logHelper.Infof("  Phase 2 - Old tombstoned resources: %d resources", deletedResources)
		logHelper.Info("[DRY-RUN] No data was modified")
	} else {
		logHelper.Infof("Retention cleanup completed successfully")
		logHelper.Infof("  Phase 1: Deleted %d ReporterRepresentations, %d CommonRepresentations",
			totalReporterRepresentations, totalCommonRepresentations)
		logHelper.Infof("  Phase 2: Deleted %d tombstoned resources completely", deletedResources)

		logHelper.Infow("msg", "Retention cleanup completed",
			"action", "RETENTION_CLEANUP",
			"retention_days", retentionDays,
			"tombstone_days", tombstoneDays,
			"reporter_type", reporterType,
			"reporter_representations_deleted", totalReporterRepresentations,
			"common_representations_deleted", totalCommonRepresentations,
			"tombstoned_resources_deleted", deletedResources,
			"outcome", "success",
		)
	}

	return nil
}

func deleteOldReporterRepresentations(db *gorm.DB, logHelper *log.Helper, dryRun bool, retentionDays int, tombstoneDays int, reporterType string, batchSize int, batchDelayMs int) (int64, error) {
	if dryRun {
		var count int64
		query := `
			SELECT COUNT(*)
			FROM reporter_representations rr_rep
			JOIN (
				SELECT reporter_resource_id, MAX(created_at) as latest_created_at
				FROM reporter_representations
				GROUP BY reporter_resource_id
			) latest ON rr_rep.reporter_resource_id = latest.reporter_resource_id
			JOIN reporter_resources rr ON rr_rep.reporter_resource_id = rr.id
			WHERE rr_rep.created_at < (latest.latest_created_at - INTERVAL '? days')
			  AND (rr.tombstone = false OR (rr.tombstone = true AND rr.updated_at >= NOW() - INTERVAL '? days'))
		`

		if reporterType != "" {
			query += " AND rr.reporter_type = ?"
			err := db.Raw(query, retentionDays, tombstoneDays, reporterType).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		} else {
			err := db.Raw(query, retentionDays, tombstoneDays).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		}

		logDryRunEstimate(logHelper, "ReporterRepresentation (old representations)", count, batchSize, batchDelayMs)
		return count, nil
	}

	var totalDeleted int64
	batchCount := 0

	logHelper.Info("Deleting old reporter_representations (active + recently tombstoned)...")

	for {
		var deleteQuery string
		var args []interface{}

		baseQuery := `
			DELETE FROM reporter_representations
			WHERE (reporter_resource_id, version, generation) IN (
				SELECT rr_rep.reporter_resource_id, rr_rep.version, rr_rep.generation
				FROM reporter_representations rr_rep
				JOIN (
					SELECT reporter_resource_id, MAX(created_at) as latest_created_at
					FROM reporter_representations
					GROUP BY reporter_resource_id
				) latest ON rr_rep.reporter_resource_id = latest.reporter_resource_id
				JOIN reporter_resources rr ON rr_rep.reporter_resource_id = rr.id
				WHERE rr_rep.created_at < (latest.latest_created_at - INTERVAL '? days')
				  AND (rr.tombstone = false OR (rr.tombstone = true AND rr.updated_at >= NOW() - INTERVAL '? days'))
		`

		if reporterType != "" {
			deleteQuery = baseQuery + " AND rr.reporter_type = ? LIMIT ?)"
			args = []interface{}{retentionDays, tombstoneDays, reporterType, batchSize}
		} else {
			deleteQuery = baseQuery + " LIMIT ?)"
			args = []interface{}{retentionDays, tombstoneDays, batchSize}
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

func deleteOldCommonRepresentations(db *gorm.DB, logHelper *log.Helper, dryRun bool, retentionDays int, tombstoneDays int, reporterType string, batchSize int, batchDelayMs int) (int64, error) {
	if dryRun {
		var count int64
		query := `
			SELECT COUNT(*)
			FROM common_representations cr
			JOIN (
				SELECT resource_id, MAX(created_at) as latest_created_at
				FROM common_representations
				GROUP BY resource_id
			) latest ON cr.resource_id = latest.resource_id
			JOIN resource r ON cr.resource_id = r.id
			LEFT JOIN reporter_resources rr ON r.id = rr.resource_id
			WHERE cr.created_at < (latest.latest_created_at - INTERVAL '? days')
			  AND (rr.tombstone = false OR (rr.tombstone = true AND rr.updated_at >= NOW() - INTERVAL '? days'))
		`

		if reporterType != "" {
			query += " AND rr.reporter_type = ?"
			err := db.Raw(query, retentionDays, tombstoneDays, reporterType).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		} else {
			err := db.Raw(query, retentionDays, tombstoneDays).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		}

		logDryRunEstimate(logHelper, "CommonRepresentation (old representations)", count, batchSize, batchDelayMs)
		return count, nil
	}

	var totalDeleted int64
	batchCount := 0

	logHelper.Info("Deleting old common_representations (active + recently tombstoned)...")

	for {
		var deleteQuery string
		var args []interface{}

		baseQuery := `
			DELETE FROM common_representations
			WHERE (resource_id, version) IN (
				SELECT cr.resource_id, cr.version
				FROM common_representations cr
				JOIN (
					SELECT resource_id, MAX(created_at) as latest_created_at
					FROM common_representations
					GROUP BY resource_id
				) latest ON cr.resource_id = latest.resource_id
				JOIN resource r ON cr.resource_id = r.id
				LEFT JOIN reporter_resources rr ON r.id = rr.resource_id
				WHERE cr.created_at < (latest.latest_created_at - INTERVAL '? days')
				  AND (rr.tombstone = false OR (rr.tombstone = true AND rr.updated_at >= NOW() - INTERVAL '? days'))
		`

		if reporterType != "" {
			deleteQuery = baseQuery + " AND rr.reporter_type = ? LIMIT ?)"
			args = []interface{}{retentionDays, tombstoneDays, reporterType, batchSize}
		} else {
			deleteQuery = baseQuery + " LIMIT ?)"
			args = []interface{}{retentionDays, tombstoneDays, batchSize}
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

func deleteOldTombstonedResources(db *gorm.DB, logHelper *log.Helper, dryRun bool, tombstoneDays int, reporterType string, batchSize int, batchDelayMs int) (int64, error) {
	if dryRun {
		var count int64
		query := `
			SELECT COUNT(DISTINCT rr.id)
			FROM reporter_resources rr
			WHERE rr.tombstone = true
			  AND rr.updated_at < NOW() - INTERVAL '? days'
		`

		if reporterType != "" {
			query += " AND rr.reporter_type = ?"
			err := db.Raw(query, tombstoneDays, reporterType).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		} else {
			err := db.Raw(query, tombstoneDays).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		}

		logDryRunEstimate(logHelper, "Old tombstoned resources (complete deletion)", count, batchSize, batchDelayMs)
		return count, nil
	}

	var totalDeleted int64
	batchCount := 0

	logHelper.Info("Deleting old tombstoned resources completely...")

	for {
		// Delete reporter_resources (cascade will delete reporter_representations)
		var deleteQuery string
		var args []interface{}

		if reporterType != "" {
			deleteQuery = `
				DELETE FROM reporter_resources
				WHERE id IN (
					SELECT id FROM reporter_resources
					WHERE tombstone = true
					  AND updated_at < NOW() - INTERVAL '? days'
					  AND reporter_type = ?
					LIMIT ?
				)
			`
			args = []interface{}{tombstoneDays, reporterType, batchSize}
		} else {
			deleteQuery = `
				DELETE FROM reporter_resources
				WHERE id IN (
					SELECT id FROM reporter_resources
					WHERE tombstone = true
					  AND updated_at < NOW() - INTERVAL '? days'
					LIMIT ?
				)
			`
			args = []interface{}{tombstoneDays, batchSize}
		}

		result := db.Exec(deleteQuery, args...)
		if result.Error != nil {
			logHelper.Errorf("Failed to delete tombstoned resources batch %d: %v", batchCount+1, result.Error)
			return totalDeleted, result.Error
		}

		if result.RowsAffected == 0 {
			break
		}

		totalDeleted += result.RowsAffected
		batchCount++

		logHelper.Infof("Batch %d: Deleted %d tombstoned resources (total so far: %d)", batchCount, result.RowsAffected, totalDeleted)

		if batchDelayMs > 0 {
			time.Sleep(time.Duration(batchDelayMs) * time.Millisecond)
		}
	}

	// Clean up orphaned resources (no reporter_resources reference them)
	orphanedQuery := `
		DELETE FROM resource
		WHERE id NOT IN (SELECT DISTINCT resource_id FROM reporter_resources)
	`
	orphanedResult := db.Exec(orphanedQuery)
	if orphanedResult.Error != nil {
		logHelper.Errorf("Failed to clean up orphaned resources: %v", orphanedResult.Error)
	} else if orphanedResult.RowsAffected > 0 {
		logHelper.Infof("Cleaned up %d orphaned resources", orphanedResult.RowsAffected)
	}

	return totalDeleted, nil
}
