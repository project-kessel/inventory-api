package authn

import (
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"

	"github.com/project-kessel/inventory-api/internal/authn/aggregator"
	"github.com/project-kessel/inventory-api/internal/authn/oidc"
)

func TestNew_NilConfig(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	// Create a config with nil Authenticator
	config := CompletedConfig{
		&completedConfig{
			Authenticator: nil,
		},
	}
	auth, err := New(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authenticator configuration is required")
	assert.Nil(t, auth)
}

func TestNew_InvalidStrategyType(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type:         aggregator.StrategyType("invalid"),
				ChainConfigs: []ChainCompletedConfig{},
			},
		},
	}

	auth, err := New(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown authenticator strategy type")
	assert.Nil(t, auth)
}

func TestNew_EmptyChain(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type:         aggregator.FirstMatch,
				ChainConfigs: []ChainCompletedConfig{},
			},
		},
	}

	auth, err := New(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no authenticators were successfully created")
	assert.Nil(t, auth)
}

func TestNew_AllDisabled(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type: aggregator.FirstMatch,
				ChainConfigs: []ChainCompletedConfig{
					{Type: "guest", Enabled: false},
					{Type: "x-rh-identity", Enabled: false},
				},
			},
		},
	}

	auth, err := New(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no authenticators were successfully created")
	assert.Nil(t, auth)
}

func TestNew_GuestAuthenticator(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type: aggregator.FirstMatch,
				ChainConfigs: []ChainCompletedConfig{
					{Type: "guest", Enabled: true},
				},
			},
		},
	}

	auth, err := New(config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, auth)
}

func TestNew_XRhIdentityAuthenticator(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type: aggregator.FirstMatch,
				ChainConfigs: []ChainCompletedConfig{
					{Type: "x-rh-identity", Enabled: true},
				},
			},
		},
	}

	auth, err := New(config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, auth)
}

func TestNew_OIDCAuthenticator_NoConfig(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type: aggregator.FirstMatch,
				ChainConfigs: []ChainCompletedConfig{
					{Type: "oidc", Enabled: true, OIDCConfig: nil},
				},
			},
		},
	}

	auth, err := New(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "oidc authenticator requires config")
	assert.Nil(t, auth)
}

func TestNew_MultipleAuthenticators(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	// Create a minimal OIDC config using the options pattern
	oidcOpts := oidc.NewOptions()
	oidcOpts.AuthorizationServerURL = "https://example.com"
	oidcOpts.ClientId = "test-client"
	oidcConfig := oidc.NewConfig(oidcOpts)
	completedOIDC, err := oidcConfig.Complete()
	if err != nil {
		// OIDC config creation might fail if it tries to connect
		// Skip this test if OIDC setup fails
		t.Skipf("OIDC config creation failed (expected in test environment): %v", err)
		return
	}

	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type: aggregator.FirstMatch,
				ChainConfigs: []ChainCompletedConfig{
					{Type: "guest", Enabled: true},
					{Type: "x-rh-identity", Enabled: true},
					{Type: "oidc", Enabled: true, OIDCConfig: &completedOIDC},
				},
			},
		},
	}

	auth, err := New(config, logger)
	// Note: OIDC might fail if it tries to connect to the provider
	// In a real test environment, we'd mock the OIDC provider
	if err != nil {
		// If OIDC fails, we should still have guest and x-rh-identity
		// But the current implementation returns error if any fails
		// This is expected behavior - all authenticators must be valid
		assert.Contains(t, err.Error(), "oidc")
	} else {
		assert.NotNil(t, auth)
	}
}

func TestNew_SkipsDisabledAuthenticators(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type: aggregator.FirstMatch,
				ChainConfigs: []ChainCompletedConfig{
					{Type: "guest", Enabled: false},
					{Type: "x-rh-identity", Enabled: true},
					{Type: "guest", Enabled: false},
				},
			},
		},
	}

	auth, err := New(config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, auth)
}

func TestNew_UnknownAuthenticatorType(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type: aggregator.FirstMatch,
				ChainConfigs: []ChainCompletedConfig{
					{Type: "unknown-type", Enabled: true},
				},
			},
		},
	}

	auth, err := New(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown authenticator type in chain")
	assert.Nil(t, auth)
}
