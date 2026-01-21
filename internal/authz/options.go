package authz

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/project-kessel/inventory-api/internal/authz/kessel"
)

// MetaAuthorizerOptions holds configuration for meta-authorization middleware
type MetaAuthorizerOptions struct {
	// Enabled controls whether meta authorization is enabled
	Enabled *bool `mapstructure:"enabled"`
	// Namespace is the namespace to use for metachecks (e.g., "rbac")
	Namespace string `mapstructure:"namespace"`
}

// TODO: presumably more will go here to authenticate Common Inventory as a service to call Kessel.
type Options struct {
	Authz          string                 `mapstructure:"impl"`
	Kessel         *kessel.Options        `mapstructure:"kessel"`
	MetaAuthorizer *MetaAuthorizerOptions `mapstructure:"metaauthorizer"`
}

const (
	AllowAll     = "allow-all"
	Kessel       = "kessel"
	RelationsAPI = "kessel-relations"
)

func NewOptions() *Options {
	enabled := true
	return &Options{
		Authz:  AllowAll,
		Kessel: kessel.NewOptions(),
		MetaAuthorizer: &MetaAuthorizerOptions{
			Enabled:   &enabled,
			Namespace: "rbac",
		},
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Authz, prefix+"impl", o.Authz, "Authz impl to use.  Options are 'allow-all' and 'kessel'.")
	o.Kessel.AddFlags(fs, prefix+"kessel")
}

func (o *Options) Validate() []error {
	var errs []error

	if o.Authz != AllowAll && o.Authz != Kessel {
		errs = append(errs, fmt.Errorf("invalid authz.impl: %s.  Options are 'allow-all' and 'kessel'", o.Authz))
	}

	if o.Authz == Kessel {
		errs = append(errs, o.Kessel.Validate()...)
	}

	return errs
}

func (o *Options) Complete() []error {
	var errs []error

	return errs
}
