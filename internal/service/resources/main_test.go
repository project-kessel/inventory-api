package resources

import (
	"fmt"
	"os"
	"testing"

	"github.com/project-kessel/inventory-api/internal/testutil"
)

// TestMain stops the shared PostgreSQL testcontainer used by database tests on
// the postgres backend. scripts/testcontainers-run.sh disables Ryuk for
// Podman, so without this the container would outlive the test binary. On the
// sqlite fallback this is a no-op.
func TestMain(m *testing.M) {
	code := m.Run()
	if err := testutil.TerminateSharedPostgres(); err != nil {
		fmt.Fprintf(os.Stderr, "PostgreSQL test cleanup failed: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
