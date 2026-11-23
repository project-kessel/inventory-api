package testlifecycle

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/project-kessel/inventory-api/internal/testutil/containerdebug"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	for _, test := range []struct {
		name       string
		testCode   int
		cleanupErr error
		wantCode   int
	}{
		{name: "success"},
		{name: "test failure", testCode: 2, wantCode: 2},
		{name: "cleanup failure", cleanupErr: errors.New("cannot stop container"), wantCode: 1},
		{name: "preserve test failure", testCode: 2, cleanupErr: errors.New("cannot stop container"), wantCode: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			var calls []string
			code := run(func() int {
				calls = append(calls, "tests")
				return test.testCode
			}, &output,
				func() error {
					calls = append(calls, "first cleanup")
					return test.cleanupErr
				},
				func() error {
					calls = append(calls, "second cleanup")
					return test.cleanupErr
				},
			)
			require.Equal(t, test.wantCode, code)
			require.Equal(t, []string{"tests", "first cleanup", "second cleanup"}, calls)
			if test.cleanupErr == nil {
				require.Empty(t, output.String())
			} else {
				require.Equal(t, 2, strings.Count(output.String(), test.cleanupErr.Error()))
			}
		})
	}
}

func TestRunWithoutCleanups(t *testing.T) {
	var output bytes.Buffer
	require.Zero(t, run(func() int { return 0 }, &output))
	require.Empty(t, output.String())
}

func TestRunRecordsFailuresBeforeCleanup(t *testing.T) {
	for _, scenario := range []string{"success", "test failure", "cleanup failure"} {
		t.Run(scenario, func(t *testing.T) {
			// Retention state is process-wide and intentionally never resets.
			cmd := exec.Command(os.Args[0], "-test.run=^TestRunRetentionChild$")
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "TEST_CONTAINERS_") && !strings.HasPrefix(entry, "TESTCONTAINERS_RYUK_DISABLED=") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			cmd.Env = append(cmd.Env, "TEST_LIFECYCLE_SCENARIO="+scenario,
				"TEST_CONTAINERS_KEEP_ON_FAILURE=true", "TESTCONTAINERS_RYUK_DISABLED=true")
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", output)
		})
	}
}

func TestRunRetentionChild(t *testing.T) {
	scenario := os.Getenv("TEST_LIFECYCLE_SCENARIO")
	if scenario == "" {
		t.Skip("subprocess-only retention check")
	}
	require.NoError(t, containerdebug.Configure())
	var output bytes.Buffer
	code := run(func() int {
		if scenario == "test failure" {
			return 2
		}
		return 0
	}, &output,
		func() error {
			require.Equal(t, scenario == "test failure", containerdebug.KeepOnFailure())
			if scenario == "cleanup failure" {
				return errors.New("first cleanup failed")
			}
			return nil
		},
		func() error {
			require.Equal(t, scenario != "success", containerdebug.KeepOnFailure())
			return nil
		},
	)
	switch scenario {
	case "success":
		require.Zero(t, code)
	case "test failure":
		require.Equal(t, 2, code)
	case "cleanup failure":
		require.Equal(t, 1, code)
	default:
		t.Fatalf("unknown scenario %q", scenario)
	}
}
