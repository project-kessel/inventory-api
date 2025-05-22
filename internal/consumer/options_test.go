package consumer

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/project-kessel/inventory-api/internal/consumer/auth"
	"github.com/project-kessel/inventory-api/internal/consumer/retry"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestNewOptions(t *testing.T) {
	test := struct {
		options         *Options
		expectedOptions *Options
	}{
		options: NewOptions(),
		expectedOptions: &Options{
			Enabled:                 true,
			ConsumerGroupID:         "inventory-consumer",
			Topic:                   "outbox.event.kessel.tuples",
			SessionTimeout:          "45000",
			HeartbeatInterval:       "3000",
			MaxPollInterval:         "300000",
			EnableAutoCommit:        "false",
			AutoOffsetReset:         "earliest",
			StatisticsInterval:      "60000",
			Debug:                   "",
			AuthOptions:             auth.NewOptions(),
			RetryOptions:            retry.NewOptions(),
			ReadAfterWriteEnabled:   true,
			ReadAfterWriteAllowlist: []string{},
		},
	}
	assert.Equal(t, test.expectedOptions, NewOptions())
}

func TestOptions_AddFlags(t *testing.T) {
	test := struct {
		options *Options
	}{
		options: NewOptions(),
	}
	prefix := "consumer"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	// the below logic ensures that every possible option defined in the Options type
	// has a defined flag for that option; auth and retry-options are skipped in favor of testing
	// in their own packages
	structValues := reflect.ValueOf(*test.options)
	for i := 0; i < structValues.Type().NumField(); i++ {
		flagName := structValues.Type().Field(i).Tag.Get("mapstructure")
		if flagName == "auth" || flagName == "retry-options" {
			continue
		} else {
			assert.NotNil(t, fs.Lookup(fmt.Sprintf("%s.%s", prefix, flagName)))
		}
	}
}

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name        string
		options     *Options
		expectError bool
	}{
		{
			name: "bootstrap servers is set",
			options: &Options{
				Enabled: true,
				BootstrapServers: []string{
					"test-server:9092",
				}},
			expectError: false,
		},
		{
			name: "bootstrap servers is empty",
			options: &Options{
				Enabled:          true,
				BootstrapServers: []string{},
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := test.options.Validate()
			if test.expectError {
				assert.NotNil(t, errs)
			} else {
				assert.Nil(t, errs)
			}
		})
	}
}
