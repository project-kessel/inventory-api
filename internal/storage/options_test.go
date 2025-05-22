package storage

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/storage/postgres"
	"github.com/project-kessel/inventory-api/internal/storage/sqlite3"
	"github.com/project-kessel/inventory-api/test/helpers"
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
			Postgres:                postgres.NewOptions(),
			SqlLite3:                sqlite3.NewOptions(),
			Database:                "sqlite3",
			DisablePersistence:      false,
			MaxSerializationRetries: 10,
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
	prefix := "storage"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	// the below logic ensures that every possible option defined in the Options type
	// has a defined flag for that option; postgres and sqlite3 options are skipped in favor of testing
	// in their own packages
	helpers.AllOptionsHaveFlags(t, prefix, fs, *test.options, []string{"postgres", "sqlite3"})
}

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name        string
		options     *Options
		expectError bool
	}{
		{
			name: "postgres database",
			options: &Options{
				Database: "postgres",
				Postgres: &postgres.Options{
					SSLMode: "",
				},
			},
			expectError: false,
		},
		{
			name: "sqlite database",
			options: &Options{
				Database: "sqlite3",
			},
			expectError: false,
		},
		{
			name: "invalid database",
			options: &Options{
				Database: "fake",
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
