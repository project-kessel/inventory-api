package authn

import (
	"github.com/spf13/pflag"

	"github.com/project-kessel/inventory-api/internal/authn/oidc"
)

// ChainEntryOptions represents options for a chain entry
type ChainEntryOptions struct {
	Type    string                 `mapstructure:"type"`
	Enabled *bool                  `mapstructure:"enabled"` // nil means enabled=true (default)
	Config  map[string]interface{} `mapstructure:"config"`
}

// AuthenticatorOptions represents options for the aggregating authenticator
type AuthenticatorOptions struct {
	Type  string              `mapstructure:"type"`
	Chain []ChainEntryOptions `mapstructure:"chain"`
}

type Options struct {
	Authenticator        *AuthenticatorOptions `mapstructure:"authenticator"`
	AllowUnauthenticated *bool                 `mapstructure:"allow-unauthenticated"` // Legacy field for backwards compatibility
	OIDC                 *oidc.Options         `mapstructure:"oidc"`                  // Legacy field for backwards compatibility
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	_ = prefix // prefix parameter is reserved for future use
	// Note: Authenticator config is typically loaded from YAML/config files
	// Command-line flags for individual authenticator configs can be added here if needed
}

func (o *Options) Validate() []error {
	var errs []error

	// Prefer new format: if new format is present, validate it and ignore old formats
	if o.Authenticator != nil {
		// New format is present, validate it
		if o.Authenticator.Type == "" {
			errs = append(errs, &ConfigError{Message: "authenticator type is required"})
		}

		if len(o.Authenticator.Chain) == 0 {
			errs = append(errs, &ConfigError{Message: "authenticator chain must contain at least one entry"})
		}

		// Validate chain entry types
		validTypes := map[string]bool{
			"oidc":          true,
			"guest":         true,
			"x-rh-identity": true,
		}

		for _, entry := range o.Authenticator.Chain {
			if entry.Type == "" {
				errs = append(errs, &ConfigError{
					Message: "chain entry type is required",
				})
				continue
			}
			if !validTypes[entry.Type] {
				errs = append(errs, &ConfigError{
					Message: "invalid chain entry type",
					Type:    entry.Type,
				})
			}
		}

		return errs
	}

	// No new format: validate old format compatibility
	legacyFormatUsed := false
	if o.AllowUnauthenticated != nil && *o.AllowUnauthenticated {
		legacyFormatUsed = true
		if o.OIDC != nil {
			errs = append(errs, &ConfigError{
				Message: "cannot use both 'allow-unauthenticated' (legacy) and 'oidc' (legacy) configuration formats",
			})
		}
		return errs
	}

	if o.OIDC != nil {
		legacyFormatUsed = true
		return errs
	}

	// No format specified
	if !legacyFormatUsed {
		errs = append(errs, &ConfigError{Message: "authenticator configuration is required"})
		return errs
	}

	return errs
}

func (o *Options) Complete() []error {
	// Options completion is handled in Config.Complete()
	return nil
}
