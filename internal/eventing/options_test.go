package eventing

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/eventing/kafka"
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
			Kafka:   kafka.NewOptions(),
			Eventer: "stdout",
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
	prefix := "eventing"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	// the below logic ensures that every possible option defined in the Options type
	// has a defined flag for that option; kakfa is skipped in favor of testing
	// in its own package
	helpers.AllOptionsHaveFlags(t, prefix, fs, *test.options, []string{"kafka"})
}

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name        string
		options     *Options
		expectError bool
	}{
		{
			name: "stdout eventer",
			options: &Options{
				Eventer: "stdout",
			},
			expectError: false,
		},
		{
			name: "kafka eventer",
			options: &Options{
				Eventer: "kafka",
			},
			expectError: false,
		},
		{
			name: "invalid eventer",
			options: &Options{
				Eventer: "fake",
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
