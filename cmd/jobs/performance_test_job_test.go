package jobs

import (
	"bytes"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	"github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculatePercentileEmpty(t *testing.T) {
	assert.Equal(t, time.Duration(0), calculatePercentile(nil, 99))
	assert.Equal(t, time.Duration(0), calculatePercentile([]time.Duration{}, 50))
}

func TestCalculatePercentileSingleValue(t *testing.T) {
	assert.Equal(t, 5*time.Millisecond, calculatePercentile([]time.Duration{5 * time.Millisecond}, 50))
	assert.Equal(t, 5*time.Millisecond, calculatePercentile([]time.Duration{5 * time.Millisecond}, 99))
}

func TestCalculatePercentileKnownDistribution(t *testing.T) {
	// Latencies 1ms..100ms; the implementation selects sorted[floor(n*p/100)],
	// i.e. the (floor(n*p/100)+1)-th smallest value.
	latencies := make([]time.Duration, 100)
	for i := range latencies {
		latencies[i] = time.Duration(i+1) * time.Millisecond
	}

	assert.Equal(t, 51*time.Millisecond, calculatePercentile(latencies, 50))
	assert.Equal(t, 96*time.Millisecond, calculatePercentile(latencies, 95))
	assert.Equal(t, 100*time.Millisecond, calculatePercentile(latencies, 99))
}

func TestCalculatePercentileDoesNotMutateInput(t *testing.T) {
	latencies := []time.Duration{30 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond}

	assert.Equal(t, 20*time.Millisecond, calculatePercentile(latencies, 50))

	assert.Equal(t, []time.Duration{30 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond}, latencies)
}

func TestPrintMetricsReportsSortedMinMax(t *testing.T) {
	var buf bytes.Buffer
	logHelper := log.NewHelper(log.NewStdLogger(&buf))

	metrics := &PerformanceMetrics{
		TotalQueries:      3,
		SuccessfulQueries: 3,
		Latencies:         []time.Duration{5 * time.Millisecond, 1 * time.Millisecond, 3 * time.Millisecond},
		StartTime:         time.Now().Add(-time.Second),
		EndTime:           time.Now(),
	}

	printMetrics(logHelper, "test", metrics)

	output := buf.String()
	assert.Contains(t, output, "Min latency: 1ms")
	assert.Contains(t, output, "Max latency: 5ms")
}

func TestExecuteRepresentationsQueries(t *testing.T) {
	db := setupTestDB(t)

	resourceID := uuid.New()
	resource := model.Resource{ID: resourceID, Type: "host"}
	require.NoError(t, db.Create(&resource).Error)

	reporterResource := model.ReporterResource{
		ID: uuid.New(),
		ReporterResourceKey: model.ReporterResourceKey{
			LocalResourceID:    "local-perf-1",
			ReporterType:       "hbi",
			ResourceType:       "host",
			ReporterInstanceID: "instance-1",
		},
		ResourceID:            resourceID,
		APIHref:               "https://api.example.com/resource/1",
		RepresentationVersion: 2,
		Generation:            1,
		Tombstone:             false,
	}
	require.NoError(t, db.Create(&reporterResource).Error)

	for _, version := range []uint{1, 2} {
		commonRep := model.CommonRepresentation{
			Representation:         model.Representation{Data: internal.JsonObject{"version": version}},
			ResourceId:             resourceID,
			Version:                version,
			ReportedByReporterType: "hbi",
		}
		require.NoError(t, db.Create(&commonRep).Error)
	}

	key := ResourceKey{
		LocalResourceID:    "local-perf-1",
		ReporterType:       "hbi",
		ResourceType:       "host",
		ReporterInstanceID: "instance-1",
		LatestVersion:      2,
	}

	require.NoError(t, executeLatestRepresentationsQuery(db, key))
	require.NoError(t, executeVersionedRepresentationsQuery(db, key))

	// A key with no matching resource returns no rows without error
	missing := ResourceKey{
		LocalResourceID:    "local-missing",
		ReporterType:       "hbi",
		ResourceType:       "host",
		ReporterInstanceID: "instance-1",
		LatestVersion:      2,
	}
	require.NoError(t, executeLatestRepresentationsQuery(db, missing))
	require.NoError(t, executeVersionedRepresentationsQuery(db, missing))
}
