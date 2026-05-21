package oidc

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"

	"github.com/project-kessel/inventory-api/internal/authn/api"
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
	kind      transport.Kind
	operation string
	headers   map[string]string
}

func (m *mockTransporter) Kind() transport.Kind            { return m.kind }
func (m *mockTransporter) Endpoint() string                { return "/test" }
func (m *mockTransporter) Operation() string               { return m.operation }
func (m *mockTransporter) RequestHeader() transport.Header { return &mockHeader{headers: m.headers} }
func (m *mockTransporter) ReplyHeader() transport.Header {
	return &mockHeader{headers: map[string]string{}}
}

// TestOAuth2Authenticator_Endpoints_NotMatched verifies that when endpoints are specified,
// the authenticator returns Ignore for requests to non-matching endpoints.
func TestOAuth2Authenticator_Endpoints_NotMatched(t *testing.T) {
	// Create authenticator with endpoints list
	auth := &OAuth2Authenticator{
		CompletedConfig: CompletedConfig{
			&completedConfig{
				&Config{
					Options: &Options{
						GrpcEndpoints: []string{
							"/kessel.inventory.v1beta2.KesselTupleService/CreateTuples",
							"/kessel.inventory.v1beta2.KesselTupleService/DeleteTuples",
						},
					},
				},
			},
		},
	}

	// Mock transporter with non-matching endpoint
	mockT := &mockTransporter{
		kind:      transport.KindGRPC,
		operation: "/kessel.inventory.v1beta2.KesselInventoryService/CreateResource",
		headers: map[string]string{
			"authorization": "Bearer valid-token-here",
		},
	}

	// Authenticate should return Ignore (endpoint not in list)
	claims, decision := auth.Authenticate(context.Background(), mockT)
	assert.Nil(t, claims)
	assert.Equal(t, api.Ignore, decision)
}

// TestOAuth2Authenticator_Endpoints_MatchedNoToken verifies that when endpoints are specified
// and the endpoint matches, but no token is present, the authenticator returns Deny (not Ignore).
func TestOAuth2Authenticator_Endpoints_MatchedNoToken(t *testing.T) {
	// Create authenticator with endpoints list
	auth := &OAuth2Authenticator{
		CompletedConfig: CompletedConfig{
			&completedConfig{
				&Config{
					Options: &Options{
						GrpcEndpoints: []string{
							"/kessel.inventory.v1beta2.KesselTupleService/CreateTuples",
							"/kessel.inventory.v1beta2.KesselTupleService/DeleteTuples",
						},
					},
				},
			},
		},
	}

	// Mock transporter with matching endpoint but no token
	mockT := &mockTransporter{
		kind:      transport.KindGRPC,
		operation: "/kessel.inventory.v1beta2.KesselTupleService/CreateTuples",
		headers:   map[string]string{}, // No authorization header
	}

	// Authenticate should return Deny (OIDC required for this endpoint)
	claims, decision := auth.Authenticate(context.Background(), mockT)
	assert.Nil(t, claims)
	assert.Equal(t, api.Deny, decision)
}

// TestOAuth2Authenticator_NoEndpoints_NoToken verifies that when no endpoints are specified
// (current behavior), missing token returns Ignore (OIDC optional).
func TestOAuth2Authenticator_NoEndpoints_NoToken(t *testing.T) {
	// Create authenticator without endpoints list (current behavior)
	auth := &OAuth2Authenticator{
		CompletedConfig: CompletedConfig{
			&completedConfig{
				&Config{
					Options: &Options{
						GrpcEndpoints: []string{}, // Empty list = no endpoint filtering
					},
				},
			},
		},
	}

	// Mock transporter without token
	mockT := &mockTransporter{
		kind:      transport.KindGRPC,
		operation: "/any/endpoint",
		headers:   map[string]string{},
	}

	// Authenticate should return Ignore (OIDC optional, current behavior)
	claims, decision := auth.Authenticate(context.Background(), mockT)
	assert.Nil(t, claims)
	assert.Equal(t, api.Ignore, decision)
}

// Note: Testing invalid token validation requires a real OIDC provider with a Verifier.
// In unit tests without a mock OIDC provider, we can only test the endpoint filtering logic.
// Token validation is tested in integration tests with a real Keycloak instance.

// TestOAuth2Authenticator_Endpoints_MultipleEndpoints verifies that multiple endpoints
// can be specified and any match triggers OIDC requirement.
func TestOAuth2Authenticator_Endpoints_MultipleEndpoints(t *testing.T) {
	endpoints := []string{
		"/kessel.inventory.v1beta2.KesselTupleService/CreateTuples",
		"/kessel.inventory.v1beta2.KesselTupleService/DeleteTuples",
		"/kessel.inventory.v1beta2.KesselTupleService/ReadTuples",
		"/kessel.inventory.v1beta2.KesselTupleService/AcquireLock",
	}

	auth := &OAuth2Authenticator{
		CompletedConfig: CompletedConfig{
			&completedConfig{
				&Config{
					Options: &Options{
						GrpcEndpoints: endpoints,
					},
				},
			},
		},
	}

	// Test each endpoint matches and requires token
	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			mockT := &mockTransporter{
				kind:      transport.KindGRPC,
				operation: endpoint,
				headers:   map[string]string{},
			}

			claims, decision := auth.Authenticate(context.Background(), mockT)
			assert.Nil(t, claims)
			assert.Equal(t, api.Deny, decision, "Expected Deny for endpoint %s without token", endpoint)
		})
	}

	// Test non-matching endpoint returns Ignore
	mockT := &mockTransporter{
		kind:      transport.KindGRPC,
		operation: "/some/other/endpoint",
		headers:   map[string]string{},
	}

	claims, decision := auth.Authenticate(context.Background(), mockT)
	assert.Nil(t, claims)
	assert.Equal(t, api.Ignore, decision)
}
