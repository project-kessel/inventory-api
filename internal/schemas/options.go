package schemas

import (
	"fmt"

	"github.com/project-kessel/inventory-api/internal/schemas/in_memory"
	"github.com/spf13/pflag"
)

type Options struct {
	// "in_memory"
	Repository string             `mapstructure:"repository"`
	InMemory   *in_memory.Options `mapstructure:"in-memory"`
}

const (
	InMemoryRepository = "in-memory"
)

func NewOptions() *Options {
	return &Options{
		Repository: InMemoryRepository,
		InMemory:   in_memory.NewOptions(),
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Repository, prefix+"schemas", o.Repository, "The schema repository to use.")
	o.InMemory.AddFlags(fs, prefix+"in-memory")
}

func (o *Options) Complete() []error {
	var errs []error
	errs = append(errs, o.InMemory.Complete()...)

	return errs
}

func (o *Options) Validate() []error {
	var errs []error
	if o.Repository != InMemoryRepository {
		errs = append(errs, fmt.Errorf("repository option must be set to %s", InMemoryRepository))
	}

	switch o.Repository {
	case InMemoryRepository:
		errs = append(errs, o.InMemory.Validate()...)
	}

	return errs
}
