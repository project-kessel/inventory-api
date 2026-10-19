package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"gorm.io/gorm"
)

type gormCommonRepCleanupRepository struct {
	db *gorm.DB
}

type commonRepChunkClassificationRow struct {
	ResourceID   uuid.UUID `gorm:"column:resource_id"`
	Exclusion    string    `gorm:"column:exclusion"`
	PrunableRows int64     `gorm:"column:prunable_rows"`
	RetainFrom   int64     `gorm:"column:retain_from"`
}

// ReadChunk discovers the chunk boundary at least chunkRows rows past the cursor and
// classifies every root in (cursor, boundary]. The boundary is the resource id of the
// chunkRows-th row in primary-key order, so the chunk is root-aligned: the boundary
// root's full history is always included and no root is ever split across chunks.
// When fewer than chunkRows rows remain, found is false and the classifications cover
// all remaining roots.
func (repository *gormCommonRepCleanupRepository) ReadChunk(
	ctx context.Context,
	policy commonRepCleanupPolicy,
	after uuid.UUID,
	chunkRows int,
) ([]commonRepRootClassification, uuid.UUID, bool, error) {
	if repository == nil || repository.db == nil {
		return nil, uuid.Nil, false, fmt.Errorf("common representation cleanup repository is not configured")
	}
	if err := policy.Validate(); err != nil {
		return nil, uuid.Nil, false, err
	}
	if chunkRows <= 0 {
		return nil, uuid.Nil, false, fmt.Errorf("chunk rows must be positive")
	}
	if err := ctx.Err(); err != nil {
		return nil, uuid.Nil, false, err
	}

	discoveryQuery := `
		SELECT resource_id FROM common_representations
		WHERE resource_id > ?
		ORDER BY resource_id, version
		LIMIT 1 OFFSET ?`
	var boundaries []uuid.UUID
	if err := repository.db.WithContext(ctx).Raw(discoveryQuery, after, chunkRows-1).Scan(&boundaries).Error; err != nil {
		return nil, uuid.Nil, false, fmt.Errorf("discover common representation cleanup chunk boundary: %w", err)
	}
	found := len(boundaries) == 1
	boundary := uuid.Nil
	if found {
		boundary = boundaries[0]
	}

	rangeFilter := "history.resource_id > ?"
	rangeArguments := []any{after}
	if found {
		rangeFilter += " AND history.resource_id <= ?"
		rangeArguments = append(rangeArguments, boundary)
	}
	typeFilter := ""
	if resourceType, hasFilter := policy.ResourceType(); hasFilter {
		typeFilter = " WHERE resource.type = ?"
		rangeArguments = append(rangeArguments, resourceType.Serialize())
	}

	// Classify each resource once, then rank only its eligible history. Checking
	// reporter state first preserves the exclusion reason for inactive resources
	// even when their current pointer is also inconsistent.
	classificationQuery := `
		WITH chunk AS (
			SELECT history.resource_id, history.version
			FROM common_representations history
			WHERE ` + rangeFilter + `
		), resource_status AS (
			SELECT chunk.resource_id,
				COALESCE(resource.common_version, MAX(chunk.version)) AS current_version,
				CASE
					WHEN NOT EXISTS (
						SELECT 1 FROM reporter_resources reporter
						WHERE reporter.resource_id = chunk.resource_id AND reporter.tombstone IS FALSE
					) THEN 'no_active_reporter'
					WHEN resource.common_version IS NOT NULL
						AND NOT BOOL_OR(chunk.version = resource.common_version) THEN 'inconsistent_pointer'
					ELSE ''
				END AS exclusion
			FROM chunk JOIN resource ON resource.id = chunk.resource_id` + typeFilter + `
			GROUP BY chunk.resource_id, resource.common_version
		), ranked AS (
			SELECT chunk.resource_id, chunk.version,
				ROW_NUMBER() OVER (PARTITION BY chunk.resource_id ORDER BY chunk.version DESC) AS version_rank
			FROM chunk JOIN resource_status status ON status.resource_id = chunk.resource_id
			WHERE status.exclusion = '' AND chunk.version <= status.current_version
		)
		SELECT status.resource_id, status.exclusion,
			COUNT(ranked.version) FILTER (WHERE ranked.version_rank > ?) AS prunable_rows,
			COALESCE(MIN(ranked.version) FILTER (WHERE ranked.version_rank <= ?), 0) AS retain_from
		FROM resource_status status LEFT JOIN ranked ON ranked.resource_id = status.resource_id
		GROUP BY status.resource_id, status.exclusion
		ORDER BY status.resource_id`

	classificationArguments := append(rangeArguments, policy.keepVersions, policy.keepVersions)
	var rows []commonRepChunkClassificationRow
	if err := repository.db.WithContext(ctx).Raw(classificationQuery, classificationArguments...).Scan(&rows).Error; err != nil {
		return nil, uuid.Nil, false, fmt.Errorf("classify common representation cleanup chunk: %w", err)
	}
	classifications := make([]commonRepRootClassification, 0, len(rows))
	for _, row := range rows {
		classifications = append(classifications, commonRepRootClassification{resourceID: row.ResourceID, exclusion: row.Exclusion, prunableRows: row.PrunableRows, retainFrom: row.RetainFrom})
	}
	return classifications, boundary, found, nil
}

// DeletePrunableRoots rechecks the current pointer and active reporter in the
// delete transaction. A SQL LIMIT enforces the row cap even if a concurrent
// report has expanded history since the scan. Index probes find the oldest
// retained row without ranking the full history of a deep root.
func (repository *gormCommonRepCleanupRepository) DeletePrunableRoots(ctx context.Context, policy commonRepCleanupPolicy, roots []commonRepCleanupRoot, rowLimit int) (commonRepCleanupImpact, error) {
	if err := policy.Validate(); err != nil {
		return commonRepCleanupImpact{}, err
	}
	if rowLimit <= 0 {
		return commonRepCleanupImpact{}, fmt.Errorf("row limit must be positive")
	}
	if len(roots) == 0 {
		return commonRepCleanupImpact{}, nil
	}
	typeFilter := ""
	captured, err := json.Marshal(roots)
	if err != nil {
		return commonRepCleanupImpact{}, fmt.Errorf("encode captured cleanup roots: %w", err)
	}
	arguments := []any{string(captured)}
	if resourceType, hasFilter := policy.ResourceType(); hasFilter {
		typeFilter = " AND resource.type = ?"
		arguments = append(arguments, resourceType.Serialize())
	}
	arguments = append(arguments, policy.keepVersions-1, rowLimit, rowLimit)
	query := `
		WITH captured AS (
			SELECT resource_id, retain_from
			FROM jsonb_to_recordset(?::jsonb) AS captured(resource_id uuid, retain_from bigint)
		), eligible_resources AS (
			-- Recheck the live pointer and reporter state in this delete transaction.
			SELECT resource.id, captured.retain_from,
				COALESCE(resource.common_version, (
					SELECT MAX(history.version) FROM common_representations history
					WHERE history.resource_id = resource.id
				)) AS current_version
			FROM resource JOIN captured ON captured.resource_id = resource.id
			WHERE captured.retain_from > 0
				AND (resource.common_version IS NULL OR EXISTS (
					SELECT 1 FROM common_representations history
					WHERE history.resource_id = resource.id AND history.version = resource.common_version
				))
				AND EXISTS (
					SELECT 1 FROM reporter_resources reporter
					WHERE reporter.resource_id = resource.id AND reporter.tombstone IS FALSE
				)` + typeFilter + `
		), deletion_cutoffs AS (
			-- The live retained window may narrow deletion, never advance the captured cutoff.
			SELECT resource.id, LEAST(retained.version, resource.retain_from) AS delete_before
			FROM eligible_resources resource
			CROSS JOIN LATERAL (
				SELECT history.version FROM common_representations history
				WHERE history.resource_id = resource.id AND history.version <= resource.current_version
				ORDER BY history.version DESC LIMIT 1 OFFSET ?
			) retained
		), batch AS (
			-- Bound each index probe and then the total rows deleted across all resources.
			SELECT candidate.resource_id, candidate.version
			FROM deletion_cutoffs cutoff
			CROSS JOIN LATERAL (
				SELECT history.resource_id, history.version FROM common_representations history
				WHERE history.resource_id = cutoff.id AND history.version < cutoff.delete_before
				ORDER BY history.version LIMIT ?
			) candidate
			ORDER BY candidate.resource_id, candidate.version LIMIT ?
		)
		DELETE FROM common_representations history USING batch
		WHERE history.resource_id = batch.resource_id AND history.version = batch.version
		RETURNING history.resource_id`
	var deletedIDs []uuid.UUID
	if err := repository.db.WithContext(ctx).Raw(query, arguments...).Scan(&deletedIDs).Error; err != nil {
		return commonRepCleanupImpact{}, fmt.Errorf("delete prunable common representations: %w", err)
	}
	return commonRepCleanupImpact{representations: int64(len(deletedIDs)), resourceRootIDs: deletedIDs}, nil
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
