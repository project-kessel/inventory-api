package containerdebug

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// Subprocesses exercise the real process-wide cache without resetting global state.
func runProcess(t *testing.T, action string, settings map[string]string) (string, error) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestProcess$")
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "TEST_CONTAINERS_") || strings.HasPrefix(key, "CONTAINERDEBUG_") || key == "TESTCONTAINERS_RYUK_DISABLED" {
			continue
		}
		command.Env = append(command.Env, entry)
	}
	// Each child joins its goroutines. Avoid paying the race detector's default
	// one-second exit delay for every isolated configuration case.
	command.Env = append(command.Env, "CONTAINERDEBUG_ACTION="+action, "TESTCONTAINERS_RYUK_DISABLED=false",
		"GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
	for key, value := range settings {
		command.Env = append(command.Env, key+"="+value)
	}
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("debug configuration subprocess timed out")
	}
	return string(output), err
}

func TestConfiguration(t *testing.T) {
	for _, test := range []struct {
		name     string
		settings map[string]string
		enabled  bool
		invalid  bool
	}{
		{name: "disabled"},
		{name: "enabled", settings: map[string]string{"TEST_CONTAINERS_DEBUG": "1"}, enabled: true},
		{name: "false", settings: map[string]string{"TEST_CONTAINERS_DEBUG": "false"}},
		{name: "keep enables debug", settings: map[string]string{"TEST_CONTAINERS_KEEP_ON_FAILURE": "1"}, enabled: true},
		{name: "invalid debug", settings: map[string]string{"TEST_CONTAINERS_DEBUG": "SECRET-invalid"}, invalid: true},
		{name: "invalid keep", settings: map[string]string{"TEST_CONTAINERS_KEEP_ON_FAILURE": "SECRET-invalid"}, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, err := runProcess(t, "registry", test.settings)
			if err != nil {
				t.Fatalf("subprocess: %v\n%s", err, output)
			}
			if strings.Contains(output, "SECRET") {
				t.Fatalf("secret in output: %s", output)
			}
			if strings.Contains(output, "CONFIG_ERROR") != test.invalid {
				t.Fatalf("config result: %s", output)
			}
			if strings.Contains(output, "container-id-123") != test.enabled {
				t.Fatalf("debug result: %s", output)
			}
			if test.enabled && (!strings.Contains(output, "inspect 'container-id-123'") || !strings.Contains(output, "logs 'container-id-123'") || !strings.Contains(output, "rm -f -v 'container-id-123'")) {
				t.Fatalf("missing inspection/cleanup commands: %s", output)
			}
			if !test.enabled && !test.invalid && strings.Contains(output, "containerdebug:") {
				t.Fatalf("disabled diagnostics produced output: %s", output)
			}
		})
	}
}

func TestCredentialsFile(t *testing.T) {
	base := filepath.Join(t.TempDir(), "credentials")
	settings := map[string]string{"TEST_CONTAINERS_DEBUG_FILE": base}
	output, err := runProcess(t, "registry", settings)
	if err != nil {
		t.Fatalf("subprocess: %v\n%s", err, output)
	}
	if strings.Contains(output, "SECRET") {
		t.Fatalf("secret in output: %s", output)
	}
	paths, err := filepath.Glob(base + ".*.jsonl")
	if err != nil || len(paths) != 1 {
		t.Fatalf("paths=%v error=%v", paths, err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(paths[0], base+"."+filepath.Base(executable)+".") {
		t.Fatalf("filename lacks binary suffix: %s", paths[0])
	}
	if !strings.Contains(output, paths[0]) {
		t.Fatalf("file not announced: %s", output)
	}
	info, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%v", info.Mode())
	}
	records := readRecords(t, paths[0])
	if len(records) != 3 {
		t.Fatalf("records=%v", records)
	}
	if records[0]["type"] != "container" || records[0]["password"] != "SECRET-password" {
		t.Fatalf("container=%v", records[0])
	}
	if records[1]["type"] != "fixture" || records[1]["token"] != "SECRET-token" || records[1]["container_id"] != "container-id-123" || records[1]["database"] != "private-db" || records[1]["test"] != "TestExample" {
		t.Fatalf("fixture=%v", records[1])
	}
	if records[2]["type"] != "retained" || records[2]["container_id"] != "container-id-123" {
		t.Fatalf("retained=%v", records[2])
	}
	output, err = runProcess(t, "registry", settings)
	if err != nil {
		t.Fatalf("second process: %v\n%s", err, output)
	}
	paths, _ = filepath.Glob(base + ".*.jsonl")
	if len(paths) != 2 {
		t.Fatalf("process filenames not unique: %v", paths)
	}
}

func readRecords(t *testing.T, path string) []map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var records []map[string]string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record map[string]string
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return records
}

func TestExclusiveCreation(t *testing.T) {
	base := filepath.Join(t.TempDir(), "credentials")
	output, err := runProcess(t, "collision", map[string]string{"TEST_CONTAINERS_DEBUG_FILE": base})
	if err != nil || !strings.Contains(output, "COLLISION_SAFE") {
		t.Fatalf("error=%v output=%s", err, output)
	}
}

func TestFileErrorIsSafeAndCached(t *testing.T) {
	output, err := runProcess(t, "config-error", map[string]string{"TEST_CONTAINERS_DEBUG_FILE": filepath.Join(t.TempDir(), "SECRET-missing", "credentials")})
	if err != nil || !strings.Contains(output, "CACHED_ERROR") || strings.Contains(output, "SECRET") {
		t.Fatalf("error=%v output=%s", err, output)
	}
}

func TestAppendErrorIsSafe(t *testing.T) {
	output, err := runProcess(t, "write-error", map[string]string{"TEST_CONTAINERS_DEBUG_FILE": filepath.Join(t.TempDir(), "credentials")})
	if err != nil || !strings.Contains(output, "APPEND_ERROR") || strings.Contains(output, "SECRET") {
		t.Fatalf("error=%v output=%s", err, output)
	}
}

func TestConcurrentRecords(t *testing.T) {
	base := filepath.Join(t.TempDir(), "credentials")
	output, err := runProcess(t, "concurrent", map[string]string{"TEST_CONTAINERS_DEBUG_FILE": base})
	if err != nil {
		t.Fatalf("error=%v output=%s", err, output)
	}
	paths, _ := filepath.Glob(base + ".*.jsonl")
	if len(paths) != 1 {
		t.Fatalf("paths=%v", paths)
	}
	records := readRecords(t, paths[0])
	if len(records) != 120 {
		t.Fatalf("got %d records", len(records))
	}
	counts := make(map[string]int)
	for _, record := range records {
		counts[record["type"]]++
	}
	for _, kind := range []string{"container", "fixture", "retained"} {
		if counts[kind] != 40 {
			t.Fatalf("counts=%v", counts)
		}
	}
}

func TestFailurePolicy(t *testing.T) {
	for _, keep := range []string{"0", "1"} {
		for _, action := range []string{"observe", "preserve", "cleanup", "passing"} {
			t.Run(keep+"/"+action, func(t *testing.T) {
				output, err := runProcess(t, action, map[string]string{"TEST_CONTAINERS_KEEP_ON_FAILURE": keep})
				if (err != nil) != (action != "passing") {
					t.Fatalf("error=%v output=%s", err, output)
				}
				want := "KEEP=false"
				if keep == "1" && action != "passing" {
					want = "KEEP=true"
				}
				if !strings.Contains(output, want) || !strings.Contains(output, "POLICY_OK") {
					t.Fatalf("missing %s or policy confirmation: %s", want, output)
				}
				if err != nil {
					exit, ok := err.(*exec.ExitError)
					if !ok || exit.ExitCode() != 1 {
						t.Fatalf("unexpected subprocess failure: %v\n%s", err, output)
					}
				}
			})
		}
	}
}

func TestCachedRyukConfiguration(t *testing.T) {
	for _, disabled := range []string{"false", "true"} {
		t.Run("disabled="+disabled, func(t *testing.T) {
			output, err := runProcess(t, "ryuk-cached", map[string]string{
				"TEST_CONTAINERS_KEEP_ON_FAILURE": "1",
				"CONTAINERDEBUG_RYUK_DISABLED":    disabled,
			})
			if err != nil || !strings.Contains(output, "RYUK_CHECKED") || strings.Contains(output, "SECRET") {
				t.Fatalf("error=%v output=%s", err, output)
			}
			if disabled == "false" && !strings.Contains(output, "set TESTCONTAINERS_RYUK_DISABLED=true before launching") {
				t.Fatalf("missing actionable cached-Ryuk error: %s", output)
			}
		})
	}
}

func TestRecordResult(t *testing.T) {
	for _, keep := range []string{"0", "1"} {
		for _, code := range []int{0, 1, -1} {
			t.Run(fmt.Sprintf("keep=%s/code=%d", keep, code), func(t *testing.T) {
				output, err := runProcess(t, "result", map[string]string{
					"TEST_CONTAINERS_KEEP_ON_FAILURE": keep,
					"CONTAINERDEBUG_EXIT_CODE":        strconv.Itoa(code),
				})
				if err != nil || !strings.Contains(output, "RESULT_OK") {
					t.Fatalf("error=%v output=%s", err, output)
				}
			})
		}
	}
}

func TestProcess(t *testing.T) {
	action := os.Getenv("CONTAINERDEBUG_ACTION")
	if action == "" {
		return
	}
	if action == "ryuk-cached" {
		disabled := os.Getenv("CONTAINERDEBUG_RYUK_DISABLED") == "true"
		t.Setenv("TESTCONTAINERS_RYUK_DISABLED", strconv.FormatBool(disabled))
		if testcontainers.ReadConfig().Config.RyukDisabled != disabled {
			t.Fatal("unexpected initial Ryuk configuration")
		}
		err := Configure()
		if disabled {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			if err == nil {
				t.Fatal("keep mode accepted cached enabled Ryuk")
			}
			if Configure() != err {
				t.Fatal("Ryuk error not cached")
			}
			fmt.Println(err)
		}
		fmt.Println("RYUK_CHECKED")
		return
	}
	if action == "collision" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		path := os.Getenv("TEST_CONTAINERS_DEBUG_FILE") + "." + filepath.Base(executable) + "." + strconv.Itoa(os.Getpid()) + ".jsonl"
		if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
			t.Fatal(err)
		}
		if Configure() == nil {
			t.Fatal("expected exclusive creation failure")
		}
		content, err := os.ReadFile(path)
		if err != nil || string(content) != "existing" {
			t.Fatal("overwrote existing file")
		}
		fmt.Println("COLLISION_SAFE")
		return
	}
	if action == "config-error" {
		RecordResult(1)
		first := Configure()
		if first == nil {
			t.Fatal("expected file error")
		}
		t.Setenv("TEST_CONTAINERS_DEBUG_FILE", "")
		if second := Configure(); second != first {
			t.Fatal("configuration error not cached")
		}
		if RegisterContainer(Container{Password: "SECRET-password"}) != first || RegisterFixture(Fixture{Token: "SECRET-token"}) != first || Retained("id") != first {
			t.Fatal("registry did not propagate cached error")
		}
		fmt.Println("CACHED_ERROR", first)
		return
	}
	if action == "concurrent" {
		var group sync.WaitGroup
		for index := 0; index < 40; index++ {
			group.Add(1)
			go func() {
				defer group.Done()
				if err := Configure(); err != nil {
					t.Error(err)
				}
			}()
		}
		group.Wait()
	}
	if err := Configure(); err != nil {
		fmt.Println("CONFIG_ERROR", err)
		return
	}
	if os.Getenv("TEST_CONTAINERS_KEEP_ON_FAILURE") == "1" && os.Getenv("TESTCONTAINERS_RYUK_DISABLED") != "true" {
		t.Fatal("Ryuk not disabled")
	}
	if os.Getenv("TEST_CONTAINERS_KEEP_ON_FAILURE") == "1" && !testcontainers.ReadConfig().Config.RyukDisabled {
		t.Fatal("effective Ryuk configuration not disabled")
	}
	t.Setenv("TEST_CONTAINERS_DEBUG", "SECRET-invalid-after-config")
	if err := Configure(); err != nil {
		t.Fatal("configuration was not cached")
	}
	switch action {
	case "result":
		code, err := strconv.Atoi(os.Getenv("CONTAINERDEBUG_EXIT_CODE"))
		if err != nil {
			t.Fatal(err)
		}
		if KeepOnFailure() {
			t.Fatal("retaining before package result")
		}
		var group sync.WaitGroup
		for index := 0; index < 40; index++ {
			group.Add(1)
			go func() {
				defer group.Done()
				RecordResult(code)
				RecordResult(0)
				KeepOnFailure()
			}()
		}
		group.Wait()
		want := code != 0 && os.Getenv("TEST_CONTAINERS_KEEP_ON_FAILURE") == "1"
		if KeepOnFailure() != want {
			t.Fatal("incorrect package result retention")
		}
		RecordResult(0)
		if KeepOnFailure() != want {
			t.Fatal("success cleared earlier package failure")
		}
		fmt.Println("RESULT_OK")
	case "write-error":
		if err := state.file.Close(); err != nil {
			t.Fatal(err)
		}
		err := RegisterContainer(Container{ID: "id", Password: "SECRET-password"})
		if err == nil {
			t.Fatal("expected append failure")
		}
		fmt.Println("APPEND_ERROR", err)
	case "registry":
		if err := RegisterContainer(Container{Kind: "postgres", ID: "container-id-123", Endpoint: "localhost:5432", Username: "test-user", Password: "SECRET-password"}); err != nil {
			t.Fatal(err)
		}
		if err := RegisterFixture(Fixture{Kind: "spicedb", ContainerID: "container-id-123", Test: "TestExample", Database: "private-db", Token: "SECRET-token"}); err != nil {
			t.Fatal(err)
		}
		if err := Retained("container-id-123"); err != nil {
			t.Fatal(err)
		}
	case "concurrent":
		var group sync.WaitGroup
		for index := 0; index < 40; index++ {
			group.Add(1)
			go func() {
				defer group.Done()
				if err := RegisterContainer(Container{ID: "concurrent-id"}); err != nil {
					t.Error(err)
				}
				if err := RegisterFixture(Fixture{ContainerID: "concurrent-id"}); err != nil {
					t.Error(err)
				}
				if err := Retained("concurrent-id"); err != nil {
					t.Error(err)
				}
			}()
		}
		group.Wait()
	case "observe", "preserve", "cleanup", "passing":
		if KeepOnFailure() {
			t.Fatal("retaining without failure")
		}
		t.Run("fixture", func(t *testing.T) {
			t.Cleanup(func() { fmt.Printf("KEEP=%v\n", KeepOnFailure()) })
			Observe(t)
			if action == "cleanup" {
				t.Cleanup(func() { t.Error("intentional cleanup failure") })
			}
			if PreserveTest(t) {
				t.Fatal("preserved passing test")
			}
			if action != "passing" && action != "cleanup" {
				t.Error("intentional subprocess failure")
			}
			if action == "preserve" {
				want := os.Getenv("TEST_CONTAINERS_KEEP_ON_FAILURE") == "1"
				if PreserveTest(t) != want || KeepOnFailure() != want {
					t.Error("incorrect immediate preservation decision")
					return
				}
			}
			fmt.Println("POLICY_OK")
		})
	default:
		t.Fatalf("unknown action %q", action)
	}
}
