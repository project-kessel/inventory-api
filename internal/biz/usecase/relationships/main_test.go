package relationships

import (
	"os"
	"testing"

	"github.com/project-kessel/inventory-api/internal/testutil"
	"github.com/project-kessel/inventory-api/internal/testutil/testlifecycle"
)

// TestMain stops shared containers even when Podman runs without Ryuk.
func TestMain(m *testing.M) {
	os.Exit(testlifecycle.Run(m, testutil.TerminateSharedPostgres))
}
