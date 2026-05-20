package jobs

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const DefaultRetentionDays = 30

func NewMetricsCollectJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var retentionDays int

	cmd := &cobra.Command{
		Use:   "metrics-collect-job",
		Short: "Collect business metrics and store in summary table",
		Long: `Runs metrics queries against the database and stores the results
		in the metrics_summary table for lightweight consumption by Inventory API replicas.
		Also cleans up data older than the retention period.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return collectMetrics(storageOptions, loggerOptions, retentionDays)
		},
	}

	cmd.Flags().IntVar(&retentionDays, "retention-days", DefaultRetentionDays, "Number of days to retain metrics summary data")

	return cmd
}

func collectMetrics(storageOptions *storage.Options, loggerOptions common.LoggerOptions, retentionDays int) error {
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "metrics_collect"))

	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.New(storageConfig, logHelper)
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close() //nolint:errcheck

	return collectMetricsWithDB(db, logHelper, retentionDays)
}

func collectMetricsWithDB(db *gorm.DB, logHelper *log.Helper, retentionDays int) error {
	logHelper.Info("Starting metrics collection job")

	// Increase work_mem for this session to eliminate Parallel Hash Join batching.
	// default work_mem (4MB) forces PostgreSQL to split into 8 batches with
	// repeated disk I/O. This is session-scoped and does not affect other connections.
	if err := db.Exec("SET work_mem = '256MB'").Error; err != nil {
		logHelper.Warnf("failed to set work_mem: %v (continuing with default)", err)
	}

	metrics := internal.JsonObject{}

	if err := collectResourcesPerWorkspaceJob(db, logHelper, metrics); err != nil {
		return err
	}

	if err := collectResourceCountJob(db, logHelper, metrics); err != nil {
		return err
	}

	summary := model.MetricsSummary{
		ID:          uuid.New(),
		CollectedAt: time.Now().UTC(),
		Metrics:     metrics,
	}

	if err := db.Session(&gorm.Session{Logger: db.Logger.LogMode(gormlogger.Silent)}).Create(&summary).Error; err != nil {
		// Failed admin operation - SEC-MON-REQ-1 compliance (#3 admin_action, #2 system_object_manipulation, #11 warnings_or_errors)
		logHelper.Errorw("msg", "Cronjob: metrics write failed",
			"action", "CREATE",
			"resource_type", "metrics_summary",
			"resource_id", summary.ID.String(),
			"principal", "system:cronjob:metrics-collect-job",
			"outcome", "failure",
			"error", err.Error(),
		)
		return err
	}

	// Cronjob metrics write - SEC-MON-REQ-1 compliance (#3 admin_action, #2 system_object_manipulation)
	logHelper.Infow("msg", "Cronjob: metrics summary written",
		"action", "CREATE",
		"resource_type", "metrics_summary",
		"resource_id", summary.ID.String(),
		"principal", "system:cronjob:metrics-collect-job",
		"outcome", "success",
	)

	if retentionDays <= 0 {
		logHelper.Warnf("invalid retention-days value %d, using default %d", retentionDays, DefaultRetentionDays)
		retentionDays = DefaultRetentionDays
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	result := db.Where("collected_at < ?", cutoff).Delete(&model.MetricsSummary{})
	if result.Error != nil {
		// Failed admin operation - SEC-MON-REQ-1 compliance (#3 admin_action, #2 system_object_manipulation, #11 warnings_or_errors)
		logHelper.Errorw("msg", "Cronjob: metrics cleanup failed",
			"action", "DELETE",
			"resource_type", "metrics_summary",
			"resource_id", fmt.Sprintf("older_than_%d_days", retentionDays),
			"principal", "system:cronjob:metrics-collect-job",
			"retention_days", retentionDays,
			"outcome", "failure",
			"error", result.Error.Error(),
		)
		return result.Error
	}

	// Cronjob metrics cleanup - SEC-MON-REQ-1 compliance (#3 admin_action, #2 system_object_manipulation)
	logHelper.Infow("msg", "Cronjob: metrics cleanup completed",
		"action", "DELETE",
		"resource_type", "metrics_summary",
		"resource_id", fmt.Sprintf("older_than_%d_days", retentionDays),
		"principal", "system:cronjob:metrics-collect-job",
		"deleted_count", result.RowsAffected,
		"retention_days", retentionDays,
		"outcome", "success",
	)

	logHelper.Info("Metrics collection job completed successfully")
	return nil
}

type workspaceResult struct {
	ResourceType string
	Count        float64
}

func collectResourcesPerWorkspaceJob(db *gorm.DB, logHelper *log.Helper, metrics internal.JsonObject) error {
	var results []workspaceResult

	err := db.Raw(`
		SELECT r.type AS resource_type,
		       COUNT(*)::float AS count
		FROM resource r
		JOIN common_representations cr
			ON cr.resource_id = r.id AND cr.version = r.common_version
		WHERE cr.data->>'workspace_id' IS NOT NULL
		  AND EXISTS (
			SELECT 1 FROM reporter_resources rr
			WHERE rr.resource_id = r.id AND NOT rr.tombstone
		  )
		GROUP BY r.type, cr.data->>'workspace_id'
	`).Scan(&results).Error

	if err != nil {
		logHelper.Errorf("failed to collect resources_per_workspace: %v", err)
		return fmt.Errorf("resources_per_workspace query failed: %w", err)
	}

	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"resource_type": r.ResourceType,
			"count":         r.Count,
		})
	}
	metrics["resources_per_workspace"] = out

	logHelper.Infof("Collected resources_per_workspace: %d groups", len(results))
	return nil
}

type reporterResult struct {
	ResourceType       string
	ReporterType       string
	ReporterInstanceID string
	Count              int64
}

func collectResourceCountJob(db *gorm.DB, logHelper *log.Helper, metrics internal.JsonObject) error {
	var results []reporterResult

	err := db.Raw(`
		SELECT resource_type,
		       reporter_type,
		       reporter_instance_id,
		       COUNT(*) AS count
		FROM reporter_resources
		WHERE NOT tombstone
		GROUP BY resource_type, reporter_type, reporter_instance_id
	`).Scan(&results).Error

	if err != nil {
		logHelper.Errorf("failed to collect resource_count: %v", err)
		return fmt.Errorf("resource_count query failed: %w", err)
	}

	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"resource_type":        r.ResourceType,
			"reporter_type":        r.ReporterType,
			"reporter_instance_id": r.ReporterInstanceID,
			"count":                r.Count,
		})
	}
	metrics["resource_count"] = out

	logHelper.Infof("Collected resource_count: %d groups", len(results))
	return nil
}
