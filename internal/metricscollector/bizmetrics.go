package metricscollector

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"gorm.io/gorm"
)

const queryTimeout = 5 * time.Minute

type workspaceResourceCount struct {
	ResourceType string
	Count        float64
}

type reporterResourceCount struct {
	ResourceType       string
	ReporterType       string
	ReporterInstanceID string
	Count              int64
}

// StartBusinessMetricsCollector runs a background goroutine that queries the database once per
// day and records business metrics. The query is also executed on restarts to ensure
// a scrape is not missed due to multiple reboots in a single day, pushing the 24 hour window out further
func StartBusinessMetricsCollector(ctx context.Context, db *gorm.DB, mc *MetricsCollector, logger *log.Helper) {
	go func() {
		collectBusinessMetrics(ctx, db, mc, logger)

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				collectBusinessMetrics(ctx, db, mc, logger)
			}
		}
	}()
}

func collectBusinessMetrics(ctx context.Context, db *gorm.DB, mc *MetricsCollector, logger *log.Helper) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	collectResourcesPerWorkspace(queryCtx, db, mc, logger)
	collectResourceCount(queryCtx, db, mc, logger)
}

func collectResourcesPerWorkspace(ctx context.Context, db *gorm.DB, mc *MetricsCollector, logger *log.Helper) {
	var results []workspaceResourceCount

	// For each resource, get the latest common representation (max version),
	// extract workspace_id from the JSONB data, and count resources per
	// workspace_id and resource_type. The result is one row per workspace per
	// resource_type with the count of resources in that workspace.
	err := db.WithContext(ctx).Raw(`
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
		logger.Errorf("failed to collect resources_per_workspace metric: %v", err)
		return
	}

	for _, r := range results {
		mc.ResourcesPerWorkspace.Record(ctx, r.Count,
			metric.WithAttributes(
				attribute.String("resource_type", r.ResourceType),
			),
		)
	}

	logger.Infof("recorded resources_per_workspace metric for %d (workspace, resource_type) groups", len(results))
}

func collectResourceCount(ctx context.Context, db *gorm.DB, mc *MetricsCollector, logger *log.Helper) {
	var results []reporterResourceCount

	err := db.WithContext(ctx).Raw(`
		SELECT resource_type,
		       reporter_type,
		       reporter_instance_id,
		       COUNT(*) AS count
		FROM reporter_resources
		WHERE NOT tombstone
		GROUP BY resource_type, reporter_type, reporter_instance_id
	`).Scan(&results).Error

	if err != nil {
		logger.Errorf("failed to collect resource_count metric: %v", err)
		return
	}

	for _, r := range results {
		mc.ResourceCount.Record(ctx, r.Count,
			metric.WithAttributes(
				attribute.String("resource_type", r.ResourceType),
				attribute.String("reporter_name", r.ReporterType),
				attribute.String("reporter_id", r.ReporterInstanceID),
			),
		)
	}

	logger.Infof("recorded resource_count metric for %d (resource_type, reporter_name, reporter_id) groups", len(results))
}
