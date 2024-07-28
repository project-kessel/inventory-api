package delegator

import (
	"net/http"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

type DelegatingAuthenticator struct {
	Authenticators []api.Authenticator
}

func New() *DelegatingAuthenticator {
	return &DelegatingAuthenticator{}
}

func (d *DelegatingAuthenticator) Add(a api.Authenticator) {
	d.Authenticators = append(d.Authenticators, a)
}

func (d *DelegatingAuthenticator) Authenticate(r *http.Request) (*api.Identity, api.Decision) {
	for _, a := range d.Authenticators {
		identity, decision := a.Authenticate(r)
		if decision == api.Ignore {
			continue
		}
		return identity, decision
	}
	return nil, api.Deny
}
