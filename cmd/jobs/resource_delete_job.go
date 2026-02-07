package jobs

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/cmd/common"
	gormrepo "github.com/project-kessel/inventory-api/internal/infrastructure/resourcerepository/gorm"
	"github.com/project-kessel/inventory-api/internal/provider"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

const (
	DeleteBatchSize = 5000
	BatchDelayMs    = 100
)

func NewResourceDeleteJobCommand(storageOptions *provider.StorageOptions, loggerOptions common.LoggerOptions) *cobra.Command {
	var dryRun bool
	var resourceType string
	var reporterType string

	cmd := &cobra.Command{
		Use:   "resource-delete-job",
		Short: "Delete resources from the database",
		Long:  "Delete resources from the database by resource type and reporter type",
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteResources(storageOptions, loggerOptions, dryRun, resourceType, reporterType)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deletion counts without executing any deletes")
	cmd.Flags().StringVar(&resourceType, "resource-type", "", "The resource type to delete (e.g., 'host', 'k8s-cluster')")
	cmd.Flags().StringVar(&reporterType, "reporter-type", "", "The reporter type to filter by (e.g., 'hbi', 'ocm')")

	_ = cmd.MarkFlagRequired("resource-type")
	_ = cmd.MarkFlagRequired("reporter-type")

	return cmd
}

func deleteResources(storageOptions *provider.StorageOptions, loggerOptions common.LoggerOptions, dryRun bool, resourceType string, reporterType string) error {
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "delete_resources"))

	if resourceType == "" {
		logHelper.Error("resource-type is required but was not provided")
		return fmt.Errorf("resource-type flag is required")
	}
	if reporterType == "" {
		logHelper.Error("reporter-type is required but was not provided")
		return fmt.Errorf("reporter-type flag is required")
	}

	db, err := provider.NewStorage(storageOptions, logHelper)
	if err != nil {
		return err
	}

	if dryRun {
		logHelper.Infof("Dry-run: %t", dryRun)
	}

	logHelper.Infof("Starting resource deletion job for resource_type=%s, reporter_type=%s", resourceType, reporterType)
	if !dryRun {
		logHelper.Infof("Using batch size: %d rows, delay between batches: %dms", DeleteBatchSize, BatchDelayMs)
	}

	// Delete ReporterResource records in batches (CASCADE will automatically delete ReporterRepresentation)
	totalReporterResources, err := deleteBatchedReporterResources(db, logHelper, dryRun, resourceType, reporterType)
	if err != nil {
		return err
	}
	logDeleteResult(logHelper, dryRun, "ReporterResource records (ReporterRepresentation cascade deleted)", totalReporterResources)

	// Delete CommonRepresentation records in batches
	totalCommonRepresentations, err := deleteBatchedCommonRepresentations(db, logHelper, dryRun, reporterType)
	if err != nil {
		return err
	}
	logDeleteResult(logHelper, dryRun, "CommonRepresentation records", totalCommonRepresentations)

	// Delete Resource records in batches
	totalResources, err := deleteBatchedResources(db, logHelper, dryRun, resourceType)
	if err != nil {
		return err
	}
	logDeleteResult(logHelper, dryRun, "Resource records", totalResources)

	if dryRun {
		logHelper.Infof("[DRY-RUN] Summary: Would delete ReporterResource=%d, CommonRepresentation=%d, Resource=%d",
			totalReporterResources, totalCommonRepresentations, totalResources)
		logHelper.Info("[DRY-RUN] No data was modified")
	} else {
		logHelper.Infof("Resource deletion job completed successfully. Total records deleted: ReporterResource=%d, CommonRepresentation=%d, Resource=%d",
			totalReporterResources, totalCommonRepresentations, totalResources)
	}

	return nil
}

func logDryRunEstimate(logHelper *log.Helper, entityName string, count int64) {
	estimatedBatches := (count + DeleteBatchSize - 1) / DeleteBatchSize
	estimatedSeconds := estimatedBatches * int64(BatchDelayMs) / 1000

	logHelper.Infof("[DRY-RUN] %s: Found %d records", entityName, count)
	logHelper.Infof("[DRY-RUN] Estimated batches: %d", estimatedBatches)
	logHelper.Infof("[DRY-RUN] Estimated time: ~%d seconds", estimatedSeconds)
}

func logDeleteResult(logHelper *log.Helper, dryRun bool, description string, count int64) {
	if dryRun {
		logHelper.Infof("[DRY-RUN] Would delete %d total %s", count, description)
	} else {
		logHelper.Infof("Completed: Deleted %d total %s", count, description)
	}
}

func deleteBatchedByIDs(db *gorm.DB, logHelper *log.Helper, entityName string, model interface{}, whereClause string, whereArgs ...interface{}) (int64, error) {
	var totalDeleted int64
	batchCount := 0

	logHelper.Infof("Starting batched deletion of %s records...", entityName)

	for {
		var ids []uuid.UUID
		result := db.Model(model).
			Where(whereClause, whereArgs...).
			Limit(DeleteBatchSize).
			Pluck("id", &ids)

		if result.Error != nil {
			logHelper.Errorf("Failed to fetch %s IDs for batch %d: %v", entityName, batchCount+1, result.Error)
			return totalDeleted, result.Error
		}

		if len(ids) == 0 {
			break
		}

		deleteResult := db.Where("id IN ?", ids).Delete(model)
		if deleteResult.Error != nil {
			logHelper.Errorf("Failed to delete %s batch %d: %v", entityName, batchCount+1, deleteResult.Error)
			return totalDeleted, deleteResult.Error
		}

		totalDeleted += deleteResult.RowsAffected
		batchCount++

		logHelper.Infof("Batch %d: Deleted %d %s records (total so far: %d)", batchCount, deleteResult.RowsAffected, entityName, totalDeleted)

		if len(ids) < DeleteBatchSize {
			break
		}

		if BatchDelayMs > 0 {
			time.Sleep(time.Duration(BatchDelayMs) * time.Millisecond)
		}
	}

	return totalDeleted, nil
}

func deleteBatchedReporterResources(db *gorm.DB, logHelper *log.Helper, dryRun bool, resourceType string, reporterType string) (int64, error) {
	if dryRun {
		var count int64
		result := db.Model(&gormrepo.ReporterResource{}).
			Where("resource_type = ? AND reporter_type = ?", resourceType, reporterType).
			Count(&count)

		if result.Error != nil {
			logHelper.Errorf("Failed to count ReporterResource records: %v", result.Error)
			return 0, result.Error
		}

		logDryRunEstimate(logHelper, "ReporterResource", count)
		return count, nil
	}

	return deleteBatchedByIDs(db, logHelper, "ReporterResource", &gormrepo.ReporterResource{},
		"resource_type = ? AND reporter_type = ?", resourceType, reporterType)
}

func deleteBatchedCommonRepresentations(db *gorm.DB, logHelper *log.Helper, dryRun bool, reporterType string) (int64, error) {
	if dryRun {
		var count int64
		result := db.Model(&gormrepo.CommonRepresentation{}).
			Where("reported_by_reporter_type = ?", reporterType).
			Count(&count)

		if result.Error != nil {
			logHelper.Errorf("Failed to count CommonRepresentation records: %v", result.Error)
			return 0, result.Error
		}

		logDryRunEstimate(logHelper, "CommonRepresentation", count)
		return count, nil
	}

	var totalDeleted int64
	batchCount := 0

	logHelper.Info("Starting batched deletion of CommonRepresentation records...")

	for {
		// Use PostgreSQL-specific subquery with CTID to batch delete
		// CTID is the physical row identifier, allowing us to LIMIT deletes
		result := db.Exec(`
			DELETE FROM common_representations
			WHERE ctid IN (
				SELECT ctid
				FROM common_representations
				WHERE reported_by_reporter_type = ?
				LIMIT ?
			)
		`, reporterType, DeleteBatchSize)

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

		if BatchDelayMs > 0 {
			time.Sleep(time.Duration(BatchDelayMs) * time.Millisecond)
		}
	}

	return totalDeleted, nil
}

func deleteBatchedResources(db *gorm.DB, logHelper *log.Helper, dryRun bool, resourceType string) (int64, error) {
	if dryRun {
		var count int64
		result := db.Model(&gormrepo.Resource{}).
			Where("type = ?", resourceType).
			Count(&count)

		if result.Error != nil {
			logHelper.Errorf("Failed to count Resource records: %v", result.Error)
			return 0, result.Error
		}

		logDryRunEstimate(logHelper, "Resource", count)
		return count, nil
	}

	return deleteBatchedByIDs(db, logHelper, "Resource", &gormrepo.Resource{},
		"type = ?", resourceType)
}
