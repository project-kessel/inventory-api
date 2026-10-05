//go:build performance

// Command summary prints the latency results produced by the performance suite.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type operationStats struct {
	Count  int     `json:"count"`
	MinMS  float64 `json:"min_ms"`
	MeanMS float64 `json:"mean_ms"`
	P50MS  float64 `json:"p50_ms"`
	P95MS  float64 `json:"p95_ms"`
	P99MS  float64 `json:"p99_ms"`
	MaxMS  float64 `json:"max_ms"`
}

type result struct {
	Scenario      string           `json:"scenario"`
	Concurrency   int              `json:"concurrency"`
	DatabaseBytes int64            `json:"database_bytes"`
	TableBytes    map[string]int64 `json:"table_bytes"`
	Settings      struct {
		Background int    `json:"background"`
		History    int    `json:"history"`
		Profile    string `json:"profile"`
	} `json:"settings"`
	Operations          map[string]operationStats `json:"operations"`
	Throughput          float64                   `json:"throughput_per_second"`
	ReadParallelWorkers *int                      `json:"read_parallel_workers"`
}

func main() {
	dir := os.Getenv("PERF_RESULTS_DIR")
	if dir == "" {
		dir = "results"
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join("test", "performance", dir)
	}

	fmt.Println("Resource lifecycle latency (ms); samples are timed requests, not seeded rows")
	fmt.Printf("%-9s %7s %-17s %7s %9s %9s %9s %9s %9s %9s\n", "scenario", "workers", "operation", "samples", "min", "average", "p50", "p95", "p99", "max")
	for _, scenario := range []string{"fresh", "history"} {
		for _, workers := range []int{1, 4} {
			path := filepath.Join(dir, fmt.Sprintf("grpc-%s-%d.json", scenario, workers))
			b, err := os.ReadFile(path)
			if err != nil {
				fail("read %s: %v", path, err)
			}
			var r result
			if err := json.Unmarshal(b, &r); err != nil {
				fail("parse %s: %v", path, err)
			}
			if r.Scenario != scenario || r.Concurrency != workers {
				fail("unexpected scenario or concurrency in %s", path)
			}
			if r.Settings.Profile == "small" {
				fmt.Println("  SMALL PROFILE: latency limits skipped; timings are informational, correctness checks remain enabled.")
			}
			for _, operation := range []string{"create", "update", "delete", "recreate"} {
				stat, ok := r.Operations[operation]
				if !ok {
					fail("missing %s samples in %s", operation, path)
				}
				fmt.Printf("%-9s %7d %-17s %7d %9.2f %9.2f %9.2f %9.2f %9.2f %9.2f\n", scenario, workers, operation, stat.Count, stat.MinMS, stat.MeanMS, stat.P50MS, stat.P95MS, stat.P99MS, stat.MaxMS)
			}
			history := 0
			if scenario == "history" {
				history = r.Settings.History
			}
			fmt.Printf("  seed: %d background resources, %d history representations", r.Settings.Background, history)
			if history > 0 {
				fmt.Printf(" (%d-%d per target)", history/workers, (history+workers-1)/workers)
			}
			if r.DatabaseBytes > 0 {
				fmt.Printf("; database %.2f GiB", float64(r.DatabaseBytes)/(1<<30))
			} else {
				fmt.Print("; database size unavailable in saved result")
			}
			fmt.Printf("; %.1f successful requests/s\n", r.Throughput)
		}
	}
	fmt.Println("Consumer repository reads (ms); excludes Kafka, tuple calculation, and Relations API")
	fmt.Printf("%-9s %7s %-21s %7s %9s %9s %9s %9s %9s %9s\n", "scenario", "clients", "lookup", "samples", "min", "average", "p50", "p95", "p99", "max")
	for _, clients := range []int{1, 4, 8} {
		path := filepath.Join(dir, fmt.Sprintf("repository-consumer-%d.json", clients))
		b, err := os.ReadFile(path)
		if err != nil {
			fail("read %s: %v", path, err)
		}
		var r result
		if err := json.Unmarshal(b, &r); err != nil {
			fail("parse %s: %v", path, err)
		}
		if r.Scenario != "consumer" || r.Concurrency != clients || r.ReadParallelWorkers == nil {
			fail("unexpected consumer result in %s", path)
		}
		if r.Settings.Profile == "small" {
			fmt.Println("  SMALL PROFILE: latency limits skipped; timings are informational, correctness checks remain enabled.")
		}
		for _, operation := range []string{"resource", "common", "reporter_current", "reporter_previous", "combined", "reporter_revival", "reporter_delete"} {
			stat, ok := r.Operations[operation]
			if !ok {
				fail("missing %s samples in %s", operation, path)
			}
			fmt.Printf("%-9s %7d %-21s %7d %9.2f %9.2f %9.2f %9.2f %9.2f %9.2f\n", "consumer", clients, operation, stat.Count, stat.MinMS, stat.MeanMS, stat.P50MS, stat.P95MS, stat.P99MS, stat.MaxMS)
		}
		fmt.Printf("  seed: %d background resources, %d reporter history rows + 3 revival rows and %d common versions under one target; database %.2f GiB; max_parallel_workers_per_gather=%d; %.1f successful reads/s\n", r.Settings.Background, r.Settings.History, r.Settings.History, float64(r.DatabaseBytes)/(1<<30), *r.ReadParallelWorkers, r.Throughput)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
