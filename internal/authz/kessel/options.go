package kessel

import (
	"fmt"

	"github.com/spf13/pflag"
)

// TODO: presumably more will go here to authenticate Common Inventory as a service to call Kessel.
type Options struct {
	URL            string `mapstructure:"url"`
	Insecure       bool   `mapstructure:"insecure-client"`
	EnableOidcAuth bool   `mapstructure:"enable-oidc-auth"`
	ClientId       string `mapstructure:"sa-client-id"`
	ClientSecret   string `mapstructure:"sa-client-secret"`
	TokenEndpoint  string `mapstructure:"sso-token-endpoint"`
}

func NewOptions() *Options {
	return &Options{
		Insecure:       false,
		EnableOidcAuth: true,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.StringVar(&o.URL, prefix+"url", o.URL, "gRPC endpoint of the kessel service.")
	fs.StringVar(&o.ClientId, prefix+"sa-client-id", o.ClientId, "service account client id")
	fs.StringVar(&o.ClientSecret, prefix+"sa-client-secret", o.ClientSecret, "service account secret")
	fs.StringVar(&o.TokenEndpoint, prefix+"sso-token-endpoint", o.TokenEndpoint, "sso token endpoint.")
	fs.BoolVar(&o.EnableOidcAuth, prefix+"enable-oidc-auth", o.EnableOidcAuth, "enable oidc token auth to connect with kessel service")
	fs.BoolVar(&o.Insecure, prefix+"insecure-client", o.Insecure, "the http client that connects to kessel should not verify certificates.")
}

func (o *Options) Validate() []error {
	var errs []error

	if len(o.URL) == 0 {
		errs = append(errs, fmt.Errorf("kessel url may not be empty"))
	}

	return errs
}

func (o *Options) Complete() []error {
	var errs []error

	return errs
}
