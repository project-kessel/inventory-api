package guest

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

type GuestAuthenticator struct{}

func New() *GuestAuthenticator {
	return &GuestAuthenticator{}
}

func (a *GuestAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Identity, api.Decision) {

	// TODO: should we use something else? ip address?
	ua := t.RequestHeader().Get("User-Agent")
	identity := &api.Identity{
		Principal: ua,
		IsGuest:   true,
	}

	return identity, api.Allow
}
