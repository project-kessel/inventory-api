package config

import (
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConfigureStorage(t *testing.T) {
	tests := []struct {
		name      string
		appconfig *clowder.AppConfig
		options   *OptionsConfig
	}{
		{
			name: "ensures DB info is set",
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
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.options.ConfigureStorage(test.appconfig.Database)
			assert.Equal(t, "postgres", test.options.Storage.Database)
			assert.Equal(t, "postgres", test.options.Storage.Postgres.Host)
			assert.Equal(t, "postgres", test.options.Storage.Postgres.DbName)
			assert.Equal(t, "5432", test.options.Storage.Postgres.Port)
			assert.Equal(t, "db-user", test.options.Storage.Postgres.User)
			assert.Equal(t, "db-password", test.options.Storage.Postgres.Password)
		})
	}
}

func TestConfigureAuthz(t *testing.T) {
	tests := []struct {
		name      string
		appconfig *clowder.AppConfig
		options   *OptionsConfig
	}{
		{
			name: "ensures Authz info is set",
			appconfig: &clowder.AppConfig{
				Endpoints: []clowder.DependencyEndpoint{
					clowder.DependencyEndpoint{
						Hostname: "kessel-relations",
					},
				},
			},
			options: NewOptionsConfig(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.options.ConfigureAuthz(test.appconfig.Endpoints[0])
			assert.Equal(t, "kessel", test.options.Authz.Authz)
			assert.Equal(t, "kessel-relations:9000", test.options.Authz.Kessel.URL)
		})
	}
}
