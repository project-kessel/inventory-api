package model

import (
	"context"
	"time"
)

// TombstoneCleanupRepository reads tombstone eligibility or deletes one bounded set of admissible tombstones.
// Age eligibility is determined only by the frozen policy cutoff; structural admissibility is separate and
// excludes unknown timestamps, missing or mismatched roots, invalid reporter keys, and inconsistent or newer
// terminal history. Implementations must preserve and report these exclusions rather than repairing them.
type TombstoneCleanupRepository interface {
	// PreviewTombstones reports the complete age scope, structural exclusions, and the deterministic eligible
	// prefix selected by updated_at and reporter-resource ID, without writing any Inventory rows or events.
	PreviewTombstones(ctx context.Context, policy TombstoneCleanupPolicy, boundedReporterRowLimit int64) (TombstoneCleanupReport, error)
	// DeleteTombstoneBatch reselects and rechecks the same frozen selector inside its transaction, deletes at most
	// batchSize structurally admissible reporter rows, and relies on the reporter-row cascade for reporter history.
	// It may delete common history and a root only for a root touched by this selected batch and only when no
	// reporter row survives. It must never perform a global orphan sweep; callers may treat its impact as committed
	// only after the enclosing transaction commits.
	DeleteTombstoneBatch(ctx context.Context, policy TombstoneCleanupPolicy, batchSize int) (TombstoneCleanupImpact, error)
}

// TombstoneCleanupTransaction scopes cleanup repository work to one bounded PostgreSQL transaction.
type TombstoneCleanupTransaction interface {
	// WithinSerializableTombstoneCleanup enforces read-only mode for previews and serializable isolation for
	// every transaction. A write transaction may retry its callback after a serialization failure, so the callback
	// must reselect eligibility, reset attempted impact, and emit committed-progress notifications only after this
	// method returns success. Cancellation rolls back the active transaction but does not undo earlier commits.
	WithinSerializableTombstoneCleanup(
		ctx context.Context,
		operationName string,
		readOnly bool,
		statementTimeout time.Duration,
		lockTimeout time.Duration,
		work func(TombstoneCleanupRepository) error,
	) error
}
