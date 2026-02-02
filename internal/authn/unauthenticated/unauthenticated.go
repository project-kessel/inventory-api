package unauthenticated

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

// UnauthenticatedAuthenticator provides unauthenticated access using the User-Agent header as the principal.
type UnauthenticatedAuthenticator struct{}

// New creates a new UnauthenticatedAuthenticator instance.
func New() *UnauthenticatedAuthenticator {
	return &UnauthenticatedAuthenticator{}
}

// Authenticate creates unauthenticated claims using the User-Agent header and always allows the request.
func (a *UnauthenticatedAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Claims, api.Decision) {
	// No claims for unauthenticated requests
	claims := api.UnauthenticatedClaims()
	return claims, api.Allow
}
