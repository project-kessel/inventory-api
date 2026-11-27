// Package testlifecycle coordinates package-level test cleanup without starting
// fixtures. Each TestMain supplies only the shared resources its package owns.
package testlifecycle

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/project-kessel/inventory-api/internal/testutil/containerdebug"
)

// Run runs m and then all cleanups in the supplied order, returning an exit code
// for TestMain. It records failures before cleanup so container owners can honor
// keep-on-failure mode. Cleanup errors are reported to stderr and fail successful
// test runs, but never replace an existing test failure's exit code.
func Run(m *testing.M, cleanups ...func() error) int {
	return run(m.Run, os.Stderr, cleanups...)
}

func run(tests func() int, output io.Writer, cleanups ...func() error) int {
	code := tests()
	containerdebug.RecordResult(code)
	for _, cleanup := range cleanups {
		if err := cleanup(); err != nil {
			// Reporting is best-effort; a write failure must not interrupt cleanup.
			_, _ = fmt.Fprintf(output, "Test cleanup failed: %v\n", err)
			if code == 0 {
				code = 1
			}
			containerdebug.RecordResult(code)
		}
	}
	return code
}
