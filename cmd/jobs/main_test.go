package jobs

import (
	"os"
	"testing"

	"github.com/project-kessel/inventory-api/internal/testutil"
	"github.com/project-kessel/inventory-api/internal/testutil/testlifecycle"
)

// TestMain stops the shared PostgreSQL testcontainer used by database tests on
// the postgres backend. scripts/testcontainers-run.sh disables Ryuk for
// Podman, so without this the container would outlive the test binary. On the
// sqlite fallback this is a no-op.
func TestMain(m *testing.M) {
	os.Exit(testlifecycle.Run(m, testutil.TerminateSharedPostgres))
}
