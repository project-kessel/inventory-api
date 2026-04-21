package metaauthorizer

import "github.com/spf13/pflag"

type Options struct {
	TupleCrudAllowlist []string `mapstructure:"tuple-crud-allowlist"`
}

func NewOptions() *Options {
	return &Options{
		TupleCrudAllowlist: []string{}, // Empty = deny all by default
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.StringArrayVar(&o.TupleCrudAllowlist, prefix+"tuple-crud-allowlist", o.TupleCrudAllowlist,
		"List of client IDs (or subject IDs) allowed to access tuple CRUD endpoints (RBAC-only). Empty list denies all. Use '*' for testing.")
}

func (o *Options) Validate() []error {
	// No validation needed - empty list is valid (deny all)
	return nil
}

func (o *Options) Complete() []error {
	return nil
}
