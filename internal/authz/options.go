package authz

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/project-kessel/inventory-api/internal/authz/kessel"
)

// TODO: presumably more will go here to authenticate Common Inventory as a service to call Kessel.
type Options struct {
	Authz  string          `mapstructure:"impl"`
	Kessel *kessel.Options `mapstructure:"kessel"`
}

const (
	AllowAll     = "allow-all"
	Kessel       = "kessel"
	RelationsAPI = "kessel-relations"
)

func NewOptions() *Options {
	return &Options{
		Authz:  AllowAll,
		Kessel: kessel.NewOptions(),
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
		errs = append(errs, fmt.Errorf("Invalid authz.impl: %s.  Options are 'allow-all' and 'kessel'", o.Authz))
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
