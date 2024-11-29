package kafka

import (
	"github.com/spf13/pflag"
)

type Options struct {
	DefaultTopic    string `mapstructure:"default-topic"`
	BuiltInFeatures string `mapstructure:"builtin-features"`
	ClientId        string `mapstructure:"client-id"`
	//MetadataBrokerList                 string `mapstructure:"metadata-broker-list"`
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
	// SaslMechanisms                     string `mapstructure:"sasl-mechanisms"`
	SaslMechanism                    string `mapstructure:"sasl-mechanism"`
	SaslKerberosServiceName          string `mapstructure:"sasl-kerberos-service-name"`
	SaslKerberosPrincipal            string `mapstructure:"sasl-kerberos-principal"`
	SaslKerberosKinitCmd             string `mapstructure:"sasl-kerberos-kinit-cmd"`
	SaslKerberosKeytab               string `mapstructure:"sasl-kerberos-keytab"`
	SaslKerberosMinTimeBeforeRelogin int    `mapstructure:"sasl-kerberos-min-time-before-relogin"`
	SaslUsername                     string `mapstructure:"sasl-username"`
	SaslPassword                     string `mapstructure:"sasl-password"`
	SaslOauthBearerConfig            string `mapstructure:"sasl-oauthbearer-config"`
	EnableSaslOauthBearerUnsecureJwt bool   `mapstructure:"enable-sasl-oauthbearer-unsecure-jwt"`
	SaslOauthBearerMethod            string `mapstructure:"sasl-oauthbearer-method"`
	SaslOauthBearerClientId          string `mapstructure:"sasl-oauthbearer-client-id"`
	SaslOauthBearerClientSecret      string `mapstructure:"sasl-oauthbearer-client-secret"`
	SaslOauthBearerScope             string `mapstructure:"sasl-oauthbearer-scope"`
	SaslOauthBearerExtensions        string `mapstructure:"sasl-oauthbearer-extensions"`
	SaslOauthBearerTokenEndpointUrl  string `mapstructure:"sasl-oauthbearer-token-endpoint-url"`
	PluginLibraryPaths               string `mapstructure:"plugin-library-paths"`
	ClientDnsLookup                  string `mapstructure:"client-dns-lookup"`
	EnableMetricsPush                bool   `mapstructure:"enable-metrics-push"`
	ClientRack                       string `mapstructure:"client-rack"`
	RetryBackoffMs                   int    `mapstructure:"retry-backoff-ms"`
	RetryBackoffMaxMs                int    `mapstructure:"retry-backoff-max-ms"`
}

func NewOptions() *Options {
	return &Options{
		DefaultTopic:    "kessel-inventory",
		BuiltInFeatures: "gzip, snappy, ssl, sasl, regex, lz4, sasl_plain, sasl_scram, plugins, zstd, sasl_oauthbearer, http, oidc",
		ClientId:        "rdkafka",
		//MetadataBrokerList:                 "",
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
		// SaslMechanisms:                     "GSSAPI",
		SaslMechanism:                    "GSSAPI",
		SaslKerberosServiceName:          "kafka",
		SaslKerberosPrincipal:            "kafkaclient",
		SaslKerberosKinitCmd:             "kinit -R -t \"%{sasl.kerberos.keytab}\" -k %{sasl.kerberos.principal} || kinit -t \"%{sasl.kerberos.keytab}\" -k %{sasl.kerberos.principal}",
		SaslKerberosKeytab:               "",
		SaslKerberosMinTimeBeforeRelogin: 60000,
		SaslUsername:                     "",
		SaslPassword:                     "",
		SaslOauthBearerConfig:            "",
		EnableSaslOauthBearerUnsecureJwt: false,
		SaslOauthBearerMethod:            "default",
		SaslOauthBearerClientId:          "",
		SaslOauthBearerClientSecret:      "",
		SaslOauthBearerScope:             "",
		SaslOauthBearerExtensions:        "",
		SaslOauthBearerTokenEndpointUrl:  "",
		PluginLibraryPaths:               "",
		ClientDnsLookup:                  "use_all_dns_ips",
		EnableMetricsPush:                true,
		ClientRack:                       "",
		RetryBackoffMs:                   100,
		RetryBackoffMaxMs:                1000,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.DefaultTopic, prefix+"default-topic", o.DefaultTopic, "The topic to use.")

	fs.StringVar(&o.BuiltInFeatures, prefix+"builtin-features", o.BuiltInFeatures, "Indicates the builtin features for this build of librdkafka. An application can either query this value or attempt to set it with its list of required features to check for library support. \n*Type: CSV flags*")
	fs.StringVar(&o.ClientId, prefix+"client-id", o.BuiltInFeatures, "Client identifier. \n*Type: string*")
	//fs.StringVar(&o.MetadataBrokerList, prefix+"metadata-broker-list", o.MetadataBrokerList, "Initial list of brokers as a CSV list of broker host or host:port. The application may also use `rd_kafka_brokers_add()` to add brokers during runtime. \n*Type: string*")
	fs.StringVar(&o.BootstrapServers, prefix+"bootstrap-servers", o.BootstrapServers, "Alias for `metadata.broker.list`: Initial list of brokers as a CSV list of broker host or host:port. The application may also use `rd_kafka_brokers_add()` to add brokers during runtime. \n*Type: string*")
	fs.IntVar(&o.MessageMaxBytes, prefix+"message-max-bytes", o.MessageMaxBytes, "Maximum Kafka protocol request message size. Due to differing framing overhead between protocol versions the producer is unable to reliably enforce a strict max message limit at produce time and may exceed the maximum size by one message in protocol ProduceRequests, the broker will enforce the the topic's `max.message.bytes` limit (see Apache Kafka documentation). \n*Type: integer*")
	fs.IntVar(&o.MessageCopyMaxBytes, prefix+"message-copy-max-bytes", o.MessageCopyMaxBytes, "Maximum size for message to be copied to buffer. Messages larger than this will be passed by reference (zero-copy) at the expense of larger iovecs. \n*Type: integer*")
	fs.IntVar(&o.ReceiveMessageMaxBytes, prefix+"receive-message-max-bytes", o.ReceiveMessageMaxBytes, "Maximum Kafka protocol response message size. This serves as a safety precaution to avoid memory exhaustion in case of protocol hickups. This value must be at least `fetch.max.bytes`  + 512 to allow for protocol overhead; the value is adjusted automatically unless the configuration property is explicitly set. \n*Type: integer*")
	fs.IntVar(&o.MaxInFlightRequestsPerConnection, prefix+"max-in-flight-requests-per-connection", o.MaxInFlightRequestsPerConnection, "Maximum number of in-flight requests per broker connection. This is a generic property applied to all broker communication, however it is primarily relevant to produce requests. In particular, note that other mechanisms limit the number of outstanding consumer fetch request per broker to one. \n*Type: integer*")
	fs.IntVar(&o.MaxInFlight, prefix+"max-in-flight", o.MaxInFlight, "Alias for `max.in.flight.requests.per.connection`: Maximum number of in-flight requests per broker connection. This is a generic property applied to all broker communication, however it is primarily relevant to produce requests. In particular, note that other mechanisms limit the number of outstanding consumer fetch request per broker to one. \n*Type: integer*")
	fs.IntVar(&o.TopicMetadataRefreshIntervalMs, prefix+"topic-metadata-refresh-interval-ms", o.TopicMetadataRefreshIntervalMs, "Period of time in milliseconds at which topic and broker metadata is refreshed in order to proactively discover any new brokers, topics, partitions or partition leader changes. Use -1 to disable the intervalled refresh (not recommended). If there are no locally referenced topics (no topic objects created, no messages produced, no subscription or no assignment) then only the broker list will be refreshed every interval but no more often than every 10s. \n*Type: integer*")
	fs.IntVar(&o.MetadataMaxAgeMs, prefix+"metadata-max-age-ms", o.MetadataMaxAgeMs, "Metadata cache max age. Defaults to topic.metadata.refresh.interval.ms * 3 \n*Type: integer*")
	fs.IntVar(&o.TopicMetadataRefreshFastIntervalMs, prefix+"topic-metadata-refresh-fast-interval-ms", o.TopicMetadataRefreshFastIntervalMs, "When a topic loses its leader a new metadata request will be enqueued immediately and then with this initial interval, exponentially increasing upto `retry.backoff.max.ms`, until the topic metadata has been refreshed. If not set explicitly, it will be defaulted to `retry.backoff.ms`. This is used to recover quickly from transitioning leader brokers. \n*Type: integer*")
	fs.BoolVar(&o.TopicMetadataRefreshSparse, prefix+"topic-metadata-refresh-sparse", o.TopicMetadataRefreshSparse, "Sparse metadata requests (consumes less network bandwidth) \n*Type: boolean*")
	fs.IntVar(&o.TopicMetadataPropagationMaxMs, prefix+"topic-metadata-propagation-max-ms", o.ApiVersionRequestTimeoutMs, "Apache Kafka topic creation is asynchronous and it takes some time for a new topic to propagate throughout the cluster to all brokers. If a client requests topic metadata after manual topic creation but before the topic has been fully propagated to the broker the client is requesting metadata from, the topic will seem to be non-existent and the client will mark the topic as such, failing queued produced messages with `ERR__UNKNOWN_TOPIC`. This setting delays marking a topic as non-existent until the configured propagation max time has passed. The maximum propagation time is calculated from the time the topic is first referenced in the client, e.g., on produce(). \n*Type: integer*")
	fs.StringVar(&o.TopicBlacklist, prefix+"topic-blacklist", o.TopicBlacklist, "Topic blacklist, a comma-separated list of regular expressions for matching topic names that should be ignored in broker metadata information as if the topics did not exist. \n*Type: pattern list*")
	fs.StringVar(&o.Debug, prefix+"debug", o.Debug, "A comma-separated list of debug contexts to enable. Detailed Producer debugging: broker,topic,msg. Consumer: consumer,cgrp,topic,fetch \n*Type: CSV flags*")
	fs.IntVar(&o.SocketTimeoutMs, prefix+"socket-timeout-ms", o.SocketTimeoutMs, "Default timeout for network requests. Producer: ProduceRequests will use the lesser value of `socket.timeout.ms` and remaining `message.timeout.ms` for the first message in the batch. Consumer: FetchRequests will use `fetch.wait.max.ms` + `socket.timeout.ms`. Admin: Admin requests will use `socket.timeout.ms` or explicitly set `rd_kafka_AdminOptions_set_operation_timeout()` value. \n*Type: integer*")
	fs.IntVar(&o.SocketSendBufferBytes, prefix+"socket-send-buffer-bytes", o.SocketConnectionSetupTimeoutMs, "Broker socket send buffer size. System default is used if 0. \n*Type: integer*")
	fs.IntVar(&o.SocketReceiveBufferBytes, prefix+"socket-receive-buffer-bytes", o.SocketConnectionSetupTimeoutMs, "Broker socket receive buffer size. System default is used if 0. \n*Type: integer*")
	fs.BoolVar(&o.SocketKeepAliveEnable, prefix+"socket-keepalive-enable", o.SocketKeepAliveEnable, "Enable TCP keep-alives (SO_KEEPALIVE) on broker sockets \n*Type: boolean*")
	fs.BoolVar(&o.SocketNagleDisable, prefix+"socket-nagle-disable", o.SocketKeepAliveEnable, "Disable the Nagle algorithm (TCP_NODELAY) on broker sockets. \n*Type: boolean*")
	fs.IntVar(&o.SocketMaxFails, prefix+"socket-max-fails", o.SocketMaxFails, "Disconnect from broker when this number of send failures (e.g., timed out requests) is reached. Disable with 0. WARNING: It is highly recommended to leave this setting at its default value of 1 to avoid the client and broker to become desynchronized in case of request timeouts. NOTE: The connection is automatically re-established. \n*Type: integer*")
	fs.IntVar(&o.BrokerAddressTtl, prefix+"broker-address-ttl", o.BrokerAddressTtl, "How long to cache the broker address resolving results (milliseconds). \n*Type: integer*")
	fs.StringVar(&o.BrokerAddressFamily, prefix+"broker-address-family", o.BrokerAddressFamily, "Allowed broker IP address families: any, v4, v6 \n*Type: enum value*")
	fs.IntVar(&o.SocketConnectionSetupTimeoutMs, prefix+"socket-connection-setup-timeout-ms", o.SocketConnectionSetupTimeoutMs, "Maximum time allowed for broker connection setup (TCP connection setup as well SSL and SASL handshake). If the connection to the broker is not fully functional after this the connection will be closed and retried. \n*Type: integer*")
	fs.IntVar(&o.ConnectionsMaxIdleMs, prefix+"connections-max-idle-ms", o.ConnectionsMaxIdleMs, "Close broker connections after the specified time of inactivity. Disable with 0. If this property is left at its default value some heuristics are performed to determine a suitable default value, this is currently limited to identifying brokers on Azure (see librdkafka issue #3109 for more info). \n*Type: integer*")
	fs.IntVar(&o.ReconnectBackoffMs, prefix+"reconnect-backoff-ms", o.ReconnectBackoffMs, "The initial time to wait before reconnecting to a broker after the connection has been closed. The time is increased exponentially until `reconnect.backoff.max.ms` is reached. -25% to +50% jitter is applied to each reconnect backoff. A value of 0 disables the backoff and reconnects immediately. \n*Type: integer*")
	fs.IntVar(&o.ReconnectBackoffMaxMs, prefix+"reconnect-backoff-max-ms", o.ReconnectBackoffMaxMs, "The maximum time to wait before reconnecting to a broker after the connection has been closed. \n*Type: integer*")
	fs.IntVar(&o.StatisticsIntervalMs, prefix+"statistics-interval-ms", o.StatisticsIntervalMs, "librdkafka statistics emit interval. The application also needs to register a stats callback using `rd_kafka_conf_set_stats_cb()`. The granularity is 1000ms. A value of 0 disables statistics. \n*Type: integer*")
	fs.IntVar(&o.EnabledEvents, prefix+"enabled-events", o.EnabledEvents, "See `rd_kafka_conf_set_events()` \n*Type: integer*")
	fs.IntVar(&o.LogLevel, prefix+"log-level", o.LogLevel, "Logging level (syslog(3) levels) \n*Type: integer*")
	fs.BoolVar(&o.LogQueue, prefix+"log-queue", o.LogQueue, "Disable spontaneous log_cb from internal librdkafka threads, instead enqueue log messages on queue set with `rd_kafka_set_log_queue()` and serve log callbacks or events through the standard poll APIs. **NOTE**: Log messages will linger in a temporary queue until the log queue has been set. \n*Type: boolean*")
	fs.BoolVar(&o.LogThreadName, prefix+"log-thread-name", o.LogThreadName, "Print internal thread name in log messages (useful for debugging librdkafka internals) \n*Type: boolean*")
	fs.BoolVar(&o.EnableRandomSeed, prefix+"enabled-random-seed", o.EnableRandomSeed, "If enabled librdkafka will initialize the PRNG with srand(current_time.milliseconds) on the first invocation of rd_kafka_new() (required only if rand_r() is not available on your platform). If disabled the application must call srand() prior to calling rd_kafka_new(). \n*Type: boolean*")
	fs.BoolVar(&o.LogConnectionClose, prefix+"log-connection-close", o.LogConnectionClose, "Log broker disconnects. It might be useful to turn this off when interacting with 0.9 brokers with an aggressive `connections.max.idle.ms` value. \n*Type: boolean*")
	fs.IntVar(&o.InternalTerminationSignal, prefix+"internal-termination-signal", o.InternalTerminationSignal, "Signal that librdkafka will use to quickly terminate on rd_kafka_destroy(). If this signal is not set then there will be a delay before rd_kafka_wait_destroyed() returns true as internal threads are timing out their system calls. If this signal is set however the delay will be minimal. The application should mask this signal as an internal signal handler is installed. \n*Type: integer*")
	fs.BoolVar(&o.ApiVersionRequest, prefix+"api-version-request", o.ApiVersionRequest, "Request broker's supported API versions to adjust functionality to available protocol features. If set to false, or the ApiVersionRequest fails, the fallback version `broker.version.fallback` will be used. **NOTE**: Depends on broker version >=0.10.0. If the request is not supported by (an older) broker the `broker.version.fallback` fallback is used. \n*Type: boolean*")
	fs.IntVar(&o.ApiVersionRequestTimeoutMs, prefix+"api-version-request-timeout-ms", o.ApiVersionRequestTimeoutMs, "Timeout for broker API version requests. \n*Type: integer*")
	fs.IntVar(&o.ApiVersionVersionFallbackMs, prefix+"api-version-version-fallback-ms", o.ApiVersionVersionFallbackMs, "Dictates how long the `broker.version.fallback` fallback is used in the case the ApiVersionRequest fails. **NOTE**: The ApiVersionRequest is only issued when a new connection to the broker is made (such as after an upgrade). \n*Type: integer*")
	fs.StringVar(&o.BrokerVersionFallback, prefix+"broker-version-fallback", o.BrokerVersionFallback, "Older broker versions (before 0.10.0) provide no way for a client to query for supported protocol features (ApiVersionRequest, see `api.version.request`) making it impossible for the client to know what features it may use. As a workaround a user may set this property to the expected broker version and the client will automatically adjust its feature set accordingly if the ApiVersionRequest fails (or is disabled). The fallback broker version will be used for `api.version.fallback.ms`. Valid values are: 0.9.0, 0.8.2, 0.8.1, 0.8.0. Any other value >= 0.10, such as 0.10.2.1, enables ApiVersionRequests. \n*Type: string*")
	fs.BoolVar(&o.AllowAutoCreateTopics, prefix+"allow-auto-create-topics", o.AllowAutoCreateTopics, "Allow automatic topic creation on the broker when subscribing to or assigning non-existent topics. The broker must also be configured with `auto.create.topics.enable=true` for this configuration to take effect. Note: the default value (true) for the producer is different from the default value (false) for the consumer. Further, the consumer default value is different from the Java consumer (true), and this property is not supported by the Java producer. Requires broker version >= 0.11.0.0, for older broker versions only the broker configuration applies. \n*Type: boolean*")
	fs.StringVar(&o.SecurityProtocol, prefix+"security-protocol", o.SecurityProtocol, "Protocol used to communicate with brokers. \n*Type: enum value*")
	fs.StringVar(&o.SslCipherSuites, prefix+"ssl-cipher-suites", o.SslCipherSuites, "A cipher suite is a named combination of authentication, encryption, MAC and key exchange algorithm used to negotiate the security settings for a network connection using TLS or SSL network protocol. See manual page for `ciphers(1)` and `SSL_CTX_set_cipher_list(3). \n*Type: string*")
	fs.StringVar(&o.SslCurvesList, prefix+"ssl-curves-list", o.SslCurvesList, "The supported-curves extension in the TLS ClientHello message specifies the curves (standard/named, or 'explicit' GF(2^k) or GF(p)) the client is willing to have the server use. See manual page for `SSL_CTX_set1_curves_list(3)`. OpenSSL >= 1.0.2 required. \n*Type: string*")
	fs.StringVar(&o.SslSigAlgsList, prefix+"ssl-sigalgs-list", o.SslSigAlgsList, "The client uses the TLS ClientHello signature_algorithms extension to indicate to the server which signature/hash algorithm pairs may be used in digital signatures. See manual page for `SSL_CTX_set1_sigalgs_list(3)`. OpenSSL >= 1.0.2 required. \n*Type: string*")
	fs.StringVar(&o.SslKeyLocation, prefix+"ssl-key-location", o.SslKeyLocation, "Path to client's private key (PEM) used for authentication. \n*Type: string*")
	fs.StringVar(&o.SslKeyPassword, prefix+"ssl-key-password", o.SslKeyPassword, "Private key passphrase (for use with `ssl.key.location` and `set_ssl_cert()`) \n*Type: string*")
	fs.StringVar(&o.SslKeyPem, prefix+"ssl-key-pem", o.SslKeyPem, "Client's private key string (PEM format) used for authentication. \n*Type: string*")
	fs.StringVar(&o.SslCertificateLocation, prefix+"ssl-certificate-location", o.SslCertificateLocation, "Path to client's public key (PEM) used for authentication. \n*Type: string*")
	fs.StringVar(&o.SslCertificatePem, prefix+"ssl-certificate-pem", o.SslCertificatePem, "Client's public key string (PEM format) used for authentication. \n*Type: string*")
	fs.StringVar(&o.SslCaLocation, prefix+"ssl-ca-location", o.SslCaLocation, "File or directory path to CA certificate(s) for verifying the broker's key. Defaults: On Windows the system's CA certificates are automatically looked up in the Windows Root certificate store. On Mac OSX this configuration defaults to `probe`. It is recommended to install openssl using Homebrew, to provide CA certificates. On Linux install the distribution's ca-certificates package. If OpenSSL is statically linked or `ssl.ca.location` is set to `probe` a list of standard paths will be probed and the first one found will be used as the default CA certificate location path. If OpenSSL is dynamically linked the OpenSSL library's default path will be used (see `OPENSSLDIR` in `openssl version -a`). \n*Type: string*")
	fs.StringVar(&o.SslCaPem, prefix+"ssl-ca-pem", o.SslCaPem, "CA certificate string (PEM format) for verifying the broker's key. \n*Type: string*")
	fs.StringVar(&o.SslCrlLocation, prefix+"ssl-crl-location", o.SslCrlLocation, "Path to CRL for verifying broker's certificate validity. \n*Type: string*")
	fs.StringVar(&o.SslKeystoreLocation, prefix+"ssl-keystore-location", o.SslKeystoreLocation, "Path to client's keystore (PKCS#12) used for authentication. \n*Type: string*")
	fs.StringVar(&o.SslKeystorePassword, prefix+"ssl-keystore-password", o.SslKeystorePassword, "Client's keystore (PKCS#12) password. \n*Type: string*")
	fs.StringVar(&o.SslProviders, prefix+"ssl-providers", o.SslProviders, "Comma-separated list of OpenSSL 3.0.x implementation providers. E.g., default,legacy. \n*Type: string*")
	fs.StringVar(&o.SslEngineId, prefix+"ssl-engine-id", o.SslEngineId, "OpenSSL engine id is the name used for loading engine. \n*Type: string*")
	fs.BoolVar(&o.EnableSslCertificateVerification, prefix+"enable-ssl-certificate-verification", o.EnableSslCertificateVerification, "Enable OpenSSL's builtin broker (server) certificate verification. This verification can be extended by the application by implementing a certificate_verify_cb. \n*Type: boolean*")
	fs.StringVar(&o.SslEndpointIdentificationAlgorithm, prefix+"ssl-endpoint-identification-algorithm", o.SslEndpointIdentificationAlgorithm, "Endpoint identification algorithm to validate broker hostname using broker certificate. https - Server (broker) hostname verification as specified in RFC2818. none - No endpoint verification. OpenSSL >= 1.0.2 required. \n*Type: enum value*")
	//	fs.StringVar(&o.SaslMechanisms, prefix+"sasl-mechanisms", o.SaslMechanisms, "SASL mechanism to use for authentication. Supported: GSSAPI, PLAIN, SCRAM-SHA-256, SCRAM-SHA-512, OAUTHBEARER. **NOTE**: Despite the name only one mechanism must be configured. \n*Type: string*")
	fs.StringVar(&o.SaslMechanism, prefix+"sasl-mechanism", o.SaslMechanism, "Alias for `sasl.mechanisms`: SASL mechanism to use for authentication. Supported: GSSAPI, PLAIN, SCRAM-SHA-256, SCRAM-SHA-512, OAUTHBEARER. **NOTE**: Despite the name only one mechanism must be configured. \n*Type: string*")
	fs.StringVar(&o.SaslKerberosServiceName, prefix+"sasl-kerberos-service-name", o.SaslKerberosServiceName, "Kerberos principal name that Kafka runs as, not including /hostname@REALM \n*Type: string*")
	fs.StringVar(&o.SaslKerberosPrincipal, prefix+"sasl-kerberos-principal", o.SaslKerberosPrincipal, "This client's Kerberos principal name. (Not supported on Windows, will use the logon user's principal). \n*Type: string*")
	fs.StringVar(&o.SaslKerberosKinitCmd, prefix+"sasl-kerberos-kinit-cmd", o.SaslKerberosKinitCmd, "Shell command to refresh or acquire the client's Kerberos ticket. This command is executed on client creation and every sasl.kerberos.min.time.before.relogin (0=disable). %{config.prop.name} is replaced by corresponding config object value. \n*Type: string*")
	fs.StringVar(&o.SaslKerberosKeytab, prefix+"sasl-kerberos-keytab", o.SaslKerberosKeytab, "Path to Kerberos keytab file. This configuration property is only used as a variable in `sasl.kerberos.kinit.cmd` as ` ... -t \"%{sasl.kerberos.keytab}\"`. \n*Type: string*")
	fs.IntVar(&o.SaslKerberosMinTimeBeforeRelogin, prefix+"sasl-kerberos-min-time-before-relogin", o.SaslKerberosMinTimeBeforeRelogin, "Minimum time in milliseconds between key refresh attempts. Disable automatic key refresh by setting this property to 0. \n*Type: integer*")
	fs.StringVar(&o.SaslUsername, prefix+"sasl-username", o.SaslUsername, "SASL username for use with the PLAIN and SASL-SCRAM-.. mechanisms \n*Type: string*")
	fs.StringVar(&o.SaslPassword, prefix+"sasl-password", o.SaslPassword, "SASL password for use with the PLAIN and SASL-SCRAM-.. mechanism \n*Type: string*")
	fs.StringVar(&o.SaslOauthBearerConfig, prefix+"sasl-oauthbearer-config", o.SaslOauthBearerConfig, "SASL/OAUTHBEARER configuration. The format is implementation-dependent and must be parsed accordingly. The default unsecured token implementation (see https://tools.ietf.org/html/rfc7515#appendix-A.5) recognizes space-separated name=value pairs with valid names including principalClaimName, principal, scopeClaimName, scope, and lifeSeconds. The default value for principalClaimName is \"sub\", the default value for scopeClaimName is \"scope\", and the default value for lifeSeconds is 3600. The scope value is CSV format with the default value being no/empty scope. For example: `principalClaimName=azp principal=admin scopeClaimName=roles scope=role1,role2 lifeSeconds=600`. In addition, SASL extensions can be communicated to the broker via `extension_NAME=value`. For example: `principal=admin extension_traceId=123` \n*Type: string*")
	fs.BoolVar(&o.EnableSaslOauthBearerUnsecureJwt, prefix+"enable-sasl-oauthbearer-unsecure-jwt", o.EnableSaslOauthBearerUnsecureJwt, "Enable the builtin unsecure JWT OAUTHBEARER token handler if no oauthbearer_refresh_cb has been set. This builtin handler should only be used for development or testing, and not in production. \n*Type: boolean*")
	fs.StringVar(&o.SaslOauthBearerMethod, prefix+"sasl-oauthbearer-method", o.SaslOauthBearerMethod, "Set to \"default\" or \"oidc\" to control which login method to be used. If set to \"oidc\", the following properties must also be be specified: `sasl.oauthbearer.client.id`, `sasl.oauthbearer.client.secret`, and `sasl.oauthbearer.token.endpoint.url`. \n*Type: enum value*")
	fs.StringVar(&o.SaslOauthBearerClientId, prefix+"sasl-oauthbearer-client-id", o.SaslOauthBearerClientId, "Public identifier for the application. Must be unique across all clients that the authorization server handles. Only used when `sasl.oauthbearer.method` is set to \"oidc\". \n*Type: string*")
	fs.StringVar(&o.SaslOauthBearerClientSecret, prefix+"sasl-oauthbearer-client-secret", o.SaslOauthBearerClientSecret, "Client secret only known to the application and the authorization server. This should be a sufficiently random string that is not guessable. Only used when `sasl.oauthbearer.method` is set to \"oidc\". \n*Type: string*")
	fs.StringVar(&o.SaslOauthBearerScope, prefix+"sasl-oauthbearer-scope", o.SaslOauthBearerScope, "Client use this to specify the scope of the access request to the broker. Only used when `sasl.oauthbearer.method` is set to \"oidc\". \n*Type: string*")
	fs.StringVar(&o.SaslOauthBearerExtensions, prefix+"sasl-oauthbearer-extensions", o.SaslOauthBearerExtensions, "Allow additional information to be provided to the broker. Comma-separated list of key=value pairs. E.g., \"supportFeatureX=true,organizationId=sales-emea\".Only used when `sasl.oauthbearer.method` is set to \"oidc\". \n*Type: string*")
	fs.StringVar(&o.SaslOauthBearerTokenEndpointUrl, prefix+"sasl-oauthbearer-token-endpoint-url", o.SaslOauthBearerTokenEndpointUrl, "OAuth/OIDC issuer token endpoint HTTP(S) URI used to retrieve token. Only used when `sasl.oauthbearer.method` is set to \"oidc\". \n*Type: string*")
	fs.StringVar(&o.PluginLibraryPaths, prefix+"plugin-library-paths", o.PluginLibraryPaths, "List of plugin libraries to load (; separated). The library search path is platform dependent (see dlopen(3) for Unix and LoadLibrary() for Windows). If no filename extension is specified the platform-specific extension (such as .dll or .so) will be appended automatically. \n*Type: string*")
	fs.StringVar(&o.ClientDnsLookup, prefix+"client-dns-lookup", o.ClientDnsLookup, "Controls how the client uses DNS lookups. By default, when the lookup returns multiple IP addresses for a hostname, they will all be attempted for connection before the connection is considered failed. This applies to both bootstrap and advertised servers. If the value is set to `resolve_canonical_bootstrap_servers_only`, each entry will be resolved and expanded into a list of canonical names. **WARNING**: `resolve_canonical_bootstrap_servers_only` must only be used with `GSSAPI` (Kerberos) as `sasl.mechanism`, as it's the only purpose of this configuration value. **NOTE**: Default here is different from the Java client's default behavior, which connects only to the first IP address returned for a hostname.  \n*Type: enum value*")
	fs.BoolVar(&o.EnableMetricsPush, prefix+"enable-metrics-push", o.EnableMetricsPush, "Whether to enable pushing of client metrics to the cluster, if the cluster has a client metrics subscription which matches this client \n*Type: boolean*")
	fs.StringVar(&o.ClientRack, prefix+"client-rack", o.ClientRack, "A rack identifier for this client. This can be any string value which indicates where this client is physically located. It corresponds with the broker config `broker.rack`. \n*Type: string*")
	fs.IntVar(&o.RetryBackoffMs, prefix+"retry-backoff-ms", o.RetryBackoffMs, "The backoff time in milliseconds before retrying a protocol request, this is the first backoff time, and will be backed off exponentially until number of retries is exhausted, and it's capped by retry.backoff.max.ms. \n*Type: integer*")
	fs.IntVar(&o.RetryBackoffMaxMs, prefix+"retry-backoff-max-ms", o.RetryBackoffMaxMs, "The max backoff time in milliseconds before retrying a protocol request, this is the atmost backoff allowed for exponentially backed off requests. \n*Type: integer*")
}

func (o *Options) Validate() []error {
	var errs []error

	return errs
}

func (o *Options) Complete() []error {
	var errs []error

	return errs
}
