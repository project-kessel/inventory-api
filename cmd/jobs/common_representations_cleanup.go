package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type commonRepCleanupOptions struct {
	keepVersions int
	resourceType string
	batchSize    int
	batchDelay   time.Duration
	maxBatches   int
}

type commonRepCleanupResult struct {
	candidateRows     int64
	remainingRows     int64
	deletedRows       int64
	committedBatches  int
	selectionDuration time.Duration
	complete          bool
}

// cleanupCommonRepresentations pins a connection for the temporary worklist.
// Resource writers must stay fenced and replication must already be drained.
// A rerun selects the remaining history afresh; no cursor or persistent state is needed.
func cleanupCommonRepresentations(ctx context.Context, db *gorm.DB, options commonRepCleanupOptions, execute bool, onBatch func(int, int64, time.Duration)) (result commonRepCleanupResult, err error) {
	err = db.WithContext(ctx).Connection(func(connection *gorm.DB) (connectionErr error) {
		selectionStarted := time.Now()
		query, arguments := commonRepCleanupCandidateQuery(options)
		if !execute {
			connectionErr = commonRepCleanupTransaction(ctx, connection, true, func(tx *gorm.DB) error {
				return tx.Raw("SELECT COUNT(*) FROM (?) candidates", gorm.Expr(query, arguments...)).Scan(&result.candidateRows).Error
			})
			result.selectionDuration = time.Since(selectionStarted)
			result.remainingRows = result.candidateRows
			result.complete = connectionErr == nil
			return connectionErr
		}

		// Connection.Close returns the session to the pool. Explicitly drop the
		// worklist even on cancellation so pooled sessions never retain it.
		defer func() {
			cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()
			dropErr := connection.WithContext(cleanupContext).Exec("DROP TABLE IF EXISTS pg_temp.common_rep_cleanup_candidates").Error
			connectionErr = errors.Join(connectionErr, dropErr)
		}()
		connectionErr = commonRepCleanupTransaction(ctx, connection, false, func(tx *gorm.DB) error {
			if err := tx.Exec(`CREATE TEMP TABLE common_rep_cleanup_candidates (
    resource_id uuid NOT NULL, version bigint NOT NULL,
    PRIMARY KEY (resource_id, version)
   ) ON COMMIT PRESERVE ROWS`).Error; err != nil {
				return err
			}
			insert := tx.Exec("INSERT INTO pg_temp.common_rep_cleanup_candidates (resource_id, version) ?", gorm.Expr(query, arguments...))
			result.candidateRows = insert.RowsAffected
			return insert.Error
		})
		result.selectionDuration = time.Since(selectionStarted)
		result.remainingRows = result.candidateRows
		if connectionErr != nil {
			return fmt.Errorf("select cleanup worklist: %w", connectionErr)
		}

		for result.remainingRows > 0 && (options.maxBatches == 0 || result.committedBatches < options.maxBatches) {
			if err := ctx.Err(); err != nil {
				return err
			}
			started := time.Now()
			var deleted int64
			if err := commonRepCleanupTransaction(ctx, connection, false, func(tx *gorm.DB) error {
				var err error
				deleted, err = deleteCommonRepCleanupBatch(tx, options.batchSize)
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

// commonRepCleanupTransaction commits history deletion and worklist removal
// together. SQL remains cancellable through the invocation context. Preview
// cannot mutate the DB.
func commonRepCleanupTransaction(ctx context.Context, connection *gorm.DB, readOnly bool, work func(*gorm.DB) error) error {
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
