package guest

import (
	"net/http"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

type GuestAuthenticator struct{}

func New() *GuestAuthenticator {
	return &GuestAuthenticator{}
}

func (a *GuestAuthenticator) Authenticate(r *http.Request) (*api.Identity, api.Decision) {

	// TODO: should we use something else? ip address?
	ua := r.Header.Get("User-Agent")
	identity := &api.Identity{
		Principal: ua,
		IsGuest:   true,
	}

	return identity, api.Allow
}
