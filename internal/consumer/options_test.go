package consumer

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/consumer/auth"
	"github.com/project-kessel/inventory-api/internal/consumer/retry"
	"github.com/project-kessel/inventory-api/internal/helpers"
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
			CommitModulo:            10,
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
	helpers.AllOptionsHaveFlags(t, prefix, fs, *test.options, []string{"auth", "retry-options"})
}

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name        string
		options     *Options
		expectError bool
	}{
		{
			name: "bootstrap servers is set and consumer is enabled",
			options: &Options{
				Enabled: true,
				BootstrapServers: []string{
					"test-server:9092",
				},
				CommitModulo: 10,
			},
			expectError: false,
		},
		{
			name: "bootstrap servers is empty and consumer is enabled",
			options: &Options{
				Enabled:          true,
				BootstrapServers: []string{},
				CommitModulo:     10,
			},
			expectError: true,
		},
		{
			name: "bootstrap servers is empty and consumer is disabled",
			options: &Options{
				Enabled:          false,
				BootstrapServers: []string{},
				CommitModulo:     10,
			},
			expectError: false,
		},
		{
			name: "bootstrap servers is set but consumer is disabled",
			options: &Options{
				Enabled: false,
				BootstrapServers: []string{
					"test-server:9092",
				},
				CommitModulo: 10,
			},
			expectError: false,
		},
		{
			name: "commit modulo is set to a positive number",
			options: &Options{
				Enabled: true,
				BootstrapServers: []string{
					"test-server:9092",
				},
				CommitModulo: 1,
			},
			expectError: false,
		},
		{
			name: "commit modulo is set to a negative number and fails",
			options: &Options{
				Enabled: true,
				BootstrapServers: []string{
					"test-server:9092",
				},
				CommitModulo: -1,
			},
			expectError: true,
		},
		{
			name: "commit modulo is set to zero and fails",
			options: &Options{
				Enabled: true,
				BootstrapServers: []string{
					"test-server:9092",
				},
				CommitModulo: 0,
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
