package metricscollector

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestMeterAndCollector(t *testing.T) (*MetricsCollector, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

	mc := &MetricsCollector{}
	err := mc.New(meter)
	require.NoError(t, err)

	return mc, reader
}

func setupTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		sqlDB.Close()
	})

	return db, mock
}

func testLogger() *log.Helper {
	return log.NewHelper(log.DefaultLogger)
}

func findMetric(t *testing.T, reader *sdkmetric.ManualReader, name string) *metricdata.Metrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return &m
			}
		}
	}
	return nil
}

func mockSummaryRow(t *testing.T, mock sqlmock.Sqlmock, metrics map[string]any) {
	t.Helper()
	metricsJSON, err := json.Marshal(metrics)
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{"id", "collected_at", "metrics"}).
		AddRow("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", time.Now().UTC(), metricsJSON)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
}

func TestRecordResourcesPerWorkspace(t *testing.T) {
	t.Run("records histogram from summary data", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)

		metrics := map[string]any{
			"resources_per_workspace": []map[string]any{
				{"resource_type": "host", "count": 3.0},
				{"resource_type": "host", "count": 15.0},
				{"resource_type": "k8s_cluster", "count": 7.0},
			},
		}

		recordResourcesPerWorkspace(context.Background(), mc, metrics, testLogger())

		m := findMetric(t, reader, prefix+"resources_per_workspace")
		require.NotNil(t, m, "expected resources_per_workspace metric to exist")

		hist, ok := m.Data.(metricdata.Histogram[float64])
		require.True(t, ok, "expected histogram data type")

		var totalCount uint64
		for _, dp := range hist.DataPoints {
			totalCount += dp.Count
		}
		assert.Equal(t, uint64(3), totalCount)
	})

	t.Run("handles missing key gracefully", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)
		metrics := map[string]any{}

		recordResourcesPerWorkspace(context.Background(), mc, metrics, testLogger())

		m := findMetric(t, reader, prefix+"resources_per_workspace")
		if m != nil {
			hist, ok := m.Data.(metricdata.Histogram[float64])
			if ok {
				var totalCount uint64
				for _, dp := range hist.DataPoints {
					totalCount += dp.Count
				}
				assert.Equal(t, uint64(0), totalCount)
			}
		}
	})

	t.Run("records correct bucket boundaries", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)

		metrics := map[string]any{
			"resources_per_workspace": []map[string]any{
				{"resource_type": "host", "count": 1.0},
				{"resource_type": "host", "count": 50.0},
				{"resource_type": "host", "count": 999.0},
			},
		}

		recordResourcesPerWorkspace(context.Background(), mc, metrics, testLogger())

		m := findMetric(t, reader, prefix+"resources_per_workspace")
		require.NotNil(t, m)

		hist, ok := m.Data.(metricdata.Histogram[float64])
		require.True(t, ok)
		require.NotEmpty(t, hist.DataPoints)

		expectedBounds := []float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000}
		for _, dp := range hist.DataPoints {
			assert.Equal(t, expectedBounds, dp.Bounds)
		}
	})
}

func TestRecordResourceCount(t *testing.T) {
	t.Run("records gauge from summary data", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)

		metrics := map[string]any{
			"resource_count": []map[string]any{
				{"resource_type": "host", "reporter_type": "hbi", "reporter_instance_id": "instance-1", "count": 42.0},
				{"resource_type": "k8s_cluster", "reporter_type": "ocm", "reporter_instance_id": "instance-2", "count": 10.0},
			},
		}

		recordResourceCount(context.Background(), mc, metrics, testLogger())

		m := findMetric(t, reader, prefix+"resource_count")
		require.NotNil(t, m, "expected resource_count metric to exist")

		gauge, ok := m.Data.(metricdata.Gauge[int64])
		require.True(t, ok, "expected gauge data type")
		assert.Len(t, gauge.DataPoints, 2)

		values := map[string]int64{}
		for _, dp := range gauge.DataPoints {
			resourceType, _ := dp.Attributes.Value("resource_type")
			reporterName, _ := dp.Attributes.Value("reporter_name")
			reporterID, _ := dp.Attributes.Value("reporter_id")
			key := fmt.Sprintf("%s/%s/%s", resourceType.AsString(), reporterName.AsString(), reporterID.AsString())
			values[key] = dp.Value
		}

		assert.Equal(t, int64(42), values["host/hbi/instance-1"])
		assert.Equal(t, int64(10), values["k8s_cluster/ocm/instance-2"])
	})

	t.Run("handles missing key gracefully", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)
		metrics := map[string]any{}

		recordResourceCount(context.Background(), mc, metrics, testLogger())

		m := findMetric(t, reader, prefix+"resource_count")
		if m != nil {
			gauge, ok := m.Data.(metricdata.Gauge[int64])
			if ok {
				assert.Empty(t, gauge.DataPoints)
			}
		}
	})
}

func TestCollectBusinessMetrics(t *testing.T) {
	t.Run("reads summary and records both metrics", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		mockSummaryRow(t, mock, map[string]any{
			"resources_per_workspace": []map[string]any{
				{"resource_type": "host", "count": 5.0},
			},
			"resource_count": []map[string]any{
				{"resource_type": "host", "reporter_type": "hbi", "reporter_instance_id": "instance-1", "count": 5.0},
			},
		})

		collectBusinessMetrics(context.Background(), db, mc, testLogger())

		require.NoError(t, mock.ExpectationsWereMet())

		histMetric := findMetric(t, reader, prefix+"resources_per_workspace")
		require.NotNil(t, histMetric, "expected resources_per_workspace metric")

		gaugeMetric := findMetric(t, reader, prefix+"resource_count")
		require.NotNil(t, gaugeMetric, "expected resource_count metric")
	})

	t.Run("handles missing summary table gracefully", func(t *testing.T) {
		mc, _ := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("relation \"metrics_summaries\" does not exist"))

		collectBusinessMetrics(context.Background(), db, mc, testLogger())

		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestStartBusinessMetricsCollector(t *testing.T) {
	t.Run("stops on context cancellation", func(t *testing.T) {
		mc, _ := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		// The initial collection will query the summary table
		mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("no rows"))

		ctx, cancel := context.WithCancel(context.Background())
		StartBusinessMetricsCollector(ctx, db, mc, testLogger())

		time.Sleep(100 * time.Millisecond)
		cancel()
		time.Sleep(100 * time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestParseMetricEntries(t *testing.T) {
	t.Run("parses []any from GORM JSONB", func(t *testing.T) {
		input := []any{
			map[string]any{"resource_type": "host", "count": 5.0},
			map[string]any{"resource_type": "k8s_cluster", "count": 3.0},
		}

		result, err := parseMetricEntries(input)
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "host", result[0]["resource_type"])
		assert.Equal(t, 5.0, result[0]["count"])
	})

	t.Run("handles JSON round-trip for unexpected types", func(t *testing.T) {
		// Simulate a type that needs JSON round-trip
		input := json.RawMessage(`[{"resource_type":"host","count":5}]`)

		result, err := parseMetricEntries(input)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "host", result[0]["resource_type"])
	})

	t.Run("skips non-map entries", func(t *testing.T) {
		input := []any{
			map[string]any{"resource_type": "host", "count": 5.0},
			"invalid",
			42,
		}

		result, err := parseMetricEntries(input)
		require.NoError(t, err)
		assert.Len(t, result, 1)
	})
}
