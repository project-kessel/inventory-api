package resources

import (
	"fmt"
	"os"
	"testing"

	"github.com/project-kessel/inventory-api/internal/testutil"
	"github.com/project-kessel/inventory-api/internal/testutil/containerdebug"
	relationstest "github.com/project-kessel/inventory-api/internal/testutil/relations"
)

// TestMain stops the shared PostgreSQL and SpiceDB testcontainers.
// scripts/testcontainers-run.sh disables Ryuk for Podman, so without this
// the containers would outlive the test binary. PostgreSQL cleanup is a no-op
// on the sqlite fallback.
func TestMain(m *testing.M) {
	code := m.Run()
	containerdebug.RecordResult(code)
	if err := relationstest.TerminateSharedSpiceDB(); err != nil {
		fmt.Fprintf(os.Stderr, "SpiceDB test cleanup failed: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	if err := testutil.TerminateSharedPostgres(); err != nil {
		fmt.Fprintf(os.Stderr, "PostgreSQL test cleanup failed: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
