package metricscollector

import (
	"context"
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

func TestCollectResourcesPerWorkspace(t *testing.T) {
	t.Run("records histogram with query results", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		rows := sqlmock.NewRows([]string{"resource_type", "count"}).
			AddRow("host", 3.0).
			AddRow("host", 15.0).
			AddRow("k8s_cluster", 7.0)
		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		collectResourcesPerWorkspace(context.Background(), db, mc, testLogger())

		require.NoError(t, mock.ExpectationsWereMet())

		m := findMetric(t, reader, prefix+"resources_per_workspace")
		require.NotNil(t, m, "expected resources_per_workspace metric to exist")

		hist, ok := m.Data.(metricdata.Histogram[float64])
		require.True(t, ok, "expected histogram data type")
		assert.NotEmpty(t, hist.DataPoints)

		var totalCount uint64
		for _, dp := range hist.DataPoints {
			totalCount += dp.Count
		}
		assert.Equal(t, uint64(3), totalCount, "expected 3 histogram observations (one per workspace/resource_type group)")
	})

	t.Run("handles empty results", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		rows := sqlmock.NewRows([]string{"resource_type", "count"})
		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		collectResourcesPerWorkspace(context.Background(), db, mc, testLogger())

		require.NoError(t, mock.ExpectationsWereMet())

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

	t.Run("handles query error gracefully", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("connection refused"))

		collectResourcesPerWorkspace(context.Background(), db, mc, testLogger())

		require.NoError(t, mock.ExpectationsWereMet())

		m := findMetric(t, reader, prefix+"resources_per_workspace")
		if m != nil {
			hist, ok := m.Data.(metricdata.Histogram[float64])
			if ok {
				var totalCount uint64
				for _, dp := range hist.DataPoints {
					totalCount += dp.Count
				}
				assert.Equal(t, uint64(0), totalCount, "no observations should be recorded on error")
			}
		}
	})

	t.Run("records correct bucket boundaries", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		rows := sqlmock.NewRows([]string{"resource_type", "count"}).
			AddRow("host", 1.0).
			AddRow("host", 50.0).
			AddRow("host", 999.0)
		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		collectResourcesPerWorkspace(context.Background(), db, mc, testLogger())

		require.NoError(t, mock.ExpectationsWereMet())

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

func TestCollectResourceCount(t *testing.T) {
	t.Run("records gauge with query results", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		rows := sqlmock.NewRows([]string{"resource_type", "reporter_type", "reporter_instance_id", "count"}).
			AddRow("host", "hbi", "instance-1", 42).
			AddRow("k8s_cluster", "ocm", "instance-2", 10)
		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		collectResourceCount(context.Background(), db, mc, testLogger())

		require.NoError(t, mock.ExpectationsWereMet())

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

	t.Run("handles empty results", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		rows := sqlmock.NewRows([]string{"resource_type", "reporter_type", "reporter_instance_id", "count"})
		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		collectResourceCount(context.Background(), db, mc, testLogger())

		require.NoError(t, mock.ExpectationsWereMet())

		m := findMetric(t, reader, prefix+"resource_count")
		if m != nil {
			gauge, ok := m.Data.(metricdata.Gauge[int64])
			if ok {
				assert.Empty(t, gauge.DataPoints)
			}
		}
	})

	t.Run("handles query error gracefully", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("connection refused"))

		collectResourceCount(context.Background(), db, mc, testLogger())

		require.NoError(t, mock.ExpectationsWereMet())

		m := findMetric(t, reader, prefix+"resource_count")
		if m != nil {
			gauge, ok := m.Data.(metricdata.Gauge[int64])
			if ok {
				assert.Empty(t, gauge.DataPoints, "no data points should be recorded on error")
			}
		}
	})
}

func TestCollectBusinessMetrics(t *testing.T) {
	t.Run("collects both metrics", func(t *testing.T) {
		mc, reader := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		workspaceRows := sqlmock.NewRows([]string{"resource_type", "count"}).
			AddRow("host", 5.0)
		mock.ExpectQuery("SELECT").WillReturnRows(workspaceRows)

		reporterRows := sqlmock.NewRows([]string{"resource_type", "reporter_type", "reporter_instance_id", "count"}).
			AddRow("host", "hbi", "instance-1", 5)
		mock.ExpectQuery("SELECT").WillReturnRows(reporterRows)

		collectBusinessMetrics(context.Background(), db, mc, testLogger())

		require.NoError(t, mock.ExpectationsWereMet())

		histMetric := findMetric(t, reader, prefix+"resources_per_workspace")
		require.NotNil(t, histMetric, "expected resources_per_workspace metric")

		gaugeMetric := findMetric(t, reader, prefix+"resource_count")
		require.NotNil(t, gaugeMetric, "expected resource_count metric")
	})
}

func TestStartBusinessMetricsCollector(t *testing.T) {
	t.Run("stops on context cancellation", func(t *testing.T) {
		mc, _ := setupTestMeterAndCollector(t)
		db, mock := setupTestDB(t)

		workspaceRows := sqlmock.NewRows([]string{"resource_type", "count"})
		mock.ExpectQuery("SELECT").WillReturnRows(workspaceRows)

		reporterRows := sqlmock.NewRows([]string{"resource_type", "reporter_type", "reporter_instance_id", "count"})
		mock.ExpectQuery("SELECT").WillReturnRows(reporterRows)

		ctx, cancel := context.WithCancel(context.Background())
		StartBusinessMetricsCollector(ctx, db, mc, testLogger())

		// Give the goroutine time to run the initial collection
		time.Sleep(100 * time.Millisecond)
		cancel()
		// Give the goroutine time to exit
		time.Sleep(100 * time.Millisecond)

		require.NoError(t, mock.ExpectationsWereMet())
	})
}
