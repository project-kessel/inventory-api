package xrhidentity

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
)

// mockTransporter is a test helper that implements transport.Transporter
type mockTransporter struct {
	headers map[string]string
}

func (m *mockTransporter) Kind() transport.Kind            { return transport.KindHTTP }
func (m *mockTransporter) Endpoint() string                { return "/test" }
func (m *mockTransporter) Operation() string               { return "test" }
func (m *mockTransporter) RequestHeader() transport.Header { return &mockHeader{headers: m.headers} }
func (m *mockTransporter) ReplyHeader() transport.Header {
	return &mockHeader{headers: make(map[string]string)}
}

type mockHeader struct {
	headers map[string]string
}

func (m *mockHeader) Get(key string) string      { return m.headers[key] }
func (m *mockHeader) Set(key, value string)      { m.headers[key] = value }
func (m *mockHeader) Add(key, value string)      { m.headers[key] = value }
func (m *mockHeader) Keys() []string             { return nil }
func (m *mockHeader) Values(key string) []string { return nil }

func TestNew(t *testing.T) {
	auth := New()
	assert.NotNil(t, auth)
	assert.IsType(t, &XRhIdentityAuthenticator{}, auth)
}

func TestAuthenticate_MissingHeader(t *testing.T) {
	auth := New()
	transporter := &mockTransporter{
		headers: make(map[string]string),
	}

	identity, decision := auth.Authenticate(context.Background(), transporter)

	assert.Nil(t, identity)
	assert.Equal(t, api.Ignore, decision)
}

func TestAuthenticate_InvalidHeader(t *testing.T) {
	auth := New()
	transporter := &mockTransporter{
		headers: map[string]string{
			"x-rh-identity": "invalid-base64-json-data!!!",
		},
	}

	identity, decision := auth.Authenticate(context.Background(), transporter)

	assert.Nil(t, identity)
	assert.Equal(t, api.Deny, decision)
}

func TestAuthenticate_ValidHeader_WithUsername(t *testing.T) {
	auth := New()

	// Use a real valid x-rh-identity header format
	// This matches the format expected by DecodeAndCheckIdentity
	validHeader := "eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjEyMzQ1NiIsIm9yZ19pZCI6IjEyMzQ1NiIsInVzZXIiOnsidXNlcm5hbWUiOiJ0ZXN0dXNlciIsImVtYWlsIjoidGVzdHVzZXJAZXhhbXBsZS5jb20iLCJ1c2VyX2lkIjoidXNlci0xMjMifSwiaW50ZXJuYWwiOnt9LCJ0eXBlIjoiVXNlciJ9fQ=="

	transporter := &mockTransporter{
		headers: map[string]string{
			"x-rh-identity": validHeader,
		},
	}

	identity, decision := auth.Authenticate(context.Background(), transporter)

	require.NotNil(t, identity)
	assert.Equal(t, api.Allow, decision)
	assert.Equal(t, "x-rh-identity", identity.AuthType)
	assert.Equal(t, "testuser", identity.Principal)
	assert.Equal(t, "123456", identity.Tenant)
	assert.Equal(t, "User", identity.Type)
	assert.Equal(t, "user-123", identity.UserID)
	assert.False(t, identity.IsGuest)
}

func TestAuthenticate_ValidHeader_WithEmailOnly(t *testing.T) {
	auth := New()

	// Use a real valid x-rh-identity header with email but no username
	validHeader := "eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6Ijc4OTAxMiIsIm9yZ19pZCI6Ijc4OTAxMiIsInVzZXIiOnsiZW1haWwiOiJ1c2VyQGV4YW1wbGUuY29tIiwidXNlcl9pZCI6InVzZXItNDU2In0sImludGVybmFsIjp7fSwidHlwZSI6IlVzZXIifX0="

	transporter := &mockTransporter{
		headers: map[string]string{
			"x-rh-identity": validHeader,
		},
	}

	identity, decision := auth.Authenticate(context.Background(), transporter)

	require.NotNil(t, identity)
	assert.Equal(t, api.Allow, decision)
	assert.Equal(t, "user@example.com", identity.Principal) // Should use email when username is missing
	assert.Equal(t, "789012", identity.Tenant)
	assert.False(t, identity.IsGuest)
}

func TestAuthenticate_ValidHeader_WithServiceAccount(t *testing.T) {
	auth := New()

	// Use a real valid x-rh-identity header with service account
	validHeader := "eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjM0NTY3OCIsIm9yZ19pZCI6IjM0NTY3OCIsInNlcnZpY2VfYWNjb3VudCI6eyJjbGllbnRfaWQiOiJzZXJ2aWNlLWFjY291bnQtMTIzIiwidXNlcm5hbWUiOiJzZXJ2aWNlLWFjY291bnQifSwiaW50ZXJuYWwiOnt9LCJ0eXBlIjoiU2VydmljZUFjY291bnQifX0="

	transporter := &mockTransporter{
		headers: map[string]string{
			"x-rh-identity": validHeader,
		},
	}

	identity, decision := auth.Authenticate(context.Background(), transporter)

	require.NotNil(t, identity)
	assert.Equal(t, api.Allow, decision)
	assert.Equal(t, "x-rh-identity", identity.AuthType)
	assert.Equal(t, "345678", identity.Tenant)
	assert.Equal(t, "ServiceAccount", identity.Type)
	assert.False(t, identity.IsGuest) // Service accounts are not guests
}

func TestAuthenticate_ValidHeader_NoUserOrServiceAccount(t *testing.T) {
	auth := New()

	// Use a real valid x-rh-identity header without user or service account
	validHeader := "eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6Ijk5OTk5OSIsIm9yZ19pZCI6Ijk5OTk5OSIsImludGVybmFsIjp7fSwidHlwZSI6IlN5c3RlbSJ9fQ=="

	transporter := &mockTransporter{
		headers: map[string]string{
			"x-rh-identity": validHeader,
		},
	}

	identity, decision := auth.Authenticate(context.Background(), transporter)

	require.NotNil(t, identity)
	assert.Equal(t, api.Allow, decision)
	assert.Equal(t, "x-rh-identity", identity.AuthType)
	assert.Equal(t, "999999", identity.Tenant)
	assert.Equal(t, "System", identity.Type)
	assert.True(t, identity.IsGuest)    // Missing user info is treated as guest
	assert.Empty(t, identity.Principal) // No principal when no user/service account
}

func TestAuthenticate_ValidHeader_WithAuthType(t *testing.T) {
	auth := New()

	// Use a real valid x-rh-identity header with custom AuthType
	validHeader := "eyJpZGVudGl0eSI6eyJhY2NvdW50X251bWJlciI6IjExMTIyMiIsIm9yZ19pZCI6IjExMTIyMiIsInVzZXIiOnsidXNlcm5hbWUiOiJjZXJ0dXNlciJ9LCJpbnRlcm5hbCI6e30sInR5cGUiOiJVc2VyIiwiYXV0aF90eXBlIjoiY2VydC1hdXRoIn19"

	transporter := &mockTransporter{
		headers: map[string]string{
			"x-rh-identity": validHeader,
		},
	}

	identity, decision := auth.Authenticate(context.Background(), transporter)

	require.NotNil(t, identity)
	assert.Equal(t, api.Allow, decision)
	assert.Equal(t, "cert-auth", identity.AuthType) // Should use platform identity's AuthType
	assert.Equal(t, "certuser", identity.Principal)
}

func TestConvertPlatformIdentity_UsernamePreference(t *testing.T) {
	platformIdentity := &identity.Identity{
		User: &identity.User{
			Username: "username",
			Email:    "email@example.com",
		},
	}

	result := convertPlatformIdentity(platformIdentity)
	assert.Equal(t, "username", result.Principal) // Username should be preferred over email
}

func TestConvertPlatformIdentity_EmailFallback(t *testing.T) {
	platformIdentity := &identity.Identity{
		User: &identity.User{
			Email: "email@example.com",
			// No username
		},
	}

	result := convertPlatformIdentity(platformIdentity)
	assert.Equal(t, "email@example.com", result.Principal) // Should use email when username is missing
}

func TestConvertPlatformIdentity_NoUserInfo(t *testing.T) {
	platformIdentity := &identity.Identity{
		AccountNumber: "123456",
		// No User or ServiceAccount
	}

	result := convertPlatformIdentity(platformIdentity)
	assert.Empty(t, result.Principal)
	assert.Equal(t, "123456", result.Tenant)
	assert.True(t, result.IsGuest) // Should be guest when no user info
}

func TestConvertPlatformIdentity_NilUser(t *testing.T) {
	platformIdentity := &identity.Identity{
		AccountNumber: "123456",
		User:          nil,
	}

	result := convertPlatformIdentity(platformIdentity)
	assert.Empty(t, result.Principal)
	assert.True(t, result.IsGuest)
}

func TestConvertPlatformIdentity_EmptyAccountNumber(t *testing.T) {
	platformIdentity := &identity.Identity{
		User: &identity.User{
			Username: "testuser",
		},
		// No AccountNumber
	}

	result := convertPlatformIdentity(platformIdentity)
	assert.Empty(t, result.Tenant)
	assert.Equal(t, "testuser", result.Principal)
}

func TestConvertPlatformIdentity_DefaultAuthType(t *testing.T) {
	platformIdentity := &identity.Identity{
		User: &identity.User{
			Username: "testuser",
		},
		// No AuthType specified
	}

	result := convertPlatformIdentity(platformIdentity)
	assert.Equal(t, "x-rh-identity", result.AuthType) // Should default to "x-rh-identity"
}
