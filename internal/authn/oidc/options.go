package oidc

import (
	"github.com/spf13/pflag"
)

type Options struct {
	ClientId               string `mapstructure:"client-id"`
	AuthorizationServerURL string `mapstructure:"authn-server-url"`
	InsecureClient         bool   `mapstructure:"insecure-client"`
	SkipClientIDCheck      bool   `mapstructure:"skip-client-id-check"`
	EnforceAudCheck        bool   `mapstructure:"enforce-aud-check"`
	SkipIssuerCheck        bool   `mapstructure:"skip-issuer-check"`
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.StringVar(&o.ClientId, prefix+"client-id", o.ClientId, "the clientId issued by the authorization server that represents the application")
	fs.StringVar(&o.AuthorizationServerURL, prefix+"authn-server-url", o.AuthorizationServerURL, "the URL to the authorization server")
	fs.BoolVarP(&o.InsecureClient, prefix+"insecure-client", "k", o.InsecureClient, "validate authorization server certs?")
	fs.BoolVarP(&o.SkipClientIDCheck, prefix+"skip-client-id-check", "", o.SkipClientIDCheck, "if true, no ClientID check performed. Must be true if ClientID field is empty.")
	fs.BoolVarP(&o.EnforceAudCheck, prefix+"enforce-aud-check", "", o.EnforceAudCheck, "enforce aud claim check for clientId configured")
	fs.BoolVarP(&o.SkipIssuerCheck, prefix+"skip-issuer-check", "", o.SkipIssuerCheck, "intended for specialized such as testing cases where the the caller wishes to defer issuer validation")
}

func (o *Options) Validate() []error {
	return nil
}

func (o *Options) Complete() []error {
	return nil
}
