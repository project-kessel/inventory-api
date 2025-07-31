package delegator

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

// DelegatingAuthenticator implements a chain of responsibility pattern for authentication.
// It delegates authentication requests to a list of authenticators until one makes a decision.
type DelegatingAuthenticator struct {
	Authenticators []api.Authenticator
}

// New creates a new DelegatingAuthenticator with an empty list of authenticators.
func New() *DelegatingAuthenticator {
	return &DelegatingAuthenticator{}
}

// Add appends an authenticator to the delegation chain.
func (d *DelegatingAuthenticator) Add(a api.Authenticator) {
	d.Authenticators = append(d.Authenticators, a)
}

// Authenticate iterates through the authenticator chain until one makes a decision.
// Returns the first non-Ignore decision, or Deny if all authenticators ignore the request.
func (d *DelegatingAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Identity, api.Decision) {
	for _, a := range d.Authenticators {
		identity, decision := a.Authenticate(ctx, t)
		if decision == api.Ignore {
			continue
		}
		return identity, decision
	}
	return nil, api.Deny
}
