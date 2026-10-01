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

func TestDefaultBaselineRejectsSlowConsumerCase(t *testing.T) {
	r := result{Protocol: "consumer", Scenario: "consumer", Concurrency: 8, Operations: map[string]stats{"update_common": {Count: 240, P95MS: 5.01}}}
	if err := checkBaseline(r); err == nil || !strings.Contains(err.Error(), "consumer/8/update_common") {
		t.Fatalf("expected consumer baseline failure, got %v", err)
	}
}

func TestSummarizeIncludesMinimumMeanAndMaximum(t *testing.T) {
	got := summarize([]time.Duration{3 * time.Millisecond, time.Millisecond, 2 * time.Millisecond})
	if got.Count != 3 || got.MinMS != 1 || got.MeanMS != 2 || got.MaxMS != 3 {
		t.Fatalf("unexpected latency summary: %+v", got)
	}
}
