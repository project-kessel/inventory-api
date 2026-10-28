// Package tombstonecleanup contains disposable maintenance tooling, following the
// existing cleanup jobs rather than adding permanent application infrastructure.
package tombstonecleanup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"go.opentelemetry.io/otel/metric/noop"
	"gorm.io/gorm"
)

type options struct {
	tombstoneDays, batchSize, maxBatches int
	batchDelay                           time.Duration
	cutoff                               time.Time
}
type result struct {
	candidates, remaining, selected, skipped, reporterResources, reporterHistory, commonHistory, roots, affected int64
	batches                                                                                                      int
	selectionDuration                                                                                            time.Duration
	batchDuration                                                                                                time.Duration
	totalReporterHistory, totalCommonHistory, totalRoots, totalAffected                                          int64
	cutoff                                                                                                       time.Time
	complete                                                                                                     bool
}

// cleanup freezes a parent worklist, tolerating online changes by skipping them.
// Atomic serializable batches preserve roots that acquire a surviving reporter.
func cleanup(ctx context.Context, db *gorm.DB, o options, execute bool, onSelection func(result), onBatch func(result)) (r result, err error) {
	r.cutoff = o.cutoff
	if r.cutoff.IsZero() {
		r.cutoff = time.Now().UTC().AddDate(0, 0, -o.tombstoneDays)
	}
	metrics := &metricscollector.MetricsCollector{}
	if err = metrics.New(noop.NewMeterProvider().Meter("tombstone-cleanup")); err != nil {
		return
	}
	manager := data.NewGormTransactionManager(metrics, 10)
	err = db.WithContext(ctx).Connection(func(connection *gorm.DB) (connectionErr error) {
		started := time.Now()
		if !execute {
			connectionErr = connection.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec("SET TRANSACTION READ ONLY").Error; err != nil {
					return err
				}
				if err := tx.Exec("SET LOCAL statement_timeout=0").Error; err != nil {
					return err
				}
				var limit int64
				if o.maxBatches > 0 {
					limit = int64(o.batchSize) * int64(o.maxBatches)
				}
				total, err := impact(tx, r.cutoff, 0)
				if err != nil {
					return err
				}
				counts := total
				if limit > 0 {
					counts, err = impact(tx, r.cutoff, limit)
					if err != nil {
						return err
					}
				}
				counts.cutoff = r.cutoff
				counts.totalReporterHistory = total.reporterHistory
				counts.totalCommonHistory = total.commonHistory
				counts.totalRoots = total.roots
				counts.totalAffected = total.affected
				r = counts
				return nil
			}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
			r.remaining = r.candidates
			r.selectionDuration = time.Since(started)
			r.complete = connectionErr == nil
			if connectionErr == nil && onSelection != nil {
				onSelection(r)
			}
			return
		}
		defer func() {
			dropCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			connectionErr = errors.Join(connectionErr, connection.WithContext(dropCtx).Exec("DROP TABLE IF EXISTS pg_temp.tombstone_cleanup_candidates,pg_temp.tombstone_cleanup_affected").Error)
		}()
		connectionErr = connection.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SET LOCAL statement_timeout=0").Error; err != nil {
				return err
			}
			if err := tx.Exec(`CREATE TEMP TABLE tombstone_cleanup_candidates (
    id uuid NOT NULL,resource_id uuid NOT NULL,updated_at timestamptz NOT NULL,
    generation bigint NOT NULL,representation_version bigint NOT NULL,
    PRIMARY KEY(updated_at,id),UNIQUE(id)) ON COMMIT PRESERVE ROWS`).Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE TEMP TABLE tombstone_cleanup_affected (resource_id uuid PRIMARY KEY) ON COMMIT PRESERVE ROWS").Error; err != nil {
				return err
			}
			inserted := tx.Exec("INSERT INTO pg_temp.tombstone_cleanup_candidates "+candidateSQL, r.cutoff)
			r.candidates = inserted.RowsAffected
			return inserted.Error
		})
		r.remaining = r.candidates
		r.selectionDuration = time.Since(started)
		if connectionErr != nil {
			return fmt.Errorf("select worklist: %w", connectionErr)
		}
		if onSelection != nil {
			onSelection(r)
		}
		for r.remaining > 0 && (o.maxBatches == 0 || r.batches < o.maxBatches) {
			if err := ctx.Err(); err != nil {
				return err
			}
			var batch result
			started := time.Now()
			err := manager.HandleSerializableTransaction("tombstone_cleanup", connection.WithContext(ctx), func(tx *gorm.DB) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if tx.Error != nil {
					return tx.Error
				}
				if err := tx.Exec("SET LOCAL statement_timeout=0").Error; err != nil {
					return err
				}
				if err := tx.Exec("SET LOCAL lock_timeout='5s'").Error; err != nil {
					return err
				}
				var err error
				batch, err = deleteBatch(tx, o.batchSize, r.cutoff)
				return err
			})
			if err != nil {
				return fmt.Errorf("delete batch: %w", err)
			}
			if batch.selected == 0 {
				return fmt.Errorf("worklist unexpectedly empty")
			}
			r.batches++
			r.selected += batch.selected
			r.skipped += batch.skipped
			r.remaining -= batch.selected
			r.reporterResources += batch.reporterResources
			r.reporterHistory += batch.reporterHistory
			r.commonHistory += batch.commonHistory
			r.roots += batch.roots
			r.affected += batch.affected
			r.batchDuration = time.Since(started)
			if onBatch != nil {
				onBatch(r)
			}
			if r.remaining > 0 && (o.maxBatches == 0 || r.batches < o.maxBatches) {
				timer := time.NewTimer(o.batchDelay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
		}
		r.complete = r.remaining == 0
		return nil
	})
	return
}
