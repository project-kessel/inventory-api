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

	logHelper.Info("Starting metrics collection job")

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

	if err := db.Create(&summary).Error; err != nil {
		logHelper.Errorf("failed to write metrics summary: %v", err)
		return err
	}

	logHelper.Infof("Metrics summary written successfully (id=%s)", summary.ID)

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	result := db.Where("collected_at < ?", cutoff).Delete(&model.MetricsSummary{})
	if result.Error != nil {
		logHelper.Errorf("failed to clean up old metrics summaries: %v", result.Error)
		return result.Error
	}
	logHelper.Infof("Cleaned up %d metrics summaries older than %d days", result.RowsAffected, retentionDays)

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
		JOIN LATERAL (
			SELECT data
			FROM common_representations
			WHERE resource_id = r.id
			ORDER BY version DESC
			LIMIT 1
		) cr ON true
		WHERE cr.data->>'workspace_id' IS NOT NULL
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
