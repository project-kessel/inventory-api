package authn

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigComplete_EnableHTTP_EnableGRPC_OIDCConfigRequiredOnlyWhenEnabledForAnyProtocol(t *testing.T) {
	t.Run("oidc transport.http=true requires config", func(t *testing.T) {
		c := &Config{
			Authenticator: &AuthenticatorConfig{
				Type: "first_match",
				Chain: []ChainEntry{
					{
						Type:      "oidc",
						Transport: &Transport{HTTP: boolPtr(true), GRPC: boolPtr(false)},
						Config:    nil,
					},
					// Keep gRPC enabled overall so we don't trip the global validation.
					{Type: "allow-unauthenticated", Transport: &Transport{HTTP: boolPtr(false), GRPC: boolPtr(true)}},
				},
			},
		}

		_, errs := c.Complete()
		assert.NotEmpty(t, errs)
		// Should specifically complain about missing oidc config (because oidc is enabled for http).
		found := false
		for _, err := range errs {
			if strings.Contains(err.Error(), "oidc authenticator requires config") {
				found = true
				break
			}
		}
		assert.True(t, found, "expected an error mentioning missing oidc config, got: %v", errs)
	})

	t.Run("oidc transport.http/grpc=false does not require config", func(t *testing.T) {
		c := &Config{
			Authenticator: &AuthenticatorConfig{
				Type: "first_match",
				Chain: []ChainEntry{
					{
						Type:      "oidc",
						Transport: &Transport{HTTP: boolPtr(false), GRPC: boolPtr(false)},
						Config:    nil,
					},
					// Satisfy global validation for both protocols
					{Type: "x-rh-identity", Transport: &Transport{HTTP: boolPtr(true), GRPC: boolPtr(true)}},
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
					{Type: "x-rh-identity", Transport: &Transport{HTTP: boolPtr(false), GRPC: boolPtr(true)}},
					{Type: "allow-unauthenticated", Transport: &Transport{HTTP: boolPtr(false), GRPC: boolPtr(true)}},
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
					{Type: "x-rh-identity", Transport: &Transport{HTTP: boolPtr(true), GRPC: boolPtr(false)}},
					{Type: "allow-unauthenticated", Transport: &Transport{HTTP: boolPtr(true), GRPC: boolPtr(false)}},
				},
			},
		}

		_, errs := c.Complete()
		assert.NotEmpty(t, errs)
		assert.Contains(t, errs[0].Error(), "enabled for grpc")
	})
}

func TestConfigComplete_WithGrpcEndpoints(t *testing.T) {
	t.Run("grpc-endpoints converted from config map to OIDC options", func(t *testing.T) {
		c := &Config{
			Authenticator: &AuthenticatorConfig{
				Type: "first_match",
				Chain: []ChainEntry{
					{
						Type:      "oidc",
						Transport: &Transport{HTTP: boolPtr(false), GRPC: boolPtr(true)},
						Config: map[string]interface{}{
							"authn-server-url": "https://example.com",
							"client-id":        "test-client",
							"grpc-endpoints": []interface{}{
								"/kessel.inventory.v1beta2.KesselTupleService/CreateTuples",
								"/kessel.inventory.v1beta2.KesselTupleService/DeleteTuples",
							},
						},
					},
					{Type: "allow-unauthenticated", Transport: &Transport{HTTP: boolPtr(true), GRPC: boolPtr(true)}},
				},
			},
		}

		completed, errs := c.Complete()
		assert.Empty(t, errs)
		assert.NotNil(t, completed.Authenticator)
		assert.Len(t, completed.Authenticator.ChainConfigs, 2)

		// Verify conversion from []interface{} (config map) to []string (GrpcEndpoints field)
		oidcChainConfig := completed.Authenticator.ChainConfigs[0]
		assert.Equal(t, "oidc", oidcChainConfig.Type)
		assert.NotNil(t, oidcChainConfig.OIDCConfig)
		assert.Equal(t, []string{
			"/kessel.inventory.v1beta2.KesselTupleService/CreateTuples",
			"/kessel.inventory.v1beta2.KesselTupleService/DeleteTuples",
		}, oidcChainConfig.OIDCConfig.GrpcEndpoints)
	})
}

func boolPtr(v bool) *bool { return &v }
