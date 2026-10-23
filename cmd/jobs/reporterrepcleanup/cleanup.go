package reporterrepcleanup

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type reporterRepCleanupOptions struct {
	keepVersions int
	batchSize    int
	batchDelay   time.Duration
	maxBatches   int
}

type reporterRepCleanupResult struct {
	candidateRows     int64
	remainingRows     int64
	deletedRows       int64
	committedBatches  int
	selectionDuration time.Duration
	complete          bool
}

// cleanupReporterRepresentations pins a connection for the temporary worklist.
// Resource writers must stay fenced and replication must already be drained.
// A rerun selects the remaining history afresh; no cursor or persistent state is needed.
func cleanupReporterRepresentations(ctx context.Context, db *gorm.DB, options reporterRepCleanupOptions, execute bool, onSelection func(int64, time.Duration), onBatch func(int, int64, time.Duration)) (result reporterRepCleanupResult, err error) {
	err = db.WithContext(ctx).Connection(func(connection *gorm.DB) (connectionErr error) {
		selectionStarted := time.Now()
		query, arguments := reporterRepCleanupCandidateQuery(options)
		if !execute {
			connectionErr = reporterRepCleanupTransaction(ctx, connection, true, func(tx *gorm.DB) error {
				return tx.Raw("SELECT COUNT(*) FROM (?) candidates", gorm.Expr(query, arguments...)).Scan(&result.candidateRows).Error
			})
			result.selectionDuration = time.Since(selectionStarted)
			result.remainingRows = result.candidateRows
			result.complete = connectionErr == nil
			if connectionErr == nil && onSelection != nil {
				onSelection(result.candidateRows, result.selectionDuration)
			}
			return connectionErr
		}

		// Connection.Close returns the session to the pool. Explicitly drop the
		// worklist even on cancellation so pooled sessions never retain it.
		defer func() {
			cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()
			dropErr := connection.WithContext(cleanupContext).Exec("DROP TABLE IF EXISTS pg_temp.reporter_rep_cleanup_candidates").Error
			connectionErr = errors.Join(connectionErr, dropErr)
		}()
		connectionErr = reporterRepCleanupTransaction(ctx, connection, false, func(tx *gorm.DB) error {
			if err := tx.Exec(`CREATE TEMP TABLE reporter_rep_cleanup_candidates (
    reporter_resource_id uuid NOT NULL, generation bigint NOT NULL, version bigint NOT NULL,
    PRIMARY KEY (reporter_resource_id, generation, version)
   ) ON COMMIT PRESERVE ROWS`).Error; err != nil {
				return err
			}
			insert := tx.Exec("INSERT INTO pg_temp.reporter_rep_cleanup_candidates (reporter_resource_id, generation, version) ?", gorm.Expr(query, arguments...))
			result.candidateRows = insert.RowsAffected
			return insert.Error
		})
		result.selectionDuration = time.Since(selectionStarted)
		result.remainingRows = result.candidateRows
		if connectionErr != nil {
			return fmt.Errorf("select cleanup worklist: %w", connectionErr)
		}
		if onSelection != nil {
			onSelection(result.candidateRows, result.selectionDuration)
		}

		for result.remainingRows > 0 && (options.maxBatches == 0 || result.committedBatches < options.maxBatches) {
			if err := ctx.Err(); err != nil {
				return err
			}
			started := time.Now()
			var deleted int64
			if err := reporterRepCleanupTransaction(ctx, connection, false, func(tx *gorm.DB) error {
				var err error
				deleted, err = deleteReporterRepCleanupBatch(tx, options.batchSize)
				return err
			}); err != nil {
				return fmt.Errorf("delete cleanup batch: %w", err)
			}
			result.committedBatches++
			result.deletedRows += deleted
			result.remainingRows -= deleted
			if onBatch != nil {
				onBatch(result.committedBatches, deleted, time.Since(started))
			}
			if result.remainingRows > 0 && (options.maxBatches == 0 || result.committedBatches < options.maxBatches) {
				timer := time.NewTimer(options.batchDelay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
		}
		result.complete = result.remainingRows == 0
		return nil
	})
	return result, err
}

// reporterRepCleanupTransaction commits history deletion and worklist removal
// together. SQL remains cancellable through the invocation context. Preview
// cannot mutate the DB.
func reporterRepCleanupTransaction(ctx context.Context, connection *gorm.DB, readOnly bool, work func(*gorm.DB) error) error {
	return connection.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if readOnly {
			if err := tx.Exec("SET TRANSACTION READ ONLY").Error; err != nil {
				return err
			}
		}
		// Override inherited database/role statement timeouts for this transaction.
		if err := tx.Exec("SET LOCAL statement_timeout = 0").Error; err != nil {
			return err
		}
		if err := tx.Exec("SET LOCAL lock_timeout = '5s'").Error; err != nil {
			return err
		}
		return work(tx)
	})
}
