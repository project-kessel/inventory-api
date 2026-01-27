package jobs

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

const (
	DefaultBatchSize  = 5000
	DefaultBatchDelay = 1000
)

func NewResourceDeleteJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var dryRun bool
	var resourceType string
	var reporterType string
	var batchSize int
	var batchDelayMs int

	cmd := &cobra.Command{
		Use:   "resource-delete-job",
		Short: "Delete resources from the database",
		Long:  "Delete resources from the database by resource type and reporter type",
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteResources(storageOptions, loggerOptions, dryRun, resourceType, reporterType, batchSize, batchDelayMs)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview deletion counts without executing any deletes")
	cmd.Flags().StringVar(&resourceType, "resource-type", "", "The resource type to delete (e.g., 'host', 'k8s-cluster')")
	cmd.Flags().StringVar(&reporterType, "reporter-type", "", "The reporter type to filter by (e.g., 'hbi', 'ocm')")
	cmd.Flags().IntVar(&batchSize, "batch-size", DefaultBatchSize, "Number of records to delete per batch")
	cmd.Flags().IntVar(&batchDelayMs, "batch-delay-ms", DefaultBatchDelay, "Delay between batches in milliseconds")

	_ = cmd.MarkFlagRequired("resource-type")
	_ = cmd.MarkFlagRequired("reporter-type")

	return cmd
}

func deleteResources(storageOptions *storage.Options, loggerOptions common.LoggerOptions, dryRun bool, resourceType string, reporterType string, batchSize int, batchDelayMs int) error {
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

	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.New(storageConfig, logHelper)
	if err != nil {
		return err
	}

	if dryRun {
		logHelper.Infof("Dry-run: %t", dryRun)
	}

	logHelper.Infof("Starting resource deletion job for resource_type=%s, reporter_type=%s", resourceType, reporterType)
	if !dryRun {
		logHelper.Infof("Using batch size: %d rows, delay between batches: %dms", batchSize, batchDelayMs)
	}

	// Delete ReporterResource records in batches (CASCADE will automatically delete ReporterRepresentation)
	totalReporterResources, err := deleteBatchedReporterResources(db, logHelper, dryRun, resourceType, reporterType, batchSize, batchDelayMs)
	if err != nil {
		return err
	}
	logDeleteResult(logHelper, dryRun, "ReporterResource records (ReporterRepresentation cascade deleted)", totalReporterResources)

	// Delete CommonRepresentation records in batches
	totalCommonRepresentations, err := deleteBatchedCommonRepresentations(db, logHelper, dryRun, reporterType, batchSize, batchDelayMs)
	if err != nil {
		return err
	}
	logDeleteResult(logHelper, dryRun, "CommonRepresentation records", totalCommonRepresentations)

	// Delete Resource records in batches
	totalResources, err := deleteBatchedResources(db, logHelper, dryRun, resourceType, batchSize, batchDelayMs)
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

func logDryRunEstimate(logHelper *log.Helper, entityName string, count int64, batchSize int, batchDelayMs int) {
	estimatedBatches := (count + int64(batchSize) - 1) / int64(batchSize)
	estimatedSeconds := estimatedBatches * int64(batchDelayMs) / 1000

	logHelper.Infof("[DRY-RUN] %s: Found %d records", entityName, count)
	logHelper.Infof("[DRY-RUN] Estimated batches: %d", estimatedBatches)
	logHelper.Infof("[DRY-RUN] Estimated time: ~%d seconds. This is based on the number of batches and the delay between batches and does not account for the actual time it takes to delete the records.", estimatedSeconds)
}

func logDeleteResult(logHelper *log.Helper, dryRun bool, description string, count int64) {
	if dryRun {
		logHelper.Infof("[DRY-RUN] Would delete %d total %s", count, description)
	} else {
		logHelper.Infof("Completed: Deleted %d total %s", count, description)
	}
}

func deleteBatchedByIDs(db *gorm.DB, logHelper *log.Helper, entityName string, model interface{}, batchSize int, batchDelayMs int, whereClause string, whereArgs ...interface{}) (int64, error) {
	var totalDeleted int64
	batchCount := 0

	// Get the table name from the model
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		logHelper.Errorf("Failed to parse model for %s: %v", entityName, err)
		return 0, err
	}
	tableName := stmt.Schema.Table

	logHelper.Infof("Starting batched deletion of %s records...", entityName)

	for {
		// Use subquery-based DELETE to avoid separate SELECT query
		query := fmt.Sprintf(`
			DELETE FROM %s
			WHERE id IN (
				SELECT id
				FROM %s
				WHERE %s
				LIMIT ?
			)
		`, tableName, tableName, whereClause)

		args := append(whereArgs, batchSize)
		result := db.Exec(query, args...)

		if result.Error != nil {
			logHelper.Errorf("Failed to delete %s batch %d: %v", entityName, batchCount+1, result.Error)
			return totalDeleted, result.Error
		}

		if result.RowsAffected == 0 {
			break
		}

		totalDeleted += result.RowsAffected
		batchCount++

		logHelper.Infof("Batch %d: Deleted %d %s records (total so far: %d)", batchCount, result.RowsAffected, entityName, totalDeleted)

		if batchDelayMs > 0 {
			time.Sleep(time.Duration(batchDelayMs) * time.Millisecond)
		}
	}

	return totalDeleted, nil
}

func deleteBatchedReporterResources(db *gorm.DB, logHelper *log.Helper, dryRun bool, resourceType string, reporterType string, batchSize int, batchDelayMs int) (int64, error) {
	if dryRun {
		var count int64
		result := db.Model(&model.ReporterResource{}).
			Where("resource_type = ? AND reporter_type = ?", resourceType, reporterType).
			Count(&count)

		if result.Error != nil {
			logHelper.Errorf("Failed to count ReporterResource records: %v", result.Error)
			return 0, result.Error
		}

		logDryRunEstimate(logHelper, "ReporterResource", count, batchSize, batchDelayMs)
		return count, nil
	}

	return deleteBatchedByIDs(db, logHelper, "ReporterResource", &model.ReporterResource{}, batchSize, batchDelayMs,
		"resource_type = ? AND reporter_type = ?", resourceType, reporterType)
}

func deleteBatchedCommonRepresentations(db *gorm.DB, logHelper *log.Helper, dryRun bool, reporterType string, batchSize int, batchDelayMs int) (int64, error) {
	if dryRun {
		var count int64
		result := db.Model(&model.CommonRepresentation{}).
			Where("reported_by_reporter_type = ?", reporterType).
			Count(&count)

		if result.Error != nil {
			logHelper.Errorf("Failed to count CommonRepresentation records: %v", result.Error)
			return 0, result.Error
		}

		logDryRunEstimate(logHelper, "CommonRepresentation", count, batchSize, batchDelayMs)
		return count, nil
	}

	var totalDeleted int64
	batchCount := 0

	logHelper.Info("Starting batched deletion of CommonRepresentation records...")

	for {
		// Use subquery with composite primary key to batch delete
		result := db.Exec(`
			DELETE FROM common_representations
			WHERE (resource_id, version) IN (
				SELECT resource_id, version
				FROM common_representations
				WHERE reported_by_reporter_type = ?
				LIMIT ?
			)
		`, reporterType, batchSize)

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

func deleteBatchedResources(db *gorm.DB, logHelper *log.Helper, dryRun bool, resourceType string, batchSize int, batchDelayMs int) (int64, error) {
	if dryRun {
		var count int64
		result := db.Model(&model.Resource{}).
			Where("type = ?", resourceType).
			Count(&count)

		if result.Error != nil {
			logHelper.Errorf("Failed to count Resource records: %v", result.Error)
			return 0, result.Error
		}

		logDryRunEstimate(logHelper, "Resource", count, batchSize, batchDelayMs)
		return count, nil
	}

	return deleteBatchedByIDs(db, logHelper, "Resource", &model.Resource{}, batchSize, batchDelayMs,
		"type = ?", resourceType)
}
