package aggregator

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

// FirstMatchAuthenticator implements a "first match" aggregation strategy.
// It allows the request if any authenticator returns Allow.
// It only denies if all authenticators return Deny (not Ignore).
type FirstMatchAuthenticator struct {
	Authenticators []api.Authenticator
	logger         *log.Helper
}

// NewFirstMatch creates a new FirstMatchAuthenticator with an empty chain.
func NewFirstMatch() *FirstMatchAuthenticator {
	return &FirstMatchAuthenticator{
		Authenticators: []api.Authenticator{},
		logger:         nil, // Logger is optional, set via SetLogger if needed
	}
}

// SetLogger sets the logger for this authenticator (optional, for debugging)
func (f *FirstMatchAuthenticator) SetLogger(logger *log.Helper) {
	f.logger = logger
}

// Add appends an authenticator to the chain.
func (f *FirstMatchAuthenticator) Add(a api.Authenticator) {
	f.Authenticators = append(f.Authenticators, a)
}

// Authenticate checks all authenticators in the chain.
// Returns Allow immediately if any authenticator returns Allow.
// Returns Deny if all authenticators return Deny, or if there's a mix of Deny and Ignore (stricter policy).
// Returns Ignore only if all authenticators return Ignore (none can handle the request).
func (f *FirstMatchAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Identity, api.Decision) {
	denyCount := 0
	ignoreCount := 0

	for i, a := range f.Authenticators {
		identity, decision := a.Authenticate(ctx, t)

		// Handle decision using switch statement
		switch decision {
		case api.Allow:
			// If any authenticator allows, return Allow immediately
			// Log which authenticator allowed (at debug level for troubleshooting)
			if f.logger != nil && identity != nil {
				f.logger.Debugf("Authentication allowed by authenticator at chain index %d (authType: %s, principal: %s)",
					i, identity.AuthType, identity.Principal)
			}
			return identity, api.Allow
		case api.Deny:
			denyCount++
		case api.Ignore:
			ignoreCount++
		}
	}

	// If we have no authenticators, deny by default
	if len(f.Authenticators) == 0 {
		return nil, api.Deny
	}

	// If all authenticators returned Deny, propagate Deny
	if denyCount == len(f.Authenticators) {
		return nil, api.Deny
	}

	// If all authenticators returned Ignore, propagate Ignore
	if ignoreCount == len(f.Authenticators) {
		return nil, api.Ignore
	}

	return nil, api.Deny
}
