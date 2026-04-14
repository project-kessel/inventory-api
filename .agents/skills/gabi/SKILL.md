---
name: gabi-sql-query
description: Run SQL queries against the Kessel Inventory database in stage or prod using the gabi CLI. Use when the user mentions gabi, querying the database, checking stage/prod data, or investigating inventory resources, reporters, or representations.
---

# Gabi SQL Query Tool

Run read-only SQL queries against the Kessel Inventory database in stage or prod using the `gabi` CLI.

## Workflow

**BEFORE running any query, you MUST:**

1. **Verify gabi is installed**: Run `gabi version`. If missing, tell the user to install it from [gabi-cli](https://github.com/app-sre/gabi-cli).
2. **Verify gabi is configured**: Run `gabi config currentprofile`. If no profile exists, guide the user through setup (see below).
3. **Confirm environment**: User must specify `stage` or `prod`. Show the user the URL from `gabi config currentprofile` and ask them to confirm it points to the intended environment.
4. **Only after all are confirmed**: Proceed with running the query.

**Safety**: `gabi` provides read-only database access. DML statements (INSERT, UPDATE, DELETE, etc.) are not permitted.

**Read replica**: In production, gabi connects to a read replica, not the primary. Features that exist only on the primary (replication slots, WAL stats, etc.) will not be available.

**Query timeout**: Queries are subject to a 30-second timeout. Prefer small, targeted queries over large joins or full-table scans. Break complex investigations into sequential steps rather than a single monolithic query.

**Output format**: `gabi exec` returns JSON (array of objects). Prefer selecting specific columns over `SELECT *` on tables with jsonb fields (`data`, `metrics`, `payload`), and always use `LIMIT`. Tokens expire -- if queries fail with auth errors, refresh with `gabi config settoken <new-token>`.

## Setup (if not configured)

```bash
gabi config init
gabi config seturl <gabi-kessel-route>       # from OpenShift Console route
gabi config settoken <sha256-login-token>     # from OpenShift "Copy login command"
gabi exec "SELECT 1"                          # verify connectivity
```

## Usage

```bash
gabi exec "<SQL>"
```

## Database Schema

Discover tables and columns dynamically:

```sql
SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name
```

```sql
SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '<table>' ORDER BY ordinal_position
```

### Table relationships

```
resource
  └─ reporter_resources (resource_id FK → resource.id)
       └─ reporter_representations (reporter_resource_id FK → reporter_resources.id)
  └─ common_representations (resource_id FK → resource.id)

outbox_events — CDC outbox for Kafka; typically empty (events are consumed and deleted)
metrics_summaries — periodic metrics snapshots; metrics column is large jsonb
```

Always query `information_schema.columns` to discover the actual column names and types before writing queries against a table.

## Investigation: Trace a Resource by Local ID

Use these queries in sequence to trace a resource from its local reporter ID through the full lifecycle.

**Step 1**: Find the reporter resource

```sql
SELECT id, resource_id, reporter_type, resource_type, reporter_instance_id, tombstone FROM reporter_resources WHERE local_resource_id = '<local-id>'
```

**Step 2**: Get the parent resource (using `resource_id` from step 1)

```sql
SELECT id, type, ktn, common_version, created_at, updated_at FROM resource WHERE id = '<resource-id-from-step-1>' LIMIT 1
```

**Step 3**: Get the latest common representation (using `resource_id`)

```sql
SELECT version, reported_by_reporter_type, reported_by_reporter_instance, transaction_id, created_at, data FROM common_representations WHERE resource_id = '<resource-id>' ORDER BY version DESC LIMIT 1
```

**Step 4**: Get the latest reporter representation (using `reporter_resources.id` from step 1)

```sql
SELECT version, generation, reporter_version, common_version, transaction_id, tombstone, created_at, data FROM reporter_representations WHERE reporter_resource_id = '<reporter-resource-id>' ORDER BY version DESC LIMIT 1
```

**Step 5**: Check outbox events (often empty -- events are transient)

```sql
SELECT id, aggregatetype, operation, txid, payload FROM outbox_events WHERE aggregateid = '<resource-id>' ORDER BY id DESC LIMIT 10
```
