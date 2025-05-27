package kafka

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/helpers"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestNewOptions(t *testing.T) {
	test := struct {
		options         *Options
		expectedOptions *Options
	}{
		options: NewOptions(),
		expectedOptions: &Options{
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
		},
	}
	assert.Equal(t, test.expectedOptions, NewOptions())
}

func TestOptions_AddFlags(t *testing.T) {
	test := struct {
		options *Options
	}{
		options: NewOptions(),
	}
	prefix := "eventing.kafka"
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)
	test.options.AddFlags(fs, prefix)

	helpers.AllOptionsHaveFlags(t, prefix, fs, *test.options, nil)
}
