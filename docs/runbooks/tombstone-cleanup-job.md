# Tombstone and orphan cleanup

This one-time PostgreSQL job removes reporter parents that are still tombstoned
and were last updated more than 30 days ago (configurable). It removes all their
reporter history by the existing FK cascade. It removes common history and the
resource root only for resource IDs touched by actual reporter deletions and
only when no reporter parent remains, including recent tombstones. Unrelated
pre-existing orphans are out of scope. All reporters and resource types are included.
There is no keep-two or seven-day representation policy in this job.

The age cutoff is frozen at invocation start, uses reporter_resources.updated_at,
and excludes timestamps equal to the cutoff or NULL. A changed/revived worklist
entry is skipped, even if it becomes tombstoned again. A report after a completed
purge creates fresh Inventory state; historical transaction IDs and generations
are not retained. SQL deletion does not emit a replacement tuple event.

## Online execution prerequisites

API writes, KIC, Debezium and the internal consumer remain enabled. An old
tombstone is not by itself proof that its delete event was processed: the internal
consumer still joins through the reporter parent to read common history.

Before each execution:

1. Verify the target cluster/namespace, deployed image, database backup/recovery
   access, metrics and alerts. Run one cleanup invocation at a time. Confirm TEMP
   and DML privileges and available PostgreSQL temporary storage. Worklists live
   in PostgreSQL, not the Job's memory or /tmp.
2. Record a UTC time and source WAL boundary while the pipeline is healthy. Verify
   the connector checkpoint has published all transactions through that boundary.
   Then capture the resulting exclusive Kafka end position for every tuple-topic
   partition. Verify the actual connector, slot, subscriptions and consumer group
   in the target environment. Do not assume stage names apply in production.
3. Verify successful internal-consumer processing through those positions, without
   skipped/failed replication. Verify committed restart positions also pass the
   last boundary event in each partition. The current consumer commits raw event
   offsets: processing the final event is not necessarily a restart-safe commit.
   With a quiet partition, wait for subsequent normal traffic and its committed
   progress; do not reset offsets to manufacture evidence. If publication,
   processing or restart safety cannot be established, do not execute.
4. Ensure the cleanup cutoff predates the captured boundary. Do not backdate or
   import reporter state during cleanup. Do not rewind consumer offsets, reset
   connector checkpoints or replay old events into purged history. Stop the job
   if the pipeline loses the verified boundary or starts replaying older data.
5. Run preview and then a capped execution before increasing the amount of work.
   Monitor consumer errors/lag, authorization outcomes, database CPU, I/O, storage,
   lock waits and batch timings. An idle pod or low lag alone is not drain proof.

`--consumer-boundary-verified` is an operator acknowledgment, not an automatic
pipeline check. It does not require pausing writes or disabling the consumer.
The policy comes from the [cleanup experiment](https://docs.google.com/document/d/18xugoI-Tv_YEsegl0QufNQyRw7xrChZbVW0-gqK0rEY/edit?tab=t.pwyzu9l7nrdf)
and the [30-day proposal](https://redhat-internal.slack.com/archives/C059NUD49PD/p1790942240731049).

## Commands

Preview is the default and runs a read-only transaction. It counts parents,
reporter history, affected resource IDs, removable common history and roots.
For a capped invocation, the main impact counts describe the oldest candidate
prefix; total eligibility is reported separately. These are snapshot estimates,
not promises about a later execution while writes continue.

```sh
inventory-api run-job tombstoned-resources-cleanup-job --tombstone-days=30
inventory-api run-job tombstoned-resources-cleanup-job --tombstone-days=30 --max-batches=1
inventory-api run-job tombstoned-resources-cleanup-job --tombstone-days=30 \
  --execute --consumer-boundary-verified --batch-size=1000 --max-batches=1
```

Omit `--max-batches` or set it to 0 to process the whole frozen worklist. Default
batch delay is 500ms. There is no job duration or statement-runtime limit.
Parents are processed oldest updated_at first, with parent ID as the deterministic
tie-breaker. Worklist selection reads reporter parents only; it does not count or
rank their history. History sizes do not affect processing order.

Batch size counts **reporter parents**: their complete histories and newly orphaned
common history can greatly exceed that count. Measure deep-history batches before
scaling up. Deletion does not itself reclaim disk; vacuum/repack is separate work.

Each batch is one serializable transaction with existing serialization retries.
Changes to reporter identity, update timestamp, generation or representation
version cause the candidate to be skipped. Reporter cascade, common/root cleanup
and worklist draining commit together. Logs report committed counts only, selected
and skipped parents, unique affected resources, durations and remaining work.
Completion means this invocation's worklist was exhausted, not that concurrent
traffic cannot produce more eligible rows later.

SIGINT/SIGTERM cancels SQL or pacing; previously committed batches remain deleted.
A failed batch rolls back. Rerun after verifying the boundary again; a fresh
worklist selects remaining eligible parents. Temporary tables are dropped even
on failure/cancellation. Never interpret a failed or cancelled Job as a completed
cleanup; inspect committed counts and run a new preview.

## OpenShift

```sh
oc process -f deploy/kessel-inventory-tombstone-cleanup-job.yaml \
  -p JOB_NAME=tombstone-preview-YYYYMMDD -p IMAGE_TAG=IMMUTABLE_TAG \
  | oc apply -n YOUR_NAMESPACE -f -
```

Use a new Job name for every run or changed configuration. For execution add
`-p EXECUTE=true -p CONSUMER_BOUNDARY_VERIFIED=true -p MAX_BATCHES=1` initially.
The template reuses Inventory's configuration Secrets and service account, with
no mounted API token. Use the same template via app-interface when approved for
stage/prod; pin the template ref and image to the reviewed build.

After execution, verify active/recent tombstone samples, shared roots, expected
per-table deletions, current authorization and continued report/delete processing.
Run another preview and retain logs, boundary evidence and batch timings.
