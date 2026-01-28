// Package provider consolidates all configuration options and factory functions
// for creating the application's dependencies. It provides a simplified interface
// that takes Options structs and produces fully-configured objects.
package provider

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/pflag"

	"github.com/project-kessel/inventory-api/cmd/common"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
)

// Options contains the settings for each configuration option.
type Options struct {
	Authn       *AuthnOptions       `mapstructure:"authn"`
	Authz       *AuthzOptions       `mapstructure:"authz"`
	Storage     *StorageOptions     `mapstructure:"storage"`
	Eventing    *EventingOptions    `mapstructure:"eventing"`
	Consumer    *ConsumerOptions    `mapstructure:"consumer"`
	Server      *ServerOptions      `mapstructure:"server"`
	Consistency *ConsistencyOptions `mapstructure:"consistency"`
	Service     *ServiceOptions     `mapstructure:"service"`
	Schema      *SchemaOptions      `mapstructure:"schema"`
}

// NewOptions returns a new Options with default values set.
func NewOptions() *Options {
	return &Options{
		Authn:       NewAuthnOptions(),
		Authz:       NewAuthzOptions(),
		Storage:     NewStorageOptions(),
		Eventing:    NewEventingOptions(),
		Consumer:    NewConsumerOptions(),
		Server:      NewServerOptions(),
		Consistency: NewConsistencyOptions(),
		Service:     NewServiceOptions(),
		Schema:      NewSchemaOptions(),
	}
}

// InjectClowdAppConfig updates service options based on values in the ClowdApp AppConfig.
func (o *Options) InjectClowdAppConfig(appconfig *clowder.AppConfig) error {
	// check for authz config
	if len(appconfig.Endpoints) > 0 {
		for _, endpoint := range appconfig.Endpoints {
			if endpoint.App == RelationsAPI {
				o.ConfigureAuthz(endpoint)
			}
		}
	}
	// check for db config
	if !common.IsNil(appconfig.Database) {
		err := o.ConfigureStorage(appconfig)
		if err != nil {
			return fmt.Errorf("failed to configure storage: %w", err)
		}
	}
	// check for consumer config
	if !common.IsNil(appconfig.Kafka) {
		o.ConfigureConsumer(appconfig)
	}
	return nil
}

// ConfigureAuthz updates Authz settings based on ClowdApp AppConfig.
func (o *Options) ConfigureAuthz(endpoint clowder.DependencyEndpoint) {
	o.Authz.Authz = AuthzKessel
	o.Authz.Kessel.URL = fmt.Sprintf("%s:%d", endpoint.Hostname, 9000)
}

// ConfigureStorage updates Storage settings based on ClowdApp AppConfig.
func (o *Options) ConfigureStorage(appconfig *clowder.AppConfig) error {
	o.Storage.Database = DatabasePostgres
	o.Storage.Postgres.Host = appconfig.Database.Hostname
	o.Storage.Postgres.Port = strconv.Itoa(appconfig.Database.Port)
	o.Storage.Postgres.User = appconfig.Database.Username
	o.Storage.Postgres.Password = appconfig.Database.Password
	o.Storage.Postgres.DbName = appconfig.Database.Name

	if appconfig.Database.RdsCa != nil {
		caPath, err := appconfig.RdsCa()
		if err != nil {
			return fmt.Errorf("failed to load rds ca: %w", err)
		}
		o.Storage.Postgres.SSLMode = "verify-full"
		o.Storage.Postgres.SSLRootCert = caPath
	}
	return nil
}

// ConfigureConsumer updates Consumer settings based on ClowdApp AppConfig.
func (o *Options) ConfigureConsumer(appconfig *clowder.AppConfig) {
	var brokers []string
	for _, broker := range appconfig.Kafka.Brokers {
		brokers = append(brokers, fmt.Sprintf("%s:%d", broker.Hostname, *broker.Port))
	}
	o.Consumer.BootstrapServers = brokers

	if len(appconfig.Kafka.Brokers) > 0 && appconfig.Kafka.Brokers[0].SecurityProtocol != nil {
		o.Consumer.Auth.SecurityProtocol = *appconfig.Kafka.Brokers[0].SecurityProtocol

		if appconfig.Kafka.Brokers[0].Sasl != nil {
			o.Consumer.Auth.SASLMechanism = *appconfig.Kafka.Brokers[0].Sasl.SaslMechanism
			o.Consumer.Auth.SASLUsername = *appconfig.Kafka.Brokers[0].Sasl.Username
			o.Consumer.Auth.SASLPassword = *appconfig.Kafka.Brokers[0].Sasl.Password
		}
	}
}

// Database type constants.
const (
	DatabasePostgres = "postgres"
	DatabaseSqlite3  = "sqlite3"
)

// StorageOptions contains options for storage/database configuration.
type StorageOptions struct {
	Postgres                *PostgresOptions `mapstructure:"postgres"`
	SqlLite3                *SqlLite3Options `mapstructure:"sqlite3"`
	Database                string           `mapstructure:"database"`
	MaxSerializationRetries int              `mapstructure:"max-serialization-retries"`
}

// NewStorageOptions returns storage options with defaults.
func NewStorageOptions() *StorageOptions {
	return &StorageOptions{
		Postgres:                NewPostgresOptions(),
		SqlLite3:                NewSqlLite3Options(),
		Database:                DatabaseSqlite3,
		MaxSerializationRetries: 10,
	}
}

// AddFlags adds storage flags to the flag set.
func (o *StorageOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Database, prefix+"database", o.Database, "The database type to use. Either sqlite3 or postgres.")
	fs.IntVar(&o.MaxSerializationRetries, prefix+"max-serialization-retries", o.MaxSerializationRetries, "Maximum number of retries for serialized transactions")

	o.Postgres.AddFlags(fs, prefix+"postgres")
	o.SqlLite3.AddFlags(fs, prefix+"sqlite3")
}

// Validate validates storage options.
func (o *StorageOptions) Validate() []error {
	var errs []error
	if o.Database != DatabasePostgres && o.Database != DatabaseSqlite3 {
		errs = append(errs, fmt.Errorf("database must be either postgres or sqlite3"))
	}

	switch o.Database {
	case DatabasePostgres:
		errs = append(errs, o.Postgres.Validate()...)
	case DatabaseSqlite3:
		errs = append(errs, o.SqlLite3.Validate()...)
	}

	return errs
}

// PostgresOptions contains Postgres connection options.
// See https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-PARAMKEYWORDS
type PostgresOptions struct {
	Host        string `mapstructure:"host"`
	Port        string `mapstructure:"port"`
	DbName      string `mapstructure:"dbname"`
	User        string `mapstructure:"user"`
	Password    string `mapstructure:"password"`
	SSLMode     string `mapstructure:"sslmode"`
	SSLRootCert string `mapstructure:"sslrootcert"`
}

// NewPostgresOptions returns postgres options with defaults.
func NewPostgresOptions() *PostgresOptions {
	return &PostgresOptions{
		Host: "localhost",
		Port: "5432",
	}
}

// AddFlags adds postgres flags to the flag set.
func (o *PostgresOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Host, prefix+"host", o.Host, "Name of host to connect to.")
	fs.StringVar(&o.Port, prefix+"port", o.Port, "Port number to connect to at the server host, or socket file name extension for Unix-domain connections.")
	fs.StringVar(&o.DbName, prefix+"dbname", o.DbName, "The database name. Defaults to be the same as the user name.")
	fs.StringVar(&o.User, prefix+"user", o.User, "PostgreSQL user name to connect as. Defaults to be the same as the operating system name of the user running the application.")
	fs.StringVar(&o.Password, prefix+"password", o.Password, "Password to be used if the server demands password authentication.")
	fs.StringVar(&o.SSLMode, prefix+"sslmode", o.SSLMode, "This option determines whether or with what priority a secure SSL TCP/IP connection will be negotiated with the server.")
	fs.StringVar(&o.SSLRootCert, prefix+"sslrootcert", o.SSLRootCert, "This parameter specifies the name of a file containing SSL certificate authority (CA) certificate(s).")
}

// Validate validates postgres options.
func (o *PostgresOptions) Validate() []error {
	if o.SSLMode != "" {
		validModes := map[string]bool{
			"disable":     true,
			"allow":       true,
			"prefer":      true,
			"require":     true,
			"verify-ca":   true,
			"verify-full": true,
		}
		if !validModes[o.SSLMode] {
			return []error{fmt.Errorf("invalid sslmode: %s", o.SSLMode)}
		}
	}
	return nil
}

// SqlLite3Options contains SQLite3 connection options.
type SqlLite3Options struct {
	DSN string `mapstructure:"dsn"`
}

// NewSqlLite3Options returns sqlite3 options with defaults.
func NewSqlLite3Options() *SqlLite3Options {
	return &SqlLite3Options{
		DSN: "inventory.db",
	}
}

// AddFlags adds sqlite3 flags to the flag set.
func (o *SqlLite3Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.DSN, prefix+"dsn", o.DSN, "The connection string to use for sqlite3.")
}

// Validate validates sqlite3 options.
func (o *SqlLite3Options) Validate() []error {
	return nil
}

// Authz type constants.
const (
	AuthzAllowAll = "allow-all"
	AuthzKessel   = "kessel"
	RelationsAPI  = "kessel-relations"
)

// AuthzOptions contains authorization configuration.
type AuthzOptions struct {
	Authz  string         `mapstructure:"impl"`
	Kessel *KesselOptions `mapstructure:"kessel"`
}

// NewAuthzOptions returns authz options with defaults.
func NewAuthzOptions() *AuthzOptions {
	return &AuthzOptions{
		Authz:  AuthzAllowAll,
		Kessel: NewKesselOptions(),
	}
}

// AddFlags adds authz flags to the flag set.
func (o *AuthzOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Authz, prefix+"impl", o.Authz, "Authz impl to use. Options are 'allow-all' and 'kessel'.")
	o.Kessel.AddFlags(fs, prefix+"kessel")
}

// Validate validates authz options.
func (o *AuthzOptions) Validate() []error {
	var errs []error

	if o.Authz != AuthzAllowAll && o.Authz != AuthzKessel {
		errs = append(errs, fmt.Errorf("invalid authz.impl: %s. Options are 'allow-all' and 'kessel'", o.Authz))
	}

	if o.Authz == AuthzKessel {
		errs = append(errs, o.Kessel.Validate()...)
	}

	return errs
}

// KesselOptions contains Kessel authorization options.
type KesselOptions struct {
	URL            string `mapstructure:"url"`
	Insecure       bool   `mapstructure:"insecure-client"`
	EnableOidcAuth bool   `mapstructure:"enable-oidc-auth"`
	ClientId       string `mapstructure:"sa-client-id"`
	ClientSecret   string `mapstructure:"sa-client-secret"`
	TokenEndpoint  string `mapstructure:"sso-token-endpoint"`
}

// NewKesselOptions returns kessel options with defaults.
func NewKesselOptions() *KesselOptions {
	return &KesselOptions{
		Insecure:       false,
		EnableOidcAuth: true,
	}
}

// AddFlags adds kessel flags to the flag set.
func (o *KesselOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
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

// Validate validates kessel options.
func (o *KesselOptions) Validate() []error {
	var errs []error

	if len(o.URL) == 0 {
		errs = append(errs, fmt.Errorf("kessel url may not be empty"))
	}

	return errs
}

// AuthnTransportOptions controls per-protocol authentication enablement.
type AuthnTransportOptions struct {
	HTTP *bool `mapstructure:"http"`
	GRPC *bool `mapstructure:"grpc"`
}

// AuthnChainEntryOptions represents options for a chain entry.
type AuthnChainEntryOptions struct {
	Type string `mapstructure:"type"`
	// Enable controls whether this authenticator is enabled at all (optional, defaults to true).
	Enable *bool `mapstructure:"enable"`

	// Transport controls per-protocol enablement (optional).
	// If omitted, defaults to enabled for both HTTP and gRPC.
	Transport *AuthnTransportOptions `mapstructure:"transport"`

	Config map[string]interface{} `mapstructure:"config"`
}

// AuthenticatorOptions represents options for the aggregating authenticator.
type AuthenticatorOptions struct {
	Type  string                   `mapstructure:"type"`
	Chain []AuthnChainEntryOptions `mapstructure:"chain"`
}

// AuthnOptions contains authentication configuration.
type AuthnOptions struct {
	Authenticator        *AuthenticatorOptions `mapstructure:"authenticator"`
	AllowUnauthenticated *bool                 `mapstructure:"allow-unauthenticated"` // Legacy field for backwards compatibility
	OIDC                 *OIDCOptions          `mapstructure:"oidc"`                  // Legacy field for backwards compatibility
}

// NewAuthnOptions returns authn options with defaults.
func NewAuthnOptions() *AuthnOptions {
	return &AuthnOptions{}
}

// AddFlags adds authn flags to the flag set.
func (o *AuthnOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	_ = prefix // prefix parameter is reserved for future use
	// Note: Authenticator config is typically loaded from YAML/config files
}

// Validate validates authn options.
func (o *AuthnOptions) Validate() []error {
	var errs []error

	// Prefer new format: if new format is present, validate it and ignore old formats
	if o.Authenticator != nil {
		// New format is present, validate it
		if o.Authenticator.Type == "" {
			errs = append(errs, fmt.Errorf("authenticator type is required"))
		}

		if len(o.Authenticator.Chain) == 0 {
			errs = append(errs, fmt.Errorf("authenticator chain must contain at least one entry"))
		}

		// Validate chain entry types
		validTypes := map[string]bool{
			"oidc":                  true,
			"guest":                 true,
			"allow-unauthenticated": true,
			"x-rh-identity":         true,
		}

		for _, entry := range o.Authenticator.Chain {
			if entry.Type == "" {
				errs = append(errs, fmt.Errorf("chain entry type is required"))
				continue
			}
			if !validTypes[entry.Type] {
				errs = append(errs, fmt.Errorf("invalid chain entry type: %s", entry.Type))
			}
		}

		return errs
	}

	// No new format: validate old format compatibility
	if o.AllowUnauthenticated != nil && *o.AllowUnauthenticated {
		if o.OIDC != nil {
			errs = append(errs, fmt.Errorf("cannot use both 'allow-unauthenticated' (legacy) and 'oidc' (legacy) configuration formats"))
		}
		return errs
	}

	if o.OIDC != nil {
		return errs
	}

	// No format specified (neither new nor old)
	errs = append(errs, fmt.Errorf("authenticator configuration is required"))
	return errs
}

// OIDCOptions contains OIDC authentication options.
type OIDCOptions struct {
	ClientId               string `mapstructure:"client-id"`
	AuthorizationServerURL string `mapstructure:"authn-server-url"`
	InsecureClient         bool   `mapstructure:"insecure-client"`
	SkipClientIDCheck      bool   `mapstructure:"skip-client-id-check"`
	EnforceAudCheck        bool   `mapstructure:"enforce-aud-check"`
	SkipIssuerCheck        bool   `mapstructure:"skip-issuer-check"`
	PrincipalUserDomain    string `mapstructure:"principal-user-domain"`
}

// NewOIDCOptions returns OIDC options with defaults.
func NewOIDCOptions() *OIDCOptions {
	return &OIDCOptions{}
}

// AddFlags adds OIDC flags to the flag set.
func (o *OIDCOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.StringVar(&o.ClientId, prefix+"client-id", o.ClientId, "the clientId issued by the authorization server that represents the application")
	fs.StringVar(&o.AuthorizationServerURL, prefix+"authn-server-url", o.AuthorizationServerURL, "the url to the authorization server")
	fs.StringVar(&o.PrincipalUserDomain, prefix+"principal-user-domain", o.PrincipalUserDomain, "Kessel requires principal IDs to be qualified by a domain")
	fs.BoolVarP(&o.InsecureClient, prefix+"insecure-client", "k", o.InsecureClient, "validate authorization server certs?")
	fs.BoolVarP(&o.SkipClientIDCheck, prefix+"skip-client-id-check", "", o.SkipClientIDCheck, "if true, no clientId check performed")
	fs.BoolVarP(&o.EnforceAudCheck, prefix+"enforce-aud-check", "", o.EnforceAudCheck, "enforce aud claim check for clientId configured")
	fs.BoolVarP(&o.SkipIssuerCheck, prefix+"skip-issuer-check", "", o.SkipIssuerCheck, "defer issuer validation")
}

// EventingOptions contains eventing configuration.
type EventingOptions struct {
	Kafka   *KafkaEventingOptions `mapstructure:"kafka"`
	Eventer string                `mapstructure:"eventer"`
}

// NewEventingOptions returns eventing options with defaults.
func NewEventingOptions() *EventingOptions {
	return &EventingOptions{
		Kafka:   NewKafkaEventingOptions(),
		Eventer: "stdout",
	}
}

// AddFlags adds eventing flags to the flag set.
func (o *EventingOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Eventer, prefix+"eventer", o.Eventer, "The eventing subsystem to use. Either stdout or kafka.")

	o.Kafka.AddFlags(fs, prefix+"kafka")
}

// Validate validates eventing options.
func (o *EventingOptions) Validate() []error {
	var errs []error
	if o.Eventer != "stdout" && o.Eventer != "kafka" {
		errs = append(errs, fmt.Errorf("eventer must be either stdout or kafka"))
	}

	if o.Eventer == "kafka" {
		errs = append(errs, o.Kafka.Validate()...)
	}

	return errs
}

// KafkaEventingOptions contains Kafka eventing options.
type KafkaEventingOptions struct {
	DefaultTopic                       string `mapstructure:"default-topic"`
	BuiltInFeatures                    string `mapstructure:"builtin-features"`
	ClientId                           string `mapstructure:"client-id"`
	BootstrapServers                   string `mapstructure:"bootstrap-servers"`
	MessageMaxBytes                    int    `mapstructure:"message-max-bytes"`
	MessageCopyMaxBytes                int    `mapstructure:"message-copy-max-bytes"`
	ReceiveMessageMaxBytes             int    `mapstructure:"receive-message-max-bytes"`
	MaxInFlightRequestsPerConnection   int    `mapstructure:"max-in-flight-requests-per-connection"`
	MaxInFlight                        int    `mapstructure:"max-in-flight"`
	TopicMetadataRefreshIntervalMs     int    `mapstructure:"topic-metadata-refresh-interval-ms"`
	MetadataMaxAgeMs                   int    `mapstructure:"metadata-max-age-ms"`
	TopicMetadataRefreshFastIntervalMs int    `mapstructure:"topic-metadata-refresh-fast-interval-ms"`
	TopicMetadataRefreshSparse         bool   `mapstructure:"topic-metadata-refresh-sparse"`
	TopicMetadataPropagationMaxMs      int    `mapstructure:"topic-metadata-propagation-max-ms"`
	TopicBlacklist                     string `mapstructure:"topic-blacklist"`
	Debug                              string `mapstructure:"debug"`
	SocketTimeoutMs                    int    `mapstructure:"socket-timeout-ms"`
	SocketSendBufferBytes              int    `mapstructure:"socket-send-buffer-bytes"`
	SocketReceiveBufferBytes           int    `mapstructure:"socket-receive-buffer-bytes"`
	SocketKeepAliveEnable              bool   `mapstructure:"socket-keepalive-enable"`
	SocketNagleDisable                 bool   `mapstructure:"socket-nagle-disable"`
	SocketMaxFails                     int    `mapstructure:"socket-max-fails"`
	BrokerAddressTtl                   int    `mapstructure:"broker-address-ttl"`
	BrokerAddressFamily                string `mapstructure:"broker-address-family"`
	SocketConnectionSetupTimeoutMs     int    `mapstructure:"socket-connection-setup-timeout-ms"`
	ConnectionsMaxIdleMs               int    `mapstructure:"connections-max-idle-ms"`
	ReconnectBackoffMs                 int    `mapstructure:"reconnect-backoff-ms"`
	ReconnectBackoffMaxMs              int    `mapstructure:"reconnect-backoff-max-ms"`
	StatisticsIntervalMs               int    `mapstructure:"statistics-interval-ms"`
	EnabledEvents                      int    `mapstructure:"enabled-events"`
	LogLevel                           int    `mapstructure:"log-level"`
	LogQueue                           bool   `mapstructure:"log-queue"`
	LogThreadName                      bool   `mapstructure:"log-thread-name"`
	EnableRandomSeed                   bool   `mapstructure:"enabled-random-seed"`
	LogConnectionClose                 bool   `mapstructure:"log-connection-close"`
	InternalTerminationSignal          int    `mapstructure:"internal-termination-signal"`
	ApiVersionRequest                  bool   `mapstructure:"api-version-request"`
	ApiVersionRequestTimeoutMs         int    `mapstructure:"api-version-request-timeout-ms"`
	ApiVersionVersionFallbackMs        int    `mapstructure:"api-version-version-fallback-ms"`
	BrokerVersionFallback              string `mapstructure:"broker-version-fallback"`
	AllowAutoCreateTopics              bool   `mapstructure:"allow-auto-create-topics"`
	SecurityProtocol                   string `mapstructure:"security-protocol"`
	SslCipherSuites                    string `mapstructure:"ssl-cipher-suites"`
	SslCurvesList                      string `mapstructure:"ssl-curves-list"`
	SslSigAlgsList                     string `mapstructure:"ssl-sigalgs-list"`
	SslKeyLocation                     string `mapstructure:"ssl-key-location"`
	SslKeyPassword                     string `mapstructure:"ssl-key-password"`
	SslKeyPem                          string `mapstructure:"ssl-key-pem"`
	SslCertificateLocation             string `mapstructure:"ssl-certificate-location"`
	SslCertificatePem                  string `mapstructure:"ssl-certificate-pem"`
	SslCaLocation                      string `mapstructure:"ssl-ca-location"`
	SslCaPem                           string `mapstructure:"ssl-ca-pem"`
	SslCrlLocation                     string `mapstructure:"ssl-crl-location"`
	SslKeystoreLocation                string `mapstructure:"ssl-keystore-location"`
	SslKeystorePassword                string `mapstructure:"ssl-keystore-password"`
	SslProviders                       string `mapstructure:"ssl-providers"`
	SslEngineId                        string `mapstructure:"ssl-engine-id"`
	EnableSslCertificateVerification   bool   `mapstructure:"enable-ssl-certificate-verification"`
	SslEndpointIdentificationAlgorithm string `mapstructure:"ssl-endpoint-identification-algorithm"`
	SaslMechanism                      string `mapstructure:"sasl-mechanism"`
	SaslKerberosServiceName            string `mapstructure:"sasl-kerberos-service-name"`
	SaslKerberosPrincipal              string `mapstructure:"sasl-kerberos-principal"`
	SaslKerberosKinitCmd               string `mapstructure:"sasl-kerberos-kinit-cmd"`
	SaslKerberosKeytab                 string `mapstructure:"sasl-kerberos-keytab"`
	SaslKerberosMinTimeBeforeRelogin   int    `mapstructure:"sasl-kerberos-min-time-before-relogin"`
	SaslUsername                       string `mapstructure:"sasl-username"`
	SaslPassword                       string `mapstructure:"sasl-password"`
	SaslOauthBearerConfig              string `mapstructure:"sasl-oauthbearer-config"`
	EnableSaslOauthBearerUnsecureJwt   bool   `mapstructure:"enable-sasl-oauthbearer-unsecure-jwt"`
	SaslOauthBearerMethod              string `mapstructure:"sasl-oauthbearer-method"`
	SaslOauthBearerClientId            string `mapstructure:"sasl-oauthbearer-client-id"`
	SaslOauthBearerClientSecret        string `mapstructure:"sasl-oauthbearer-client-secret"`
	SaslOauthBearerScope               string `mapstructure:"sasl-oauthbearer-scope"`
	SaslOauthBearerExtensions          string `mapstructure:"sasl-oauthbearer-extensions"`
	SaslOauthBearerTokenEndpointUrl    string `mapstructure:"sasl-oauthbearer-token-endpoint-url"`
	PluginLibraryPaths                 string `mapstructure:"plugin-library-paths"`
	ClientDnsLookup                    string `mapstructure:"client-dns-lookup"`
	EnableMetricsPush                  bool   `mapstructure:"enable-metrics-push"`
	ClientRack                         string `mapstructure:"client-rack"`
	RetryBackoffMs                     int    `mapstructure:"retry-backoff-ms"`
	RetryBackoffMaxMs                  int    `mapstructure:"retry-backoff-max-ms"`
}

// NewKafkaEventingOptions returns kafka eventing options with defaults.
func NewKafkaEventingOptions() *KafkaEventingOptions {
	return &KafkaEventingOptions{
		DefaultTopic:                       "kessel-inventory",
		BuiltInFeatures:                    "gzip, snappy, ssl, sasl, regex, lz4, sasl_plain, sasl_scram, plugins, zstd, sasl_oauthbearer, http, oidc",
		ClientId:                           "rdkafka",
		BootstrapServers:                   "",
		MessageMaxBytes:                    1000000,
		MessageCopyMaxBytes:                65535,
		ReceiveMessageMaxBytes:             100000000,
		MaxInFlightRequestsPerConnection:   1000000,
		MaxInFlight:                        1000000,
		TopicMetadataRefreshIntervalMs:     300000,
		MetadataMaxAgeMs:                   900000,
		TopicMetadataRefreshFastIntervalMs: 100,
		TopicMetadataRefreshSparse:         true,
		TopicMetadataPropagationMaxMs:      30000,
		TopicBlacklist:                     "",
		Debug:                              "",
		SocketTimeoutMs:                    60000,
		SocketSendBufferBytes:              0,
		SocketReceiveBufferBytes:           0,
		SocketKeepAliveEnable:              false,
		SocketNagleDisable:                 false,
		SocketMaxFails:                     1,
		BrokerAddressTtl:                   1000,
		BrokerAddressFamily:                "any",
		SocketConnectionSetupTimeoutMs:     30000,
		ConnectionsMaxIdleMs:               0,
		ReconnectBackoffMs:                 100,
		ReconnectBackoffMaxMs:              10000,
		StatisticsIntervalMs:               0,
		EnabledEvents:                      0,
		LogLevel:                           6,
		LogQueue:                           false,
		LogThreadName:                      true,
		EnableRandomSeed:                   true,
		LogConnectionClose:                 true,
		InternalTerminationSignal:          0,
		ApiVersionRequest:                  true,
		ApiVersionRequestTimeoutMs:         10000,
		ApiVersionVersionFallbackMs:        0,
		BrokerVersionFallback:              "0.10.0",
		AllowAutoCreateTopics:              false,
		SecurityProtocol:                   "plaintext",
		SslCipherSuites:                    "",
		SslCurvesList:                      "",
		SslSigAlgsList:                     "",
		SslKeyLocation:                     "",
		SslKeyPassword:                     "",
		SslKeyPem:                          "",
		SslCertificateLocation:             "",
		SslCertificatePem:                  "",
		SslCaLocation:                      "",
		SslCaPem:                           "",
		SslCrlLocation:                     "",
		SslKeystoreLocation:                "",
		SslKeystorePassword:                "",
		SslProviders:                       "",
		SslEngineId:                        "dynamic",
		EnableSslCertificateVerification:   true,
		SslEndpointIdentificationAlgorithm: "https",
		SaslMechanism:                      "GSSAPI",
		SaslKerberosServiceName:            "kafka",
		SaslKerberosPrincipal:              "kafkaclient",
		SaslKerberosKinitCmd:               "kinit -R -t \"%{sasl.kerberos.keytab}\" -k %{sasl.kerberos.principal} || kinit -t \"%{sasl.kerberos.keytab}\" -k %{sasl.kerberos.principal}",
		SaslKerberosKeytab:                 "",
		SaslKerberosMinTimeBeforeRelogin:   60000,
		SaslUsername:                       "",
		SaslPassword:                       "",
		SaslOauthBearerConfig:              "",
		EnableSaslOauthBearerUnsecureJwt:   false,
		SaslOauthBearerMethod:              "default",
		SaslOauthBearerClientId:            "",
		SaslOauthBearerClientSecret:        "",
		SaslOauthBearerScope:               "",
		SaslOauthBearerExtensions:          "",
		SaslOauthBearerTokenEndpointUrl:    "",
		PluginLibraryPaths:                 "",
		ClientDnsLookup:                    "use_all_dns_ips",
		EnableMetricsPush:                  true,
		ClientRack:                         "",
		RetryBackoffMs:                     100,
		RetryBackoffMaxMs:                  1000,
	}
}

// AddFlags adds kafka eventing flags to the flag set.
func (o *KafkaEventingOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.DefaultTopic, prefix+"default-topic", o.DefaultTopic, "The topic to use.")
	fs.StringVar(&o.BuiltInFeatures, prefix+"builtin-features", o.BuiltInFeatures, "Indicates the builtin features for this build of librdkafka.")
	fs.StringVar(&o.ClientId, prefix+"client-id", o.ClientId, "Client identifier.")
	fs.StringVar(&o.BootstrapServers, prefix+"bootstrap-servers", o.BootstrapServers, "Initial list of brokers as a CSV list of broker host or host:port.")
	fs.IntVar(&o.MessageMaxBytes, prefix+"message-max-bytes", o.MessageMaxBytes, "Maximum Kafka protocol request message size.")
	fs.StringVar(&o.SecurityProtocol, prefix+"security-protocol", o.SecurityProtocol, "Protocol used to communicate with brokers.")
	fs.StringVar(&o.SaslMechanism, prefix+"sasl-mechanism", o.SaslMechanism, "SASL mechanism to use for authentication.")
	fs.StringVar(&o.SaslUsername, prefix+"sasl-username", o.SaslUsername, "SASL username for use with the PLAIN and SASL-SCRAM-.. mechanisms")
	fs.StringVar(&o.SaslPassword, prefix+"sasl-password", o.SaslPassword, "SASL password for use with the PLAIN and SASL-SCRAM-.. mechanism")
	fs.StringVar(&o.Debug, prefix+"debug", o.Debug, "A comma-separated list of debug contexts to enable.")
}

// Validate validates kafka eventing options.
func (o *KafkaEventingOptions) Validate() []error {
	return nil
}

// ConsumerAuthOptions contains Kafka consumer authentication options.
type ConsumerAuthOptions struct {
	Enabled          bool   `mapstructure:"enabled"`
	SecurityProtocol string `mapstructure:"security-protocol"`
	SASLMechanism    string `mapstructure:"sasl-mechanism"`
	SASLUsername     string `mapstructure:"sasl-username"`
	SASLPassword     string `mapstructure:"sasl-password"`
	CACertLocation   string `mapstructure:"ca-cert-location"`
}

// NewConsumerAuthOptions returns consumer auth options with defaults.
func NewConsumerAuthOptions() *ConsumerAuthOptions {
	return &ConsumerAuthOptions{
		Enabled: true,
	}
}

// AddFlags adds consumer auth flags to the flag set.
func (o *ConsumerAuthOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.BoolVar(&o.Enabled, prefix+"enabled", o.Enabled, "enables authentication using confirm auth settings")
	fs.StringVar(&o.SecurityProtocol, prefix+"security-protocol", o.SecurityProtocol, "security protocol to use for authentication")
	fs.StringVar(&o.SASLMechanism, prefix+"sasl-mechanism", o.SASLMechanism, "sets the SASL mechanism")
	fs.StringVar(&o.SASLUsername, prefix+"sasl-username", o.SASLUsername, "sets the username to use for authentication")
	fs.StringVar(&o.SASLPassword, prefix+"sasl-password", o.SASLPassword, "sets the password to use for authentication")
	fs.StringVar(&o.CACertLocation, prefix+"ca-cert-location", o.CACertLocation, "sets the location of the Kafka clusters' CA certificate")
}

// ConsumerRetryOptions contains Kafka consumer retry options.
type ConsumerRetryOptions struct {
	ConsumerMaxRetries  int `mapstructure:"consumer-max-retries"`
	OperationMaxRetries int `mapstructure:"operation-max-retries"`
	BackoffFactor       int `mapstructure:"backoff-factor"`
	MaxBackoffSeconds   int `mapstructure:"max-backoff-seconds"`
}

// NewConsumerRetryOptions returns consumer retry options with defaults.
func NewConsumerRetryOptions() *ConsumerRetryOptions {
	return &ConsumerRetryOptions{
		ConsumerMaxRetries:  2,
		OperationMaxRetries: 3,
		BackoffFactor:       5,
		MaxBackoffSeconds:   30,
	}
}

// AddFlags adds consumer retry flags to the flag set.
func (o *ConsumerRetryOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.IntVar(&o.ConsumerMaxRetries, prefix+"consumer-max-retries", o.ConsumerMaxRetries, "sets the max number of retries to process a message before killing consumer")
	fs.IntVar(&o.OperationMaxRetries, prefix+"operation-max-retries", o.OperationMaxRetries, "sets the max number of retries to execute a request before failing out")
	fs.IntVar(&o.BackoffFactor, prefix+"backoff-factor", o.BackoffFactor, "value used to calculate backoff between requests/restarts")
	fs.IntVar(&o.MaxBackoffSeconds, prefix+"max-backoff-seconds", o.MaxBackoffSeconds, "maximum amount of time between retries for the consumer in seconds")
}

// ConsumerOptions contains Kafka consumer options.
type ConsumerOptions struct {
	Enabled                 bool                  `mapstructure:"enabled"`
	BootstrapServers        []string              `mapstructure:"bootstrap-servers"`
	ConsumerGroupID         string                `mapstructure:"consumer-group-id"`
	Topic                   string                `mapstructure:"topic"`
	CommitModulo            int                   `mapstructure:"commit-modulo"`
	SessionTimeout          string                `mapstructure:"session-timeout"`
	HeartbeatInterval       string                `mapstructure:"heartbeat-interval"`
	MaxPollInterval         string                `mapstructure:"max-poll-interval"`
	EnableAutoCommit        string                `mapstructure:"enable-auto-commit"`
	AutoOffsetReset         string                `mapstructure:"auto-offset-reset"`
	StatisticsInterval      string                `mapstructure:"statistics-interval-ms"`
	Debug                   string                `mapstructure:"debug"`
	Retry                   *ConsumerRetryOptions `mapstructure:"retry-options"`
	Auth                    *ConsumerAuthOptions  `mapstructure:"auth"`
	ReadAfterWriteEnabled   bool                  `mapstructure:"read-after-write-enabled"`
	ReadAfterWriteAllowlist []string              `mapstructure:"read-after-write-allowlist"`
}

// NewConsumerOptions returns consumer options with defaults.
func NewConsumerOptions() *ConsumerOptions {
	return &ConsumerOptions{
		Enabled:                 true,
		ConsumerGroupID:         "inventory-consumer",
		Topic:                   "outbox.event.kessel.tuples",
		CommitModulo:            10,
		SessionTimeout:          "45000",
		HeartbeatInterval:       "3000",
		MaxPollInterval:         "300000",
		EnableAutoCommit:        "false",
		AutoOffsetReset:         "earliest",
		StatisticsInterval:      "60000",
		Debug:                   "",
		Auth:                    NewConsumerAuthOptions(),
		Retry:                   NewConsumerRetryOptions(),
		ReadAfterWriteEnabled:   true,
		ReadAfterWriteAllowlist: []string{},
	}
}

// AddFlags adds consumer flags to the flag set.
func (o *ConsumerOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.BoolVar(&o.Enabled, prefix+"enabled", o.Enabled, "Toggle for enabling or disabling the consumer")
	fs.StringSliceVar(&o.BootstrapServers, prefix+"bootstrap-servers", o.BootstrapServers, "sets the bootstrap server address and port for Kafka")
	fs.StringVar(&o.ConsumerGroupID, prefix+"consumer-group-id", o.ConsumerGroupID, "sets the Kafka consumer group name")
	fs.StringVar(&o.Topic, prefix+"topic", o.Topic, "Kafka topic to monitor for events")
	fs.IntVar(&o.CommitModulo, prefix+"commit-modulo", o.CommitModulo, "defines the modulo used to calculate when to commit offsets")
	fs.BoolVar(&o.ReadAfterWriteEnabled, prefix+"read-after-write-enabled", o.ReadAfterWriteEnabled, "Toggle for enabling or disabling the read after write consistency workflow")
	fs.StringArrayVar(&o.ReadAfterWriteAllowlist, prefix+"read-after-write-allowlist", o.ReadAfterWriteAllowlist, "List of services that require all requests to be read-after-write enabled")
	fs.StringVar(&o.SessionTimeout, prefix+"session-timeout", o.SessionTimeout, "time a consumer can live without sending heartbeat")
	fs.StringVar(&o.HeartbeatInterval, prefix+"heartbeat-interval", o.HeartbeatInterval, "interval between heartbeats sent to Kafka")
	fs.StringVar(&o.MaxPollInterval, prefix+"max-poll-interval", o.MaxPollInterval, "length of time consumer can go without polling before considered dead")
	fs.StringVar(&o.EnableAutoCommit, prefix+"enable-auto-commit", o.EnableAutoCommit, "enables auto commit on consumer when messages are consumed")
	fs.StringVar(&o.AutoOffsetReset, prefix+"auto-offset-reset", o.AutoOffsetReset, "action to take when there is no initial offset in offset store")
	fs.StringVar(&o.StatisticsInterval, prefix+"statistics-interval-ms", o.StatisticsInterval, "librdkafka statistics emit interval")
	fs.StringVar(&o.Debug, prefix+"debug", o.Debug, "a comma-separated list of debug contexts to enable")

	o.Auth.AddFlags(fs, prefix+"auth")
	o.Retry.AddFlags(fs, prefix+"retry-options")
}

// Validate validates consumer options.
func (o *ConsumerOptions) Validate() []error {
	var errs []error

	if len(o.BootstrapServers) == 0 && o.Enabled {
		errs = append(errs, fmt.Errorf("bootstrap servers can not be empty"))
	}

	if o.CommitModulo <= 0 {
		errs = append(errs, fmt.Errorf("commit modulo must be a positive, non-zero integer value"))
	}
	return errs
}

// ServerOptions contains server configuration.
type ServerOptions struct {
	Id        string        `mapstructure:"id"`
	Name      string        `mapstructure:"name"`
	PublicUrl string        `mapstructure:"public_url"`
	Grpc      *GrpcOptions  `mapstructure:"grpc"`
	Http      *HttpOptions  `mapstructure:"http"`
	Pprof     *PprofOptions `mapstructure:"pprof"`
}

// NewServerOptions returns server options with defaults.
func NewServerOptions() *ServerOptions {
	id, _ := os.Hostname()
	return &ServerOptions{
		Id:        id,
		Name:      "kessel-inventory-api",
		PublicUrl: "http://localhost:8000",
		Grpc:      NewGrpcOptions(),
		Http:      NewHttpOptions(),
		Pprof:     NewPprofOptions(),
	}
}

// AddFlags adds server flags to the flag set.
func (o *ServerOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Id, prefix+"id", o.Id, "id of the server")
	fs.StringVar(&o.Name, prefix+"name", o.Name, "name of the server")
	fs.StringVar(&o.PublicUrl, prefix+"public_url", o.PublicUrl, "Public url where the server is reachable")

	o.Grpc.AddFlags(fs, prefix+"grpc")
	o.Http.AddFlags(fs, prefix+"http")
	o.Pprof.AddFlags(fs, prefix+"pprof")
}

// Validate validates server options.
func (o *ServerOptions) Validate() []error {
	var errs []error
	errs = append(errs, o.Grpc.Validate()...)
	errs = append(errs, o.Http.Validate()...)
	errs = append(errs, o.Pprof.Validate()...)
	return errs
}

// GrpcOptions contains gRPC server options.
type GrpcOptions struct {
	Addr           string `mapstructure:"address"`
	Timeout        int    `mapstructure:"timeout-seconds"`
	ServingCertFile string `mapstructure:"certfile"`
	PrivateKeyFile  string `mapstructure:"keyfile"`
	ClientCAFile    string `mapstructure:"client-ca-file"`
	SNI             string `mapstructure:"sni-servername"`
	CertOpt         int    `mapstructure:"certopt"`
}

// NewGrpcOptions returns gRPC options with defaults.
func NewGrpcOptions() *GrpcOptions {
	return &GrpcOptions{
		Addr:    "0.0.0.0:9000",
		Timeout: 300,
		CertOpt: 3, // https://pkg.go.dev/crypto/tls#ClientAuthType
	}
}

// AddFlags adds gRPC flags to the flag set.
func (o *GrpcOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Addr, prefix+"address", o.Addr, "host and port on which the server listens")
	fs.IntVar(&o.Timeout, prefix+"timeout-seconds", o.Timeout, "request timeout in seconds")
	fs.StringVar(&o.ServingCertFile, prefix+"certfile", o.ServingCertFile, "the file containing the server's serving cert")
	fs.StringVar(&o.PrivateKeyFile, prefix+"keyfile", o.PrivateKeyFile, "the file containing the server's private key for the serving cert")
	fs.StringVar(&o.ClientCAFile, prefix+"client-ca-file", o.ClientCAFile, "the file containing the CA used to validate client certificates")
	fs.StringVar(&o.SNI, prefix+"sni-servername", o.SNI, "SNI server name used by client certificates")
	fs.IntVar(&o.CertOpt, prefix+"certopt", o.CertOpt, "the certificate option to use for client certificate authentication")
}

// Validate validates gRPC options.
func (o *GrpcOptions) Validate() []error {
	var errs []error

	if o.Timeout < 0 {
		errs = append(errs, fmt.Errorf("grpc timeout-seconds must be >= 0"))
	}

	if (o.ServingCertFile == "" && o.PrivateKeyFile != "") || (o.ServingCertFile != "" && o.PrivateKeyFile == "") {
		errs = append(errs, fmt.Errorf("both grpc certfile and keyfile must be populated"))
	}

	if o.CertOpt < 0 || o.CertOpt > 4 {
		errs = append(errs, fmt.Errorf("grpc certopt must be >= 0 and <= 4"))
	}

	return errs
}

// HttpOptions contains HTTP server options.
type HttpOptions struct {
	Addr           string `mapstructure:"address"`
	Timeout        int    `mapstructure:"timeout-seconds"`
	ServingCertFile string `mapstructure:"certfile"`
	PrivateKeyFile  string `mapstructure:"keyfile"`
	ClientCAFile    string `mapstructure:"client-ca-file"`
	SNI             string `mapstructure:"sni-servername"`
	CertOpt         int    `mapstructure:"certopt"`
}

// NewHttpOptions returns HTTP options with defaults.
func NewHttpOptions() *HttpOptions {
	return &HttpOptions{
		Addr:    "0.0.0.0:8000",
		Timeout: 300,
		CertOpt: 3, // https://pkg.go.dev/crypto/tls#ClientAuthType
	}
}

// AddFlags adds HTTP flags to the flag set.
func (o *HttpOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Addr, prefix+"address", o.Addr, "host and port on which the server listens")
	fs.IntVar(&o.Timeout, prefix+"timeout-seconds", o.Timeout, "request timeout in seconds")
	fs.StringVar(&o.ServingCertFile, prefix+"certfile", o.ServingCertFile, "the file containing the server's serving cert")
	fs.StringVar(&o.PrivateKeyFile, prefix+"keyfile", o.PrivateKeyFile, "the file containing the server's private key for the serving cert")
	fs.StringVar(&o.ClientCAFile, prefix+"client-ca-file", o.ClientCAFile, "the file containing the CA used to validate client certificates")
	fs.StringVar(&o.SNI, prefix+"sni-servername", o.SNI, "SNI server name used by client certificates")
	fs.IntVar(&o.CertOpt, prefix+"certopt", o.CertOpt, "the certificate option to use for client certificate authentication")
}

// Validate validates HTTP options.
func (o *HttpOptions) Validate() []error {
	var errs []error

	if o.Timeout < 0 {
		errs = append(errs, fmt.Errorf("http timeout-seconds must be >= 0"))
	}

	if (o.ServingCertFile == "" && o.PrivateKeyFile != "") || (o.ServingCertFile != "" && o.PrivateKeyFile == "") {
		errs = append(errs, fmt.Errorf("both http certfile and keyfile must be populated"))
	}

	if o.CertOpt < 0 || o.CertOpt > 4 {
		errs = append(errs, fmt.Errorf("http certopt must be >= 0 and <= 4"))
	}

	return errs
}

// PprofOptions contains pprof server options.
type PprofOptions struct {
	Enabled bool   `mapstructure:"enabled"`
	Port    string `mapstructure:"port"`
	Addr    string `mapstructure:"addr"`
}

const (
	DefaultPprofPort = "5000"
	DefaultPprofAddr = "0.0.0.0"
)

// NewPprofOptions returns pprof options with defaults.
func NewPprofOptions() *PprofOptions {
	return &PprofOptions{
		Enabled: false,
		Port:    DefaultPprofPort,
		Addr:    DefaultPprofAddr,
	}
}

// AddFlags adds pprof flags to the flag set.
func (o *PprofOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.BoolVar(&o.Enabled, prefix+"enabled", o.Enabled, "enable pprof profiling server")
	fs.StringVar(&o.Port, prefix+"port", o.Port, "port for pprof server")
	fs.StringVar(&o.Addr, prefix+"addr", o.Addr, "address for pprof server to bind to")
}

// Validate validates pprof options.
func (o *PprofOptions) Validate() []error {
	var errs []error

	if o.Enabled {
		if o.Port == "" {
			errs = append(errs, fmt.Errorf("pprof port must be set when pprof is enabled"))
		}
		if o.Addr == "" {
			errs = append(errs, fmt.Errorf("pprof addr must be set when pprof is enabled"))
		}
	}

	return errs
}

// GetListenAddr returns the full listen address for pprof.
func (o *PprofOptions) GetListenAddr() string {
	return fmt.Sprintf("%s:%s", o.Addr, o.Port)
}

// Complete fills in default values for pprof options.
func (o *PprofOptions) Complete() {
	if o.Port == "" {
		o.Port = DefaultPprofPort
	}
	if o.Addr == "" {
		o.Addr = DefaultPprofAddr
	}
}

// ConsistencyOptions contains consistency configuration.
type ConsistencyOptions struct {
	ReadAfterWriteEnabled   bool     `mapstructure:"read-after-write-enabled"`
	ReadAfterWriteAllowlist []string `mapstructure:"read-after-write-allowlist"`
}

// NewConsistencyOptions returns consistency options with defaults.
func NewConsistencyOptions() *ConsistencyOptions {
	return &ConsistencyOptions{
		ReadAfterWriteEnabled:   true,
		ReadAfterWriteAllowlist: []string{},
	}
}

// AddFlags adds consistency flags to the flag set.
func (o *ConsistencyOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.BoolVar(&o.ReadAfterWriteEnabled, prefix+"read-after-write-enabled", o.ReadAfterWriteEnabled, "Toggle for enabling or disabling the read after write consistency workflow")
	fs.StringArrayVar(&o.ReadAfterWriteAllowlist, prefix+"read-after-write-allowlist", o.ReadAfterWriteAllowlist, "List of services that require all requests to be read-after-write enabled")
}

// ServiceOptions contains service-level options.
type ServiceOptions struct{}

// NewServiceOptions returns service options with defaults.
func NewServiceOptions() *ServiceOptions {
	return &ServiceOptions{}
}

// AddFlags adds service flags to the flag set.
func (o *ServiceOptions) AddFlags() {}

// Schema repository type constants.
const (
	SchemaRepositoryInMemory = "in-memory"
)

// InMemorySchemaType constants.
const (
	SchemaTypeEmpty = "empty"
	SchemaTypeJSON  = "json"
	SchemaTypeDir   = "dir"
)

// InMemorySchemaOptions contains in-memory schema options.
type InMemorySchemaOptions struct {
	Type string `mapstructure:"type"`
	Path string `mapstructure:"path"`
}

// NewInMemorySchemaOptions returns in-memory schema options with defaults.
func NewInMemorySchemaOptions() *InMemorySchemaOptions {
	return &InMemorySchemaOptions{
		Type: SchemaTypeEmpty,
	}
}

// AddFlags adds in-memory schema flags to the flag set.
func (o *InMemorySchemaOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Type, prefix+"Type", o.Type, "Type of loading the repository from: empty, json or dir.")
	fs.StringVar(&o.Path, prefix+"Path", o.Path, "The Path to the schema data.")
}

// Validate validates in-memory schema options.
func (o *InMemorySchemaOptions) Validate() []error {
	if o.Type != SchemaTypeEmpty && o.Type != SchemaTypeJSON && o.Type != SchemaTypeDir {
		return []error{fmt.Errorf("invalid repository type: %s. valid values are %s, %s or %s", o.Type, SchemaTypeEmpty, SchemaTypeJSON, SchemaTypeDir)}
	}

	if o.Type == SchemaTypeJSON || o.Type == SchemaTypeDir {
		if o.Path == "" {
			return []error{fmt.Errorf("path is required when Type is set to %s", o.Type)}
		}
	}

	return nil
}

// SchemaOptions contains schema repository configuration.
type SchemaOptions struct {
	Repository string                 `mapstructure:"repository"`
	InMemory   *InMemorySchemaOptions `mapstructure:"in-memory"`
}

// NewSchemaOptions returns schema options with defaults.
func NewSchemaOptions() *SchemaOptions {
	return &SchemaOptions{
		Repository: SchemaRepositoryInMemory,
		InMemory:   NewInMemorySchemaOptions(),
	}
}

// AddFlags adds schema flags to the flag set.
func (o *SchemaOptions) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Repository, prefix+"schemas", o.Repository, "The schema repository to use.")
	o.InMemory.AddFlags(fs, prefix+"in-memory")
}

// Validate validates schema options.
func (o *SchemaOptions) Validate() []error {
	var errs []error
	if o.Repository != SchemaRepositoryInMemory {
		errs = append(errs, fmt.Errorf("repository option must be set to %s", SchemaRepositoryInMemory))
	}

	switch o.Repository {
	case SchemaRepositoryInMemory:
		errs = append(errs, o.InMemory.Validate()...)
	}

	return errs
}
