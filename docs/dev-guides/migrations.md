## Database Migrations

Inventory uses gormigrate on top of GORM

### Running migrations
```sh
make migrate
# or
./bin/inventory-api migrate --config .inventory-api.yaml
```

### Adding a new migration
*  Create a file under `internal/data/migrations/schema/` named `YYYYMMDDHHMMSS_<short_name>.go`.
  * Example: `Oct 23 2025 at 3:00pm` would be `20251023150000`
  * Migration IDs must be strictly increasing. If you create a migration, submit an MR, and another MR is merged before yours is able to be merged, you must update the ID to represent a date later than any previous migration.
* Export a function that returns `*gormigrate.Migration { ID, Migrate, Rollback }`.
* Add your function to `MigrationsList` in `internal/data/migrations/migration.go`.

Tips:
- Your migration's name should be used in the file name and in the function name and should adequately represent the actions your migration is taking. If your migration is doing too much to fit in a name, you should consider creating multiple migrations.
- Use inline structs (or explicit SQL) inside each migration to represent the schema at that point in time.

### Data Backfills
Keep migration files focused on structure (schema) changes and ensure they execute quickly. Because migrations run during the application startup phase, long-running operations will block the service from launching and may cause deployment timeouts. If you need to perform a large data backfill or heavy transformation, use a migration only to add the necessary columns, then handle the actual data processing separately using a background worker or a one-off Kubernetes Job after the deployment succeeds.

### Models in Migrations

Models should be represented inline in the migration file and separately in the data layer code that is used to create/update/delete resources for the associated models.

**Do not import models from the data layer**. When a migration imports from the data layer and uses models defined in it, the migration may work the first time it is run. Eventually, the models could change so that the migration breaks, causing any new deployments to fail on old migrations. Migrations are intended to be self-contained and should not rely on the data layer code. They are snapshots of the associated schema at a given point in time.

## Migration tests

By default our continuous integration pipeline runs all migrations in order against a fresh postgres database in a container. See `.github/workflows/migration-test.yml` for more details.

In addition due to the nature of migrations being run at application startup, migrations will also be run during our e2e tests.

In most cases, it shouldn't be necessary to create an explicit test for a migration. However, if a migration poses a significant risk you can write targeted tests using the migration helpers.

- Use `data.MigrateTo(db, logger, "<migration_id>")` to migrate up to and including a specific migration (for e2e, this is Postgres-only)
- Insert any rows needed to exercise your migration logic
- Run the remaining migrations by calling `data.Migrate(db, logger)` to apply all subsequent migrations up to the latest.

This pattern lets you test that your migration behaves correctly when applied on top of existing data.

## Auditing applied migrations

Gormigrate tracks migrations via the `migrations` table which can be used to audit applied migrations.

```sql
SELECT * FROM migrations;
```
