package health

import (
	"os"
	"testing"

	"github.com/project-kessel/inventory-api/internal/testutil"
	relationstest "github.com/project-kessel/inventory-api/internal/testutil/relations"
	"github.com/project-kessel/inventory-api/internal/testutil/testlifecycle"
)

// TestMain stops the shared PostgreSQL and SpiceDB testcontainers.
// scripts/testcontainers-run.sh disables Ryuk for Podman, so without this the
// containers would outlive the test binary. PostgreSQL cleanup is a no-op on
// the sqlite fallback.
func TestMain(m *testing.M) {
	os.Exit(testlifecycle.Run(m, relationstest.TerminateSharedSpiceDB, testutil.TerminateSharedPostgres))
}
