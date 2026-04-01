package data

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

const (
	RelationsImplAllowAll = "allow-all"
	// RelationsImplSpiceDB selects the SpiceDB-backed implementation.
	// The value "kessel" is kept for config/CLI backward compatibility.
	RelationsImplSpiceDB = "kessel"
)

// RelationsOptionsRoot holds the top-level CLI options for the relations repository.
type RelationsOptionsRoot struct {
	Impl    string            `mapstructure:"impl"`
	SpiceDB *RelationsOptions `mapstructure:"kessel"`
}

// RelationsOptions holds the SpiceDB-specific connection options.
type RelationsOptions struct {
	URL            string `mapstructure:"url"`
	Insecure       bool   `mapstructure:"insecure-client"`
	EnableOidcAuth bool   `mapstructure:"enable-oidc-auth"`
	ClientId       string `mapstructure:"sa-client-id"`
	ClientSecret   string `mapstructure:"sa-client-secret"`
	TokenEndpoint  string `mapstructure:"sso-token-endpoint"`
}

// NewRelationsOptionsRoot creates default options.
func NewRelationsOptionsRoot() *RelationsOptionsRoot {
	return &RelationsOptionsRoot{
		Impl:    RelationsImplAllowAll,
		SpiceDB: NewRelationsOptions(),
	}
}

// NewRelationsOptions creates default SpiceDB-specific options.
func NewRelationsOptions() *RelationsOptions {
	return &RelationsOptions{
		Insecure:       false,
		EnableOidcAuth: true,
	}
}

// AddFlags registers CLI flags for the relations options.
func (o *RelationsOptionsRoot) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.StringVar(&o.Impl, prefix+"impl", o.Impl, "Relations impl to use.  Options are 'allow-all' and 'kessel'.")
	o.SpiceDB.AddFlags(fs, prefix+"kessel")
}

// AddFlags registers CLI flags for SpiceDB-specific options.
func (o *RelationsOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.StringVar(&o.URL, prefix+"url", o.URL, "gRPC endpoint of the relations service.")
	fs.StringVar(&o.ClientId, prefix+"sa-client-id", o.ClientId, "service account client id")
	fs.StringVar(&o.ClientSecret, prefix+"sa-client-secret", o.ClientSecret, "service account secret")
	fs.StringVar(&o.TokenEndpoint, prefix+"sso-token-endpoint", o.TokenEndpoint, "sso token endpoint.")
	fs.BoolVar(&o.EnableOidcAuth, prefix+"enable-oidc-auth", o.EnableOidcAuth, "enable oidc token auth to connect with relations service")
	fs.BoolVar(&o.Insecure, prefix+"insecure-client", o.Insecure, "the http client that connects to the relations service should not verify certificates.")
}

// Validate checks the options for errors.
func (o *RelationsOptionsRoot) Validate() []error {
	var errs []error
	if o.Impl != RelationsImplAllowAll && o.Impl != RelationsImplSpiceDB {
		errs = append(errs, fmt.Errorf("invalid authz.impl: %s.  Options are 'allow-all' and 'kessel'", o.Impl))
	}
	if o.Impl == RelationsImplSpiceDB {
		errs = append(errs, o.SpiceDB.Validate()...)
	}
	return errs
}

// Validate checks the SpiceDB-specific options.
func (o *RelationsOptions) Validate() []error {
	var errs []error
	if strings.TrimSpace(o.URL) == "" {
		errs = append(errs, fmt.Errorf("relations gRPC url (authz.kessel.url) may not be empty"))
	}
	if o.EnableOidcAuth {
		if strings.TrimSpace(o.ClientId) == "" {
			errs = append(errs, fmt.Errorf("authz.kessel.sa-client-id must be non-empty when enable-oidc-auth is true"))
		}
		if strings.TrimSpace(o.ClientSecret) == "" {
			errs = append(errs, fmt.Errorf("authz.kessel.sa-client-secret must be non-empty when enable-oidc-auth is true"))
		}
		if strings.TrimSpace(o.TokenEndpoint) == "" {
			errs = append(errs, fmt.Errorf("authz.kessel.sso-token-endpoint must be non-empty when enable-oidc-auth is true"))
		}
	}
	return errs
}

// Complete is a no-op that matches the existing pattern.
func (o *RelationsOptionsRoot) Complete() []error {
	return nil
}
