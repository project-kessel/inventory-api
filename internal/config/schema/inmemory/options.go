package inmemory

import (
	"fmt"

	"github.com/spf13/pflag"
)

const (
	EmptyRepository = "empty"
	JSONRepository  = "json"
	DirRepository   = "dir"
)

type Options struct {
	Type string `mapstructure:"type"`
	Path string `mapstructure:"path"`
}

func NewOptions() *Options {
	return &Options{
		Type: JSONRepository,
		Path: "schema_cache.json",
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Type, prefix+"Type", o.Type, "Type of loading the repository from: empty, json or dir.")
	fs.StringVar(&o.Path, prefix+"Path", o.Path, "The Path to the schema data.")
}

func (o *Options) Complete() []error {
	return nil
}

func (o *Options) Validate() []error {
	if o.Type != EmptyRepository && o.Type != JSONRepository && o.Type != DirRepository {
		return []error{fmt.Errorf("invalid repository type: %s. valid values are %s, %s or %s", o.Type, EmptyRepository, JSONRepository, DirRepository)}
	}

	if o.Type == JSONRepository || o.Type == DirRepository {
		if o.Path == "" {
			return []error{fmt.Errorf("path is required when Type is set to %s", o.Type)}
		}
	}

	return nil
}
