package spicedb

import (
	"fmt"

	"github.com/spf13/pflag"
)

type Options struct {
	Endpoint        string `mapstructure:"endpoint"`
	Token           string `mapstructure:"token"`
	TokenFile       string `mapstructure:"token-file"`
	SchemaFile      string `mapstructure:"schema-file"`
	UseTLS          bool   `mapstructure:"use-tls"`
	FullyConsistent bool   `mapstructure:"fully-consistent"`
	ManageSchema    bool   `mapstructure:"manage-schema"`
}

func NewOptions() *Options {
	return &Options{
		UseTLS:          true,
		FullyConsistent: false,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.StringVar(&o.Endpoint, prefix+"endpoint", o.Endpoint, "SpiceDB gRPC endpoint (host:port)")
	fs.StringVar(&o.Token, prefix+"token", o.Token, "SpiceDB bearer token")
	fs.StringVar(&o.TokenFile, prefix+"token-file", o.TokenFile, "Path to file containing SpiceDB bearer token")
	fs.StringVar(&o.SchemaFile, prefix+"schema-file", o.SchemaFile, "Path to SpiceDB schema file")
	fs.BoolVar(&o.UseTLS, prefix+"use-tls", o.UseTLS, "Enable TLS for SpiceDB connection")
	fs.BoolVar(&o.FullyConsistent, prefix+"fully-consistent", o.FullyConsistent, "Use fully consistent reads (slower but strongest consistency)")
	fs.BoolVar(&o.ManageSchema, prefix+"manage-schema", o.ManageSchema, "Call WriteSchema on startup to manage the SpiceDB schema lifecycle")
}

func (o *Options) Validate() []error {
	var errs []error

	if len(o.Endpoint) == 0 {
		errs = append(errs, fmt.Errorf("spicedb endpoint may not be empty"))
	}

	if len(o.Token) == 0 && len(o.TokenFile) == 0 {
		errs = append(errs, fmt.Errorf("either spicedb token or token-file must be provided"))
	}

	if o.ManageSchema && len(o.SchemaFile) == 0 {
		errs = append(errs, fmt.Errorf("spicedb schema-file is required when manage-schema is enabled"))
	}

	return errs
}

func (o *Options) Complete() []error {
	var errs []error

	return errs
}
