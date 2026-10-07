package jobs

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

// Shared constants for retention cleanup jobs
const (
	DefaultRetentionCleanupBatchSize     = 10000
	DefaultRetentionCleanupBatchDelay    = 500
	DefaultRetentionCleanupDays          = 7
	DefaultRetentionCleanupTombstoneDays = 30
	DefaultTombstoneCleanupBatchSize     = 1000 // Smaller batches for full deletion
)

// deleteOldReporterRepresentations deletes reporter_representations older than N days from latest per resource.
// Excludes tombstoned resources older than M days (handled by deleteOldTombstonedResources).
//
// OPTIMIZED: Uses temp table to materialize "latest timestamp per resource" once,
// then reuses it for all batch deletions. This avoids recalculating the expensive
// window function (MAX + GROUP BY on ~10M rows) for every batch.
func deleteOldReporterRepresentations(db *gorm.DB, logHelper *log.Helper, dryRun bool, retentionDays int, tombstoneDays int, reporterType string, batchSize int, batchDelayMs int) (int64, error) {
	if dryRun {
		var count int64
		query := `
			SELECT COUNT(*)
			FROM reporter_representations rr_rep
			JOIN (
				SELECT reporter_resource_id, MAX(created_at) as latest_created_at
				FROM reporter_representations
				GROUP BY reporter_resource_id
			) latest ON rr_rep.reporter_resource_id = latest.reporter_resource_id
			JOIN reporter_resources rr ON rr_rep.reporter_resource_id = rr.id
			WHERE rr_rep.created_at < (latest.latest_created_at - (? || ' days')::INTERVAL)
			  AND (rr.tombstone = false OR (rr.tombstone = true AND rr.updated_at >= NOW() - (? || ' days')::INTERVAL))
		`

		if reporterType != "" {
			query += " AND rr.reporter_type = ?"
			err := db.Raw(query, fmt.Sprintf("%d", retentionDays), fmt.Sprintf("%d", tombstoneDays), reporterType).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		} else {
			err := db.Raw(query, fmt.Sprintf("%d", retentionDays), fmt.Sprintf("%d", tombstoneDays)).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		}

		logDryRunEstimate(logHelper, "ReporterRepresentation (old representations)", count, batchSize, batchDelayMs)
		return count, nil
	}

	// OPTIMIZATION: Create temp table with latest timestamps (calculated ONCE)
	logHelper.Info("Creating temp table for latest timestamps...")
	createTempTable := `
		CREATE TEMP TABLE IF NOT EXISTS latest_reporter_timestamps (
			reporter_resource_id UUID PRIMARY KEY,
			latest_created_at TIMESTAMP WITH TIME ZONE NOT NULL
		)
	`
	if err := db.Exec(createTempTable).Error; err != nil {
		logHelper.Errorf("Failed to create temp table: %v", err)
		return 0, err
	}

	// Populate temp table with latest timestamps per resource
	populateTemp := `
		INSERT INTO latest_reporter_timestamps (reporter_resource_id, latest_created_at)
		SELECT reporter_resource_id, MAX(created_at) as latest_created_at
		FROM reporter_representations
		GROUP BY reporter_resource_id
	`
	startTime := time.Now()
	if err := db.Exec(populateTemp).Error; err != nil {
		logHelper.Errorf("Failed to populate temp table: %v", err)
		return 0, err
	}
	logHelper.Infof("Temp table populated in %v", time.Since(startTime))

	// Create index on temp table for faster joins
	createIndex := `CREATE INDEX IF NOT EXISTS idx_latest_reporter_timestamps ON latest_reporter_timestamps(reporter_resource_id)`
	if err := db.Exec(createIndex).Error; err != nil {
		logHelper.Warnf("Failed to create index on temp table (non-fatal): %v", err)
	}

	// Now delete in batches using the temp table (FAST - no recalculation)
	var totalDeleted int64
	batchCount := 0

	logHelper.Info("Deleting old reporter_representations using temp table...")

	for {
		var deleteQuery string
		var args []interface{}

		// Use temp table instead of recalculating window function
		baseQuery := `
			DELETE FROM reporter_representations
			WHERE (reporter_resource_id, version, generation) IN (
				SELECT rr_rep.reporter_resource_id, rr_rep.version, rr_rep.generation
				FROM reporter_representations rr_rep
				JOIN latest_reporter_timestamps lt ON rr_rep.reporter_resource_id = lt.reporter_resource_id
				JOIN reporter_resources rr ON rr_rep.reporter_resource_id = rr.id
				WHERE rr_rep.created_at < (lt.latest_created_at - (? || ' days')::INTERVAL)
				  AND (rr.tombstone = false OR (rr.tombstone = true AND rr.updated_at >= NOW() - (? || ' days')::INTERVAL))
		`

		if reporterType != "" {
			deleteQuery = baseQuery + " AND rr.reporter_type = ? LIMIT ?)"
			args = []interface{}{fmt.Sprintf("%d", retentionDays), fmt.Sprintf("%d", tombstoneDays), reporterType, batchSize}
		} else {
			deleteQuery = baseQuery + " LIMIT ?)"
			args = []interface{}{fmt.Sprintf("%d", retentionDays), fmt.Sprintf("%d", tombstoneDays), batchSize}
		}

		result := db.Exec(deleteQuery, args...)
		if result.Error != nil {
			logHelper.Errorf("Failed to delete ReporterRepresentation batch %d: %v", batchCount+1, result.Error)
			return totalDeleted, result.Error
		}

		if result.RowsAffected == 0 {
			break
		}

		totalDeleted += result.RowsAffected
		batchCount++

		logHelper.Infof("Batch %d: Deleted %d ReporterRepresentation records (total so far: %d)", batchCount, result.RowsAffected, totalDeleted)

		if batchDelayMs > 0 {
			time.Sleep(time.Duration(batchDelayMs) * time.Millisecond)
		}
	}

	// Manually drop temp table
	db.Exec("DROP TABLE IF EXISTS latest_reporter_timestamps")
	logHelper.Info("Cleanup complete, temp table dropped")

	return totalDeleted, nil
}

// deleteOldCommonRepresentations deletes common_representations older than N days from latest per resource.
// Excludes tombstoned resources older than M days (handled by deleteOldTombstonedResources).
//
// OPTIMIZED: Uses temp table to materialize "latest timestamp per resource" once,
// then reuses it for all batch deletions. This avoids recalculating the expensive
// window function (MAX + GROUP BY on ~10M rows) for every batch.
func deleteOldCommonRepresentations(db *gorm.DB, logHelper *log.Helper, dryRun bool, retentionDays int, tombstoneDays int, reporterType string, batchSize int, batchDelayMs int) (int64, error) {
	if dryRun {
		var count int64
		query := `
			SELECT COUNT(*)
			FROM common_representations cr
			JOIN (
				SELECT resource_id, MAX(created_at) as latest_created_at
				FROM common_representations
				GROUP BY resource_id
			) latest ON cr.resource_id = latest.resource_id
			JOIN resource r ON cr.resource_id = r.id
			LEFT JOIN reporter_resources rr ON r.id = rr.resource_id
			WHERE cr.created_at < (latest.latest_created_at - (? || ' days')::INTERVAL)
			  AND (rr.tombstone = false OR (rr.tombstone = true AND rr.updated_at >= NOW() - (? || ' days')::INTERVAL))
		`

		if reporterType != "" {
			query += " AND rr.reporter_type = ?"
			err := db.Raw(query, fmt.Sprintf("%d", retentionDays), fmt.Sprintf("%d", tombstoneDays), reporterType).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		} else {
			err := db.Raw(query, fmt.Sprintf("%d", retentionDays), fmt.Sprintf("%d", tombstoneDays)).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		}

		logDryRunEstimate(logHelper, "CommonRepresentation (old representations)", count, batchSize, batchDelayMs)
		return count, nil
	}

	// OPTIMIZATION: Create temp table with latest timestamps (calculated ONCE)
	logHelper.Info("Creating temp table for latest timestamps...")
	createTempTable := `
		CREATE TEMP TABLE IF NOT EXISTS latest_common_timestamps (
			resource_id UUID PRIMARY KEY,
			latest_created_at TIMESTAMP WITH TIME ZONE NOT NULL
		)
	`
	if err := db.Exec(createTempTable).Error; err != nil {
		logHelper.Errorf("Failed to create temp table: %v", err)
		return 0, err
	}

	// Populate temp table with latest timestamps per resource
	populateTemp := `
		INSERT INTO latest_common_timestamps (resource_id, latest_created_at)
		SELECT resource_id, MAX(created_at) as latest_created_at
		FROM common_representations
		GROUP BY resource_id
	`
	startTime := time.Now()
	if err := db.Exec(populateTemp).Error; err != nil {
		logHelper.Errorf("Failed to populate temp table: %v", err)
		return 0, err
	}
	logHelper.Infof("Temp table populated in %v", time.Since(startTime))

	// Create index on temp table for faster joins
	createIndex := `CREATE INDEX IF NOT EXISTS idx_latest_common_timestamps ON latest_common_timestamps(resource_id)`
	if err := db.Exec(createIndex).Error; err != nil {
		logHelper.Warnf("Failed to create index on temp table (non-fatal): %v", err)
	}

	// Now delete in batches using the temp table (FAST)
	var totalDeleted int64
	batchCount := 0

	logHelper.Info("Deleting old common_representations using temp table...")

	for {
		var deleteQuery string
		var args []interface{}

		// Use temp table instead of recalculating window function
		baseQuery := `
			DELETE FROM common_representations
			WHERE (resource_id, version) IN (
				SELECT cr.resource_id, cr.version
				FROM common_representations cr
				JOIN latest_common_timestamps lt ON cr.resource_id = lt.resource_id
				JOIN resource r ON cr.resource_id = r.id
				LEFT JOIN reporter_resources rr ON r.id = rr.resource_id
				WHERE cr.created_at < (lt.latest_created_at - (? || ' days')::INTERVAL)
				  AND (rr.tombstone = false OR (rr.tombstone = true AND rr.updated_at >= NOW() - (? || ' days')::INTERVAL))
		`

		if reporterType != "" {
			deleteQuery = baseQuery + " AND rr.reporter_type = ? LIMIT ?)"
			args = []interface{}{fmt.Sprintf("%d", retentionDays), fmt.Sprintf("%d", tombstoneDays), reporterType, batchSize}
		} else {
			deleteQuery = baseQuery + " LIMIT ?)"
			args = []interface{}{fmt.Sprintf("%d", retentionDays), fmt.Sprintf("%d", tombstoneDays), batchSize}
		}

		result := db.Exec(deleteQuery, args...)
		if result.Error != nil {
			logHelper.Errorf("Failed to delete CommonRepresentation batch %d: %v", batchCount+1, result.Error)
			return totalDeleted, result.Error
		}

		if result.RowsAffected == 0 {
			break
		}

		totalDeleted += result.RowsAffected
		batchCount++

		logHelper.Infof("Batch %d: Deleted %d CommonRepresentation records (total so far: %d)", batchCount, result.RowsAffected, totalDeleted)

		if batchDelayMs > 0 {
			time.Sleep(time.Duration(batchDelayMs) * time.Millisecond)
		}
	}

	// Manually drop temp table
	db.Exec("DROP TABLE IF EXISTS latest_common_timestamps")
	logHelper.Info("Cleanup complete, temp table dropped")

	return totalDeleted, nil
}

// deleteOldTombstonedResources deletes tombstoned resources and all their representations completely.
func deleteOldTombstonedResources(db *gorm.DB, logHelper *log.Helper, dryRun bool, tombstoneDays int, reporterType string, batchSize int, batchDelayMs int) (int64, error) {
	if dryRun {
		var count int64
		query := `
			SELECT COUNT(DISTINCT rr.id)
			FROM reporter_resources rr
			WHERE rr.tombstone = true
			  AND rr.updated_at < NOW() - (? || ' days')::INTERVAL
		`

		if reporterType != "" {
			query += " AND rr.reporter_type = ?"
			err := db.Raw(query, fmt.Sprintf("%d", tombstoneDays), reporterType).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		} else {
			err := db.Raw(query, fmt.Sprintf("%d", tombstoneDays)).Scan(&count).Error
			if err != nil {
				return 0, err
			}
		}

		logDryRunEstimate(logHelper, "Old tombstoned resources (complete deletion)", count, batchSize, batchDelayMs)
		return count, nil
	}

	var totalDeleted int64
	batchCount := 0

	logHelper.Info("Deleting old tombstoned resources completely...")

	for {
		// Delete reporter_resources (cascade will delete reporter_representations)
		var deleteQuery string
		var args []interface{}

		if reporterType != "" {
			deleteQuery = `
				DELETE FROM reporter_resources
				WHERE id IN (
					SELECT id FROM reporter_resources
					WHERE tombstone = true
					  AND updated_at < NOW() - (? || ' days')::INTERVAL
					  AND reporter_type = ?
					LIMIT ?
				)
			`
			args = []interface{}{fmt.Sprintf("%d", tombstoneDays), reporterType, batchSize}
		} else {
			deleteQuery = `
				DELETE FROM reporter_resources
				WHERE id IN (
					SELECT id FROM reporter_resources
					WHERE tombstone = true
					  AND updated_at < NOW() - (? || ' days')::INTERVAL
					LIMIT ?
				)
			`
			args = []interface{}{fmt.Sprintf("%d", tombstoneDays), batchSize}
		}

		result := db.Exec(deleteQuery, args...)
		if result.Error != nil {
			logHelper.Errorf("Failed to delete tombstoned resources batch %d: %v", batchCount+1, result.Error)
			return totalDeleted, result.Error
		}

		if result.RowsAffected == 0 {
			break
		}

		totalDeleted += result.RowsAffected
		batchCount++

		logHelper.Infof("Batch %d: Deleted %d tombstoned resources (total so far: %d)", batchCount, result.RowsAffected, totalDeleted)

		if batchDelayMs > 0 {
			time.Sleep(time.Duration(batchDelayMs) * time.Millisecond)
		}
	}

	// Clean up orphaned resources (no reporter_resources reference them)
	orphanedQuery := `
		DELETE FROM resource
		WHERE id NOT IN (SELECT DISTINCT resource_id FROM reporter_resources)
	`
	orphanedResult := db.Exec(orphanedQuery)
	if orphanedResult.Error != nil {
		logHelper.Errorf("Failed to clean up orphaned resources: %v", orphanedResult.Error)
	} else if orphanedResult.RowsAffected > 0 {
		logHelper.Infof("Cleaned up %d orphaned resources", orphanedResult.RowsAffected)
	}

	return totalDeleted, nil
}
