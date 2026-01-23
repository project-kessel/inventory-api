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

type identityType string

const (
	identityTypeUser           identityType = "User"
	identityTypeSystem         identityType = "System"
	identityTypeServiceAccount identityType = "ServiceAccount"
)

// New creates a new XRhIdentityAuthenticator instance.
func New() *XRhIdentityAuthenticator {
	return &XRhIdentityAuthenticator{}
}

// Authenticate parses the x-rh-identity header and returns an identity if valid.
// Returns Ignore if the header is missing (allowing other authenticators to try).
// Returns Deny if the header is present but invalid.
func (a *XRhIdentityAuthenticator) Authenticate(ctx context.Context, t transport.Transporter) (*api.Claims, api.Decision) {
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

	// Convert platform identity to our internal Claims format
	internalClaims := convertPlatformClaims(&xrhid.Identity)
	return internalClaims, api.Allow
}

// convertPlatformClaims converts a platform-go-middlewares Identity to our internal Claims format
func convertPlatformClaims(platformIdentity *identity.Identity) *api.Claims {
	internalClaims := &api.Claims{
		AuthType: api.AuthTypeXRhIdentity,
	}

	if platformIdentity.Type == string(identityTypeUser) {
		// Set organization (prefer org_id, fallback to account_number)
		if platformIdentity.OrgID != "" {
			internalClaims.OrganizationId = api.OrganizationId(platformIdentity.OrgID)
		}
		// Set subject from user information
		if platformIdentity.User != nil {
			if platformIdentity.User.UserID != "" {
				internalClaims.SubjectId = api.SubjectId(platformIdentity.User.UserID)
			}
		}
	}

	if platformIdentity.Type == string(identityTypeSystem) {
		// TODO: add claims mapping for System identity type if needed
		_ = platformIdentity
	}

	if platformIdentity.Type == string(identityTypeServiceAccount) {
		// TODO: add claims mapping for ServiceAccount identity type if needed
		_ = platformIdentity
	}

	return internalClaims
}
