package resources_test

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/require"
)

// TestServerFixturesDoNotProvisionUnusedDependencies guards against provisioning
// databases merely to register unrelated endpoints. An invalid backend exposes
// accidental fixture creation even after the shared container has started.
func TestServerFixturesDoNotProvisionUnusedDependencies(t *testing.T) {
	t.Setenv("TEST_DB", "unused-by-this-test")

	newConfig := func(t *testing.T) TestServerConfig {
		return TestServerConfig{
			Usecase: newTestUsecase(t, testUsecaseConfig{
				Relations: struct{ model.RelationsRepository }{}, // unused by server construction
			}),
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
