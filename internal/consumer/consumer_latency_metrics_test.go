package consumer

import (
	"context"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	relationstest "github.com/project-kessel/inventory-api/internal/testutil/relations"
)

const (
	fetchRepresentationsDurationMetric = "consumer_fetch_representations_duration_seconds"
	messageProcessingDurationMetric    = "consumer_message_processing_duration_seconds"
)

// latencySample identifies one histogram series by its operation and outcome labels.
type latencySample struct {
	operation string
	outcome   string
}

// TestProcessMessage_RecordsLatencyHistograms drives messages through the real
// consumer path against the test database and checks the representation-fetch
// and whole-message latency histograms (RHCLOUD-53646) record one sample with
// the expected operation and outcome labels.
func TestProcessMessage_RecordsLatencyHistograms(t *testing.T) {
	tests := []struct {
		name string
		// localResourceId names the saved fixture; empty means no resource is saved.
		localResourceId string
		message         string
		operation       string
		// breakDB closes the database pool before processing so the
		// representation fetch fails at the database boundary.
		breakDB         bool
		expectErr       bool
		expectFetch     *latencySample
		expectProcessed latencySample
	}{
		{
			name:            "created succeeds",
			localResourceId: "latency-create",
			message:         `{"payload":{"reporter_resource_key":{"local_resource_id":"latency-create","resource_type":"integration","reporter":{"reporter_type":"notifications","reporter_instance_id":"notif-instance-1"}},"common_version":0,"reporter_representation_version":0,"reporter_generation":0}}`,
			operation:       string(model.OperationTypeCreated),
			expectFetch:     &latencySample{operation: "created", outcome: "success"},
			expectProcessed: latencySample{operation: "created", outcome: "success"},
		},
		{
			name:            "updated succeeds",
			localResourceId: "latency-update",
			message:         `{"payload":{"reporter_resource_key":{"local_resource_id":"latency-update","resource_type":"integration","reporter":{"reporter_type":"notifications","reporter_instance_id":"notif-instance-1"}},"common_version":0,"reporter_representation_version":0,"reporter_generation":0}}`,
			operation:       string(model.OperationTypeUpdated),
			expectFetch:     &latencySample{operation: "updated", outcome: "success"},
			expectProcessed: latencySample{operation: "updated", outcome: "success"},
		},
		{
			name:            "deleted succeeds",
			localResourceId: "latency-delete",
			message:         `{"payload":{"reporter_resource_key":{"local_resource_id":"latency-delete","resource_type":"integration","reporter":{"reporter_type":"notifications","reporter_instance_id":"notif-instance-1"}},"common_version":0,"reporter_representation_version":1,"reporter_generation":0}}`,
			operation:       string(model.OperationTypeDeleted),
			expectFetch:     &latencySample{operation: "deleted", outcome: "success"},
			expectProcessed: latencySample{operation: "deleted", outcome: "success"},
		},
		{
			name:            "created fetch fails",
			message:         `{"payload":{"reporter_resource_key":{"local_resource_id":"latency-create-fail","resource_type":"integration","reporter":{"reporter_type":"notifications","reporter_instance_id":"notif-instance-1"}},"common_version":0,"reporter_representation_version":0,"reporter_generation":0}}`,
			operation:       string(model.OperationTypeCreated),
			breakDB:         true,
			expectErr:       true,
			expectFetch:     &latencySample{operation: "created", outcome: "error"},
			expectProcessed: latencySample{operation: "created", outcome: "error"},
		},
		{
			name:            "deleted fetch fails",
			message:         `{"payload":{"reporter_resource_key":{"local_resource_id":"latency-delete-fail","resource_type":"integration","reporter":{"reporter_type":"notifications","reporter_instance_id":"notif-instance-1"}},"common_version":0,"reporter_representation_version":1,"reporter_generation":0}}`,
			operation:       string(model.OperationTypeDeleted),
			breakDB:         true,
			expectErr:       true,
			expectFetch:     &latencySample{operation: "deleted", outcome: "error"},
			expectProcessed: latencySample{operation: "deleted", outcome: "error"},
		},
		{
			// A create without a common version on the common-only path returns
			// before fetching; it still counts as a successfully processed message.
			name:            "created without common version returns early",
			message:         `{"payload":{"reporter_resource_key":{"local_resource_id":"latency-early","resource_type":"integration","reporter":{"reporter_type":"notifications","reporter_instance_id":"notif-instance-1"}},"reporter_representation_version":0}}`,
			operation:       string(model.OperationTypeCreated),
			expectProcessed: latencySample{operation: "created", outcome: "success"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var relationsRepo model.RelationsRepository
			if test.localResourceId != "" {
				relationsRepo = relationstest.New(t)
			}
			tester := TestCase{}
			require.Nil(t, tester.TestSetup(t, relationsRepo))

			reader := sdkmetric.NewManualReader()
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			mc := &metricscollector.MetricsCollector{}
			require.NoError(t, mc.New(provider.Meter("test")))
			tester.inv.MetricsCollector = mc

			if test.localResourceId != "" {
				fixture, err := model.NewResourceFixture(test.localResourceId, "integration", "notifications", "notif-instance-1", "workspace-v0")
				require.NoError(t, err)
				require.NoError(t, tester.inv.ResourceRepository.Save(tester.inv.DB, *fixture.Resource, model.OperationTypeCreated, fixture.InitialTransactionId))
			}

			if test.breakDB {
				sqlDB, err := tester.inv.DB.DB()
				require.NoError(t, err)
				require.NoError(t, sqlDB.Close())
			}

			_, err := tester.inv.ProcessMessage(map[string]string{"operation": test.operation}, true, &kafka.Message{Value: []byte(test.message)})
			if test.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			var rm metricdata.ResourceMetrics
			require.NoError(t, reader.Collect(context.Background(), &rm))

			wantFetch := map[latencySample]uint64{}
			if test.expectFetch != nil {
				wantFetch[*test.expectFetch] = 1
			}
			require.Equal(t, wantFetch, histogramCounts(t, rm, fetchRepresentationsDurationMetric))
			require.Equal(t, map[latencySample]uint64{test.expectProcessed: 1}, histogramCounts(t, rm, messageProcessingDurationMetric))
		})
	}
}

// histogramCounts returns the sample count of every series of the named
// float64 histogram, keyed by its operation and outcome labels. Each series
// must carry exactly those two labels.
func histogramCounts(t *testing.T, rm metricdata.ResourceMetrics, name string) map[latencySample]uint64 {
	t.Helper()
	counts := map[latencySample]uint64{}
	for _, scopeMetrics := range rm.ScopeMetrics {
		for _, m := range scopeMetrics.Metrics {
			if m.Name != name {
				continue
			}
			histogram, ok := m.Data.(metricdata.Histogram[float64])
			require.True(t, ok, "%s must be a float64 histogram, got %T", name, m.Data)
			for _, point := range histogram.DataPoints {
				require.Equal(t, 2, point.Attributes.Len(), "%s must carry only operation and outcome labels", name)
				operation, _ := point.Attributes.Value(attribute.Key("operation"))
				outcome, _ := point.Attributes.Value(attribute.Key("outcome"))
				counts[latencySample{operation: operation.AsString(), outcome: outcome.AsString()}] += point.Count
			}
		}
	}
	return counts
}
