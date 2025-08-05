package guest

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

// GuestAuthenticator provides guest authentication using the User-Agent header as the principal.
type GuestAuthenticator struct{}

// New creates a new GuestAuthenticator instance.
func New() *GuestAuthenticator {
	return &GuestAuthenticator{}
}

// Authenticate creates a guest identity using the User-Agent header and always allows the request.
func (a *GuestAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Identity, api.Decision) {

	// TODO: should we use something else? ip address?
	ua := t.RequestHeader().Get("User-Agent")
	identity := &api.Identity{
		Principal: ua,
		IsGuest:   true,
	}

	return identity, api.Allow
}
