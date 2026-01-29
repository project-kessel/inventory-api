package authz

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/authz/kessel"
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
			Authz:  AllowAll,
			Kessel: kessel.NewOptions(),
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
	prefix := "authz"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	// the below logic ensures that every possible option defined in the Options type
	// has a defined flag for that option; kessel section is skipped
	// in favor of testing in their own packages or via config files
	helpers.AllOptionsHaveFlags(t, prefix, fs, *test.options, []string{"kessel"})
}

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name        string
		options     *Options
		expectError bool
	}{
		{
			name: "allow all impl",
			options: &Options{
				Authz: "allow-all",
			},
			expectError: false,
		},
		{
			name: "kessel impl",
			options: &Options{
				Authz: "kessel",
				Kessel: &kessel.Options{
					URL: "relations-api",
				},
			},
			expectError: false,
		},
		{
			name: "invalid impl",
			options: &Options{
				Authz: "fake",
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
