package authn

import (
	"github.com/spf13/pflag"

	"github.com/project-kessel/inventory-api/internal/authn/oidc"
	"github.com/project-kessel/inventory-api/internal/authn/psk"
)

type Options struct {
	AllowUnauthenticated bool `mapstructure:"allow-unauthenticated"`

	Oidc          *oidc.Options `mapstructure:"oidc"`
	PreSharedKeys *psk.Options  `mapstructure:"psk"`
}

func NewOptions() *Options {
	return &Options{
		Oidc:          oidc.NewOptions(),
		PreSharedKeys: psk.NewOptions(),
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.BoolVar(&o.AllowUnauthenticated, prefix+"allow-unauthenticated", o.AllowUnauthenticated, "Allow unauthenticated access to the server?")

	o.Oidc.AddFlags(fs, prefix+"oidc")
	o.PreSharedKeys.AddFlags(fs, prefix+"psk")
}

func (o *Options) Validate() []error {
	var errs []error

	errs = append(errs, o.Oidc.Validate()...)
	errs = append(errs, o.PreSharedKeys.Validate()...)

	return errs
}

func (o *Options) Complete() []error {
	var errs []error

	errs = append(errs, o.Oidc.Complete()...)
	errs = append(errs, o.PreSharedKeys.Complete()...)

	return errs
}
