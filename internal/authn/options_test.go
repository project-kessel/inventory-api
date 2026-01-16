package authn

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/authn/oidc"
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
	// allow-unauthenticated and oidc are legacy fields for backwards compatibility, also loaded from YAML
	helpers.AllOptionsHaveFlags(t, prefix, fs, *test.options, []string{"authenticator", "allow-unauthenticated", "oidc"})
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
					{Type: "allow-unauthenticated"},
				},
			},
		}
		errs := options.Validate()
		assert.Empty(t, errs, "new format should not produce validation errors")
	})

	t.Run("new format preferred when both present", func(t *testing.T) {
		enabled := true
		options := &Options{
			AllowUnauthenticated: &enabled,
			Authenticator: &AuthenticatorOptions{
				Type: "first_match",
				Chain: []ChainEntryOptions{
					{Type: "allow-unauthenticated"},
				},
			},
		}
		errs := options.Validate()
		// New format is preferred, so validation should pass (validates new format only)
		assert.Empty(t, errs, "new format should be preferred, validation should pass")
	})

	t.Run("legacy oidc format", func(t *testing.T) {
		options := &Options{
			OIDC: &oidc.Options{
				AuthorizationServerURL: "http://keycloak:8084/realms/redhat-external",
				SkipClientIDCheck:      true,
			},
			Authenticator: nil,
		}
		errs := options.Validate()
		assert.Empty(t, errs, "legacy oidc format should not produce validation errors")
	})

	t.Run("new format preferred when legacy oidc and new format both present", func(t *testing.T) {
		options := &Options{
			OIDC: &oidc.Options{
				AuthorizationServerURL: "http://keycloak:8084/realms/redhat-external",
			},
			Authenticator: &AuthenticatorOptions{
				Type: "first_match",
				Chain: []ChainEntryOptions{
					{Type: "allow-unauthenticated"},
				},
			},
		}
		errs := options.Validate()
		// New format is preferred, so validation should pass (validates new format only)
		assert.Empty(t, errs, "new format should be preferred, validation should pass")
	})
}

func TestNewConfig_BackwardsCompatibility(t *testing.T) {
	t.Run("legacy allow-unauthenticated converts to allow-unauthenticated authenticator", func(t *testing.T) {
		enabled := true
		options := &Options{
			AllowUnauthenticated: &enabled,
			Authenticator:        nil,
		}
		cfg := NewConfig(options)
		assert.NotNil(t, cfg.Authenticator, "config should have authenticator")
		assert.Equal(t, "first_match", cfg.Authenticator.Type)
		assert.Len(t, cfg.Authenticator.Chain, 1)
		assert.Equal(t, "allow-unauthenticated", cfg.Authenticator.Chain[0].Type)
		assert.Nil(t, cfg.Authenticator.Chain[0].EnableHTTP, "enable_http should be nil (defaults to true)")
		assert.Nil(t, cfg.Authenticator.Chain[0].EnableGRPC, "enable_grpc should be nil (defaults to true)")
	})

	t.Run("legacy oidc converts to oidc authenticator", func(t *testing.T) {
		options := &Options{
			OIDC: &oidc.Options{
				AuthorizationServerURL: "http://keycloak:8084/realms/redhat-external",
				SkipClientIDCheck:      true,
				SkipIssuerCheck:        true,
				PrincipalUserDomain:    "localhost",
			},
			Authenticator: nil,
		}
		cfg := NewConfig(options)
		assert.NotNil(t, cfg.Authenticator, "config should have authenticator")
		assert.Equal(t, "first_match", cfg.Authenticator.Type)
		assert.Len(t, cfg.Authenticator.Chain, 1)
		assert.Equal(t, "oidc", cfg.Authenticator.Chain[0].Type)
		assert.Nil(t, cfg.Authenticator.Chain[0].EnableHTTP, "enable_http should be nil (defaults to true)")
		assert.Nil(t, cfg.Authenticator.Chain[0].EnableGRPC, "enable_grpc should be nil (defaults to true)")
		assert.NotNil(t, cfg.Authenticator.Chain[0].Config, "oidc config should be present")
		assert.Equal(t, "http://keycloak:8084/realms/redhat-external", cfg.Authenticator.Chain[0].Config["authn-server-url"])
		assert.Equal(t, true, cfg.Authenticator.Chain[0].Config["skip-client-id-check"])
		assert.Equal(t, true, cfg.Authenticator.Chain[0].Config["skip-issuer-check"])
		assert.Equal(t, "localhost", cfg.Authenticator.Chain[0].Config["principal-user-domain"])
	})

	t.Run("new format preserved", func(t *testing.T) {
		options := &Options{
			AllowUnauthenticated: nil,
			OIDC:                 nil,
			Authenticator: &AuthenticatorOptions{
				Type: "first_match",
				Chain: []ChainEntryOptions{
					{Type: "allow-unauthenticated"},
					{Type: "x-rh-identity"},
				},
			},
		}
		cfg := NewConfig(options)
		assert.NotNil(t, cfg.Authenticator)
		assert.Equal(t, "first_match", cfg.Authenticator.Type)
		assert.Len(t, cfg.Authenticator.Chain, 2)
		assert.Equal(t, "allow-unauthenticated", cfg.Authenticator.Chain[0].Type)
		assert.Equal(t, "x-rh-identity", cfg.Authenticator.Chain[1].Type)
	})

	t.Run("new format preferred over legacy oidc when both present", func(t *testing.T) {
		options := &Options{
			OIDC: &oidc.Options{
				AuthorizationServerURL: "http://legacy-keycloak:8084/realms/redhat-external",
			},
			Authenticator: &AuthenticatorOptions{
				Type: "first_match",
				Chain: []ChainEntryOptions{
					{
						Type: "oidc",
						Config: map[string]interface{}{
							"authn-server-url": "http://new-keycloak:8084/realms/redhat-external",
						},
					},
				},
			},
		}
		cfg := NewConfig(options)
		assert.NotNil(t, cfg.Authenticator)
		assert.Equal(t, "first_match", cfg.Authenticator.Type)
		assert.Len(t, cfg.Authenticator.Chain, 1)
		assert.Equal(t, "oidc", cfg.Authenticator.Chain[0].Type)
		// Should use new format (new-keycloak), not legacy format (legacy-keycloak)
		assert.Equal(t, "http://new-keycloak:8084/realms/redhat-external", cfg.Authenticator.Chain[0].Config["authn-server-url"])
	})

	t.Run("new format preferred over allow-unauthenticated when both present", func(t *testing.T) {
		enabled := true
		options := &Options{
			AllowUnauthenticated: &enabled,
			Authenticator: &AuthenticatorOptions{
				Type: "first_match",
				Chain: []ChainEntryOptions{
					{Type: "x-rh-identity"},
				},
			},
		}
		cfg := NewConfig(options)
		assert.NotNil(t, cfg.Authenticator)
		assert.Equal(t, "first_match", cfg.Authenticator.Type)
		assert.Len(t, cfg.Authenticator.Chain, 1)
		// Should use new format (x-rh-identity), not legacy format (guest)
		assert.Equal(t, "x-rh-identity", cfg.Authenticator.Chain[0].Type)
	})
}
