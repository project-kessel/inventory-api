package relationships

import (
	"fmt"
	"os"
	"testing"

	"github.com/project-kessel/inventory-api/internal/testutil"
	"github.com/project-kessel/inventory-api/internal/testutil/containerdebug"
)

// TestMain stops shared containers even when Podman runs without Ryuk.
func TestMain(m *testing.M) {
	code := m.Run()
	containerdebug.RecordResult(code)
	if err := testutil.TerminateSharedPostgres(); err != nil {
		fmt.Fprintf(os.Stderr, "PostgreSQL test cleanup failed: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
