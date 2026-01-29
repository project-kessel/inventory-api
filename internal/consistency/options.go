package consistency

import (
	"github.com/spf13/pflag"
)

type Options struct {
	ReadAfterWriteEnabled          bool     `mapstructure:"read-after-write-enabled"`
	ReadAfterWriteAllowlist        []string `mapstructure:"read-after-write-allowlist"`
	DefaultToAtLeastAsAcknowledged bool     `mapstructure:"default-to-at-least-as-acknowledged"`
}

func NewOptions() *Options {
	return &Options{
		ReadAfterWriteEnabled:          true,
		ReadAfterWriteAllowlist:        []string{},
		DefaultToAtLeastAsAcknowledged: true,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.BoolVar(&o.ReadAfterWriteEnabled, prefix+"read-after-write-enabled", o.ReadAfterWriteEnabled, "Toggle for enabling or disabling the read after write consistency workflow (default: true)")
	fs.StringArrayVar(&o.ReadAfterWriteAllowlist, prefix+"read-after-write-allowlist", o.ReadAfterWriteAllowlist, "List of services that require all requests to be read-after-write enabled (default: [])")
	fs.BoolVar(&o.DefaultToAtLeastAsAcknowledged, prefix+"default-to-at-least-as-acknowledged", o.DefaultToAtLeastAsAcknowledged, "Default to at_least_as_acknowledged consistency for Check operations (default: false)")
}

func (o *Options) Validate() []error {
	var errs []error

	return errs
}

func (o *Options) Complete() []error {
	return nil
}
