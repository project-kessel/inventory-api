package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompleteConfig(t *testing.T) {
	t.Run("given DNS", func(t *testing.T) {
		config := Config{
			Options: nil,
			DSN:     "foobar",
		}
		assert.Equal(t, "foobar", config.Complete().DSN)
	})

	t.Run("All options are empty", func(t *testing.T) {
		config := Config{
			Options: &Options{
				Host:        "",
				Port:        "",
				DbName:      "",
				User:        "",
				Password:    "",
				SSLMode:     "",
				SSLRootCert: "",
			},
			DSN: "",
		}
		assert.Equal(t, "", config.Complete().DSN)
	})

	t.Run("All options not empty", func(t *testing.T) {
		config := Config{
			Options: &Options{
				Host:        "my-host",
				Port:        "my-port",
				DbName:      "my-database",
				User:        "my-username",
				Password:    "my-password",
				SSLMode:     "my-ssl-mode",
				SSLRootCert: "my-ssl-root-cert",
			},
			DSN: "",
		}
		assert.Equal(t, "host=my-host port=my-port dbname=my-database user=my-username password=my-password sslmode=my-ssl-mode sslrootcert=my-ssl-root-cert", config.Complete().DSN)
	})
}
