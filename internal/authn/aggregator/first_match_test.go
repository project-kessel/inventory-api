package aggregator

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

// mockAuthenticator is a test helper that returns predefined decisions
type mockAuthenticator struct {
	identity *api.Identity
	decision api.Decision
}

func (m *mockAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Identity, api.Decision) {
	return m.identity, m.decision
}

// mockTransporter is a test helper that implements transport.Transporter
type mockTransporter struct{}

func (m *mockTransporter) Kind() transport.Kind            { return transport.KindHTTP }
func (m *mockTransporter) Endpoint() string                { return "/test" }
func (m *mockTransporter) Operation() string               { return "test" }
func (m *mockTransporter) RequestHeader() transport.Header { return &mockHeader{} }
func (m *mockTransporter) ReplyHeader() transport.Header   { return &mockHeader{} }

type mockHeader struct{}

func (m *mockHeader) Get(key string) string      { return "" }
func (m *mockHeader) Set(key, value string)      {}
func (m *mockHeader) Add(key, value string)      {}
func (m *mockHeader) Keys() []string             { return nil }
func (m *mockHeader) Values(key string) []string { return nil }

func TestNewFirstMatch(t *testing.T) {
	auth := NewFirstMatch()
	assert.NotNil(t, auth)
	assert.Empty(t, auth.Authenticators)
}

func TestFirstMatchAuthenticator_Add(t *testing.T) {
	auth := NewFirstMatch()
	mock1 := &mockAuthenticator{
		identity: &api.Identity{Principal: "user1", AuthType: "test1"},
		decision: api.Allow,
	}
	mock2 := &mockAuthenticator{
		identity: &api.Identity{Principal: "user2", AuthType: "test2"},
		decision: api.Deny,
	}

	auth.Add(mock1)
	assert.Len(t, auth.Authenticators, 1)

	auth.Add(mock2)
	assert.Len(t, auth.Authenticators, 2)
}

func TestFirstMatchAuthenticator_Authenticate_Allow(t *testing.T) {
	tests := []struct {
		name           string
		authenticators []*mockAuthenticator
		wantIdentity   *api.Identity
		wantDecision   api.Decision
	}{
		{
			name: "first authenticator allows",
			authenticators: []*mockAuthenticator{
				{identity: &api.Identity{Principal: "user1", AuthType: "test1"}, decision: api.Allow},
				{identity: &api.Identity{Principal: "user2", AuthType: "test2"}, decision: api.Deny},
			},
			wantIdentity: &api.Identity{Principal: "user1", AuthType: "test1"},
			wantDecision: api.Allow,
		},
		{
			name: "second authenticator allows",
			authenticators: []*mockAuthenticator{
				{identity: nil, decision: api.Ignore},
				{identity: &api.Identity{Principal: "user2", AuthType: "test2"}, decision: api.Allow},
			},
			wantIdentity: &api.Identity{Principal: "user2", AuthType: "test2"},
			wantDecision: api.Allow,
		},
		{
			name: "middle authenticator allows",
			authenticators: []*mockAuthenticator{
				{identity: nil, decision: api.Ignore},
				{identity: &api.Identity{Principal: "user2", AuthType: "test2"}, decision: api.Allow},
				{identity: nil, decision: api.Deny},
			},
			wantIdentity: &api.Identity{Principal: "user2", AuthType: "test2"},
			wantDecision: api.Allow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewFirstMatch()
			for _, a := range tt.authenticators {
				auth.Add(a)
			}

			identity, decision := auth.Authenticate(context.Background(), &mockTransporter{})
			assert.Equal(t, tt.wantDecision, decision)
			if tt.wantIdentity != nil {
				assert.NotNil(t, identity)
				assert.Equal(t, tt.wantIdentity.Principal, identity.Principal)
				assert.Equal(t, tt.wantIdentity.AuthType, identity.AuthType)
			}
		})
	}
}

func TestFirstMatchAuthenticator_Authenticate_Deny(t *testing.T) {
	tests := []struct {
		name           string
		authenticators []*mockAuthenticator
		wantDecision   api.Decision
	}{
		{
			name: "all deny",
			authenticators: []*mockAuthenticator{
				{identity: nil, decision: api.Deny},
				{identity: nil, decision: api.Deny},
			},
			wantDecision: api.Deny,
		},
		{
			name: "all ignore - returns Ignore",
			authenticators: []*mockAuthenticator{
				{identity: nil, decision: api.Ignore},
				{identity: nil, decision: api.Ignore},
			},
			wantDecision: api.Ignore,
		},
		{
			name: "mix of deny and ignore - denies if any deny",
			authenticators: []*mockAuthenticator{
				{identity: nil, decision: api.Deny},
				{identity: nil, decision: api.Ignore},
			},
			wantDecision: api.Deny,
		},
		{
			name:           "empty chain - denies by default",
			authenticators: []*mockAuthenticator{},
			wantDecision:   api.Deny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewFirstMatch()
			for _, a := range tt.authenticators {
				auth.Add(a)
			}

			identity, decision := auth.Authenticate(context.Background(), &mockTransporter{})
			assert.Equal(t, tt.wantDecision, decision)
			assert.Nil(t, identity)
		})
	}
}

func TestFirstMatchAuthenticator_Authenticate_ReturnsImmediatelyOnAllow(t *testing.T) {
	// Test that we return immediately on Allow and don't check remaining authenticators
	callCount := 0
	mock1 := &mockAuthenticator{
		identity: &api.Identity{Principal: "user1", AuthType: "test1"},
		decision: api.Allow,
	}
	mock2 := &mockAuthenticator{
		identity: nil,
		decision: api.Deny,
	}

	// Create a custom authenticator that tracks calls
	trackedMock2 := &trackingAuthenticator{
		authenticator: mock2,
		callCount:     &callCount,
	}

	auth := NewFirstMatch()
	auth.Add(mock1)
	auth.Add(trackedMock2)

	identity, decision := auth.Authenticate(context.Background(), &mockTransporter{})
	assert.Equal(t, api.Allow, decision)
	assert.NotNil(t, identity)
	assert.Equal(t, "user1", identity.Principal)
	// mock2 should not have been called because mock1 returned Allow
	assert.Equal(t, 0, callCount)
}

type trackingAuthenticator struct {
	authenticator *mockAuthenticator
	callCount     *int
}

func (t *trackingAuthenticator) Authenticate(ctx context.Context, transporter transport.Transporter) (*api.Identity, api.Decision) {
	*t.callCount++
	return t.authenticator.Authenticate(ctx, transporter)
}
