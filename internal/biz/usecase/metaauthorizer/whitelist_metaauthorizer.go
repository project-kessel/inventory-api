package metaauthorizer

import (
	"context"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
)

// WhitelistMetaAuthorizer implements a whitelist-based authorization check.
// It matches against ClientID from OIDC claims.
// Designed for restricting deprecated tuple CRUD endpoints to specific services.
// Only allows gRPC connections with OIDC authentication and valid ClientID.
type WhitelistMetaAuthorizer struct {
	allowlist []string
}

// NewWhitelistMetaAuthorizer creates a new whitelist-based meta authorizer.
// allowlist: list of ClientIDs permitted to access the operation
// Empty allowlist denies all requests (fail-closed).
// "*" wildcard permits all requests (for testing/development only).
func NewWhitelistMetaAuthorizer(allowlist []string) *WhitelistMetaAuthorizer {
	return &WhitelistMetaAuthorizer{
		allowlist: allowlist,
	}
}

func (w *WhitelistMetaAuthorizer) Check(_ context.Context, _ MetaObject, _ Relation, authzCtx authnapi.AuthzContext) (bool, error) {
	// Deny if not authenticated
	if !authzCtx.IsAuthenticated() {
		return false, nil
	}

	// Deny if not gRPC (tuple CRUD endpoints are gRPC-only)
	if authzCtx.Protocol != authnapi.ProtocolGRPC {
		return false, nil
	}

	// Deny if not OIDC (ClientID only populated for OIDC)
	if authzCtx.Subject.AuthType != authnapi.AuthTypeOIDC {
		return false, nil
	}

	// Check whitelist
	return isInAllowlist(authzCtx.Subject, w.allowlist), nil
}

// isInAllowlist checks if the caller's ClientID is in the allowlist.
// Requires non-empty ClientID from OIDC claims.
// Supports "*" wildcard to allow all.
func isInAllowlist(claims *authnapi.Claims, allowlist []string) bool {
	// Deny if ClientID is empty (OIDC service-to-service auth requires client_id)
	if string(claims.ClientID) == "" {
		return false
	}

	for _, allowed := range allowlist {
		if allowed == "*" {
			return true
		}
		// Match on ClientID (stable service identifier from OIDC client_id claim)
		if allowed == string(claims.ClientID) {
			return true
		}
	}
	return false
}
