package service

import (
	"github.com/spf13/pflag"
)

// Keeping this around for other Service Related Options

type Options struct {
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
}

func (o *Options) Validate() []error {
	var errs []error
	return errs
}

func (o *Options) Complete() []error {
	return nil
}
