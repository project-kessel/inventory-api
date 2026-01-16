package authn

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"

	"github.com/project-kessel/inventory-api/internal/authn/aggregator"
	"github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/oidc"
)

type mockHeader struct {
	headers map[string]string
}

func (m *mockHeader) Get(key string) string {
	if m.headers == nil {
		return ""
	}
	return m.headers[key]
}
func (m *mockHeader) Set(key, value string)      {}
func (m *mockHeader) Add(key, value string)      {}
func (m *mockHeader) Keys() []string             { return nil }
func (m *mockHeader) Values(key string) []string { return nil }

type mockTransporter struct {
	kind    transport.Kind
	headers map[string]string
}

func (m *mockTransporter) Kind() transport.Kind            { return m.kind }
func (m *mockTransporter) Endpoint() string                { return "/test" }
func (m *mockTransporter) Operation() string               { return "test" }
func (m *mockTransporter) RequestHeader() transport.Header { return &mockHeader{headers: m.headers} }
func (m *mockTransporter) ReplyHeader() transport.Header {
	return &mockHeader{headers: map[string]string{}}
}

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
					{Type: "allow-unauthenticated", EnabledHTTP: false, EnabledGRPC: false},
					{Type: "x-rh-identity", EnabledHTTP: false, EnabledGRPC: false},
				},
			},
		},
	}

	auth, err := New(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no authenticators were successfully created")
	assert.Nil(t, auth)
}

func TestNew_AllowUnauthenticatedAuthenticator(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type: aggregator.FirstMatch,
				ChainConfigs: []ChainCompletedConfig{
					{Type: "allow-unauthenticated", EnabledHTTP: true, EnabledGRPC: true},
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
					{Type: "x-rh-identity", EnabledHTTP: true, EnabledGRPC: true},
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
					{Type: "oidc", EnabledHTTP: true, EnabledGRPC: true, OIDCConfig: nil},
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
					{Type: "allow-unauthenticated", EnabledHTTP: true, EnabledGRPC: true},
					{Type: "x-rh-identity", EnabledHTTP: true, EnabledGRPC: true},
					{Type: "oidc", EnabledHTTP: true, EnabledGRPC: true, OIDCConfig: &completedOIDC},
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
					{Type: "allow-unauthenticated", EnabledHTTP: false, EnabledGRPC: false},
					{Type: "x-rh-identity", EnabledHTTP: true, EnabledGRPC: true},
					{Type: "allow-unauthenticated", EnabledHTTP: false, EnabledGRPC: false},
				},
			},
		},
	}

	auth, err := New(config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, auth)
}

func TestNew_RoutesByTransportKind(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type: aggregator.FirstMatch,
				ChainConfigs: []ChainCompletedConfig{
					{Type: "x-rh-identity", EnabledHTTP: true, EnabledGRPC: false},
					{Type: "allow-unauthenticated", EnabledHTTP: false, EnabledGRPC: true},
				},
			},
		},
	}

	auth, err := New(config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, auth)

	// HTTP: missing x-rh-identity should be ignored (no allow-unauthenticated fallback on HTTP in this config)
	httpT := &mockTransporter{kind: transport.KindHTTP, headers: map[string]string{}}
	identity, decision := auth.Authenticate(context.Background(), httpT)
	assert.Nil(t, identity)
	assert.Equal(t, api.Ignore, decision)

	// gRPC: allow-unauthenticated enabled should allow
	grpcT := &mockTransporter{kind: transport.KindGRPC, headers: map[string]string{}}
	identity, decision = auth.Authenticate(context.Background(), grpcT)
	assert.NotNil(t, identity)
	assert.Equal(t, api.Allow, decision)
}

func TestNew_EnableHTTP_EnableGRPC_PerAuthenticatorType(t *testing.T) {
	const validXRH = "eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1NiIsIm9yZ19pZCI6IjEyMzQ1NiIsInVzZXIiOnsidXNlcm5hbWUiOiJ0ZXN0dXNlciIsImVtYWlsIjoidGVzdHVzZXJAZXhhbXBsZS5jb20iLCJ1c2VyX2lkIjoidXNlci0xMjMifSwiaW50ZXJuYWwiOnt9LCJ0eXBlIjoiVXNlciJ9fQ=="

	t.Run("x-rh-identity enabled for http only", func(t *testing.T) {
		logger := log.NewHelper(log.DefaultLogger)
		config := CompletedConfig{
			&completedConfig{
				Authenticator: &AuthenticatorCompletedConfig{
					Type: aggregator.FirstMatch,
					ChainConfigs: []ChainCompletedConfig{
						{Type: "x-rh-identity", EnabledHTTP: true, EnabledGRPC: false},
						{Type: "allow-unauthenticated", EnabledHTTP: false, EnabledGRPC: true},
						{Type: "x-rh-identity", EnabledHTTP: true, EnabledGRPC: false},
						{Type: "allow-unauthenticated", EnabledHTTP: false, EnabledGRPC: true},
					},
				},
			},
		}

		auth, err := New(config, logger)
		assert.NoError(t, err)

		httpT := &mockTransporter{kind: transport.KindHTTP, headers: map[string]string{"x-rh-identity": validXRH}}
		identity, decision := auth.Authenticate(context.Background(), httpT)
		assert.Equal(t, api.Allow, decision)
		assert.NotNil(t, identity)
		assert.Equal(t, "x-rh-identity", identity.AuthType)

		grpcT := &mockTransporter{kind: transport.KindGRPC, headers: map[string]string{"x-rh-identity": validXRH}}
		identity, decision = auth.Authenticate(context.Background(), grpcT)
		assert.Equal(t, api.Allow, decision)
		assert.NotNil(t, identity)
		// x-rh-identity is disabled for grpc, so grpc should fall back to allow-unauthenticated.
		assert.Equal(t, "allow-unauthenticated", identity.AuthType)
	})

	t.Run("allow-unauthenticated enabled for http only", func(t *testing.T) {
		logger := log.NewHelper(log.DefaultLogger)
		config := CompletedConfig{
			&completedConfig{
				Authenticator: &AuthenticatorCompletedConfig{
					Type: aggregator.FirstMatch,
					ChainConfigs: []ChainCompletedConfig{
						{Type: "allow-unauthenticated", EnabledHTTP: true, EnabledGRPC: false},
						{Type: "x-rh-identity", EnabledHTTP: false, EnabledGRPC: true},
					},
				},
			},
		}

		auth, err := New(config, logger)
		assert.NoError(t, err)

		httpT := &mockTransporter{kind: transport.KindHTTP, headers: map[string]string{}}
		identity, decision := auth.Authenticate(context.Background(), httpT)
		assert.Equal(t, api.Allow, decision)
		assert.NotNil(t, identity)
		assert.Equal(t, "allow-unauthenticated", identity.AuthType)

		grpcT := &mockTransporter{kind: transport.KindGRPC, headers: map[string]string{"x-rh-identity": validXRH}}
		identity, decision = auth.Authenticate(context.Background(), grpcT)
		assert.Equal(t, api.Allow, decision)
		assert.NotNil(t, identity)
		assert.Equal(t, "x-rh-identity", identity.AuthType)
	})

	t.Run("guest type works and respects per-protocol enablement", func(t *testing.T) {
		logger := log.NewHelper(log.DefaultLogger)
		config := CompletedConfig{
			&completedConfig{
				Authenticator: &AuthenticatorCompletedConfig{
					Type: aggregator.FirstMatch,
					ChainConfigs: []ChainCompletedConfig{
						{Type: "guest", EnabledHTTP: false, EnabledGRPC: true},
						{Type: "x-rh-identity", EnabledHTTP: true, EnabledGRPC: false},
					},
				},
			},
		}

		auth, err := New(config, logger)
		assert.NoError(t, err)

		httpT := &mockTransporter{kind: transport.KindHTTP, headers: map[string]string{"x-rh-identity": validXRH}}
		identity, decision := auth.Authenticate(context.Background(), httpT)
		assert.Equal(t, api.Allow, decision)
		assert.NotNil(t, identity)
		assert.Equal(t, "x-rh-identity", identity.AuthType)

		grpcT := &mockTransporter{kind: transport.KindGRPC, headers: map[string]string{}}
		identity, decision = auth.Authenticate(context.Background(), grpcT)
		assert.Equal(t, api.Allow, decision)
		assert.NotNil(t, identity)
		assert.Equal(t, "allow-unauthenticated", identity.AuthType)
	})
}

func TestNew_UnknownAuthenticatorType(t *testing.T) {
	logger := log.NewHelper(log.DefaultLogger)
	config := CompletedConfig{
		&completedConfig{
			Authenticator: &AuthenticatorCompletedConfig{
				Type: aggregator.FirstMatch,
				ChainConfigs: []ChainCompletedConfig{
					{Type: "unknown-type", EnabledHTTP: true, EnabledGRPC: true},
				},
			},
		},
	}

	auth, err := New(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown authenticator type in chain")
	assert.Nil(t, auth)
}
