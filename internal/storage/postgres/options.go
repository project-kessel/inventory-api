package postgres

import (
	"fmt"

	"github.com/spf13/pflag"
)

type Options struct {
	// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-PARAMKEYWORDS
	Host        string `mapstructure:"host"`
	Port        string `mapstructure:"port"`
	DbName      string `mapstructure:"dbname"`
	User        string `mapstructure:"user"`
	Password    string `mapstructure:"password"`
	SSLMode     string `mapstructure:"sslmode"`
	SSLRootCert string `mapstructure:"sslrootcert"`
	// HostAddr                string `mapstructure:"hostaddr"`
	// Passfile                string `mapstructure:"passfile"`
	// RequireAuth             string `mapstructure:"require_auth"`
	// ChannelBinding          string `mapstructure:"channel_binding"`
	// ConnectTimeout          string `mapstructure:"connect_timeout"`
	// ClientEncoding          string `mapstructure:"client_encoding"`
	// Options                 string `mapstructure:"options"`
	// ApplicationName         string `mapstructure:"application_name"`
	// FallbackApplicationName string `mapstructure:"fallback_application_name"`
	// KeepAlives              string `mapstructure:"keepalives"`
	// KeepAlivesIdle          string `mapstructure:"keepalives_idle"`
	// KeepAlivesInterval      string `mapstructure:"keepalives_interval"`
	// KeepAlivesCount         string `mapstructure:"keepalives_count"`
	// TCPUserTimeout          string `mapstructure:"tcp_user_timeout"`
	// Replication             string `mapstructure:"replication"`
	// GSSEncMode              string `mapstructure:"gssencmode"`
	// RequireSSL              string `mapstructure:"requiressl"`
	// SSLCompression          string `mapstructure:"sslcompression"`
	// SSLCert                 string `mapstructure:"sslcert"`
	// SSLKey                  string `mapstructure:"sslkey"`
	// SSLPassword             string `mapstructure:"sslpassword"`
	// SSLCertMode             string `mapstructure:"sslcertmode"`
	// SSLRootCert             string `mapstructure:"sslrootcert"`
	// SSLCRL                  string `mapstructure:"sslcrl"`
	// SSLCRLDir               string `mapstructure:"sslcrldir"`
	// SSLSNI                  string `mapstructure:"sslsni"`
	// RequirePeer             string `mapstructure:"requirepeer"`
	// SSLMinProtocolVersion   string `mapstructure:"ssl_min_protocol_version"`
	// SSLMaxProtocolVersion   string `mapstructure:"ssl_max_protocol_version"`
	// KrbSrvName              string `mapstructure:"krbsrvname"`
	// GSSLib                  string `mapstructure:"gsslib"`
	// GSSDelegation           string `mapstructure:"gssdelegation"`
	// Service                 string `mapstructure:"service"`
	// TargetSessionAttrs      string `mapstructure:"target_session_attrs"`
	// LoadBalanceHosts        string `mapstructure:"load_balance_hosts"`
}

func NewOptions() *Options {
	return &Options{
		Host: "localhost",
		Port: "5432",
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Host, prefix+"host", o.Host, "Name of host to connect to.")
	fs.StringVar(&o.Port, prefix+"port", o.Port, "Port number to connect to at the server host, or socket file name extension for Unix-domain connections.")
	fs.StringVar(&o.DbName, prefix+"dbname", o.DbName, "The database name. Defaults to be the same as the user name.")
	fs.StringVar(&o.User, prefix+"user", o.User, "PostgreSQL user name to connect as. Defaults to be the same as the operating system name of the user running the application.")
	fs.StringVar(&o.Password, prefix+"password", o.Password, "Password to be used if the server demands password authentication.")
	fs.StringVar(&o.SSLMode, prefix+"sslmode", o.SSLMode, "This option determines whether or with what priority a secure SSL TCP/IP connection will be negotiated with the server. See https://www.postgresql.org/docs/16/libpq-connect.html#LIBPQ-CONNECT-SSLMODE")
	fs.StringVar(&o.SSLRootCert, prefix+"sslrootcert", o.SSLRootCert, "This parameter specifies the name of a file containing SSL certificate authority (CA) certificate(s). If the file exists, the server's certificate will be verified to be signed by one of these authorities. The default is ~/.postgresql/root.crt. The special value system may be specified instead, in which case the system's trusted CA roots will be loaded. The exact locations of these root certificates differ by SSL implementation and platform. For OpenSSL in particular, the locations may be further modified by the SSL_CERT_DIR and SSL_CERT_FILE environment variables.")
}

func (o *Options) Complete() error {
	return nil
}

func (o *Options) Validate() []error {
	if o.SSLMode != "" {
		validModes := map[string]int{
			"disable":     0,
			"allow":       0,
			"prefer":      0,
			"require":     0,
			"verify-ca":   0,
			"verify-full": 0,
		}
		if _, ok := validModes[o.SSLMode]; !ok {
			return []error{fmt.Errorf("invalid sslmode: %s", o.SSLMode)}
		}
	}
	return nil
}
