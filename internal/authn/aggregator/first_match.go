package aggregator

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"

	"github.com/project-kessel/inventory-api/internal/authn/api"
)

// FirstMatchAuthenticator implements a "first match" aggregation strategy.
// It allows the request if any authenticator returns Allow.
// It only denies if all authenticators return Deny (not Ignore).
type FirstMatchAuthenticator struct {
	Authenticators []api.Authenticator
}

// NewFirstMatch creates a new FirstMatchAuthenticator with an empty chain.
func NewFirstMatch() *FirstMatchAuthenticator {
	return &FirstMatchAuthenticator{
		Authenticators: []api.Authenticator{},
	}
}

// Add appends an authenticator to the chain.
func (f *FirstMatchAuthenticator) Add(a api.Authenticator) {
	f.Authenticators = append(f.Authenticators, a)
}

// Authenticate checks all authenticators in the chain.
// Returns Allow immediately if any authenticator returns Allow.
// Returns Deny only if all authenticators return Deny (not Ignore).
// Returns Ignore if all authenticators return Ignore (none can handle the request).
func (f *FirstMatchAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Identity, api.Decision) {
	denyCount := 0
	ignoreCount := 0

	for _, a := range f.Authenticators {
		identity, decision := a.Authenticate(ctx, t)

		// Handle decision using switch statement
		switch decision {
		case api.Allow:
			// If any authenticator allows, return Allow immediately
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

	// Only deny if all authenticators returned Deny
	if denyCount == len(f.Authenticators) {
		return nil, api.Deny
	}

	// If all authenticators returned Ignore, propagate Ignore
	if ignoreCount == len(f.Authenticators) {
		return nil, api.Ignore
	}

	// Mix of Deny and Ignore: if any Deny, deny (stricter policy)
	// This means: if at least one explicitly denies, deny the request
	if denyCount > 0 {
		return nil, api.Deny
	}

	// Should not reach here, but default to deny for safety
	return nil, api.Deny
}
