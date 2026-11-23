package resources_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestServerFixturesDoNotProvisionUnusedDependencies guards against provisioning
// databases or SpiceDB merely to register unrelated endpoints. An invalid backend
// exposes accidental fixture creation even after the shared containers have started.
func TestServerFixturesDoNotProvisionUnusedDependencies(t *testing.T) {
	t.Setenv("TEST_DB", "unused-by-this-test")

	newConfig := func(t *testing.T) TestServerConfig {
		return TestServerConfig{
			Usecase:       newTestUsecase(t, testUsecaseConfig{}),
			Authenticator: &DenyAuthenticator{},
		}
	}

	t.Run("grpc", func(t *testing.T) {
		require.NotPanics(t, func() {
			require.NotNil(t, newTestGRPCServer(t, newConfig(t)))
		})
	})
	t.Run("http", func(t *testing.T) {
		require.NotPanics(t, func() {
			require.NotEmpty(t, newTestHTTPServer(t, newConfig(t)))
		})
	})
}
