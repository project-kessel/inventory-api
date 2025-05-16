package metricscollector

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
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
