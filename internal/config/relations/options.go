package relations

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/project-kessel/inventory-api/internal/config/relations/kessel"
	"github.com/project-kessel/inventory-api/internal/config/relations/spicedb"
)

type Options struct {
	// Authz selects the relations implementation ("allow-all", "kessel", or "spicedb").
	// Named "Authz" for backward compatibility with the --authz.* CLI flags.
	Authz   string           `mapstructure:"impl"`
	Kessel  *kessel.Options  `mapstructure:"kessel"`
	SpiceDB *spicedb.Options `mapstructure:"spicedb"`
}

const (
	AllowAll     = "allow-all"
	Kessel       = "kessel"
	SpiceDB      = "spicedb"
	RelationsAPI = "kessel-relations"
)

func NewOptions() *Options {
	return &Options{
		Authz:   AllowAll,
		Kessel:  kessel.NewOptions(),
		SpiceDB: spicedb.NewOptions(),
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	if o.Kessel == nil {
		o.Kessel = kessel.NewOptions()
	}
	if o.SpiceDB == nil {
		o.SpiceDB = spicedb.NewOptions()
	}

	fs.StringVar(&o.Authz, prefix+"impl", o.Authz, "Authz impl to use.  Options are 'allow-all', 'kessel', and 'spicedb'.")
	o.Kessel.AddFlags(fs, prefix+"kessel")
	o.SpiceDB.AddFlags(fs, prefix+"spicedb")
}

func (o *Options) Validate() []error {
	var errs []error

	if o.Authz != AllowAll && o.Authz != Kessel && o.Authz != SpiceDB {
		errs = append(errs, fmt.Errorf("invalid authz.impl: %s.  Options are 'allow-all', 'kessel', and 'spicedb'", o.Authz))
	}

	if o.Authz == Kessel {
		if o.Kessel == nil {
			errs = append(errs, fmt.Errorf("authz.kessel config is required when authz.impl=%q", Kessel))
		} else {
			errs = append(errs, o.Kessel.Validate()...)
		}
	}

	if o.Authz == SpiceDB {
		if o.SpiceDB == nil {
			errs = append(errs, fmt.Errorf("authz.spicedb config is required when authz.impl=%q", SpiceDB))
		} else {
			errs = append(errs, o.SpiceDB.Validate()...)
		}
	}

	return errs
}

func (o *Options) Complete() []error {
	var errs []error

	return errs
}
