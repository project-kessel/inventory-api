package sqlite3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompleteConfig(t *testing.T) {
	t.Run("All options empty", func(t *testing.T) {
		config := Config{
			Options: &Options{
				DSN: "",
			},
		}
		completed := config.Complete()
		assert.NotNil(t, completed)
		assert.Equal(t, "", completed.DSN)
	})

	t.Run("DSN set", func(t *testing.T) {
		config := Config{
			Options: &Options{
				DSN: "inventory.db",
			},
		}
		completed := config.Complete()
		assert.NotNil(t, completed)
		assert.Equal(t, "inventory.db", completed.DSN)
	})
}
