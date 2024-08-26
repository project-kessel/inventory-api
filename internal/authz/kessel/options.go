package kessel

import (
	"fmt"

	"github.com/spf13/pflag"
)

// TODO: presumably more will go here to authenticate Common Inventory as a service to call Kessel.
type Options struct {
	URL                   string `mapstructure:"url"`
	Insecure              bool   `mapstructure:"insecure-client"`
	enable_oidc_auth      bool   `mapstructure:"enable_oidc_auth"`
	sa_client_id          string `mapstructure:"sa_client_id"`
	sa_client_secret      string `mapstructure:"sa_client_secret"`
	sso_token_endpoint    string `mapstructure:"sso_token_endpoint"`
	token_refresh_minutes int    `mapstructure:"token_refresh_minutes"`
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.IntVar(&o.token_refresh_minutes, prefix+"token_refresh_minutes", o.token_refresh_minutes, "token refresh interval in minutes")
	fs.StringVar(&o.URL, prefix+"url", o.URL, "gRPC endpoint of the kessel service.")
	fs.StringVar(&o.sa_client_id, prefix+"sa_client_id", o.sa_client_id, "service account client id")
	fs.StringVar(&o.sa_client_secret, prefix+"sa_client_secret", o.sa_client_secret, "service account secret")
	fs.StringVar(&o.sso_token_endpoint, prefix+"sso_token_endpoint", o.sso_token_endpoint, "sso token endpoint.")
	fs.BoolVar(&o.enable_oidc_auth, prefix+"enable_oidc_auth", o.enable_oidc_auth, "enable oidc token auth to connect with kessel service")
	fs.BoolVar(&o.Insecure, prefix+"insecure-client", o.Insecure, "the http client that connects to kessel should not verify certificates.")
}

func (o *Options) Validate() []error {
	var errs []error

	if len(o.URL) == 0 {
		errs = append(errs, fmt.Errorf("kessel URL may not be empty"))
	}

	return errs
}

func (o *Options) Complete() []error {
	var errs []error

	return errs
}
