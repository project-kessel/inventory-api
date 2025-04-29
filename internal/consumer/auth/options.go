package auth

import "github.com/spf13/pflag"

type Options struct {
	Enabled          bool   `mapstructure:"enabled"`
	SecurityProtocol string `mapstructure:"security-protocol"`
	SASLMechanism    string `mapstructure:"sasl-mechanism"`
	SASLUsername     string `mapstructure:"sasl-username"`
	SASLPassword     string `mapstructure:"sasl-password"`
}

func NewOptions() *Options {
	return &Options{
		Enabled: false,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.BoolVar(&o.Enabled, prefix+"enabled", o.Enabled, "enables authentication using confirm auth settings (default: false)")
	fs.StringVar(&o.SecurityProtocol, prefix+"security-protocol", o.SecurityProtocol, "security protocol to use for authentication)")
	fs.StringVar(&o.SASLMechanism, prefix+"sasl-mechanism", o.SASLMechanism, "sets the SASL mechanism")
	fs.StringVar(&o.SASLUsername, prefix+"sasl-username", o.SASLUsername, "sets the username to use for authentication")
	fs.StringVar(&o.SASLPassword, prefix+"sasl-password", o.SASLPassword, "sets the password to use for authentication")
}
