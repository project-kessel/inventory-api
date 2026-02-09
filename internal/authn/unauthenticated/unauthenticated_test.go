package unauthenticated

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

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

func TestAuthenticate_ReturnsUnauthenticatedClaims(t *testing.T) {
	auth := New()
	claims, decision := auth.Authenticate(context.Background(), &mockTransporter{})

	if assert.NotNil(t, claims) {
		assert.Equal(t, api.AuthTypeAllowUnauthenticated, claims.AuthType)
		assert.False(t, claims.IsAuthenticated())
	}
	assert.Equal(t, api.Allow, decision)
}
