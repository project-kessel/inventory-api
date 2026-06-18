package inmemory

import (
	"fmt"

	"github.com/spf13/pflag"
)

const (
	EmptyRepository       = "empty"
	JSONRepository        = "json"
	DirRepository         = "dir"
	UnifiedYAMLRepository = "unified_yaml"
)

type Options struct {
	Type string `mapstructure:"type"`
	Path string `mapstructure:"path"`
}

func NewOptions() *Options {
	return &Options{
		Type: EmptyRepository,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Type, prefix+"Type", o.Type, "Type of loading the repository from: empty, json, dir, or unified_yaml.")
	fs.StringVar(&o.Path, prefix+"Path", o.Path, "The Path to the schema data.")
}

func (o *Options) Complete() []error {
	return nil
}

func (o *Options) Validate() []error {
	if o.Type != EmptyRepository && o.Type != JSONRepository && o.Type != DirRepository && o.Type != UnifiedYAMLRepository {
		return []error{fmt.Errorf("invalid repository type: %s. valid values are %s, %s, %s, or %s", o.Type, EmptyRepository, JSONRepository, DirRepository, UnifiedYAMLRepository)}
	}

	if o.Type == JSONRepository || o.Type == DirRepository || o.Type == UnifiedYAMLRepository {
		if o.Path == "" {
			return []error{fmt.Errorf("path is required when Type is set to %s", o.Type)}
		}
	}

	return nil
}
