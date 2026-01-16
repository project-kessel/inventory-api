package factory

import (
	"fmt"

	"github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authn/oidc"
	"github.com/project-kessel/inventory-api/internal/authn/unauthenticated"
	"github.com/project-kessel/inventory-api/internal/authn/xrhidentity"
)

// AuthenticatorType represents the type of authenticator
type AuthenticatorType string

const (
	TypeOIDC AuthenticatorType = "oidc"
	// TypeGuest is deprecated; kept for backwards compatibility with older configs.
	TypeGuest AuthenticatorType = "guest"
	// TypeAllowUnauthenticated is the preferred name for the unauthenticated (allow-all) authenticator.
	TypeAllowUnauthenticated AuthenticatorType = "allow-unauthenticated"
	TypeXRhIdentity          AuthenticatorType = "x-rh-identity"
)

// CreateAuthenticator creates an authenticator of the specified type with the given config.
// The config parameter should be the appropriate CompletedConfig type for the authenticator:
// - oidc: *oidc.CompletedConfig
// - guest: nil (no config needed)
// - x-rh-identity: nil (no config needed)
func CreateAuthenticator(authType AuthenticatorType, config interface{}) (api.Authenticator, error) {
	switch authType {
	case TypeOIDC:
		oidcConfig, ok := config.(*oidc.CompletedConfig)
		if !ok {
			return nil, fmt.Errorf("oidc authenticator requires *oidc.CompletedConfig, got %T", config)
		}
		if oidcConfig == nil {
			return nil, fmt.Errorf("oidc authenticator requires non-nil config")
		}
		return oidc.New(*oidcConfig)

	case TypeGuest, TypeAllowUnauthenticated:
		if config != nil {
			return nil, fmt.Errorf("guest authenticator does not require config")
		}
		return unauthenticated.New(), nil

	case TypeXRhIdentity:
		if config != nil {
			return nil, fmt.Errorf("x-rh-identity authenticator does not require config")
		}
		return xrhidentity.New(), nil

	default:
		return nil, fmt.Errorf("unknown authenticator type: %s", authType)
	}
}
