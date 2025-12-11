package xrhidentity

import (
	"context"

	"github.com/go-kratos/kratos/v2/transport"

	"github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
)

// XRhIdentityAuthenticator provides authentication using the x-rh-identity header
// from Red Hat ConsoleDot/Cloud Platform.
type XRhIdentityAuthenticator struct{}

// New creates a new XRhIdentityAuthenticator instance.
func New() *XRhIdentityAuthenticator {
	return &XRhIdentityAuthenticator{}
}

// Authenticate parses the x-rh-identity header and returns an identity if valid.
// Returns Ignore if the header is missing (allowing other authenticators to try).
// Returns Deny if the header is present but invalid.
func (a *XRhIdentityAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Identity, api.Decision) {
	// Get the x-rh-identity header
	identityHeader := t.RequestHeader().Get("x-rh-identity")
	if identityHeader == "" {
		// Header missing - ignore and let other authenticators try
		return nil, api.Ignore
	}

	// Parse the identity header using platform-go-middlewares
	xrhid, err := identity.DecodeAndCheckIdentity(identityHeader)
	if err != nil {
		// Header present but invalid - deny
		return nil, api.Deny
	}

	// Convert platform identity to our internal Identity format
	internalIdentity := convertPlatformIdentity(&xrhid.Identity)
	return internalIdentity, api.Allow
}

// convertPlatformIdentity converts a platform-go-middlewares Identity to our internal Identity format
func convertPlatformIdentity(platformIdentity *identity.Identity) *api.Identity {
	internalIdentity := &api.Identity{
		AuthType: "x-rh-identity",
	}

	// Set principal from user information
	if platformIdentity.User != nil {
		if platformIdentity.User.Username != "" {
			internalIdentity.Principal = platformIdentity.User.Username
		} else if platformIdentity.User.Email != "" {
			internalIdentity.Principal = platformIdentity.User.Email
		}
	}

	// Set account number as tenant if available
	if platformIdentity.AccountNumber != "" {
		internalIdentity.Tenant = platformIdentity.AccountNumber
	}

	// Set type and other fields if available
	if platformIdentity.Type != "" {
		internalIdentity.Type = platformIdentity.Type
	}

	// Set auth type from platform identity if available
	if platformIdentity.AuthType != "" {
		internalIdentity.AuthType = platformIdentity.AuthType
	}
	if platformIdentity.User != nil && platformIdentity.User.UserID != "" {
		internalIdentity.UserID = platformIdentity.User.UserID
	}

	// Check if this is an internal user (not a service account)
	if platformIdentity.User != nil {
		// Internal users are not guests
		internalIdentity.IsGuest = false
	} else if platformIdentity.ServiceAccount != nil {
		// Service accounts are not guests
		internalIdentity.IsGuest = false
	} else {
		// Missing user info might be treated as guest
		internalIdentity.IsGuest = true
	}

	return internalIdentity
}
