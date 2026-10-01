# Performance baseline suite

Run the full suite with `make test-performance` (Docker) or `make test-performance-podman` (rootless Podman). For Podman, start the socket with `systemctl --user start podman.socket` first. The Make target configures `DOCKER_HOST` and the Testcontainers reaper for that run. CI runs `make test-performance` on pull requests and `main` and uploads the JSON results.

The suite starts PostgreSQL 16.6 through Testcontainers, applies the real migrations, and seeds 100,000 background resources plus 227,872 reporter history versions by default. The gRPC workload measures resource create, update, delete, and recreate at 1 and 4 workers in fresh and history scenarios. The consumer workload calls `ProcessMessage` directly at 1, 4, and 8 clients for create, workspace-changing update, unchanged update, and delete events. Its Relations backend is stateless; Kafka polling and delivery are outside the timed calls. This branch's consumer reads common representations, so these measurements do not exercise the previous-reporter query.

`baseline.json` sets provisional p95 limits of 1000 ms for gRPC lifecycle operations and 5 ms for consumer operations. The test fails on operation errors, missing consumer samples or baseline entries, and p95 values above those limits. Calibrate the limits from repeated runs on the CI runner before treating them as service objectives.

`PERF_PROFILE=small make test-performance` runs a quick harness check. `PERF_BACKGROUND`, `PERF_HISTORY`, `PERF_PAYLOAD_BYTES`, `PERF_ROUNDS`, `PERF_UPDATES`, `PERF_READ_ITERATIONS`, `PERF_READ_PARALLEL_WORKERS`, and `PERF_REQUEST_TIMEOUT_SECONDS` override workload settings. `PERF_BASELINE` selects another baseline file. Results are written to `test/performance/results/` by default; `PERF_RESULTS_DIR` changes the output directory. The Make target prints sample counts, min/average/p50/p95/p99/max latency, database size, and throughput after a passing run.

The default history fixture occupies about 0.53 GiB locally; its repeated payload compresses well. It demonstrates a history-heavy workload but does not match the roughly 10 GiB production database.
