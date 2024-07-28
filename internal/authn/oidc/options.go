package oidc

import (
	"github.com/spf13/pflag"
)

type Options struct {
	ClientId               string `mapstructure:"client-id"`
	AuthorizationServerURL string `mapstructure:"authn-server-url"`
	InsecureClient         bool   `mapstructure:"insecure-client"`
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.StringVar(&o.ClientId, prefix+"client-id", "", "the clientId issued by the authorization server that represents the application")
	fs.StringVar(&o.AuthorizationServerURL, prefix+"authn-server-url", "", "the URL to the authorization server")
	fs.BoolVarP(&o.InsecureClient, prefix+"insecure-client", "k", false, "validate authorization server certs?")
}

func (o *Options) Validate() []error {
	return nil
}

func (o *Options) Complete() []error {
	return nil
}
