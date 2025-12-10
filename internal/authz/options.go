package authz

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/project-kessel/inventory-api/internal/authz/spicedb"
)

type Options struct {
	Authz   string           `mapstructure:"impl"`
	SpiceDB *spicedb.Options `mapstructure:"spicedb"`
}

const (
	AllowAll = "allow-all"
	SpiceDB  = "spicedb"
)

func NewOptions() *Options {
	return &Options{
		Authz:   AllowAll,
		SpiceDB: spicedb.NewOptions(),
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Authz, prefix+"impl", o.Authz, "Authz impl to use.  Options are 'allow-all' and 'spicedb'.")
	o.SpiceDB.AddFlags(fs, prefix+"spicedb")
}

func (o *Options) Validate() []error {
	var errs []error

	if o.Authz != AllowAll && o.Authz != SpiceDB {
		errs = append(errs, fmt.Errorf("invalid authz.impl: %s.  Options are 'allow-all' and 'spicedb'", o.Authz))
	}

	if o.Authz == SpiceDB {
		errs = append(errs, o.SpiceDB.Validate()...)
	}

	return errs
}

func (o *Options) Complete() []error {
	var errs []error

	return errs
}
