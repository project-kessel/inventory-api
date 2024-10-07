package config

import (
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/authn"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/eventing"
	"github.com/project-kessel/inventory-api/internal/server"
	"github.com/project-kessel/inventory-api/internal/storage"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"strconv"
)

// OptionsConfig contains the settings for each configuration option
type OptionsConfig struct {
	Authn    *authn.Options
	Authz    *authz.Options
	Storage  *storage.Options
	Eventing *eventing.Options
	Server   *server.Options
}

// NewOptionsConfig returns a new OptionsConfig with default options set
func NewOptionsConfig() *OptionsConfig {
	return &OptionsConfig{
		authn.NewOptions(),
		authz.NewOptions(),
		storage.NewOptions(),
		eventing.NewOptions(),
		server.NewOptions(),
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
		log.Debugf("Storage Configuration: Host: %s, DB: %s, Port: %s",
			options.Storage.Postgres.Host,
			options.Storage.Postgres.DbName,
			options.Storage.Postgres.Port,
		)
	}

	if options.Authz.Authz == authz.Kessel {
		log.Debugf("Authz Configuration: URL: %s, Insecure?: %t, OIDC?: %t",
			options.Authz.Kessel.URL,
			options.Authz.Kessel.Insecure,
			options.Authz.Kessel.EnableOidcAuth,
		)
	}
}

// InjectClowdAppConfig updates service options based on values in the ClowdApp AppConfig
func (o *OptionsConfig) InjectClowdAppConfig() {
	// check for authz config
	for _, endpoint := range clowder.LoadedConfig.Endpoints {
		if endpoint.App == authz.RelationsAPI {
			o.ConfigureAuthz(endpoint)
		}
	}
	// check for db config
	if clowder.LoadedConfig.Database != nil {
		o.ConfigureStorage(clowder.LoadedConfig.Database)
	}
}

// ConfigureAuthz updates Authz settings based on ClowdApp AppConfig
func (o *OptionsConfig) ConfigureAuthz(endpoint clowder.DependencyEndpoint) {
	o.Authz.Authz = authz.Kessel
	o.Authz.Kessel.URL = fmt.Sprintf("%s:%d", endpoint.Hostname, 9000)
}

// ConfigureStorage updates Storage settings based on ClowdApp AppConfig
func (o *OptionsConfig) ConfigureStorage(database *clowder.DatabaseConfig) {
	o.Storage.Database = storage.Postgres
	o.Storage.Postgres.Host = database.Hostname
	o.Storage.Postgres.Port = strconv.Itoa(database.Port)
	o.Storage.Postgres.User = database.Username
	o.Storage.Postgres.Password = database.Password
	o.Storage.Postgres.DbName = database.Name
}
