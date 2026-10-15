package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"gorm.io/gorm"
)

type gormCommonRepCleanupRepository struct {
	db *gorm.DB
}

type commonRepCleanupCandidateRow struct {
	ResourceID uuid.UUID `gorm:"column:resource_id"`
	Version    uint      `gorm:"column:version"`
}

type commonRepCleanupScopeCounts struct {
	ScopedActiveRoots         int64 `gorm:"column:scoped_active_roots"`
	ExcludedInconsistentRoots int64 `gorm:"column:excluded_inconsistent_roots"`
}

type commonRepCleanupImpactCounts struct {
	Representations int64 `gorm:"column:representations"`
	DistinctRoots   int64 `gorm:"column:distinct_roots"`
}

// PreviewCommonHistory reports total prunability and bounded impact without modifying rows.
func (repository *gormCommonRepCleanupRepository) PreviewCommonHistory(
	ctx context.Context,
	policy commonRepCleanupPolicy,
	boundedRowLimit int64,
) (commonRepCleanupReport, error) {
	if repository == nil || repository.db == nil {
		return commonRepCleanupReport{}, fmt.Errorf("common representation cleanup repository is not configured")
	}
	if err := policy.Validate(); err != nil {
		return commonRepCleanupReport{}, err
	}
	if boundedRowLimit <= 0 {
		return commonRepCleanupReport{}, fmt.Errorf("bounded row limit must be positive")
	}
	if err := ctx.Err(); err != nil {
		return commonRepCleanupReport{}, err
	}

	var snapshotAt time.Time
	if err := repository.db.WithContext(ctx).Raw("SELECT transaction_timestamp()").Scan(&snapshotAt).Error; err != nil {
		return commonRepCleanupReport{}, fmt.Errorf("read common representation cleanup snapshot time: %w", err)
	}

	classifiedCTE, arguments := commonRepClassifiedCTE(policy)
	var scopeCounts commonRepCleanupScopeCounts
	scopeQuery := classifiedCTE + `
		SELECT (SELECT COUNT(*) FROM scoped_roots) AS scoped_active_roots,
		       (SELECT COUNT(*) FROM classified_roots WHERE exclusion_reason = 'inconsistent_pointer') AS excluded_inconsistent_roots`
	if err := repository.db.WithContext(ctx).Raw(scopeQuery, arguments...).Scan(&scopeCounts).Error; err != nil {
		return commonRepCleanupReport{}, fmt.Errorf("count common representation cleanup scope: %w", err)
	}

	outOfScopeQuery, outOfScopeArguments := commonRepOutOfScopeRootsQuery(policy)
	var rootsWithoutActiveReporter int64
	if err := repository.db.WithContext(ctx).Raw(outOfScopeQuery, outOfScopeArguments...).Scan(&rootsWithoutActiveReporter).Error; err != nil {
		return commonRepCleanupReport{}, fmt.Errorf("count common representation cleanup out-of-scope roots: %w", err)
	}

	totalImpact, err := repository.readImpact(ctx, classifiedCTE, arguments, 0)
	if err != nil {
		return commonRepCleanupReport{}, fmt.Errorf("count total common representation cleanup impact: %w", err)
	}
	boundedImpact, err := repository.readImpact(ctx, classifiedCTE, arguments, boundedRowLimit)
	if err != nil {
		return commonRepCleanupReport{}, fmt.Errorf("count bounded common representation cleanup impact: %w", err)
	}

	total, err := newCommonRepCleanupImpact(totalImpact.Representations, totalImpact.DistinctRoots)
	if err != nil {
		return commonRepCleanupReport{}, fmt.Errorf("validate total common representation cleanup impact: %w", err)
	}
	bounded, err := newCommonRepCleanupImpact(boundedImpact.Representations, boundedImpact.DistinctRoots)
	if err != nil {
		return commonRepCleanupReport{}, fmt.Errorf("validate bounded common representation cleanup impact: %w", err)
	}
	report, err := newCommonRepCleanupReport(
		time.Now().UTC(), snapshotAt, policy.KeepVersions(), boundedRowLimit,
		scopeCounts.ScopedActiveRoots, rootsWithoutActiveReporter, scopeCounts.ExcludedInconsistentRoots,
		total, bounded,
	)
	if err != nil {
		return commonRepCleanupReport{}, fmt.Errorf("build common representation cleanup report: %w", err)
	}
	return report, nil
}

// readImpact counts one deterministic prunable prefix.
func (repository *gormCommonRepCleanupRepository) readImpact(
	ctx context.Context,
	classifiedCTE string,
	arguments []any,
	limit int64,
) (commonRepCleanupImpactCounts, error) {
	queryArguments := append([]any(nil), arguments...)
	selection := "SELECT resource_id FROM prunable"
	if limit > 0 {
		selection += " ORDER BY current_version DESC, resource_id ASC, version ASC LIMIT ?"
		queryArguments = append(queryArguments, limit)
	}
	query := classifiedCTE + `
	SELECT COUNT(*) AS representations,
	       COUNT(DISTINCT resource_id) AS distinct_roots
	FROM (` + selection + `) AS bounded_prunable`
	var impact commonRepCleanupImpactCounts
	if err := repository.db.WithContext(ctx).Raw(query, queryArguments...).Scan(&impact).Error; err != nil {
		return commonRepCleanupImpactCounts{}, err
	}
	return impact, nil
}

// DeleteCommonHistoryBatch reselects and deletes one bounded batch of prunable common
// representation rows. Retained versions, roots, and out-of-scope rows are never deleted.
func (repository *gormCommonRepCleanupRepository) DeleteCommonHistoryBatch(
	ctx context.Context,
	policy commonRepCleanupPolicy,
	batchSize int,
) (commonRepCleanupImpact, error) {
	if repository == nil || repository.db == nil {
		return commonRepCleanupImpact{}, fmt.Errorf("common representation cleanup repository is not configured")
	}
	if err := policy.Validate(); err != nil {
		return commonRepCleanupImpact{}, err
	}
	if batchSize <= 0 {
		return commonRepCleanupImpact{}, fmt.Errorf("common representation cleanup batch size must be positive")
	}
	if err := ctx.Err(); err != nil {
		return commonRepCleanupImpact{}, err
	}

	classifiedCTE, arguments := commonRepClassifiedCTE(policy)
	candidateQuery := classifiedCTE + `
		SELECT resource_id, version FROM prunable
		ORDER BY current_version DESC, resource_id ASC, version ASC
		LIMIT ?`
	candidateArguments := append(append([]any(nil), arguments...), batchSize)
	var candidates []commonRepCleanupCandidateRow
	if err := repository.db.WithContext(ctx).Raw(candidateQuery, candidateArguments...).Scan(&candidates).Error; err != nil {
		return commonRepCleanupImpact{}, fmt.Errorf("select prunable common representation batch: %w", err)
	}
	if len(candidates) == 0 {
		return newCommonRepCleanupImpactWithRootIDs(0, nil)
	}

	resourceRootIDs := make([]bizmodel.ResourceId, 0, len(candidates))
	seenResourceIDs := make(map[uuid.UUID]struct{}, len(candidates))
	for _, candidate := range candidates {
		if _, seen := seenResourceIDs[candidate.ResourceID]; !seen {
			resourceRootID, err := commonRepRootIDFromUUID(candidate.ResourceID)
			if err != nil {
				return commonRepCleanupImpact{}, fmt.Errorf("prunable common representation has invalid resource root identity: %w", err)
			}
			seenResourceIDs[candidate.ResourceID] = struct{}{}
			resourceRootIDs = append(resourceRootIDs, resourceRootID)
		}
	}

	deleteQuery := classifiedCTE + `,
		selected AS (
			SELECT resource_id, version FROM prunable
			ORDER BY current_version DESC, resource_id ASC, version ASC
			LIMIT ?
		)
		DELETE FROM common_representations AS cr
		USING selected
		WHERE cr.resource_id = selected.resource_id
		  AND cr.version = selected.version`
	deleted := repository.db.WithContext(ctx).Exec(deleteQuery, candidateArguments...)
	if deleted.Error != nil {
		return commonRepCleanupImpact{}, fmt.Errorf("delete prunable common representation batch: %w", deleted.Error)
	}
	if deleted.RowsAffected != int64(len(candidates)) {
		return commonRepCleanupImpact{}, fmt.Errorf("selected common representation batch changed before deletion: selected %d rows, deleted %d", len(candidates), deleted.RowsAffected)
	}

	impact, err := newCommonRepCleanupImpactWithRootIDs(deleted.RowsAffected, resourceRootIDs)
	if err != nil {
		return commonRepCleanupImpact{}, fmt.Errorf("validate committed common representation cleanup impact: %w", err)
	}
	return impact, nil
}

// commonRepClassifiedCTE scopes active roots, resolves the authoritative current version,
// and marks prunable rows per the per-resource retention rule. The authoritative current
// version is the resource.common_version pointer, falling back to MAX(version) only when
// the pointer is NULL. Roots whose pointer does not resolve to an existing row are
// excluded rather than guessed.
func commonRepClassifiedCTE(policy commonRepCleanupPolicy) (string, []any) {
	scopedPredicate := `EXISTS (
			SELECT 1 FROM reporter_resources AS rr
			WHERE rr.resource_id = root.id AND rr.tombstone IS FALSE
		)`
	arguments := []any{}
	if resourceType, hasFilter := policy.ResourceType(); hasFilter {
		scopedPredicate += " AND root.type = ?"
		arguments = append(arguments, resourceType.Serialize())
	}
	arguments = append(arguments, policy.KeepVersions())
	query := `WITH scoped_roots AS (
		SELECT root.id AS resource_id, root.common_version
		FROM resource AS root
		WHERE ` + scopedPredicate + `
	), authoritative AS (
		SELECT scoped.resource_id, scoped.common_version,
		       COALESCE(scoped.common_version, (
		           SELECT MAX(cr.version) FROM common_representations AS cr
		           WHERE cr.resource_id = scoped.resource_id
		       )) AS current_version
		FROM scoped_roots AS scoped
	), classified_roots AS (
		SELECT authoritative.resource_id, authoritative.current_version,
		       CASE
		           WHEN authoritative.current_version IS NULL THEN 'no_common_history'
		           WHEN authoritative.common_version IS NOT NULL AND NOT EXISTS (
		               SELECT 1 FROM common_representations AS pointed
		               WHERE pointed.resource_id = authoritative.resource_id
		                 AND pointed.version = authoritative.common_version
		           ) THEN 'inconsistent_pointer'
		           ELSE NULL
		       END AS exclusion_reason
		FROM authoritative
	), prunable AS (
		SELECT cr.resource_id, cr.version, roots.current_version
		FROM common_representations AS cr
		JOIN classified_roots AS roots ON roots.resource_id = cr.resource_id
		WHERE roots.exclusion_reason IS NULL
		  AND cr.version <= roots.current_version - ?
	)`
	return query, arguments
}

// commonRepOutOfScopeRootsQuery counts roots that have common history but no active
// reporter; they are the tombstone cleanup workstream's domain and are never touched here.
func commonRepOutOfScopeRootsQuery(policy commonRepCleanupPolicy) (string, []any) {
	query := `SELECT COUNT(*) FROM resource AS root
		WHERE EXISTS (
			SELECT 1 FROM common_representations AS cr WHERE cr.resource_id = root.id
		)
		AND NOT EXISTS (
			SELECT 1 FROM reporter_resources AS rr
			WHERE rr.resource_id = root.id AND rr.tombstone IS FALSE
		)`
	arguments := []any{}
	if resourceType, hasFilter := policy.ResourceType(); hasFilter {
		query += " AND root.type = ?"
		arguments = append(arguments, resourceType.Serialize())
	}
	return query, arguments
}

type gormCommonRepCleanupTransaction struct {
	db                 *gorm.DB
	transactionManager bizmodel.TransactionManager
}

// newCommonRepCleanupTransaction adapts the shared transaction manager to cleanup repository work.
func newCommonRepCleanupTransaction(db *gorm.DB, transactionManager bizmodel.TransactionManager) commonRepCleanupTransaction {
	return &gormCommonRepCleanupTransaction{db: db, transactionManager: transactionManager}
}

// WithinSerializableCommonRepCleanup runs bounded repository work with transaction-local timeouts.
func (transaction *gormCommonRepCleanupTransaction) WithinSerializableCommonRepCleanup(
	ctx context.Context,
	operationName string,
	readOnly bool,
	statementTimeout time.Duration,
	lockTimeout time.Duration,
	work func(commonRepCleanupRepository) error,
) error {
	if transaction == nil || transaction.db == nil || transaction.transactionManager == nil {
		return fmt.Errorf("common representation cleanup transaction adapter is not configured")
	}
	if transaction.db.Name() != "postgres" {
		return fmt.Errorf("common representation cleanup requires PostgreSQL")
	}
	if statementTimeout <= 0 || lockTimeout <= 0 || lockTimeout > statementTimeout {
		return fmt.Errorf("common representation cleanup transaction timeouts must be positive and lock timeout cannot exceed statement timeout")
	}
	if work == nil {
		return fmt.Errorf("common representation cleanup transaction work is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	return transaction.transactionManager.HandleSerializableTransaction(operationName, transaction.db.WithContext(ctx), func(tx *gorm.DB) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if tx.Error != nil {
			return tx.Error
		}
		if readOnly {
			if err := tx.Exec("SET TRANSACTION READ ONLY").Error; err != nil {
				return fmt.Errorf("set cleanup transaction read-only: %w", err)
			}
		}
		if err := setLocalCommonRepCleanupTimeout(tx, "statement_timeout", statementTimeout); err != nil {
			return fmt.Errorf("set common representation cleanup statement timeout: %w", err)
		}
		if err := setLocalCommonRepCleanupTimeout(tx, "lock_timeout", lockTimeout); err != nil {
			return fmt.Errorf("set common representation cleanup lock timeout: %w", err)
		}
		return work(&gormCommonRepCleanupRepository{db: tx.WithContext(ctx)})
	})
}

func setLocalCommonRepCleanupTimeout(tx *gorm.DB, setting string, timeout time.Duration) error {
	if setting != "statement_timeout" && setting != "lock_timeout" {
		return fmt.Errorf("unsupported PostgreSQL timeout setting %q", setting)
	}
	milliseconds := int64(timeout / time.Millisecond)
	if timeout%time.Millisecond != 0 {
		milliseconds++
	}
	if milliseconds < 1 {
		milliseconds = 1
	}
	return tx.Exec(fmt.Sprintf("SET LOCAL %s = '%dms'", setting, milliseconds)).Error
}
