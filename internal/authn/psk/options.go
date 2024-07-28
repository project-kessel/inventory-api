package psk

import (
	"github.com/spf13/pflag"
)

type Options struct {
	PreSharedKeyFile string `mapstructure:"pre-shared-key-file"`
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.StringVar(&o.PreSharedKeyFile, prefix+"pre-shared-key-file", "", "A file of identities with pre-shared keys that allow them to authenticate.")
}

func (o *Options) Validate() []error {
	return nil
}

func (o *Options) Complete() []error {
	return nil
}
