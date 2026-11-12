package resources

import (
	"fmt"
	"os"
	"testing"

	"github.com/project-kessel/inventory-api/internal/testutil"
	relationstest "github.com/project-kessel/inventory-api/internal/testutil/relations"
)

// TestMain stops shared containers even when Podman runs without Ryuk.
func TestMain(m *testing.M) {
	code := m.Run()
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
