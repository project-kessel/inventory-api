package http

import (
	"fmt"

	"github.com/spf13/pflag"
)

type Options struct {
	Addr    string `mapstructure:"address"`
	Timeout int    `mapstructure:"timeout-seconds"`

	ServingCertFile string `mapstructure:"certfile"`
	PrivateKeyFile  string `mapstructure:"keyfile"`

	ClientCAFile string `mapstructure:"client-ca-file"`
	SNI          string `mapstructure:"sni-servername"`
	CertOpt      int    `mapstructure:"certopt"`
}

func NewOptions() *Options {
	return &Options{
		Addr:    "0.0.0.0:8000",
		Timeout: 300,
		CertOpt: 3, // https://pkg.go.dev/crypto/tls#ClientAuthType
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Addr, prefix+"address", o.Addr, "host and port on which the server listens")

	fs.IntVar(&o.Timeout, prefix+"timeout-seconds", o.Timeout, "request timeout in seconds")

	fs.StringVar(&o.ServingCertFile, prefix+"certfile", o.ServingCertFile, "the file containing the server's serving cert")
	fs.StringVar(&o.PrivateKeyFile, prefix+"keyfile", o.PrivateKeyFile, "the file containing the server's private key for the serving cert")

	fs.StringVar(&o.ClientCAFile, prefix+"client-ca-file", o.ClientCAFile, "the file containing the CA used to validate client certificates")
	fs.StringVar(&o.SNI, prefix+"sni-servername", o.SNI, "SNI server name used by client certificates.  See https://www.rfc-editor.org/rfc/rfc4366.html#section-3.1")
	fs.IntVar(&o.CertOpt, prefix+"certopt", o.CertOpt, "the certificate option to use for client certificate authentication.  See https://pkg.go.dev/crypto/tls#ClientAuthType")
}

func (o *Options) Complete() []error {
	return nil
}

func (o *Options) Validate() []error {
	var errors []error

	if o.Timeout < 0 {
		err := fmt.Errorf("read-timeout-seconds must be >= 0")
		errors = append(errors, err)
	}

	if (o.ServingCertFile == "" && o.PrivateKeyFile != "") || (o.ServingCertFile != "" && o.PrivateKeyFile == "") {
		err := fmt.Errorf("Both certfile and keyfile must be populated")
		errors = append(errors, err)
	}

	if o.CertOpt < 0 || o.CertOpt > 4 {
		err := fmt.Errorf("CertOpt must be >= 0 and <= 4")
		errors = append(errors, err)
	}

	return errors
}
