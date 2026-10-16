package model

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CommonRepCleanupRepository scans root-aligned chunks and rechecks retention
// inside every delete transaction. Deletion must enforce rowLimit in SQL.
type CommonRepCleanupRepository interface {
	// ReadChunk returns whole-root classifications after an exclusive UUID cursor;
	// uuid.Nil starts the pass. The boundary may extend beyond the row target.
	ReadChunk(context.Context, CommonRepCleanupPolicy, uuid.UUID, int) ([]CommonRepRootClassification, uuid.UUID, bool, error)
	// DeletePrunableRoots rechecks eligibility and deletes at most rowLimit rows.
	DeletePrunableRoots(context.Context, CommonRepCleanupPolicy, []ResourceId, int) (CommonRepCleanupImpact, error)
}

// CommonRepCleanupTransaction scopes repository work to serializable transactions.
// Retried callbacks replace attempted counts; progress is recorded after commit.
type CommonRepCleanupTransaction interface {
	// WithinSerializableCommonRepCleanup retries the callback after serialization
	// conflicts; callers account for work only after this method succeeds.
	WithinSerializableCommonRepCleanup(context.Context, string, bool, time.Duration, time.Duration, func(CommonRepCleanupRepository) error) error
}
