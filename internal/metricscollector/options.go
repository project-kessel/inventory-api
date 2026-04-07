package metricscollector

import (
	"github.com/spf13/pflag"
)

type Options struct {
	Enabled bool `mapstructure:"enabled"`
}

func NewOptions() *Options {
	return &Options{
		Enabled: false,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.BoolVar(&o.Enabled, prefix+"enabled", o.Enabled, "enable business metrics collection from the metrics summary table (default: false)")
}
