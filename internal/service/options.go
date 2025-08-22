package service

import (
	"github.com/spf13/pflag"
)

type Options struct {
	UseNew bool `mapstructure:"use_new"`
}

func NewOptions() *Options {
	return &Options{
		UseNew: false,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.BoolVar(&o.UseNew, prefix+"use_new", o.UseNew, "Toggle for using new resource reporting flow (default: false)")
}

func (o *Options) Validate() []error {
	var errs []error
	return errs
}

func (o *Options) Complete() []error {
	return nil
}
