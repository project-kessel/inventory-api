package config

import (
	"fmt"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"

	"github.com/project-kessel/inventory-api/internal/authn"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/consistency"
	"github.com/project-kessel/inventory-api/internal/consumer"
	"github.com/project-kessel/inventory-api/internal/eventing"
	"github.com/project-kessel/inventory-api/internal/server"
	"github.com/project-kessel/inventory-api/internal/storage"
)

// OptionsConfig contains the settings for each configuration option
type OptionsConfig struct {
	Authn       *authn.Options
	Authz       *authz.Options
	Storage     *storage.Options
	Eventing    *eventing.Options
	Consumer    *consumer.Options
	Server      *server.Options
	Consistency *consistency.Options
}

// NewOptionsConfig returns a new OptionsConfig with default options set
func NewOptionsConfig() *OptionsConfig {
	return &OptionsConfig{
		authn.NewOptions(),
		authz.NewOptions(),
		storage.NewOptions(),
		eventing.NewOptions(),
		consumer.NewOptions(),
		server.NewOptions(),
		consistency.NewOptions(),
	}
}

// LogConfigurationInfo outputs connection details to logs when in debug for testing (no secret data is output)
func LogConfigurationInfo(options *OptionsConfig) {
	log.Debugf("Server Configuration: Public URL: %s, HTTP Listener: %s, GRPC Listener: %s",
		options.Server.PublicUrl,
		options.Server.HttpOptions.Addr,
		options.Server.GrpcOptions.Addr)

	if options.Authn.Oidc.AuthorizationServerURL != "" {
		log.Debugf("Authn Configuration: URL: %s, ClientID: %s",
			options.Authn.Oidc.AuthorizationServerURL,
			options.Authn.Oidc.ClientId,
		)
	}

	if options.Storage.Database == storage.Postgres {
		log.Debugf("Storage Configuration: Host: %s, DB: %s, Port: %s, SSLMode: %s, RDSCACert: %s",
			options.Storage.Postgres.Host,
			options.Storage.Postgres.DbName,
			options.Storage.Postgres.Port,
			options.Storage.Postgres.SSLMode,
			options.Storage.Postgres.SSLRootCert,
		)
	}

	if options.Authz.Authz == authz.Kessel {
		log.Debugf("Authz Configuration: URL: %s, Insecure?: %t, OIDC?: %t",
			options.Authz.Kessel.URL,
			options.Authz.Kessel.Insecure,
			options.Authz.Kessel.EnableOidcAuth,
		)
	}

	log.Debugf("Consumer Configuration: Bootstrap Server: %s, Topic: %s, Consumer Max Retries: %d, Operation Max Retries: %d, Backoff Factor: %d",
		options.Consumer.BootstrapServers,
		options.Consumer.Topic,
		options.Consumer.RetryOptions.ConsumerMaxRetries,
		options.Consumer.RetryOptions.OperationMaxRetries,
		options.Consumer.RetryOptions.BackoffFactor,
	)

	log.Debugf("Consumer Auth Settings: Enabled: %v, Security Protocol: %s, Mechanism: %s, Username: %s",
		options.Consumer.AuthOptions.Enabled,
		options.Consumer.AuthOptions.SecurityProtocol,
		options.Consumer.AuthOptions.SASLMechanism,
		options.Consumer.AuthOptions.SASLUsername)

	log.Debugf("Consistency Configuration: Read-After-Write Enabled: %t, Read-After-Write Allowlist: %+s",
		options.Consistency.ReadAfterWriteEnabled,
		options.Consistency.ReadAfterWriteAllowlist,
	)

}

// InjectClowdAppConfig updates service options based on values in the ClowdApp AppConfig
func (o *OptionsConfig) InjectClowdAppConfig() error {
	// check for authz config
	for _, endpoint := range clowder.LoadedConfig.Endpoints {
		if endpoint.App == authz.RelationsAPI {
			o.ConfigureAuthz(endpoint)
		}
	}
	// check for db config
	if clowder.LoadedConfig.Database != nil {
		err := o.ConfigureStorage(clowder.LoadedConfig)
		if err != nil {
			return fmt.Errorf("failed to configure storage: %w", err)
		}
	}
	// check for consumer config
	if o.Consumer.Enabled {
		o.ConfigureConsumer(clowder.LoadedConfig)
	}
	return nil
}

// ConfigureAuthz updates Authz settings based on ClowdApp AppConfig
func (o *OptionsConfig) ConfigureAuthz(endpoint clowder.DependencyEndpoint) {
	o.Authz.Authz = authz.Kessel
	o.Authz.Kessel.URL = fmt.Sprintf("%s:%d", endpoint.Hostname, 9000)
}

// ConfigureStorage updates Storage settings based on ClowdApp AppConfig
func (o *OptionsConfig) ConfigureStorage(appconfig *clowder.AppConfig) error {
	o.Storage.Database = storage.Postgres
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

// ConfigureConsumer updates Consumer settings based on ClowdApp AppConfig
func (o *OptionsConfig) ConfigureConsumer(appconfig *clowder.AppConfig) {
	var brokers []string
	for _, broker := range appconfig.Kafka.Brokers {
		brokers = append(brokers, fmt.Sprintf("%s:%d", broker.Hostname, *broker.Port))
	}
	o.Consumer.BootstrapServers = brokers

	if o.Consumer.AuthOptions.Enabled {
		if appconfig.Kafka.Brokers[0].SecurityProtocol != nil {
			o.Consumer.AuthOptions.SecurityProtocol = *appconfig.Kafka.Brokers[0].SecurityProtocol
		}

		if appconfig.Kafka.Brokers[0].Sasl != nil {
			o.Consumer.AuthOptions.SASLMechanism = *appconfig.Kafka.Brokers[0].Sasl.SaslMechanism
			o.Consumer.AuthOptions.SASLUsername = *appconfig.Kafka.Brokers[0].Sasl.Username
			o.Consumer.AuthOptions.SASLPassword = *appconfig.Kafka.Brokers[0].Sasl.Password
		}
	}
}
