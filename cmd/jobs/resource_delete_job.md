# Resource Delete Job 

Batch deletion job for removing resources from the database by resource type and reporter type. Handles millions of records safely using 5000 row batches with 100ms delays between batches.

## WARNING

**This job ONLY works for reporters with a single resource type.**

1. The CommonRepresentation deletion phase filters by `reporter_type` only, NOT by `resource_type`. This is intentional for the initial implementation.

**Impact**: If a reporter has multiple resource types (e.g., both "host" and "k8s-cluster"), running this job will delete CommonRepresentations for ALL resource types, not just the one specified.

2. The Resource deletion phase filters by `resource_type` only, NOT by `reporter_type`. This is intentional for the initial implementation.

**Impact**: If a resource type is reported by multiple reporters (e.g., both "hbi" and "ocm"), running this job will delete Resources for ALL reporters, not just the one specified.

## When to Use This Job

**Use when:**
- Deleting a single resource type, reported by a single reporter
- Deleting any size of datasets (1-1M+ records)
- Need batched operations to minimize database impact

**Don't use when:**
- Reporter has multiple resource types (see constraint above)
- Deleting a single resource type, reported by more than one reporter
- Need fine-grained filtering beyond `resource_type` and `reporter_type`

These limitations are not meant to be immutable. They are a first-pass solution that allows us to delete large sets of resources from the database. **PRs are welcome to help improve logic to support more complex scenarios.**

## Running Locally

### 1. Ensure you have postgres running locally
```bash
> make db/setup
> make migrate
```

### 2. Insert your records into the database

You can do this via any means you prefer

### 3. Build the Inventory API binary
```bash
make local-build
```

### 4. Run the job via the Inventory API binary
```bash
./bin/inventory-api run-job resource-delete-job \
  --resource-type=host \ # replace with your resource type
  --reporter-type=hbi \ # replace with your reporter type
  --config .inventory-api.yaml \
  --dry-run=false
```

## Prerequisites

### 1. Verify Single Resource/Reporter Type Constraint

Run this query to verify the reporter only touches one resource type:

```sql
SELECT resource_type, COUNT(*) as count
FROM reporter_resources
WHERE reporter_type = '<your-reporter-type>'
GROUP BY resource_type;
```

Run this query to verify the resource type is only being reported by one reporter:
```sql
SELECT reporter_type, COUNT(*) as count
FROM reporter_resources
WHERE resource_type = '<your-resource-type>'
GROUP BY reporter_type;
```

**Safe to proceed**: Queries return exactly ONE row matching your `resource-type` and `reporter-type`.

## Usage

### Step 1: Dry-Run

```bash
./inventory-api run-job resource-delete-job \
  --resource-type=host \
  --reporter-type=hbi \
  --dry-run \
  --config .inventory-api.yaml
```

### Step 2: Execute Deletion

```bash
./inventory-api run-job resource-delete-job \
  --resource-type=host \
  --reporter-type=hbi \
  --config .inventory-api.yaml
```

**Delete Processes:**
- **Phase 1**: Deletes ReporterResource records (CASCADE auto-deletes ReporterRepresentation)
- **Phase 2**: Deletes CommonRepresentation records (filters by reporter_type only)
- **Phase 3**: Deletes Resource records (filters by resource_type only)

### Step 3: Verify Completion

```sql
-- Should all return 0
SELECT COUNT(*) FROM reporter_resources
WHERE resource_type = 'host' AND reporter_type = 'hbi';

SELECT COUNT(*) FROM common_representations
WHERE reported_by_reporter_type = 'hbi';

SELECT COUNT(*) FROM resource
WHERE type = 'host';
```

## If the Job Fails

**Safe to re-run**: The job is idempotent. If it fails midway:

1. Check logs to identify which phase failed
2. Fix the underlying issue (connection, disk space, etc.)
3. Re-run the exact same command
4. Job will pick up where it left off (only deletes remaining records)

**Why re-running is safe:**
- WHERE clauses only match remaining records
- Already-deleted records are skipped
- Each batch is atomic (success or rollback)

**Known limitation**: No transaction wrapper across all phases. If Phase 1 completes but Phase 2 fails, you'll have temporary inconsistency until you re-run to completion.

## Troubleshooting

**Dry-run shows zero records**: Check spelling of resource_type and reporter_type (case-sensitive).

**Wrong counts after completion**: New data may have been inserted during deletion. Re-run to clean up.

## Safety Checklist

Before running in production:

- [ ] Verified reporter has only ONE resource type (SQL query above)
- [ ] Verified resource type is only being reported by one reporter (SQL query above)
- [ ] Ran dry-run and validated counts
- [ ] Ready to monitor logs during execution
