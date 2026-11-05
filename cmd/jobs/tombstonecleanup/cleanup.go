// Package tombstonecleanup contains disposable maintenance tooling, following the
// existing cleanup jobs rather than adding permanent application infrastructure.
package tombstonecleanup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"go.opentelemetry.io/otel/metric/noop"
	"gorm.io/gorm"
)

// rowsPerParentAtFinalization bounds what one trimmed parent can still delete
// atomically: its current reporter snapshot, itself, the root's referenced and
// latest common versions, and the root.
const rowsPerParentAtFinalization = 5

type options struct {
	tombstoneDays      int
	rowsPerTransaction int
	batchDelay         time.Duration
	maxBatches         int
	// cutoff pins the eligibility cutoff in tests; zero derives it from tombstoneDays.
	cutoff time.Time
}

// parentsPerBatch keeps a batch's finalization within rowsPerTransaction.
func (o options) parentsPerBatch() int {
	return o.rowsPerTransaction / rowsPerParentAtFinalization
}

// maxParents is the worklist cap for a capped run; zero means unlimited.
func (o options) maxParents() int64 {
	if o.maxBatches == 0 {
		return 0
	}
	return int64(o.parentsPerBatch()) * int64(o.maxBatches)
}

type result struct {
	cutoff time.Time
	// candidates are all eligible parents (preview) or the parents placed in the worklist (execute).
	candidates int64
	// eligible and affected describe the full eligible impact; preview only.
	eligible counts
	affected int64
	// deleted counts committed deletions; execute only.
	deleted                          counts
	skipped, remaining               int64
	batches, transactions            int
	selectionDuration, batchDuration time.Duration
	complete                         bool
}

// progress receives a snapshot of committed progress after each step.
type progress func(message string, r result)

// cleanup previews or deletes expired tombstoned parents oldest first.
//
// Execution freezes a worklist of eligible parents, then handles it in batches of
// parentsPerBatch. Each batch first trims history in transactions of at most
// rowsPerTransaction rows, so a parent with deep history spans many small
// transactions, then atomically removes the batch's remaining snapshots, parents
// and orphaned roots. Every transaction is serializable and rechecks eligibility,
// so concurrent revivals and new reporters on a root are preserved.
func cleanup(ctx context.Context, db *gorm.DB, o options, execute bool, report progress) (r result, err error) {
	r.cutoff = o.cutoff
	if r.cutoff.IsZero() {
		r.cutoff = time.Now().UTC().AddDate(0, 0, -o.tombstoneDays)
	}
	if report == nil {
		report = func(string, result) {}
	}
	if !execute {
		return preview(ctx, db, r, report)
	}
	metrics := &metricscollector.MetricsCollector{}
	if err = metrics.New(noop.NewMeterProvider().Meter("tombstone-cleanup")); err != nil {
		return
	}
	err = db.WithContext(ctx).Connection(func(connection *gorm.DB) (connectionErr error) {
		// Connection.Close returns the session to the pool, so drop the worklist
		// even on cancellation.
		defer func() {
			dropCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			connectionErr = errors.Join(connectionErr, connection.WithContext(dropCtx).Exec(dropWorklistSQL).Error)
		}()
		e := executor{ctx: ctx, connection: connection, options: o, transactions: data.NewGormTransactionManager(metrics, 10), report: report, r: &r}
		return e.run()
	})
	return
}

func preview(ctx context.Context, db *gorm.DB, r result, report progress) (result, error) {
	started := time.Now()
	var row struct{ Parents, Affected, ReporterHistory, CommonHistory, Roots int64 }
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(statementTimeoutSQL).Error; err != nil {
			return err
		}
		return tx.Raw(previewSQL, r.cutoff, r.cutoff).Scan(&row).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	r.selectionDuration = time.Since(started)
	if err != nil {
		return r, err
	}
	r.candidates, r.remaining, r.affected = row.Parents, row.Parents, row.Affected
	r.eligible = counts{reporterHistory: row.ReporterHistory, commonHistory: row.CommonHistory, parents: row.Parents, roots: row.Roots}
	r.complete = true
	report("tombstone cleanup selection complete", r)
	return r, nil
}

type executor struct {
	ctx          context.Context
	connection   *gorm.DB
	options      options
	transactions bizmodel.TransactionManager
	report       progress
	r            *result
}

func (e *executor) run() error {
	o, r := e.options, e.r
	if err := e.selectWorklist(); err != nil {
		return fmt.Errorf("select worklist: %w", err)
	}
	e.report("tombstone cleanup selection complete", *r)
	limit := int64(o.rowsPerTransaction)
	for e.hasNextBatch() {
		started := time.Now()
		var ids []uuid.UUID
		if err := e.connection.WithContext(e.ctx).Raw(selectBatchSQL, o.parentsPerBatch()).Scan(&ids).Error; err != nil {
			return fmt.Errorf("select batch: %w", err)
		}
		if len(ids) == 0 {
			return fmt.Errorf("worklist unexpectedly empty with %d parents remaining", r.remaining)
		}
		e.report("tombstone cleanup batch started", *r)
		for {
			var trimmed counts
			if err := e.commit("tombstone_cleanup_trim", func(tx *gorm.DB) (err error) {
				trimmed, err = trimHistory(tx, ids, r.cutoff, limit)
				return
			}); err != nil {
				return fmt.Errorf("trim history: %w", err)
			}
			r.deleted.add(trimmed)
			e.report("tombstone cleanup history chunk committed", *r)
			if trimmed.total() < limit {
				break
			}
			if err := e.pause(); err != nil {
				return err
			}
		}
		var finalized counts
		var skipped int64
		if err := e.commit("tombstone_cleanup_finalize", func(tx *gorm.DB) (err error) {
			finalized, skipped, err = finalizeBatch(tx, ids, r.cutoff, limit)
			return
		}); err != nil {
			return fmt.Errorf("finalize batch: %w", err)
		}
		r.deleted.add(finalized)
		r.skipped += skipped
		r.remaining -= int64(len(ids))
		r.batches++
		r.batchDuration = time.Since(started)
		e.report("tombstone cleanup batch committed", *r)
		if e.hasNextBatch() {
			if err := e.pause(); err != nil {
				return err
			}
		}
	}
	r.complete = r.remaining == 0
	return nil
}

func (e *executor) selectWorklist() error {
	started := time.Now()
	defer func() { e.r.selectionDuration = time.Since(started) }()
	return e.connection.WithContext(e.ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(statementTimeoutSQL).Error; err != nil {
			return err
		}
		if err := tx.Exec(createWorklistSQL).Error; err != nil {
			return err
		}
		var inserted *gorm.DB
		if maxParents := e.options.maxParents(); maxParents > 0 {
			// One extra parent lets a capped run report complete=false when
			// eligible parents remain beyond its cap.
			inserted = tx.Exec(insertLimitedWorklistSQL, e.r.cutoff, maxParents+1)
		} else {
			inserted = tx.Exec(insertWorklistSQL, e.r.cutoff)
		}
		e.r.candidates, e.r.remaining = inserted.RowsAffected, inserted.RowsAffected
		return inserted.Error
	})
}

func (e *executor) hasNextBatch() bool {
	return e.r.remaining > 0 && (e.options.maxBatches == 0 || e.r.batches < e.options.maxBatches)
}

// commit runs work in one serializable transaction, retrying serialization failures.
func (e *executor) commit(operation string, work func(*gorm.DB) error) error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	err := e.transactions.HandleSerializableTransaction(operation, e.connection.WithContext(e.ctx), func(tx *gorm.DB) error {
		if tx.Error != nil {
			return tx.Error
		}
		if err := tx.Exec(statementTimeoutSQL).Error; err != nil {
			return err
		}
		if err := tx.Exec(lockTimeoutSQL).Error; err != nil {
			return err
		}
		return work(tx)
	})
	if err == nil {
		e.r.transactions++
	}
	return err
}

func (e *executor) pause() error {
	timer := time.NewTimer(e.options.batchDelay)
	defer timer.Stop()
	select {
	case <-e.ctx.Done():
		return e.ctx.Err()
	case <-timer.C:
		return nil
	}
}
