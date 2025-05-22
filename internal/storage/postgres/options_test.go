package postgres

import (
	"fmt"
	"reflect"
	"testing"

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
			Host: "localhost",
			Port: "5432",
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
	prefix := "consumer.postgres"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	// the below logic ensures that every possible option defined in the Options type
	// has a defined flag for that option; auth and retry-options are skipped in favor of testing
	// in their own packages
	structValues := reflect.ValueOf(*test.options)
	for i := 0; i < structValues.Type().NumField(); i++ {
		flagName := structValues.Type().Field(i).Tag.Get("mapstructure")
		assert.NotNil(t, fs.Lookup(fmt.Sprintf("%s.%s", prefix, flagName)))
	}
}

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name        string
		options     *Options
		expectError bool
	}{
		{
			name: "valid SSL mode - disable",
			options: &Options{
				SSLMode: "disable",
			},
			expectError: false,
		},
		{
			name: "valid SSL mode - allow",
			options: &Options{
				SSLMode: "allow",
			},
			expectError: false,
		},
		{
			name: "valid SSL mode - prefer",
			options: &Options{
				SSLMode: "prefer",
			},
			expectError: false,
		},
		{
			name: "valid SSL mode - require",
			options: &Options{
				SSLMode: "require",
			},
			expectError: false,
		},
		{
			name: "valid SSL mode - verify-ca",
			options: &Options{
				SSLMode: "verify-ca",
			},
			expectError: false,
		},
		{
			name: "valid SSL mode - verify-full",
			options: &Options{
				SSLMode: "verify-full",
			},
			expectError: false,
		},
		{
			name: "valid SSL mode - fake",
			options: &Options{
				SSLMode: "fake",
			},
			expectError: true,
		},
		{
			name: "No SSL Mode",
			options: &Options{
				SSLMode: "",
			},
			expectError: false,
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
