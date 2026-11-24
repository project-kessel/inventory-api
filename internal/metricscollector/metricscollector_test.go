package metricscollector

import (
	"errors"
	"reflect"
	"testing"
	"time"

	promclient "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestMetrics_New(t *testing.T) {
	test := struct {
		mc MetricsCollector
	}{
		mc: MetricsCollector{},
	}
	err := test.mc.New(otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel"))
	assert.Nil(t, err)

	structValues := reflect.ValueOf(test.mc)
	numField := structValues.NumField()

	// ensures all fields in struct are properly instantiated
	for i := 0; i < numField; i++ {
		field := structValues.Field(i)
		assert.True(t, field.IsValid())
		assert.True(t, !field.IsZero())
	}
	// ensures the number of fields in the type and instantiated version match
	assert.Equal(t, reflect.TypeOf(MetricsCollector{}).NumField(), reflect.TypeOf(test.mc).NumField())
}

// TestConsumerLatencyHistograms_PrometheusNames records through the same
// Prometheus exporter configuration the server uses and checks the series names
// the Grafana panels and alerts for RHCLOUD-53646 query: the exporter must not
// add a unit or counter suffix to these histograms.
func TestConsumerLatencyHistograms_PrometheusNames(t *testing.T) {
	registry := promclient.NewRegistry()
	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	require.NoError(t, err)
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))

	mc := &MetricsCollector{}
	require.NoError(t, mc.New(provider.Meter("test")))

	start := time.Now().Add(-20 * time.Millisecond)
	RecordDuration(mc.FetchRepresentationsDuration, start, "created", nil)
	RecordDuration(mc.MessageProcessingDuration, start, "deleted", errors.New("fetch failed"))

	families, err := registry.Gather()
	require.NoError(t, err)

	byName := map[string]*dto.MetricFamily{}
	for _, family := range families {
		byName[family.GetName()] = family
	}

	tests := []struct {
		name      string
		operation string
		outcome   string
	}{
		{name: "consumer_fetch_representations_duration_seconds", operation: "created", outcome: "success"},
		{name: "consumer_message_processing_duration_seconds", operation: "deleted", outcome: "error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			family, ok := byName[test.name]
			require.True(t, ok, "series %s not exposed", test.name)
			require.Equal(t, dto.MetricType_HISTOGRAM, family.GetType())
			require.Len(t, family.GetMetric(), 1)

			series := family.GetMetric()[0]
			labels := map[string]string{}
			for _, label := range series.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			require.Equal(t, test.operation, labels["operation"])
			require.Equal(t, test.outcome, labels["outcome"])

			histogram := series.GetHistogram()
			require.Equal(t, uint64(1), histogram.GetSampleCount())
			require.GreaterOrEqual(t, histogram.GetSampleSum(), 0.02, "duration must be recorded in seconds")
			require.Less(t, histogram.GetSampleSum(), 10.0, "duration must be recorded in seconds")
		})
	}
}
