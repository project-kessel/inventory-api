//go:build performance

package performance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBaselineRejectsSlowAndMissingOperations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, []byte(`{"fresh/1/create":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PERF_BASELINE", path)
	r := result{Protocol: "grpc", Scenario: "fresh", Concurrency: 1, Operations: map[string]stats{"create": {Count: 10, P95MS: 2}}}
	if err := checkBaseline(r); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected p95 failure, got %v", err)
	}
	r.Operations = map[string]stats{"delete": {Count: 10, P95MS: .5}}
	if err := checkBaseline(r); err == nil || !strings.Contains(err.Error(), "missing baseline") {
		t.Fatalf("expected missing baseline failure, got %v", err)
	}
}

func TestDefaultBaselineRejectsSlowRepositoryCase(t *testing.T) {
	r := result{Protocol: "repository", Scenario: "consumer", Concurrency: 8, Operations: map[string]stats{"reporter_previous": {Count: 240, P95MS: 5.01}}}
	if err := checkBaseline(r); err == nil || !strings.Contains(err.Error(), "consumer/8/reporter_previous") {
		t.Fatalf("expected repository baseline failure, got %v", err)
	}
}

func TestSmallProfileSkipsLatencyLimits(t *testing.T) {
	for _, scenario := range []string{"fresh", "consumer"} {
		t.Run(scenario, func(t *testing.T) {
			r := result{Scenario: scenario, Concurrency: 1, Settings: settings{Profile: "small"}, Operations: map[string]stats{}}
			operations := []string{"create", "update", "delete", "recreate"}
			if scenario == "consumer" {
				operations = []string{"resource", "common", "reporter_current", "reporter_previous", "combined", "reporter_revival", "reporter_delete"}
			}
			for _, operation := range operations {
				r.Operations[operation] = stats{Count: 3, P95MS: 1000}
			}
			if err := checkBaseline(r); err != nil {
				t.Fatalf("small profile should accept slow samples: %v", err)
			}
			r.Settings.Profile = ""
			if err := checkBaseline(r); err == nil || !strings.Contains(err.Error(), "exceeds") {
				t.Fatalf("full profile should reject slow samples: %v", err)
			}
			r.Settings.Profile = "small"
			r.Operations["unexpected"] = stats{Count: 3}
			if err := checkBaseline(r); err == nil || !strings.Contains(err.Error(), "missing baseline") {
				t.Fatalf("small profile should still validate baseline entries: %v", err)
			}
		})
	}
}

func TestBaselineRejectsStaleOperation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, []byte(`{"fresh/1/create":50,"fresh/1/obsolete":50,"fresh/4/update":50}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PERF_BASELINE", path)
	r := result{Protocol: "grpc", Scenario: "fresh", Concurrency: 1, Operations: map[string]stats{"create": {Count: 10, P95MS: 2}}}
	if err := checkBaseline(r); err == nil || !strings.Contains(err.Error(), "stale baseline: fresh/1/obsolete") {
		t.Fatalf("expected stale baseline failure, got %v", err)
	}
}

func TestSummarizeIncludesMinimumMeanAndMaximum(t *testing.T) {
	got := summarize([]time.Duration{3 * time.Millisecond, time.Millisecond, 2 * time.Millisecond})
	if got.Count != 3 || got.MinMS != 1 || got.MeanMS != 2 || got.MaxMS != 3 {
		t.Fatalf("unexpected latency summary: %+v", got)
	}
}
