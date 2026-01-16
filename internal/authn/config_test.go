package authn

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigComplete_EnableHTTP_EnableGRPC_OIDCConfigRequiredOnlyWhenEnabledForAnyProtocol(t *testing.T) {
	t.Run("oidc enable_http=true requires config", func(t *testing.T) {
		c := &Config{
			Authenticator: &AuthenticatorConfig{
				Type: "first_match",
				Chain: []ChainEntry{
					{
						Type:       "oidc",
						EnableHTTP: boolPtr(true),
						EnableGRPC: boolPtr(false),
						Config:     nil,
					},
					// Keep gRPC enabled overall so we don't trip the global validation.
					{Type: "allow-unauthenticated", EnableHTTP: boolPtr(false), EnableGRPC: boolPtr(true)},
				},
			},
		}

		_, errs := c.Complete()
		assert.NotEmpty(t, errs)
		// Should specifically complain about missing oidc config (because oidc is enabled for http).
		assert.Contains(t, errs[0].Error(), "oidc authenticator requires config")
	})

	t.Run("oidc enable_http/enable_grpc=false does not require config", func(t *testing.T) {
		c := &Config{
			Authenticator: &AuthenticatorConfig{
				Type: "first_match",
				Chain: []ChainEntry{
					{
						Type:       "oidc",
						EnableHTTP: boolPtr(false),
						EnableGRPC: boolPtr(false),
						Config:     nil,
					},
					// Satisfy global validation for both protocols
					{Type: "x-rh-identity", EnableHTTP: boolPtr(true), EnableGRPC: boolPtr(true)},
				},
			},
		}

		_, errs := c.Complete()
		assert.Empty(t, errs)
	})
}

func TestConfigComplete_EnableHTTP_EnableGRPC_RequiresAtLeastOneAuthenticatorPerProtocol(t *testing.T) {
	t.Run("no http-enabled authenticators fails", func(t *testing.T) {
		c := &Config{
			Authenticator: &AuthenticatorConfig{
				Type: "first_match",
				Chain: []ChainEntry{
					{Type: "x-rh-identity", EnableHTTP: boolPtr(false), EnableGRPC: boolPtr(true)},
					{Type: "allow-unauthenticated", EnableHTTP: boolPtr(false), EnableGRPC: boolPtr(true)},
				},
			},
		}

		_, errs := c.Complete()
		assert.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Error(), "enabled for http")
	})

	t.Run("no grpc-enabled authenticators fails", func(t *testing.T) {
		c := &Config{
			Authenticator: &AuthenticatorConfig{
				Type: "first_match",
				Chain: []ChainEntry{
					{Type: "x-rh-identity", EnableHTTP: boolPtr(true), EnableGRPC: boolPtr(false)},
					{Type: "allow-unauthenticated", EnableHTTP: boolPtr(true), EnableGRPC: boolPtr(false)},
				},
			},
		}

		_, errs := c.Complete()
		assert.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Error(), "enabled for grpc")
	})
}

func boolPtr(v bool) *bool { return &v }
