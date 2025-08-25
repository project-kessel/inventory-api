package service

import (
	"github.com/spf13/pflag"
)

type Options struct {
	UseV1beta2Db bool `mapstructure:"use_v1beta2_db"`
}

func NewOptions() *Options {
	return &Options{
		UseV1beta2Db: false,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.BoolVar(&o.UseV1beta2Db, prefix+"use_v1beta2_db", o.UseV1beta2Db, "Toggle for using v1beta2 database operations (default: false)")
}

func (o *Options) Validate() []error {
	var errs []error
	return errs
}

func (o *Options) Complete() []error {
	return nil
}
