// Package containerdebug provides opt-in diagnostics and failure retention for
// shared test containers. Call Configure before any testcontainers operation,
// including provider health checks: testcontainers caches its Ryuk configuration.
// Container owners remain responsible for normal cleanup and must consult
// KeepOnFailure before terminating their shared containers. TestMain should call
// RecordResult with m.Run's exit code before shared-container cleanup to also
// retain containers when tests without observed fixtures fail.
package containerdebug

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

// Container describes a shared container. Endpoint must be a credential-free
// host/address, not a connection string. Password is written only to the
// explicitly requested owner-only credentials file.
type Container struct {
	Kind     string
	ID       string
	Endpoint string
	Username string
	Password string
}

// Fixture associates a test with its private database or SpiceDB datastore.
// Token is written only to the explicitly requested credentials file.
type Fixture struct {
	Kind        string
	ContainerID string
	Test        string
	Database    string
	Token       string
}

var state struct {
	once    sync.Once
	err     error
	enabled bool
	keep    bool
	file    *os.File
	mutex   sync.Mutex
	failed  bool
}

// Configure reads TEST_CONTAINERS_DEBUG, TEST_CONTAINERS_DEBUG_FILE, and
// TEST_CONTAINERS_KEEP_ON_FAILURE once per process, caching errors as well as
// success. Boolean flags accept strconv.ParseBool values; unset or empty is false.
// A nonempty file path or enabled keep flag also enables diagnostics. The file
// path is suffixed with the executable basename and PID and created exclusively
// with mode 0600. Credentials are never printed to stdout or returned in errors.
// When keep is enabled, Configure sets TESTCONTAINERS_RYUK_DISABLED=true before
// returning. It must run before testcontainers first reads its configuration,
// not merely before the first runtime connection: ReadConfig, session-ID and
// label helpers can also populate testcontainers' process-wide cache. Configure
// verifies the effective cached configuration and returns an actionable error if
// Ryuk remains enabled. It cannot change that cache or undo an already-started
// Ryuk. If another package's initialization may read it first, set
// TESTCONTAINERS_RYUK_DISABLED=true in the launching environment.
func Configure() error {
	state.once.Do(func() {
		debug, err := booleanEnvironment("TEST_CONTAINERS_DEBUG")
		if err != nil {
			state.err = err
			return
		}
		keep, err := booleanEnvironment("TEST_CONTAINERS_KEEP_ON_FAILURE")
		if err != nil {
			state.err = err
			return
		}
		if keep {
			if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
				state.err = errors.New("containerdebug: cannot disable Ryuk")
				return
			}
			if !testcontainers.ReadConfig().Config.RyukDisabled {
				state.err = errors.New("containerdebug: cached testcontainers configuration still enables Ryuk; set TESTCONTAINERS_RYUK_DISABLED=true before launching the test binary")
				return
			}
		}
		base := os.Getenv("TEST_CONTAINERS_DEBUG_FILE")
		if base != "" {
			executable, err := os.Executable()
			if err != nil {
				state.err = errors.New("containerdebug: cannot determine executable name")
				return
			}
			path := base + "." + filepath.Base(executable) + "." + strconv.Itoa(os.Getpid()) + ".jsonl"
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|os.O_APPEND, 0600)
			if err != nil {
				state.err = errors.New("containerdebug: cannot exclusively create TEST_CONTAINERS_DEBUG_FILE (check parent directory and existing file)")
				return
			}
			// Keep this descriptor for the process lifetime: appends must continue to
			// target the exclusively created file, even if its pathname is replaced.
			state.file = file
			fmt.Printf("containerdebug: credentials file %q\n", path)
		}
		state.enabled = debug || keep || base != ""
		state.keep = keep
	})
	return state.err
}

func booleanEnvironment(name string) (bool, error) {
	value := os.Getenv(name)
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("containerdebug: %s must be a boolean", name)
	}
	return parsed, nil
}

type record struct {
	Type        string `json:"type"`
	Kind        string `json:"kind,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
	Endpoint    string `json:"endpoint,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Test        string `json:"test,omitempty"`
	Database    string `json:"database,omitempty"`
	Token       string `json:"token,omitempty"`
}

// RegisterContainer prints nonsecret connection details and inspection commands,
// and optionally appends full credentials to the JSONL file. It is a no-op when
// diagnostics are disabled. It also propagates cached configuration errors.
func RegisterContainer(container Container) error {
	return writeRecord(record{Type: "container", Kind: container.Kind, ContainerID: container.ID,
		Endpoint: container.Endpoint, Username: container.Username, Password: container.Password})
}

// RegisterFixture prints a nonsecret test-to-fixture association and optionally
// appends its token to the credentials file. It is a no-op when debug is disabled.
func RegisterFixture(fixture Fixture) error {
	return writeRecord(record{Type: "fixture", Kind: fixture.Kind, ContainerID: fixture.ContainerID,
		Test: fixture.Test, Database: fixture.Database, Token: fixture.Token})
}

// Retained reports a retained container and manual cleanup commands that also
// remove anonymous volumes,
// and appends retained metadata to the optional credentials file. The caller
// decides retention; this function does not stop or preserve containers itself.
func Retained(id string) error {
	return writeRecord(record{Type: "retained", ContainerID: id})
}

func writeRecord(entry record) error {
	if err := Configure(); err != nil {
		return err
	}
	if !state.enabled {
		return nil
	}
	state.mutex.Lock()
	defer state.mutex.Unlock()
	safe := entry
	safe.Password = ""
	safe.Token = ""
	summary, err := json.Marshal(safe)
	if err != nil {
		return errors.New("containerdebug: cannot encode diagnostic summary")
	}
	if _, err := fmt.Fprintf(os.Stdout, "containerdebug: %s\n", summary); err != nil {
		return errors.New("containerdebug: cannot write diagnostic summary")
	}
	if entry.Type == "container" || entry.Type == "retained" {
		// Shell quoting keeps even unusual IDs inert when commands are copied.
		id := "'" + strings.ReplaceAll(entry.ContainerID, "'", "'\"'\"'") + "'"
		for _, runtime := range []string{"podman", "docker"} {
			if _, err := fmt.Fprintf(os.Stdout, "containerdebug: %s inspect %s; %s logs %s\n", runtime, id, runtime, id); err != nil {
				return errors.New("containerdebug: cannot write inspection commands")
			}
			if entry.Type == "retained" {
				if _, err := fmt.Fprintf(os.Stdout, "containerdebug: cleanup: %s rm -f -v %s\n", runtime, id); err != nil {
					return errors.New("containerdebug: cannot write cleanup command")
				}
			}
		}
	}
	if state.file != nil {
		if err := json.NewEncoder(state.file).Encode(entry); err != nil {
			return errors.New("containerdebug: cannot append credentials record")
		}
	}
	return nil
}

// Observe registers cleanup to remember a test's failure for package-level
// retention. Register it before fixture cleanup so later cleanup failures are
// observed too (testing runs cleanups in reverse registration order).
func Observe(t testing.TB) {
	t.Helper()
	if err := Configure(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { PreserveTest(t) })
}

// PreserveTest records whether t has failed and returns true only when keeping
// failed fixtures is enabled and this test failed. PostgreSQL fixture cleanup
// uses it to avoid dropping a failed test's private database.
func PreserveTest(t testing.TB) bool {
	t.Helper()
	failed := t.Failed()
	err := Configure()
	state.mutex.Lock()
	defer state.mutex.Unlock()
	state.failed = state.failed || failed
	return err == nil && state.keep && failed
}

// RecordResult remembers any nonzero package test exit code, including failures
// unrelated to observed fixtures. Call it from TestMain with m.Run's result before
// shared-container cleanup. A zero code never clears an earlier failure. It does
// not initialize configuration or log anything; callers must still check
// Configure's error through the normal setup path.
func RecordResult(exitCode int) {
	state.mutex.Lock()
	defer state.mutex.Unlock()
	state.failed = state.failed || exitCode != 0
}

// KeepOnFailure reports whether keep mode is enabled and an observed test or
// recorded package result failed. Call it after test cleanup, normally from
// TestMain after m.Run() and RecordResult.
func KeepOnFailure() bool {
	if err := Configure(); err != nil {
		return false
	}
	state.mutex.Lock()
	defer state.mutex.Unlock()
	return state.keep && state.failed
}
