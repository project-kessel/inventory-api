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
			config.SetKey("debug", c.Debug)
		}

		config = &kafka.ConfigMap{}
		config.SetKey("builtin.features", c.BuiltInFeatures)
		config.SetKey("client.id", c.ClientId)
		config.SetKey("metadata.broker.list", c.MetadataBrokerList)
		config.SetKey("bootstrap.servers", c.BootstrapServers)
		config.SetKey("message.max.bytes", c.MessageMaxBytes)
		config.SetKey("message.copy.max.bytes", c.MessageCopyMaxBytes)
		config.SetKey("receive.message.max.bytes", c.ReceiveMessageMaxBytes)
		config.SetKey("max.in.flight.requests.per.connection", c.MaxInFlightRequestsPerConnection)
		config.SetKey("max.in.flight", c.MaxInFlight)
		config.SetKey("topic.metadata.refresh.interval.ms", c.TopicMetadataRefreshIntervalMs)
		config.SetKey("topic.metadata.refresh.fast.interval.ms", c.TopicMetadataRefreshFastIntervalMs)
		config.SetKey("topic.metadata.refresh.sparse", c.TopicMetadataRefreshSparse)
		config.SetKey("metadata.max.age.ms", c.MetadataMaxAgeMs)
		config.SetKey("topic.metadata.propagation.max.ms", c.TopicMetadataPropagationMaxMs)
		config.SetKey("topic.blacklist", c.TopicBlacklist)
		config.SetKey("socket.timeout.ms", c.SocketTimeoutMs)
		config.SetKey("socket.send.buffer.bytes", c.SocketSendBufferBytes)
		config.SetKey("socket.receive.buffer.bytes", c.SocketReceiveBufferBytes)
		config.SetKey("socket.keepalive.enable", c.SocketKeepAliveEnable)
		config.SetKey("socket.nagle.disable", c.SocketNagleDisable)
		config.SetKey("socket.max.fails", c.SocketMaxFails)
		config.SetKey("broker.address.ttl", c.BrokerAddressTtl)
		config.SetKey("broker.address.family", c.BrokerAddressFamily)
		config.SetKey("socket.connection.setup.timeout.ms", c.SocketConnectionSetupTimeoutMs)
		config.SetKey("connections.max.idle.ms", c.ConnectionsMaxIdleMs)
		config.SetKey("reconnect.backoff.ms", c.ReconnectBackoffMs)
		config.SetKey("reconnect.backoff.max.ms", c.ReconnectBackoffMaxMs)
		config.SetKey("statistics.interval.ms", c.StatisticsIntervalMs)
		config.SetKey("enabled_events", c.EnabledEvents)
		config.SetKey("log_level", c.LogLevel)
		config.SetKey("log.queue", c.LogQueue)
		config.SetKey("log.thread.name", c.LogThreadName)
		config.SetKey("enable.random.seed", c.EnableRandomSeed)
		config.SetKey("log.connection.close", c.LogConnectionClose)
		config.SetKey("internal.termination.signal", c.InternalTerminationSignal)
		config.SetKey("api.version.request", c.ApiVersionRequest)
		config.SetKey("api.version.request.timeout.ms", c.ApiVersionRequestTimeoutMs)
		config.SetKey("api.version.fallback.ms", c.ApiVersionVersionFallbackMs)
		config.SetKey("broker.version.fallback", c.BrokerVersionFallback)
		config.SetKey("allow.auto.create.topics", c.AllowAutoCreateTopics)
		config.SetKey("security.protocol", c.SecurityProtocol)
		config.SetKey("ssl.cipher.suites", c.SslCipherSuites)
		config.SetKey("ssl.curves.list", c.SslCurvesList)
		config.SetKey("ssl.sigalgs.list", c.SslSigAlgsList)
		config.SetKey("ssl.key.location", c.SslKeyLocation)
		config.SetKey("ssl.key.password", c.SslKeyPassword)
		config.SetKey("ssl.key.pem", c.SslKeyPem)
		config.SetKey("ssl.certificate.location", c.SslCertificateLocation)
		config.SetKey("ssl.certificate.pem", c.SslCertificatePem)
		config.SetKey("ssl.ca.location", c.SslCaLocation)
		config.SetKey("ssl.ca.pem", c.SslCaPem)
		config.SetKey("ssl.crl.location", c.SslCrlLocation)
		config.SetKey("ssl.keystore.location", c.SslKeystoreLocation)
		config.SetKey("ssl.keystore.password", c.SslKeystorePassword)
		config.SetKey("ssl.providers", c.SslProviders)
		config.SetKey("ssl.engine.id", c.SslEngineId)
		config.SetKey("enable.ssl.certificate.verification", c.EnableSslCertificateVerification)
		config.SetKey("ssl.endpoint.identification.algorithm", c.SslEndpointIdentificationAlgorithm)
		// config.SetKey("sasl.mechanisms", c.SaslMechanisms)
		config.SetKey("sasl.mechanism", c.SaslMechanism)
		config.SetKey("sasl.kerberos.service.name", c.SaslKerberosServiceName)
		config.SetKey("sasl.kerberos.principal", c.SaslKerberosPrincipal)
		config.SetKey("sasl.kerberos.kinit.cmd", c.SaslKerberosKinitCmd)
		config.SetKey("sasl.kerberos.keytab", c.SaslKerberosKeytab)
		config.SetKey("sasl.kerberos.min.time.before.relogin", c.SaslKerberosMinTimeBeforeRelogin)
		config.SetKey("sasl.username", c.SaslUsername)
		config.SetKey("sasl.password", c.SaslPassword)
		config.SetKey("sasl.oauthbearer.config", c.SaslOauthBearerConfig)
		config.SetKey("enable.sasl.oauthbearer.unsecure.jwt", c.EnableSaslOauthBearerUnsecureJwt)
		config.SetKey("sasl.oauthbearer.method", c.SaslOauthBearerMethod)
		config.SetKey("sasl.oauthbearer.client.id", c.SaslOauthBearerClientId)
		config.SetKey("sasl.oauthbearer.client.secret", c.SaslOauthBearerClientSecret)
		config.SetKey("sasl.oauthbearer.scope", c.SaslOauthBearerScope)
		config.SetKey("sasl.oauthbearer.extensions", c.SaslOauthBearerExtensions)
		config.SetKey("sasl.oauthbearer.token.endpoint.url", c.SaslOauthBearerTokenEndpointUrl)
		config.SetKey("plugin.library.paths", c.PluginLibraryPaths)
		config.SetKey("client.dns.lookup", c.ClientDnsLookup)
		config.SetKey("enable.metrics.push", c.EnableMetricsPush)
		config.SetKey("client.rack", c.ClientRack)
		config.SetKey("retry.backoff.ms", c.RetryBackoffMs)
		config.SetKey("retry.backoff.max.ms", c.RetryBackoffMaxMs)
	}

	return CompletedConfig{&completedConfig{
		DefaultTopic: c.DefaultTopic,
		KafkaConfig:  config,
	}}, nil
}
