package config

import (
	"os"
	"testing"

	. "github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/authz/kessel"
	"github.com/project-kessel/inventory-api/internal/consumer"
	"github.com/project-kessel/inventory-api/internal/consumer/auth"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/project-kessel/inventory-api/internal/storage/postgres"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/stretchr/testify/assert"
)

var testCA = `-----BEGIN CERTIFICATE-----\nFAKE-CA\n-----END CERTIFICATE-----`

func ParseCA(t *testing.T, path string) string {
	file, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("failed to read file %s: %s", path, err)
		return ""
	}
	return string(file)
}

func TestConfigureStorage(t *testing.T) {
	test := struct {
		name      string
		appconfig *clowder.AppConfig
		options   *OptionsConfig
	}{
		name: "ensures DB info is set",
		appconfig: &clowder.AppConfig{
			Database: &clowder.DatabaseConfig{
				Hostname: "postgres",
				Name:     "postgres",
				Port:     5432,
				Username: "db-user",
				Password: "db-password",
				RdsCa:    &testCA,
			},
		},
		options: NewOptionsConfig(),
	}

	t.Run(test.name, func(t *testing.T) {
		err := test.options.ConfigureStorage(test.appconfig)
		assert.Nil(t, err)

		assert.NotNil(t, test.options.Storage.Postgres.SSLRootCert)
		contents := ParseCA(t, test.options.Storage.Postgres.SSLRootCert)
		assert.Equal(t, testCA, contents)

		assert.Equal(t, "postgres", test.options.Storage.Database)
		assert.Equal(t, "postgres", test.options.Storage.Postgres.Host)
		assert.Equal(t, "postgres", test.options.Storage.Postgres.DbName)
		assert.Equal(t, "5432", test.options.Storage.Postgres.Port)
		assert.Equal(t, "db-user", test.options.Storage.Postgres.User)
		assert.Equal(t, "db-password", test.options.Storage.Postgres.Password)
	})

	testNoCA := struct {
		name      string
		appconfig *clowder.AppConfig
		options   *OptionsConfig
	}{
		name: "ensures DB info is set when no CA cert is provided",
		appconfig: &clowder.AppConfig{
			Database: &clowder.DatabaseConfig{
				Hostname: "postgres",
				Name:     "postgres",
				Port:     5432,
				Username: "db-user",
				Password: "db-password",
			},
		},
		options: NewOptionsConfig(),
	}

	t.Run(testNoCA.name, func(t *testing.T) {
		err := testNoCA.options.ConfigureStorage(testNoCA.appconfig)
		assert.Nil(t, err)
		assert.Equal(t, testNoCA.options.Storage.Postgres.SSLRootCert, "")

		assert.Equal(t, "postgres", testNoCA.options.Storage.Database)
		assert.Equal(t, "postgres", testNoCA.options.Storage.Postgres.Host)
		assert.Equal(t, "postgres", testNoCA.options.Storage.Postgres.DbName)
		assert.Equal(t, "5432", testNoCA.options.Storage.Postgres.Port)
		assert.Equal(t, "db-user", testNoCA.options.Storage.Postgres.User)
		assert.Equal(t, "db-password", testNoCA.options.Storage.Postgres.Password)
	})
}

func TestConfigureAuthz(t *testing.T) {
	test := struct {
		name      string
		appconfig *clowder.AppConfig
		options   *OptionsConfig
	}{
		name: "ensures Authz info is set",
		appconfig: &clowder.AppConfig{
			Endpoints: []clowder.DependencyEndpoint{
				clowder.DependencyEndpoint{
					App:      authz.RelationsAPI,
					Hostname: "kessel-relations",
				},
			},
		},
		options: NewOptionsConfig(),
	}

	t.Run(test.name, func(t *testing.T) {
		test.options.ConfigureAuthz(test.appconfig.Endpoints[0])
		assert.Equal(t, "kessel", test.options.Authz.Authz)
		assert.Equal(t, "kessel-relations:9000", test.options.Authz.Kessel.URL)
	})

}

func TestConfigureConsumer(t *testing.T) {
	tests := []struct {
		name        string
		appconfig   *clowder.AppConfig
		options     *OptionsConfig
		authEnabled bool
		expected    []string
	}{
		{
			name: "ensure boostrap server is set properly when only one is provided - no auth settings",
			appconfig: &clowder.AppConfig{
				Kafka: &clowder.KafkaConfig{
					Brokers: []clowder.BrokerConfig{
						{
							Hostname: "test-kafka-server",
							Port:     ToPointer(9092),
						},
					},
				},
			},
			options:     NewOptionsConfig(),
			authEnabled: false,
			expected:    []string{"test-kafka-server:9092"},
		},
		{
			name: "ensure boostrap server is set properly when multiple are provided - no auth settings",
			appconfig: &clowder.AppConfig{
				Kafka: &clowder.KafkaConfig{
					Brokers: []clowder.BrokerConfig{
						{
							Hostname: "test-kafka-server-01",
							Port:     ToPointer(9092),
						},
						{
							Hostname: "test-kafka-server-02",
							Port:     ToPointer(9092),
						},
						{
							Hostname: "test-kafka-server-03",
							Port:     ToPointer(9092),
						},
					},
				},
			},
			options:     NewOptionsConfig(),
			authEnabled: false,
			expected:    []string{"test-kafka-server-01:9092", "test-kafka-server-02:9092", "test-kafka-server-03:9092"},
		},
		{
			name: "ensure sasl settings are configured when present",
			appconfig: &clowder.AppConfig{
				Kafka: &clowder.KafkaConfig{
					Brokers: []clowder.BrokerConfig{
						{
							Hostname:         "test-kafka-server-01",
							Port:             ToPointer(9092),
							SecurityProtocol: ToPointer("SASL_SSL"),
							Sasl: &clowder.KafkaSASLConfig{
								Password:         ToPointer("test-password"),
								SaslMechanism:    ToPointer("SCRAM-SHA-512"),
								SecurityProtocol: ToPointer("SASL_SSL"),
								Username:         ToPointer("test-user"),
							},
						},
					},
				},
			},
			options:     NewOptionsConfig(),
			authEnabled: true,
			expected:    []string{"test-kafka-server-01:9092"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.options.Consumer.AuthOptions.Enabled = test.authEnabled
			test.options.ConfigureConsumer(test.appconfig)
			assert.Equal(t, test.expected, test.options.Consumer.BootstrapServers)
			if test.authEnabled {
				assert.Equal(t, test.options.Consumer.AuthOptions.SecurityProtocol, *test.appconfig.Kafka.Brokers[0].SecurityProtocol)
				assert.Equal(t, test.options.Consumer.AuthOptions.SASLMechanism, *test.appconfig.Kafka.Brokers[0].Sasl.SaslMechanism)
				assert.Equal(t, test.options.Consumer.AuthOptions.SASLUsername, *test.appconfig.Kafka.Brokers[0].Sasl.Username)
				assert.Equal(t, test.options.Consumer.AuthOptions.SASLPassword, *test.appconfig.Kafka.Brokers[0].Sasl.Password)
			} else {
				assert.Equal(t, test.options.Consumer.AuthOptions.SecurityProtocol, "")
				assert.Equal(t, test.options.Consumer.AuthOptions.SASLMechanism, "")
				assert.Equal(t, test.options.Consumer.AuthOptions.SASLUsername, "")
				assert.Equal(t, test.options.Consumer.AuthOptions.SASLPassword, "")
			}
		})
	}
}

func TestInjectClowdAppConfig(t *testing.T) {
	authzTest := struct {
		name      string
		appconfig *clowder.AppConfig
		options   *OptionsConfig
		expected  *OptionsConfig
	}{
		name: "Authz is configured and injected, storage and consumer are ignored",
		appconfig: &clowder.AppConfig{
			Endpoints: []clowder.DependencyEndpoint{
				clowder.DependencyEndpoint{
					App:      authz.RelationsAPI,
					Hostname: "kessel-relations",
				},
			},
		},
		options: NewOptionsConfig(),
		expected: &OptionsConfig{
			Authz: &authz.Options{
				Authz: authz.Kessel,
				Kessel: &kessel.Options{
					URL: "kessel-relations:9000",
				},
			},
		},
	}

	t.Run(authzTest.name, func(t *testing.T) {
		err := authzTest.options.InjectClowdAppConfig(authzTest.appconfig)
		assert.NoError(t, err)
		assert.Equal(t, authzTest.expected.Authz.Authz, authzTest.options.Authz.Authz)
		assert.Equal(t, authzTest.expected.Authz.Kessel.URL, authzTest.options.Authz.Kessel.URL)
		assert.Nil(t, authzTest.expected.Storage)
		assert.Nil(t, authzTest.expected.Consumer)
	})

	storageTest := struct {
		name      string
		appconfig *clowder.AppConfig
		options   *OptionsConfig
		expected  *OptionsConfig
	}{
		name: "Storage is configured and injected, authz and consumer are ignored",
		appconfig: &clowder.AppConfig{
			Endpoints: []clowder.DependencyEndpoint{},
			Database: &clowder.DatabaseConfig{
				Hostname: "postgres",
				Name:     "postgres",
				Port:     5432,
				Username: "db-user",
				Password: "db-password",
			},
		},
		options: NewOptionsConfig(),
		expected: &OptionsConfig{
			Storage: &storage.Options{
				Database: storage.Postgres,
				Postgres: &postgres.Options{
					Host:     "postgres",
					Port:     "5432",
					User:     "db-user",
					Password: "db-password",
					DbName:   "postgres",
				},
			},
		},
	}

	t.Run(storageTest.name, func(t *testing.T) {
		err := storageTest.options.InjectClowdAppConfig(storageTest.appconfig)
		assert.NoError(t, err)
		assert.Equal(t, storageTest.expected.Storage.Database, storageTest.options.Storage.Database)
		assert.Equal(t, storageTest.expected.Storage.Postgres.Host, storageTest.options.Storage.Postgres.Host)
		assert.Equal(t, storageTest.expected.Storage.Postgres.Port, storageTest.options.Storage.Postgres.Port)
		assert.Equal(t, storageTest.expected.Storage.Postgres.User, storageTest.options.Storage.Postgres.User)
		assert.Equal(t, storageTest.expected.Storage.Postgres.Password, storageTest.options.Storage.Postgres.Password)
		assert.Equal(t, storageTest.expected.Storage.Postgres.DbName, storageTest.options.Storage.Postgres.DbName)
		assert.Nil(t, storageTest.expected.Authz)
		assert.Nil(t, storageTest.expected.Consumer)
	})

	consumerTest := struct {
		name      string
		appconfig *clowder.AppConfig
		options   *OptionsConfig
		expected  *OptionsConfig
	}{
		name: "Consumer is configured and injected with no auth settings, authz and storage are ignored",
		appconfig: &clowder.AppConfig{
			Endpoints: []clowder.DependencyEndpoint{},
			Kafka: &clowder.KafkaConfig{
				Brokers: []clowder.BrokerConfig{
					{
						Hostname: "test-kafka-server",
						Port:     ToPointer(9092),
					},
				},
			},
		},
		options: NewOptionsConfig(),
		expected: &OptionsConfig{
			Consumer: &consumer.Options{
				BootstrapServers: []string{"test-kafka-server:9092"},
			},
		},
	}
	t.Run(consumerTest.name, func(t *testing.T) {
		err := consumerTest.options.InjectClowdAppConfig(consumerTest.appconfig)
		assert.NoError(t, err)
		assert.Equal(t, consumerTest.expected.Consumer.BootstrapServers, consumerTest.options.Consumer.BootstrapServers)
		assert.Nil(t, consumerTest.expected.Authz)
		assert.Nil(t, consumerTest.expected.Storage)
	})

	consumerAuthTest := struct {
		name      string
		appconfig *clowder.AppConfig
		options   *OptionsConfig
		expected  *OptionsConfig
	}{
		name: "Consumer is configured and injected with auth settings, authz and storage are ignored",
		appconfig: &clowder.AppConfig{
			Endpoints: []clowder.DependencyEndpoint{},
			Kafka: &clowder.KafkaConfig{
				Brokers: []clowder.BrokerConfig{
					{
						Hostname:         "test-kafka-server-01",
						Port:             ToPointer(9092),
						SecurityProtocol: ToPointer("SASL_SSL"),
						Sasl: &clowder.KafkaSASLConfig{
							Password:         ToPointer("test-password"),
							SaslMechanism:    ToPointer("SCRAM-SHA-512"),
							SecurityProtocol: ToPointer("SASL_SSL"),
							Username:         ToPointer("test-user"),
						},
					},
				},
			},
		},
		options: NewOptionsConfig(),
		expected: &OptionsConfig{
			Consumer: &consumer.Options{
				BootstrapServers: []string{"test-kafka-server-01:9092"},
				AuthOptions: &auth.Options{
					SecurityProtocol: "SASL_SSL",
					SASLMechanism:    "SCRAM-SHA-512",
					SASLUsername:     "test-user",
					SASLPassword:     "test-password",
				},
			},
		},
	}
	t.Run(consumerAuthTest.name, func(t *testing.T) {
		err := consumerAuthTest.options.InjectClowdAppConfig(consumerAuthTest.appconfig)
		assert.NoError(t, err)
		assert.Equal(t, consumerAuthTest.expected.Consumer.BootstrapServers, consumerAuthTest.options.Consumer.BootstrapServers)
		assert.Equal(t, consumerAuthTest.expected.Consumer.AuthOptions.SecurityProtocol, consumerAuthTest.options.Consumer.AuthOptions.SecurityProtocol)
		assert.Equal(t, consumerAuthTest.expected.Consumer.AuthOptions.SASLMechanism, consumerAuthTest.options.Consumer.AuthOptions.SASLMechanism)
		assert.Equal(t, consumerAuthTest.expected.Consumer.AuthOptions.SASLUsername, consumerAuthTest.options.Consumer.AuthOptions.SASLUsername)
		assert.Equal(t, consumerAuthTest.expected.Consumer.AuthOptions.SASLPassword, consumerAuthTest.options.Consumer.AuthOptions.SASLPassword)
		assert.Nil(t, consumerAuthTest.expected.Authz)
		assert.Nil(t, consumerAuthTest.expected.Storage)
	})

	noConfigTest := struct {
		name      string
		appconfig *clowder.AppConfig
		options   *OptionsConfig
		expected  *OptionsConfig
	}{
		name:      "No values found in AppConfig -- Clowder changes nothing",
		appconfig: &clowder.AppConfig{},
		options:   NewOptionsConfig(),
		expected:  NewOptionsConfig(),
	}

	t.Run(noConfigTest.name, func(t *testing.T) {
		err := noConfigTest.options.InjectClowdAppConfig(noConfigTest.appconfig)
		assert.NoError(t, err)
		assert.Equal(t, noConfigTest.expected.Authz, noConfigTest.options.Authz)
		assert.Equal(t, noConfigTest.expected.Storage, noConfigTest.options.Storage)
		assert.Equal(t, noConfigTest.expected.Consumer, noConfigTest.options.Consumer)
	})
}
