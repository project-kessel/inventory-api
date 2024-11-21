package config

import (
	"os"
	"testing"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/stretchr/testify/assert"
)

func ParseCA(t *testing.T, path string) string {
	file, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("failed to read file %s: %s", path, err)
		return ""
	}
	return string(file)
}

func TestConfigureStorage(t *testing.T) {
	var testCA = `-----BEGIN CERTIFICATE-----\nFAKE-CA\n-----END CERTIFICATE-----`

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
