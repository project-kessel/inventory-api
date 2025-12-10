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
}

func NewOptions() *Options {
	return &Options{
		Endpoint:        "localhost:50051",
		Token:           "",
		TokenFile:       "",
		SchemaFile:      "deploy/schema.zed",
		UseTLS:          false,
		FullyConsistent: false,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Endpoint, prefix+"endpoint", o.Endpoint, "SpiceDB gRPC endpoint")
	fs.StringVar(&o.Token, prefix+"token", o.Token, "SpiceDB pre-shared key")
	fs.StringVar(&o.TokenFile, prefix+"token-file", o.TokenFile, "Path to file containing SpiceDB token")
	fs.StringVar(&o.SchemaFile, prefix+"schema-file", o.SchemaFile, "Path to SpiceDB schema file")
	fs.BoolVar(&o.UseTLS, prefix+"use-tls", o.UseTLS, "Use TLS for SpiceDB connection")
	fs.BoolVar(&o.FullyConsistent, prefix+"fully-consistent", o.FullyConsistent, "Use fully consistent reads by default")
}

func (o *Options) Validate() []error {
	var errs []error

	if o.Endpoint == "" {
		errs = append(errs, fmt.Errorf("spicedb.endpoint is required"))
	}

	if o.Token == "" && o.TokenFile == "" {
		errs = append(errs, fmt.Errorf("either spicedb.token or spicedb.token-file must be provided"))
	}

	if o.SchemaFile == "" {
		errs = append(errs, fmt.Errorf("spicedb.schema-file is required"))
	}

	return errs
}

func NewConfig(o *Options) *Config {
	return &Config{
		Endpoint:        o.Endpoint,
		Token:           o.Token,
		TokenFile:       o.TokenFile,
		SchemaFile:      o.SchemaFile,
		UseTLS:          o.UseTLS,
		FullyConsistent: o.FullyConsistent,
	}
}
