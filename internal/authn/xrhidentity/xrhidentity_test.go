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

	claims, decision := auth.Authenticate(context.Background(), transporter)

	assert.Nil(t, claims)
	assert.Equal(t, api.Ignore, decision)
}

func TestAuthenticate_InvalidHeader(t *testing.T) {
	auth := New()
	transporter := &mockTransporter{
		headers: map[string]string{
			"x-rh-identity": "invalid-base64-json-data!!!",
		},
	}

	claims, decision := auth.Authenticate(context.Background(), transporter)

	assert.Nil(t, claims)
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

	claims, decision := auth.Authenticate(context.Background(), transporter)

	require.NotNil(t, claims)
	assert.Equal(t, api.Allow, decision)
	assert.Equal(t, api.AuthTypeXRhIdentity, claims.AuthType)
	assert.Equal(t, api.SubjectId("user-123"), claims.SubjectId)
	assert.Equal(t, api.OrganizationId("123456"), claims.OrganizationId)
	assert.Empty(t, claims.Issuer)
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

	claims, decision := auth.Authenticate(context.Background(), transporter)

	require.NotNil(t, claims)
	assert.Equal(t, api.Allow, decision)
	assert.Equal(t, api.SubjectId("user-456"), claims.SubjectId) // UserID preferred when present
	assert.Equal(t, api.OrganizationId("789012"), claims.OrganizationId)
	assert.Empty(t, claims.Issuer)
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

	claims, decision := auth.Authenticate(context.Background(), transporter)

	require.NotNil(t, claims)
	assert.Equal(t, api.Allow, decision)
	assert.Equal(t, api.AuthTypeXRhIdentity, claims.AuthType)
	assert.Empty(t, claims.SubjectId)
	assert.Empty(t, claims.OrganizationId)
	assert.Empty(t, claims.Issuer)
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

	claims, decision := auth.Authenticate(context.Background(), transporter)

	require.NotNil(t, claims)
	assert.Equal(t, api.Allow, decision)
	assert.Equal(t, api.AuthTypeXRhIdentity, claims.AuthType)
	assert.Empty(t, claims.OrganizationId)
	assert.Empty(t, claims.SubjectId) // No subject when no user/service account
	assert.Empty(t, claims.Issuer)
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

	claims, decision := auth.Authenticate(context.Background(), transporter)

	require.NotNil(t, claims)
	assert.Equal(t, api.Allow, decision)
	assert.Equal(t, api.AuthTypeXRhIdentity, claims.AuthType)
	assert.Empty(t, claims.SubjectId)
}

func TestConvertPlatformClaims_UserIDPreferred(t *testing.T) {
	platformIdentity := &identity.Identity{
		Type: "User",
		User: &identity.User{
			UserID:   "user-123",
			Username: "username",
			Email:    "email@example.com",
		},
	}

	result := convertPlatformClaims(platformIdentity)
	assert.Equal(t, api.SubjectId("user-123"), result.SubjectId)
}

func TestConvertPlatformClaims_NoUserID(t *testing.T) {
	platformIdentity := &identity.Identity{
		Type: "User",
		User: &identity.User{
			Email: "email@example.com",
			// No username
		},
	}

	result := convertPlatformClaims(platformIdentity)
	assert.Empty(t, result.SubjectId)
}

func TestConvertPlatformClaims_NoUserInfo(t *testing.T) {
	platformIdentity := &identity.Identity{
		Type:          "User",
		AccountNumber: "123456",
		// No User or ServiceAccount
	}

	result := convertPlatformClaims(platformIdentity)
	assert.Empty(t, result.SubjectId)
	assert.Empty(t, result.OrganizationId)
}

func TestConvertPlatformClaims_NilUser(t *testing.T) {
	platformIdentity := &identity.Identity{
		Type:          "User",
		AccountNumber: "123456",
		User:          nil,
	}

	result := convertPlatformClaims(platformIdentity)
	assert.Empty(t, result.SubjectId)
}

func TestConvertPlatformClaims_EmptyAccountNumber(t *testing.T) {
	platformIdentity := &identity.Identity{
		Type: "User",
		User: &identity.User{
			Username: "testuser",
		},
		// No AccountNumber
	}

	result := convertPlatformClaims(platformIdentity)
	assert.Empty(t, result.OrganizationId)
	assert.Empty(t, result.SubjectId)
}

func TestConvertPlatformClaims_DefaultAuthType(t *testing.T) {
	platformIdentity := &identity.Identity{
		Type: "User",
		User: &identity.User{
			Username: "testuser",
		},
		// No AuthType specified
	}

	result := convertPlatformClaims(platformIdentity)
	assert.Equal(t, api.AuthTypeXRhIdentity, result.AuthType) // Should default to "x-rh-identity"
}
