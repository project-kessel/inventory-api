package resources

import (
	"fmt"
	"os"
	"testing"

	"github.com/project-kessel/inventory-api/internal/testutil"
)

// TestMain stops the shared PostgreSQL container even when Podman runs without
// Ryuk. Cleanup is a no-op for the SQLite fallback.
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
