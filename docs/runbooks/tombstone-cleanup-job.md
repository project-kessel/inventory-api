# Tombstone and orphan cleanup

This one-time PostgreSQL job removes reporter parents that are still tombstoned
and were last updated more than 30 days ago (configurable), together with their
reporter history. When the last reporter of a resource is removed, the job also
removes that resource's common history and its `resource` row. A resource that
keeps any other reporter, including a recent tombstone, keeps its root and common
history. Pre-existing orphans are out of scope. All reporters and resource types
are included.

The cutoff is frozen when the job starts and compares against
`reporter_resources.updated_at`. Timestamps equal to the cutoff, or NULL, are not
eligible. A purged resource that is reported again gets fresh Inventory state.
SQL deletion does not emit replacement tuple events.

## How a run works

1. The job selects eligible parents into a temporary worklist, oldest
   `updated_at` first with the parent ID breaking ties. A capped run stores only
   the parents it can reach.
2. It takes the next batch of `rows-per-transaction / 5` parents (200 by default).
3. It trims the batch's history in transactions of at most `rows-per-transaction`
   rows. Each parent keeps its current reporter snapshot. A root whose reporters
   are all in the batch keeps its referenced common version and its latest common
   version. A parent with 100,000 history rows therefore spans about 100
   transactions instead of one large one.
4. One final transaction deletes the remaining snapshots, the parents and the
   roots left without a reporter, then removes the batch from the worklist. After
   trimming, each parent has at most five rows left, so this transaction also fits
   the budget. If history was added during the batch and the transaction would
   exceed the budget, it rolls back and the job fails; rerun it.

Every transaction is serializable and rechecks eligibility. A parent revived or
updated during its batch is skipped and keeps its current snapshot. History that
was trimmed before the change stays deleted. A root that gains a reporter during
the batch keeps its root and remaining common history.

`--batch-delay` (default 500ms) applies between batches and between full history
chunks. Every transaction sets `statement_timeout=0`, overriding inherited database
and role timeouts, and `lock_timeout='5s'`. The initial connection ping times out
after 30 seconds. The OpenShift template sets no `activeDeadlineSeconds`.
Deleting rows does not reclaim disk space; vacuum and repack are separate work.

## Online execution

API writes, KIC, Debezium and the internal consumer stay enabled. This relies on
expired tombstones not changing, because HBI hard-deletes on its side. Execution
requires only `--execute`. The job does not establish or verify a consumer
checkpoint.

The configured age does not prove that replication has completed. The consumer
still reads reporter parents and common history when it processes events. Purging
history that an unprocessed or replayed event needs can prevent authorization
cleanup. Healthy consumer lag does not rule this out.

Before execution:

1. Verify the target cluster and namespace, deployed image, database backup and
   recovery access, metrics and alerts. Run one invocation at a time. Confirm TEMP
   and DML privileges and available PostgreSQL temporary storage; the worklist
   lives in PostgreSQL, not in the Job's memory.
2. Do not start while the consumer is failing, lagging, or replaying from
   rewound offsets.
3. Run a preview, then a capped execution, before increasing the amount of work.
   Monitor consumer errors and lag, authorization outcomes, database CPU, I/O,
   storage, lock waits and batch timings.

The policy comes from the [cleanup experiment](https://docs.google.com/document/d/18xugoI-Tv_YEsegl0QufNQyRw7xrChZbVW0-gqK0rEY/edit?tab=t.pwyzu9l7nrdf)
and the [30-day proposal](https://redhat-internal.slack.com/archives/C059NUD49PD/p1790942240731049).

## Commands

Preview is the default and runs in a read-only transaction. It reports every
eligible parent, the reporter history, the affected resource IDs, and the common
history and roots that would be removed. Counts are estimates while writes
continue. For a capped run, the configuration log shows `max_parents`, the most
parents that run can remove.

```sh
inventory-api run-job tombstoned-resources-cleanup-job --tombstone-days=30
inventory-api run-job tombstoned-resources-cleanup-job --tombstone-days=30 \
  --execute --rows-per-transaction=1000 --max-batches=1
```

`--max-batches=0` (the default) processes every eligible parent. Rows per
transaction must be between 5 and 50,000.

## Logs and recovery

The job logs its configuration, worklist selection, the start of each batch, each
committed history chunk, each committed batch, and a final result. Every entry
carries committed per-table deletion counts. The final entry has
`status=succeeded` or `status=failed`; failures log at ERROR. `complete=false` on
a successful run means eligible parents remain beyond the cap.

SIGINT or SIGTERM cancels the active SQL or delay. Committed transactions stay
deleted and the active transaction rolls back. A parent interrupted mid-trim keeps
its current snapshot and stays eligible, so a rerun selects a fresh worklist and
continues. Never read a failed or cancelled Job as a completed cleanup; inspect
its committed counts and run a new preview.

## OpenShift

```sh
oc process -f deploy/kessel-inventory-tombstone-cleanup-job.yaml \
  -p JOB_NAME=tombstone-preview-YYYYMMDD -p IMAGE_TAG=IMMUTABLE_TAG \
  | oc apply -n YOUR_NAMESPACE -f -
```

Use a new Job name for every run or configuration change. For the first execution
add `-p EXECUTE=true -p MAX_BATCHES=1`. The template reuses Inventory's
configuration Secrets and service account and mounts no API token. Use the same
template through app-interface when approved for stage or prod, pinning the
template ref and image to the reviewed build.

After execution, check samples of active and recent tombstones and shared roots,
compare per-table deletions with the logs, and confirm current authorization and
continued report and delete processing. Run another preview and keep the logs.
