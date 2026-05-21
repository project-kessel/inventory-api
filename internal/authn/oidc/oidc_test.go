package oidc

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

type fakeHeader struct {
	headers map[string]string
}

func (m *fakeHeader) Get(key string) string {
	if m.headers == nil {
		return ""
	}
	return m.headers[key]
}
func (m *fakeHeader) Set(key, value string)      {}
func (m *fakeHeader) Add(key, value string)      {}
func (m *fakeHeader) Keys() []string             { return nil }
func (m *fakeHeader) Values(key string) []string { return nil }

type fakeTransporter struct {
	kind      transport.Kind
	operation string
	headers   map[string]string
}

func (m *fakeTransporter) Kind() transport.Kind            { return m.kind }
func (m *fakeTransporter) Endpoint() string                { return "/test" }
func (m *fakeTransporter) Operation() string               { return m.operation }
func (m *fakeTransporter) RequestHeader() transport.Header { return &fakeHeader{headers: m.headers} }
func (m *fakeTransporter) ReplyHeader() transport.Header {
	return &fakeHeader{headers: map[string]string{}}
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

	// Fake transporter with non-matching endpoint
	fakeT := &fakeTransporter{
		kind:      transport.KindGRPC,
		operation: "/kessel.inventory.v1beta2.KesselInventoryService/CreateResource",
		headers: map[string]string{
			"authorization": "Bearer valid-token-here",
		},
	}

	// Authenticate should return Ignore (endpoint not in list)
	claims, decision := auth.Authenticate(context.Background(), fakeT)
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

	// Fake transporter with matching endpoint but no token
	fakeT := &fakeTransporter{
		kind:      transport.KindGRPC,
		operation: "/kessel.inventory.v1beta2.KesselTupleService/CreateTuples",
		headers:   map[string]string{}, // No authorization header
	}

	// Authenticate should return Deny (OIDC required for this endpoint)
	claims, decision := auth.Authenticate(context.Background(), fakeT)
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

	// Fake transporter without token
	fakeT := &fakeTransporter{
		kind:      transport.KindGRPC,
		operation: "/any/endpoint",
		headers:   map[string]string{},
	}

	// Authenticate should return Ignore (OIDC optional, current behavior)
	claims, decision := auth.Authenticate(context.Background(), fakeT)
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
			fakeT := &fakeTransporter{
				kind:      transport.KindGRPC,
				operation: endpoint,
				headers:   map[string]string{},
			}

			claims, decision := auth.Authenticate(context.Background(), fakeT)
			assert.Nil(t, claims)
			assert.Equal(t, api.Deny, decision, "Expected Deny for endpoint %s without token", endpoint)
		})
	}

	// Test non-matching endpoint returns Ignore
	fakeT := &fakeTransporter{
		kind:      transport.KindGRPC,
		operation: "/some/other/endpoint",
		headers:   map[string]string{},
	}

	claims, decision := auth.Authenticate(context.Background(), fakeT)
	assert.Nil(t, claims)
	assert.Equal(t, api.Ignore, decision)
}
