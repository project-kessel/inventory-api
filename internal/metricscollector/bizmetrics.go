package metricscollector

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"gorm.io/gorm"
)

const queryTimeout = 5 * time.Minute

// StartBusinessMetricsCollector runs a background goroutine that reads the
// latest metrics summary from the database on startup and then every 24 hours.
// The queries are performed by the metrics-collect-job CronJob; this
// collector only reads the pre-computed results.
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

	var summary model.MetricsSummary
	err := db.WithContext(queryCtx).
		Order("collected_at DESC").
		First(&summary).Error

	if err != nil {
		logger.Errorf("failed to read metrics summary: %v", err)
		return
	}

	logger.Infof("reading metrics summary from %s (id=%s)", summary.CollectedAt.Format(time.RFC3339), summary.ID)

	// Each recorder handles one metric key from the JSONB.
	// To add a new metric: add a recordXxx function and call it here.
	recordResourcesPerWorkspace(ctx, mc, summary.Metrics, logger)
	recordResourceCount(ctx, mc, summary.Metrics, logger)
}

func recordResourcesPerWorkspace(ctx context.Context, mc *MetricsCollector, metrics map[string]any, logger *log.Helper) {
	raw, ok := metrics["resources_per_workspace"]
	if !ok {
		logger.Warn("metrics summary missing resources_per_workspace key")
		return
	}

	entries, err := parseMetricEntries(raw)
	if err != nil {
		logger.Errorf("failed to parse resources_per_workspace: %v", err)
		return
	}

	for _, e := range entries {
		resourceType, _ := e["resource_type"].(string)
		count, _ := e["count"].(float64)

		mc.ResourcesPerWorkspace.Record(ctx, count,
			metric.WithAttributes(
				attribute.String("resource_type", resourceType),
			),
		)
	}

	logger.Infof("recorded resources_per_workspace metric for %d groups", len(entries))
}

func recordResourceCount(ctx context.Context, mc *MetricsCollector, metrics map[string]any, logger *log.Helper) {
	raw, ok := metrics["resource_count"]
	if !ok {
		logger.Warn("metrics summary missing resource_count key")
		return
	}

	entries, err := parseMetricEntries(raw)
	if err != nil {
		logger.Errorf("failed to parse resource_count: %v", err)
		return
	}

	for _, e := range entries {
		resourceType, _ := e["resource_type"].(string)
		reporterType, _ := e["reporter_type"].(string)
		reporterInstanceID, _ := e["reporter_instance_id"].(string)
		count, _ := e["count"].(float64)

		mc.ResourceCount.Record(ctx, int64(count),
			metric.WithAttributes(
				attribute.String("resource_type", resourceType),
				attribute.String("reporter_name", reporterType),
				attribute.String("reporter_id", reporterInstanceID),
			),
		)
	}

	logger.Infof("recorded resource_count metric for %d groups", len(entries))
}

// parseMetricEntries converts the raw JSONB value (which GORM deserializes as
// []interface{}) into a slice of string-keyed maps. This handles the type
// assertion dance that comes from unmarshalling JSONB through GORM.
func parseMetricEntries(raw any) ([]map[string]any, error) {
	switch v := raw.(type) {
	case []any:
		result := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				result = append(result, m)
			}
		}
		return result, nil
	default:
		// Fall back to JSON round-trip for unexpected types
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		var result []map[string]any
		if err := json.Unmarshal(b, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
}
