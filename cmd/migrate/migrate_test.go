package migrate

import (
	"testing"

	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/stretchr/testify/require"
)

func TestDisabledMigrationSkipsBeforeUsingStorageOptions(t *testing.T) {
	cmd := NewCommand(nil, common.LoggerOptions{})
	cmd.SetArgs([]string{"--enabled=false"})

	require.NoError(t, cmd.Execute())
}

func TestMigrationsAreEnabledByDefault(t *testing.T) {
	cmd := NewCommand(nil, common.LoggerOptions{})

	enabledFlag := cmd.Flags().Lookup("enabled")
	require.NotNil(t, enabledFlag)
	require.Equal(t, "true", enabledFlag.DefValue)
}
