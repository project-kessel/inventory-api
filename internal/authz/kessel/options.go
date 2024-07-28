package kessel

import (
	"fmt"

	"github.com/spf13/pflag"
)

// TODO: presumably more will go here to authenticate Common Inventory as a service to call Kessel.
type Options struct {
	URL      string `mapstructure:"url"`
	Insecure bool   `mapstructure:"insecure-client"`
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.URL, prefix+"url", o.URL, "https endpoint of the kessel service.")
	fs.BoolVar(&o.Insecure, prefix+"insecure-client", o.Insecure, "the http client that connects to kessel should not verify certificates.")
}

func (o *Options) Validate() []error {
	var errs []error

	if len(o.URL) == 0 {
		errs = append(errs, fmt.Errorf("kessel URL may not be empty"))
	}

	return errs
}

func (o *Options) Complete() []error {
	var errs []error

	return errs
}
