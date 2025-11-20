## Database Migrations

Inventory uses gormigrate on top of GORM

### Running migrations
```sh
make migrate
# or
./bin/inventory-api migrate --config .inventory-api.yaml
```

### Adding a new migration
*  Create a file under `internal/data/migrations/steps/` named `YYYYMMDDHHMMSS_<short_name>.go`.
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

TODO

### Record Deletions

TODO

## Migration tests

TODO
