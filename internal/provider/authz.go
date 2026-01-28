package provider

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/project-kessel/inventory-api/internal/authz/allow"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/authz/kessel"
)

// NewAuthorizer creates a new authorizer from options.
func NewAuthorizer(ctx context.Context, opts *AuthzOptions, logger *log.Helper) (authzapi.Authorizer, error) {
	if errs := opts.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("authz validation failed: %v", errs)
	}

	switch opts.Authz {
	case AuthzAllowAll:
		return allow.New(logger), nil
	case AuthzKessel:
		return newKesselAuthorizer(ctx, opts.Kessel, logger)
	default:
		return nil, fmt.Errorf("unrecognized authz.impl: %s", opts.Authz)
	}
}

// newKesselAuthorizer creates a Kessel authorizer.
func newKesselAuthorizer(ctx context.Context, opts *KesselOptions, logger *log.Helper) (authzapi.Authorizer, error) {
	// Convert our options to kessel.Options
	kesselOpts := &kessel.Options{
		URL:            opts.URL,
		Insecure:       opts.Insecure,
		EnableOidcAuth: opts.EnableOidcAuth,
		ClientId:       opts.ClientId,
		ClientSecret:   opts.ClientSecret,
		TokenEndpoint:  opts.TokenEndpoint,
	}

	// Create config and complete it
	cfg := kessel.NewConfig(kesselOpts)
	completedCfg, errs := cfg.Complete(ctx)
	if errs != nil {
		return nil, fmt.Errorf("failed to complete kessel config: %v", errs)
	}

	return kessel.New(ctx, completedCfg, logger)
}

// CheckAuthorizer returns a string indicating which authorizer type is configured.
func CheckAuthorizer(opts *AuthzOptions) string {
	switch opts.Authz {
	case AuthzAllowAll:
		return "AllowAll"
	case AuthzKessel:
		return "Kessel"
	default:
		return "Unknown"
	}
}
