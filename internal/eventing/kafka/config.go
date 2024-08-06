package kafka

import (
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Config struct {
	*Options

	// this can be set manually for testing
	KafkaConfig *kafka.ConfigMap
}

type completedConfig struct {
	DefaultTopic string
	KafkaConfig  *kafka.ConfigMap
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	return &Config{
		Options: o,
	}
}

func (c *Config) Complete() (CompletedConfig, error) {
	var config *kafka.ConfigMap

	if c.KafkaConfig != nil {
		config = c.KafkaConfig
	} else {
		// the debug key doesn't allow an empty value
		if c.Debug != "" {
			if err := config.SetKey("debug", c.Debug); err != nil {
				return CompletedConfig{}, err
			}
		}

		if err := config.SetKey("builtin.features", c.BuiltInFeatures); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("client.id", c.ClientId); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("metadata.broker.list", c.MetadataBrokerList); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("bootstrap.servers", c.BootstrapServers); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("message.max.bytes", c.MessageMaxBytes); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("message.copy.max.bytes", c.MessageCopyMaxBytes); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("receive.message.max.bytes", c.ReceiveMessageMaxBytes); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("max.in.flight.requests.per.connection", c.MaxInFlightRequestsPerConnection); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("max.in.flight", c.MaxInFlight); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("topic.metadata.refresh.interval.ms", c.TopicMetadataRefreshIntervalMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("topic.metadata.refresh.fast.interval.ms", c.TopicMetadataRefreshFastIntervalMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("topic.metadata.refresh.sparse", c.TopicMetadataRefreshSparse); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("metadata.max.age.ms", c.MetadataMaxAgeMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("topic.metadata.propagation.max.ms", c.TopicMetadataPropagationMaxMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("topic.blacklist", c.TopicBlacklist); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("socket.timeout.ms", c.SocketTimeoutMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("socket.send.buffer.bytes", c.SocketSendBufferBytes); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("socket.receive.buffer.bytes", c.SocketReceiveBufferBytes); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("socket.keepalive.enable", c.SocketKeepAliveEnable); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("socket.nagle.disable", c.SocketNagleDisable); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("socket.max.fails", c.SocketMaxFails); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("broker.address.ttl", c.BrokerAddressTtl); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("broker.address.family", c.BrokerAddressFamily); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("socket.connection.setup.timeout.ms", c.SocketConnectionSetupTimeoutMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("connections.max.idle.ms", c.ConnectionsMaxIdleMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("reconnect.backoff.ms", c.ReconnectBackoffMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("reconnect.backoff.max.ms", c.ReconnectBackoffMaxMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("statistics.interval.ms", c.StatisticsIntervalMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("enabled_events", c.EnabledEvents); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("log_level", c.LogLevel); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("log.queue", c.LogQueue); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("log.thread.name", c.LogThreadName); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("enable.random.seed", c.EnableRandomSeed); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("log.connection.close", c.LogConnectionClose); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("internal.termination.signal", c.InternalTerminationSignal); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("api.version.request", c.ApiVersionRequest); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("api.version.request.timeout.ms", c.ApiVersionRequestTimeoutMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("api.version.fallback.ms", c.ApiVersionVersionFallbackMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("broker.version.fallback", c.BrokerVersionFallback); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("allow.auto.create.topics", c.AllowAutoCreateTopics); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("security.protocol", c.SecurityProtocol); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.cipher.suites", c.SslCipherSuites); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.curves.list", c.SslCurvesList); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.sigalgs.list", c.SslSigAlgsList); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.key.location", c.SslKeyLocation); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.key.password", c.SslKeyPassword); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.key.pem", c.SslKeyPem); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.certificate.location", c.SslCertificateLocation); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.certificate.pem", c.SslCertificatePem); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.ca.location", c.SslCaLocation); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.ca.pem", c.SslCaPem); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.crl.location", c.SslCrlLocation); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.keystore.location", c.SslKeystoreLocation); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.keystore.password", c.SslKeystorePassword); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.providers", c.SslProviders); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.engine.id", c.SslEngineId); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("enable.ssl.certificate.verification", c.EnableSslCertificateVerification); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("ssl.endpoint.identification.algorithm", c.SslEndpointIdentificationAlgorithm); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.mechanism", c.SaslMechanism); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.kerberos.service.name", c.SaslKerberosServiceName); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.kerberos.principal", c.SaslKerberosPrincipal); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.kerberos.kinit.cmd", c.SaslKerberosKinitCmd); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.kerberos.keytab", c.SaslKerberosKeytab); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.kerberos.min.time.before.relogin", c.SaslKerberosMinTimeBeforeRelogin); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.username", c.SaslUsername); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.password", c.SaslPassword); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.oauthbearer.config", c.SaslOauthBearerConfig); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("enable.sasl.oauthbearer.unsecure.jwt", c.EnableSaslOauthBearerUnsecureJwt); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.oauthbearer.method", c.SaslOauthBearerMethod); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.oauthbearer.client.id", c.SaslOauthBearerClientId); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.oauthbearer.client.secret", c.SaslOauthBearerClientSecret); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.oauthbearer.scope", c.SaslOauthBearerScope); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.oauthbearer.extensions", c.SaslOauthBearerExtensions); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("sasl.oauthbearer.token.endpoint.url", c.SaslOauthBearerTokenEndpointUrl); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("plugin.library.paths", c.PluginLibraryPaths); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("client.dns.lookup", c.ClientDnsLookup); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("enable.metrics.push", c.EnableMetricsPush); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("client.rack", c.ClientRack); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("retry.backoff.ms", c.RetryBackoffMs); err != nil {
			return CompletedConfig{}, err
		}
		if err := config.SetKey("retry.backoff.max.ms", c.RetryBackoffMaxMs); err != nil {
			return CompletedConfig{}, err
		}
	}

	return CompletedConfig{&completedConfig{
		DefaultTopic: c.DefaultTopic,
		KafkaConfig:  config,
	}}, nil
}
