package consumer

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetrics_New(t *testing.T) {
	test := TestCase{
		name:        "TestMetricsNew_DefaultMeters",
		description: "ensures a metrics collector is configured with all default meters",
	}
	errs := test.TestSetup()
	assert.Nil(t, errs)

	structValues := reflect.ValueOf(test.metrics)
	numField := structValues.NumField()

	// ensures all fields in struct are properly instantiated
	for i := 0; i < numField; i++ {
		field := structValues.Field(i)
		assert.True(t, field.IsValid())
		assert.True(t, !field.IsZero())
	}
	// ensures the number of fields in the type and instantiated version match
	assert.Equal(t, reflect.TypeOf(MetricsCollector{}).NumField(), reflect.TypeOf(test.metrics).NumField())
}
