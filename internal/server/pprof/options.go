package pprof

import (
	"fmt"

	"github.com/spf13/pflag"
)

const (
	DefaultPprofPort = "5000"
	DefaultPprofAddr = "0.0.0.0"
)

type Options struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    string `mapstructure:"port"`
	Addr    string `mapstructure:"addr"`
}

func NewOptions() *Options {
	return &Options{
		Enabled: false,
		Port:    DefaultPprofPort,
		Addr:    DefaultPprofAddr,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.BoolVar(&o.Enabled, prefix+"enabled", o.Enabled, "enable pprof profiling server")
	fs.StringVar(&o.Port, prefix+"port", o.Port, "port for pprof server")
	fs.StringVar(&o.Addr, prefix+"addr", o.Addr, "address for pprof server to bind to")
}

func (o *Options) Complete() []error {
	var errors []error

	if o.Port == "" {
		o.Port = DefaultPprofPort
	}

	if o.Addr == "" {
		o.Addr = DefaultPprofAddr
	}

	return errors
}

func (o *Options) Validate() []error {
	var errors []error

	if o.Enabled {
		if o.Port == "" {
			errors = append(errors, fmt.Errorf("pprof port must be set when pprof is enabled"))
		}
		if o.Addr == "" {
			errors = append(errors, fmt.Errorf("pprof addr must be set when pprof is enabled"))
		}
	}

	return errors
}

func (o *Options) GetListenAddr() string {
	return fmt.Sprintf("%s:%s", o.Addr, o.Port)
}
