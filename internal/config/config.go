package config

import (
	"fmt"
	"strconv"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"

	"github.com/project-kessel/inventory-api/internal/authn"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/consistency"
	"github.com/project-kessel/inventory-api/internal/consumer"
	"github.com/project-kessel/inventory-api/internal/eventing"
	"github.com/project-kessel/inventory-api/internal/server"
	"github.com/project-kessel/inventory-api/internal/service"
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
	Service     *service.Options
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
		service.NewOptions(),
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

	if options.Authz.Authz == authz.SpiceDB {
		log.Debugf("Authz Configuration: Endpoint: %s, UseTLS?: %t",
			options.Authz.SpiceDB.Endpoint,
			options.Authz.SpiceDB.UseTLS,
		)
	}

	log.Debugf("Consumer Configuration: Bootstrap Server: %s, Topic: %s, Consumer Max Retries: %d, Operation Max Retries: %d, Backoff Factor: %d, Max Backoff Seconds: %d",
		options.Consumer.BootstrapServers,
		options.Consumer.Topic,
		options.Consumer.RetryOptions.ConsumerMaxRetries,
		options.Consumer.RetryOptions.OperationMaxRetries,
		options.Consumer.RetryOptions.BackoffFactor,
		options.Consumer.RetryOptions.MaxBackoffSeconds,
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
func (o *OptionsConfig) InjectClowdAppConfig(appconfig *clowder.AppConfig) error {
	// check for authz config
	if len(appconfig.Endpoints) > 0 {
		for _, endpoint := range appconfig.Endpoints {
			// Skip Authz configuration as we're using SpiceDB directly now
			_ = endpoint
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

// ConfigureAuthz updates Authz settings based on ClowdApp AppConfig
// Note: This is deprecated as we now use SpiceDB directly
func (o *OptionsConfig) ConfigureAuthz(endpoint clowder.DependencyEndpoint) {
	// No-op: SpiceDB configuration should be done via config files
	_ = endpoint
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

	if len(appconfig.Kafka.Brokers) > 0 && appconfig.Kafka.Brokers[0].SecurityProtocol != nil {
		o.Consumer.AuthOptions.SecurityProtocol = *appconfig.Kafka.Brokers[0].SecurityProtocol

		if appconfig.Kafka.Brokers[0].Sasl != nil {
			o.Consumer.AuthOptions.SASLMechanism = *appconfig.Kafka.Brokers[0].Sasl.SaslMechanism
			o.Consumer.AuthOptions.SASLUsername = *appconfig.Kafka.Brokers[0].Sasl.Username
			o.Consumer.AuthOptions.SASLPassword = *appconfig.Kafka.Brokers[0].Sasl.Password
		}
	}
}
