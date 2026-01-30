package selfsubject

import (
	"fmt"

	"github.com/spf13/pflag"
)

// Options holds settings for self-subject derivation strategies.
type Options struct {
	RedHatRbac *RedHatRbacOptions `mapstructure:"redhatRbacSelfSubjectStratergy"`
}

// RedHatRbacOptions configures the Red Hat RBAC strategy.
type RedHatRbacOptions struct {
	Enabled             bool                    `mapstructure:"enabled"`
	XRhIdentityDomain   string                  `mapstructure:"issuerDomain"`
	OIDCIssuerDomains   []OIDCIssuerDomainEntry `mapstructure:"oidcIssuerDomains"`
	OIDCIssuerDomainMap map[string]string       `mapstructure:"oidcIssuerDomainMap"`
}

// OIDCIssuerDomainEntry maps an OIDC issuer to a domain.
type OIDCIssuerDomainEntry struct {
	Issuer string `mapstructure:"iss"`
	Domain string `mapstructure:"domain"`
}

// NewOptions returns a new Options with default values.
func NewOptions() *Options {
	return &Options{
		RedHatRbac: &RedHatRbacOptions{},
	}
}

// AddFlags registers CLI flags for self-subject options.
func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.BoolVar(&o.RedHatRbac.Enabled, prefix+"redhat-rbac.enabled", o.RedHatRbac.Enabled, "Enable Red Hat RBAC self-subject strategy")
	fs.StringVar(&o.RedHatRbac.XRhIdentityDomain, prefix+"redhat-rbac.issuer-domain", o.RedHatRbac.XRhIdentityDomain, "Domain for x-rh-identity subjects")
}

// Validate checks that the configuration is valid.
func (o *Options) Validate() []error {
	var errs []error

	if o.RedHatRbac != nil && o.RedHatRbac.Enabled {
		if o.RedHatRbac.XRhIdentityDomain == "" {
			errs = append(errs, fmt.Errorf("selfsubject.redhat-rbac.issuer-domain is required when enabled"))
		}
		if len(o.RedHatRbac.OIDCIssuerDomainMap) == 0 {
			errs = append(errs, fmt.Errorf("selfsubject.redhat-rbac.oidcIssuerDomains is required when enabled"))
		}
	}

	return errs
}

// Complete finalizes the configuration.
func (o *Options) Complete() []error {
	if o == nil || o.RedHatRbac == nil {
		return nil
	}

	if len(o.RedHatRbac.OIDCIssuerDomains) > 0 {
		mapped, err := buildOIDCIssuerDomainMap(o.RedHatRbac.OIDCIssuerDomains)
		if err != nil {
			return []error{err}
		}
		o.RedHatRbac.OIDCIssuerDomainMap = mapped
		return nil
	}

	if o.RedHatRbac.OIDCIssuerDomainMap == nil {
		o.RedHatRbac.OIDCIssuerDomainMap = map[string]string{}
	}
	return nil
}

func buildOIDCIssuerDomainMap(entries []OIDCIssuerDomainEntry) (map[string]string, error) {
	out := make(map[string]string)
	for _, entry := range entries {
		if entry.Issuer == "" {
			return nil, fmt.Errorf("selfsubject.redhat-rbac.oidcIssuerDomains.iss is required")
		}
		if entry.Domain == "" {
			return nil, fmt.Errorf("selfsubject.redhat-rbac.oidcIssuerDomains.domain is required")
		}
		out[entry.Issuer] = entry.Domain
	}
	return out, nil
}

// Build constructs the configured SelfSubjectStrategy or returns nil when disabled.
func (o *Options) Build() SelfSubjectStrategy {
	if o == nil || o.RedHatRbac == nil || !o.RedHatRbac.Enabled {
		return nil
	}

	return NewRedHatRbacSelfSubjectStrategy(RedHatRbacSelfSubjectStrategyConfig{
		Enabled:             o.RedHatRbac.Enabled,
		XRhIdentityDomain:   o.RedHatRbac.XRhIdentityDomain,
		OIDCIssuerDomainMap: o.RedHatRbac.OIDCIssuerDomainMap,
	})
}
