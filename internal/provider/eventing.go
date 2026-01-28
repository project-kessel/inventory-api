package provider

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/eventing/kafka"
	"github.com/project-kessel/inventory-api/internal/eventing/stdout"
)

// NewEventingManager creates a new eventing manager from options.
// The source parameter is used as the Source URI in cloudevents.
func NewEventingManager(opts *EventingOptions, source string, logger *log.Helper) (eventingapi.Manager, error) {
	if errs := opts.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("eventing validation failed: %v", errs)
	}

	switch opts.Eventer {
	case "stdout":
		return stdout.New(logger)
	case "kafka":
		return newKafkaManager(opts.Kafka, source, logger)
	default:
		return nil, fmt.Errorf("unrecognized eventer type: %s", opts.Eventer)
	}
}

// newKafkaManager creates a Kafka eventing manager.
func newKafkaManager(opts *KafkaEventingOptions, source string, logger *log.Helper) (eventingapi.Manager, error) {
	// Convert our options to kafka.Options
	kafkaOpts := &kafka.Options{
		DefaultTopic:                       opts.DefaultTopic,
		BuiltInFeatures:                    opts.BuiltInFeatures,
		ClientId:                           opts.ClientId,
		BootstrapServers:                   opts.BootstrapServers,
		MessageMaxBytes:                    opts.MessageMaxBytes,
		MessageCopyMaxBytes:                opts.MessageCopyMaxBytes,
		ReceiveMessageMaxBytes:             opts.ReceiveMessageMaxBytes,
		MaxInFlightRequestsPerConnection:   opts.MaxInFlightRequestsPerConnection,
		MaxInFlight:                        opts.MaxInFlight,
		TopicMetadataRefreshIntervalMs:     opts.TopicMetadataRefreshIntervalMs,
		MetadataMaxAgeMs:                   opts.MetadataMaxAgeMs,
		TopicMetadataRefreshFastIntervalMs: opts.TopicMetadataRefreshFastIntervalMs,
		TopicMetadataRefreshSparse:         opts.TopicMetadataRefreshSparse,
		TopicMetadataPropagationMaxMs:      opts.TopicMetadataPropagationMaxMs,
		TopicBlacklist:                     opts.TopicBlacklist,
		Debug:                              opts.Debug,
		SocketTimeoutMs:                    opts.SocketTimeoutMs,
		SocketSendBufferBytes:              opts.SocketSendBufferBytes,
		SocketReceiveBufferBytes:           opts.SocketReceiveBufferBytes,
		SocketKeepAliveEnable:              opts.SocketKeepAliveEnable,
		SocketNagleDisable:                 opts.SocketNagleDisable,
		SocketMaxFails:                     opts.SocketMaxFails,
		BrokerAddressTtl:                   opts.BrokerAddressTtl,
		BrokerAddressFamily:                opts.BrokerAddressFamily,
		SocketConnectionSetupTimeoutMs:     opts.SocketConnectionSetupTimeoutMs,
		ConnectionsMaxIdleMs:               opts.ConnectionsMaxIdleMs,
		ReconnectBackoffMs:                 opts.ReconnectBackoffMs,
		ReconnectBackoffMaxMs:              opts.ReconnectBackoffMaxMs,
		StatisticsIntervalMs:               opts.StatisticsIntervalMs,
		EnabledEvents:                      opts.EnabledEvents,
		LogLevel:                           opts.LogLevel,
		LogQueue:                           opts.LogQueue,
		LogThreadName:                      opts.LogThreadName,
		EnableRandomSeed:                   opts.EnableRandomSeed,
		LogConnectionClose:                 opts.LogConnectionClose,
		InternalTerminationSignal:          opts.InternalTerminationSignal,
		ApiVersionRequest:                  opts.ApiVersionRequest,
		ApiVersionRequestTimeoutMs:         opts.ApiVersionRequestTimeoutMs,
		ApiVersionVersionFallbackMs:        opts.ApiVersionVersionFallbackMs,
		BrokerVersionFallback:              opts.BrokerVersionFallback,
		AllowAutoCreateTopics:              opts.AllowAutoCreateTopics,
		SecurityProtocol:                   opts.SecurityProtocol,
		SslCipherSuites:                    opts.SslCipherSuites,
		SslCurvesList:                      opts.SslCurvesList,
		SslSigAlgsList:                     opts.SslSigAlgsList,
		SslKeyLocation:                     opts.SslKeyLocation,
		SslKeyPassword:                     opts.SslKeyPassword,
		SslKeyPem:                          opts.SslKeyPem,
		SslCertificateLocation:             opts.SslCertificateLocation,
		SslCertificatePem:                  opts.SslCertificatePem,
		SslCaLocation:                      opts.SslCaLocation,
		SslCaPem:                           opts.SslCaPem,
		SslCrlLocation:                     opts.SslCrlLocation,
		SslKeystoreLocation:                opts.SslKeystoreLocation,
		SslKeystorePassword:                opts.SslKeystorePassword,
		SslProviders:                       opts.SslProviders,
		SslEngineId:                        opts.SslEngineId,
		EnableSslCertificateVerification:   opts.EnableSslCertificateVerification,
		SslEndpointIdentificationAlgorithm: opts.SslEndpointIdentificationAlgorithm,
		SaslMechanism:                      opts.SaslMechanism,
		SaslKerberosServiceName:            opts.SaslKerberosServiceName,
		SaslKerberosPrincipal:              opts.SaslKerberosPrincipal,
		SaslKerberosKinitCmd:               opts.SaslKerberosKinitCmd,
		SaslKerberosKeytab:                 opts.SaslKerberosKeytab,
		SaslKerberosMinTimeBeforeRelogin:   opts.SaslKerberosMinTimeBeforeRelogin,
		SaslUsername:                       opts.SaslUsername,
		SaslPassword:                       opts.SaslPassword,
		SaslOauthBearerConfig:              opts.SaslOauthBearerConfig,
		EnableSaslOauthBearerUnsecureJwt:   opts.EnableSaslOauthBearerUnsecureJwt,
		SaslOauthBearerMethod:              opts.SaslOauthBearerMethod,
		SaslOauthBearerClientId:            opts.SaslOauthBearerClientId,
		SaslOauthBearerClientSecret:        opts.SaslOauthBearerClientSecret,
		SaslOauthBearerScope:               opts.SaslOauthBearerScope,
		SaslOauthBearerExtensions:          opts.SaslOauthBearerExtensions,
		SaslOauthBearerTokenEndpointUrl:    opts.SaslOauthBearerTokenEndpointUrl,
		PluginLibraryPaths:                 opts.PluginLibraryPaths,
		ClientDnsLookup:                    opts.ClientDnsLookup,
		EnableMetricsPush:                  opts.EnableMetricsPush,
		ClientRack:                         opts.ClientRack,
		RetryBackoffMs:                     opts.RetryBackoffMs,
		RetryBackoffMaxMs:                  opts.RetryBackoffMaxMs,
	}

	cfg := kafka.NewConfig(kafkaOpts)
	completedCfg, err := cfg.Complete()
	if err != nil {
		return nil, fmt.Errorf("failed to complete kafka config: %w", err)
	}

	km, err := kafka.New(completedCfg, source, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka manager: %w", err)
	}
	return km, nil
}
