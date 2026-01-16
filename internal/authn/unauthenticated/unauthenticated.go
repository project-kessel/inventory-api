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

// Authenticate creates an unauthenticated identity using the User-Agent header and always allows the request.
func (a *UnauthenticatedAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Identity, api.Decision) {
	// TODO: should we use something else? ip address?
	ua := t.RequestHeader().Get("User-Agent")
	identity := &api.Identity{
		Principal: ua,
		IsGuest:   true,
		AuthType:  "allow-unauthenticated",
	}

	return identity, api.Allow
}
