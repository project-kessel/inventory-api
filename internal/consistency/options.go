package consistency

import (
	"github.com/spf13/pflag"
)

type Options struct {
	ReadAfterWriteEnabled   bool     `mapstructure:"read-after-write-enabled"`
	ReadAfterWriteAllowlist []string `mapstructure:"read-after-write-allowlist"`
}

func NewOptions() *Options {
	return &Options{
		ReadAfterWriteEnabled:   true,
		ReadAfterWriteAllowlist: []string{},
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.BoolVar(&o.ReadAfterWriteEnabled, prefix+"read-after-write-enabled", o.ReadAfterWriteEnabled, "Toggle for enabling or disabling the read after write consistency workflow (default: true)")
	fs.StringArrayVar(&o.ReadAfterWriteAllowlist, prefix+"read-after-write-allowlist", o.ReadAfterWriteAllowlist, "List of services that require all requests to be read-after-write enabled (default: [])")

}

func (o *Options) Validate() []error {
	var errs []error

	return errs
}

func (o *Options) Complete() []error {
	return nil
}
