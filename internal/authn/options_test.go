package authn

import (
	"testing"

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
			Authenticator: nil, // Authenticator is populated from config files
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
	prefix := "authn"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	// the below logic ensures that every possible option defined in the Options type
	// has a defined flag for that option; authenticator config is typically loaded from YAML
	// allow-unauthenticated is a legacy field for backwards compatibility, also loaded from YAML
	helpers.AllOptionsHaveFlags(t, prefix, fs, *test.options, []string{"authenticator", "allow-unauthenticated"})
}

func TestOptions_Validate_BackwardsCompatibility(t *testing.T) {
	t.Run("legacy allow-unauthenticated format", func(t *testing.T) {
		enabled := true
		options := &Options{
			AllowUnauthenticated: &enabled,
			Authenticator:        nil,
		}
		errs := options.Validate()
		assert.Empty(t, errs, "legacy format should not produce validation errors")
	})

	t.Run("new format", func(t *testing.T) {
		options := &Options{
			AllowUnauthenticated: nil,
			Authenticator: &AuthenticatorOptions{
				Type: "first_match",
				Chain: []ChainEntryOptions{
					{Type: "guest"},
				},
			},
		}
		errs := options.Validate()
		assert.Empty(t, errs, "new format should not produce validation errors")
	})

	t.Run("both formats should error", func(t *testing.T) {
		enabled := true
		options := &Options{
			AllowUnauthenticated: &enabled,
			Authenticator: &AuthenticatorOptions{
				Type: "first_match",
				Chain: []ChainEntryOptions{
					{Type: "guest"},
				},
			},
		}
		errs := options.Validate()
		assert.NotEmpty(t, errs, "using both formats should produce validation error")
		assert.Contains(t, errs[0].Error(), "cannot use both")
	})
}

func TestNewConfig_BackwardsCompatibility(t *testing.T) {
	t.Run("legacy allow-unauthenticated converts to guest authenticator", func(t *testing.T) {
		enabled := true
		options := &Options{
			AllowUnauthenticated: &enabled,
			Authenticator:        nil,
		}
		cfg := NewConfig(options)
		assert.NotNil(t, cfg.Authenticator, "config should have authenticator")
		assert.Equal(t, "first_match", cfg.Authenticator.Type)
		assert.Len(t, cfg.Authenticator.Chain, 1)
		assert.Equal(t, "guest", cfg.Authenticator.Chain[0].Type)
		assert.Nil(t, cfg.Authenticator.Chain[0].Enabled, "enabled should be nil (defaults to true)")
	})

	t.Run("new format preserved", func(t *testing.T) {
		options := &Options{
			AllowUnauthenticated: nil,
			Authenticator: &AuthenticatorOptions{
				Type: "first_match",
				Chain: []ChainEntryOptions{
					{Type: "guest"},
					{Type: "x-rh-identity"},
				},
			},
		}
		cfg := NewConfig(options)
		assert.NotNil(t, cfg.Authenticator)
		assert.Equal(t, "first_match", cfg.Authenticator.Type)
		assert.Len(t, cfg.Authenticator.Chain, 2)
		assert.Equal(t, "guest", cfg.Authenticator.Chain[0].Type)
		assert.Equal(t, "x-rh-identity", cfg.Authenticator.Chain[1].Type)
	})
}
