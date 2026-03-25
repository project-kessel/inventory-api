package jobs

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricsCollectJobCommand(t *testing.T) {
	cmd := NewMetricsCollectJobCommand(nil, common.LoggerOptions{})

	assert.Equal(t, "metrics-collect-job", cmd.Use)
	assert.NotEmpty(t, cmd.Short)

	retentionFlag := cmd.Flags().Lookup("retention-days")
	require.NotNil(t, retentionFlag)
	assert.Equal(t, "30", retentionFlag.DefValue)
}

func TestCollectResourceCountJob(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	createTestReporterResource(t, db, "host", "hbi")
	createTestReporterResource(t, db, "host", "hbi")
	createTestReporterResource(t, db, "k8s-cluster", "ocm")

	metrics := internal.JsonObject{}
	err := collectResourceCountJob(db, logger, metrics)
	require.NoError(t, err)

	raw, ok := metrics["resource_count"]
	require.True(t, ok)

	entries, ok := raw.([]map[string]any)
	require.True(t, ok)
	assert.Len(t, entries, 2)

	counts := map[string]int64{}
	for _, e := range entries {
		key := e["resource_type"].(string) + "/" + e["reporter_type"].(string)
		counts[key] = e["count"].(int64)
	}
	assert.Equal(t, int64(2), counts["host/hbi"])
	assert.Equal(t, int64(1), counts["k8s-cluster/ocm"])
}

func TestCollectResourceCountJob_ExcludesTombstoned(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	// Create a normal resource
	createTestReporterResource(t, db, "host", "hbi")

	// Create a tombstoned resource
	resourceID := uuid.New()
	require.NoError(t, db.Create(&model.Resource{ID: resourceID, Type: "host"}).Error)
	require.NoError(t, db.Create(&model.ReporterResource{
		ID: uuid.New(),
		ReporterResourceKey: model.ReporterResourceKey{
			LocalResourceID:    "tombstoned-" + resourceID.String(),
			ReporterType:       "hbi",
			ResourceType:       "host",
			ReporterInstanceID: "instance-123",
		},
		ResourceID: resourceID,
		Tombstone:  true,
	}).Error)

	metrics := internal.JsonObject{}
	err := collectResourceCountJob(db, logger, metrics)
	require.NoError(t, err)

	entries := metrics["resource_count"].([]map[string]any)
	assert.Len(t, entries, 1)
	assert.Equal(t, int64(1), entries[0]["count"])
}

func TestCollectResourceCountJob_EmptyDatabase(t *testing.T) {
	db := setupTestDB(t)
	logger := testLogger()

	metrics := internal.JsonObject{}
	err := collectResourceCountJob(db, logger, metrics)
	require.NoError(t, err)

	entries := metrics["resource_count"].([]map[string]any)
	assert.Empty(t, entries)
}

func TestMetricsSummaryWriteAndCleanup(t *testing.T) {
	db := setupTestDB(t)

	// Migrate the metrics_summary table
	err := db.AutoMigrate(&model.MetricsSummary{})
	require.NoError(t, err)

	// Write an old summary (40 days ago)
	oldSummary := model.MetricsSummary{
		ID:          uuid.New(),
		CollectedAt: time.Now().UTC().AddDate(0, 0, -40),
		Metrics:     internal.JsonObject{"test": "old"},
	}
	require.NoError(t, db.Create(&oldSummary).Error)

	// Write a recent summary (5 days ago)
	recentSummary := model.MetricsSummary{
		ID:          uuid.New(),
		CollectedAt: time.Now().UTC().AddDate(0, 0, -5),
		Metrics:     internal.JsonObject{"test": "recent"},
	}
	require.NoError(t, db.Create(&recentSummary).Error)

	// Write today's summary
	todaySummary := model.MetricsSummary{
		ID:          uuid.New(),
		CollectedAt: time.Now().UTC(),
		Metrics:     internal.JsonObject{"test": "today"},
	}
	require.NoError(t, db.Create(&todaySummary).Error)

	// Verify all 3 exist
	var count int64
	db.Model(&model.MetricsSummary{}).Count(&count)
	assert.Equal(t, int64(3), count)

	// Clean up with 30-day retention
	cutoff := time.Now().UTC().AddDate(0, 0, -DefaultRetentionDays)
	result := db.Where("collected_at < ?", cutoff).Delete(&model.MetricsSummary{})
	require.NoError(t, result.Error)
	assert.Equal(t, int64(1), result.RowsAffected)

	// Verify only 2 remain
	db.Model(&model.MetricsSummary{}).Count(&count)
	assert.Equal(t, int64(2), count)

	// Verify the old one was deleted
	var remaining []model.MetricsSummary
	db.Find(&remaining)
	for _, s := range remaining {
		assert.NotEqual(t, oldSummary.ID, s.ID)
	}
}

func TestMetricsSummaryWriteAndCleanup_NoOldData(t *testing.T) {
	db := setupTestDB(t)
	err := db.AutoMigrate(&model.MetricsSummary{})
	require.NoError(t, err)

	summary := model.MetricsSummary{
		ID:          uuid.New(),
		CollectedAt: time.Now().UTC(),
		Metrics:     internal.JsonObject{"test": "today"},
	}
	require.NoError(t, db.Create(&summary).Error)

	cutoff := time.Now().UTC().AddDate(0, 0, -DefaultRetentionDays)
	result := db.Where("collected_at < ?", cutoff).Delete(&model.MetricsSummary{})
	require.NoError(t, result.Error)
	assert.Equal(t, int64(0), result.RowsAffected)

	var count int64
	db.Model(&model.MetricsSummary{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestMetricsSummaryLatestRead(t *testing.T) {
	db := setupTestDB(t)
	err := db.AutoMigrate(&model.MetricsSummary{})
	require.NoError(t, err)

	// Create summaries at different times
	older := model.MetricsSummary{
		ID:          uuid.New(),
		CollectedAt: time.Now().UTC().AddDate(0, 0, -2),
		Metrics:     internal.JsonObject{"version": "older"},
	}
	require.NoError(t, db.Create(&older).Error)

	latest := model.MetricsSummary{
		ID:          uuid.New(),
		CollectedAt: time.Now().UTC(),
		Metrics:     internal.JsonObject{"version": "latest"},
	}
	require.NoError(t, db.Create(&latest).Error)

	// Read the latest
	var result model.MetricsSummary
	require.NoError(t, db.Order("collected_at DESC").First(&result).Error)

	assert.Equal(t, latest.ID, result.ID)
}
