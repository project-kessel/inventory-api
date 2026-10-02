# Performance baseline suite

Run the full suite with `make test-performance`; it detects Docker or rootless Podman automatically. For Podman, start the socket with `systemctl --user start podman.socket` first. Use `make test-performance-podman` to force Podman, or set `DOCKER_HOST` to select a specific endpoint. CI runs the standard target on pull requests and `main` and uploads the JSON results.

The suite starts PostgreSQL 16.6 through Testcontainers and applies the real migrations. The gRPC workload measures resource create, update, delete, and recreate at 1 and 4 workers in fresh and history scenarios. The consumer workload calls `ProcessMessage` directly at 1, 4, and 8 clients for create, workspace-changing update, unchanged update, and delete events. Its Relations backend is stateless; Kafka polling and delivery are outside the timed calls.

## Fixture shape and tuning

Each scenario truncates and reseeds the tables before timing; rows from earlier scenarios do not accumulate. By default, `PERF_BACKGROUND=100000` inserts 100,000 background resources. Each has one common representation, one reporter resource, and one reporter representation. These rows add database volume but are not the resources receiving measured requests.

`PERF_HISTORY=227872` adds that many reporter representation versions to the history target resources. The gRPC history scenario divides the total among its workers: one target has all 227,872 versions with one worker; four targets have 56,968 each with four workers. The fresh scenario has no extra history. The consumer scenario puts all 227,872 versions under **one** target; its 1, 4, and 8 clients all repeatedly process the same event payloads for that target, not one target per client. This intentionally measures a warm, concentrated workload; varied resources and versions would require another scenario. It also adds two common representation versions needed by its event cases. All targets use the same reporter type and instance (`hbi`/`perf`); worker and client counts do not represent reporter counts.

`PERF_PAYLOAD_BYTES=1024` controls the reporter payload in seeded rows and measured gRPC requests. The payload is repeated `x` characters, not random data, so PostgreSQL compresses it well and increasing this setting may add less disk usage than expected. `PERF_BACKGROUND` changes the number of background resources; `PERF_HISTORY` changes the total extra history versions. `PERF_ROUNDS`, `PERF_UPDATES`, and `PERF_READ_ITERATIONS` change measured work rather than the initial seed. For example, `PERF_BACKGROUND=300000 PERF_HISTORY=500000 PERF_PAYLOAD_BYTES=2048 make test-performance` runs a larger fixture. Check the database size in the result summary when tuning toward production scale.

This branch's consumer reads common representations, so its measurements do not exercise the previous-reporter query, even though reporter history is present in the database.

`baseline.json` sets provisional p95 limits of 50 ms for gRPC lifecycle operations and 5 ms for consumer operations. The test fails on operation errors, missing consumer samples, missing or stale baseline entries for a measured case, and p95 values above those limits. Calibrate the limits from repeated runs on the CI runner before treating them as service objectives.

`PERF_PROFILE=small make test-performance` runs a quick harness check. `PERF_BACKGROUND`, `PERF_HISTORY`, `PERF_PAYLOAD_BYTES`, `PERF_ROUNDS`, `PERF_UPDATES`, `PERF_READ_ITERATIONS`, `PERF_READ_PARALLEL_WORKERS`, and `PERF_REQUEST_TIMEOUT_SECONDS` override workload settings. `PERF_BASELINE` selects another baseline file. Results are written to `test/performance/results/` by default; `PERF_RESULTS_DIR` changes the output directory. The Make target prints sample counts, min/average/p50/p95/p99/max latency, database size, and throughput after a passing run.

The default history fixture occupies about 0.53 GiB locally; its repeated payload compresses well. It demonstrates a history-heavy workload but does not match the roughly 10 GiB production database.
