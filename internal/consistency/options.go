package consistency

import (
	"github.com/spf13/pflag"
)

type Options struct {
	ReadAfterWriteEnabled          bool     `mapstructure:"read-after-write-enabled"`
	ReadAfterWriteAllowlist        []string `mapstructure:"read-after-write-allowlist"`
	DefaultToAtLeastAsAcknowledged bool     `mapstructure:"default-to-at-least-as-acknowledged"`
	IdempotencyCheckEnabled        bool     `mapstructure:"idempotency-check-enabled"`
}

func NewOptions() *Options {
	return &Options{
		ReadAfterWriteEnabled:          true,
		ReadAfterWriteAllowlist:        []string{},
		DefaultToAtLeastAsAcknowledged: true,
		IdempotencyCheckEnabled:        true,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.BoolVar(&o.ReadAfterWriteEnabled, prefix+"read-after-write-enabled", o.ReadAfterWriteEnabled, "Toggle for enabling or disabling the read after write consistency workflow (default: true)")
	fs.StringArrayVar(&o.ReadAfterWriteAllowlist, prefix+"read-after-write-allowlist", o.ReadAfterWriteAllowlist, "List of services that require all requests to be read-after-write enabled (default: [])")
	fs.BoolVar(&o.DefaultToAtLeastAsAcknowledged, prefix+"default-to-at-least-as-acknowledged", o.DefaultToAtLeastAsAcknowledged, "Default to at_least_as_acknowledged consistency for Check operations (default: true)")
	fs.BoolVar(&o.IdempotencyCheckEnabled, prefix+"idempotency-check-enabled", o.IdempotencyCheckEnabled, "Toggle for enabling or disabling transaction ID idempotency checks. Disable to allow reprocessing of events with duplicate transaction IDs (default: true)")
}

func (o *Options) Validate() []error {
	var errs []error

	return errs
}

func (o *Options) Complete() []error {
	return nil
}
