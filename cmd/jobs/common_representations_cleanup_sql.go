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

type commonRepChunkClassificationRow struct {
	ResourceID   uuid.UUID `gorm:"column:resource_id"`
	Exclusion    string    `gorm:"column:exclusion"`
	PrunableRows int64     `gorm:"column:prunable_rows"`
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

	rangeFilter := "cr.resource_id > ?"
	rangeArguments := []any{after}
	if found {
		rangeFilter += " AND cr.resource_id <= ?"
		rangeArguments = append(rangeArguments, boundary)
	}
	typeFilter := ""
	if resourceType, hasFilter := policy.ResourceType(); hasFilter {
		typeFilter = " WHERE r.type = ?"
		rangeArguments = append(rangeArguments, resourceType.Serialize())
	}

	classificationQuery := `
		WITH chunk AS (
			SELECT cr.resource_id, cr.version
			FROM common_representations cr
			WHERE ` + rangeFilter + `
		), typed AS (
			SELECT DISTINCT c.resource_id AS resource_id, r.common_version
			FROM chunk c JOIN resource r ON r.id = c.resource_id` + typeFilter + `
		), scoped AS (
			SELECT t.resource_id, t.common_version
			FROM typed t
			WHERE EXISTS (
				SELECT 1 FROM reporter_resources rr
				WHERE rr.resource_id = t.resource_id AND rr.tombstone IS FALSE
			)
		), out_of_scope AS (
			SELECT t.resource_id FROM typed t
			EXCEPT
			SELECT s.resource_id FROM scoped s
		), authoritative AS (
			SELECT s.resource_id, s.common_version,
				COALESCE(s.common_version, (
					SELECT MAX(c.version) FROM chunk c WHERE c.resource_id = s.resource_id
				)) AS current_version
			FROM scoped s
		), classified AS (
			SELECT a.resource_id, a.current_version,
				CASE WHEN a.common_version IS NOT NULL AND NOT EXISTS (
					SELECT 1 FROM chunk c
					WHERE c.resource_id = a.resource_id AND c.version = a.common_version
				) THEN 'inconsistent_pointer' ELSE '' END AS exclusion
			FROM authoritative a
		), ranked AS (
			SELECT c.resource_id, c.version,
				ROW_NUMBER() OVER (PARTITION BY c.resource_id ORDER BY c.version DESC) AS version_rank
			FROM chunk c JOIN classified cl ON cl.resource_id = c.resource_id
			WHERE cl.exclusion = '' AND c.version <= cl.current_version
		), per_root AS (
			SELECT cl.resource_id, cl.exclusion,
				COUNT(r.version) FILTER (WHERE r.version_rank > ?) AS prunable_rows
			FROM classified cl LEFT JOIN ranked r ON r.resource_id = cl.resource_id
			GROUP BY cl.resource_id, cl.exclusion
			UNION ALL
			SELECT o.resource_id, 'no_active_reporter', 0
			FROM out_of_scope o
		)
		SELECT resource_id, exclusion, prunable_rows FROM per_root ORDER BY resource_id`

	classificationArguments := append(rangeArguments, policy.KeepVersions())
	var rows []commonRepChunkClassificationRow
	if err := repository.db.WithContext(ctx).Raw(classificationQuery, classificationArguments...).Scan(&rows).Error; err != nil {
		return nil, uuid.Nil, false, fmt.Errorf("classify common representation cleanup chunk: %w", err)
	}
	classifications := make([]commonRepRootClassification, 0, len(rows))
	for _, row := range rows {
		classification, err := newCommonRepRootClassification(row.ResourceID, row.Exclusion, row.PrunableRows)
		if err != nil {
			return nil, uuid.Nil, false, fmt.Errorf("classify common representation cleanup chunk: %w", err)
		}
		classifications = append(classifications, classification)
	}
	return classifications, boundary, found, nil
}

// DeletePrunableRoots re-classifies the given roots inside the caller's transaction
// and deletes every prunable row they still have in one bounded statement. The
// classification is embedded in the delete, so selection and deletion cannot drift
// apart within the statement.
func (repository *gormCommonRepCleanupRepository) DeletePrunableRoots(
	ctx context.Context,
	policy commonRepCleanupPolicy,
	resourceIDs []uuid.UUID,
) (commonRepCleanupImpact, error) {
	if repository == nil || repository.db == nil {
		return commonRepCleanupImpact{}, fmt.Errorf("common representation cleanup repository is not configured")
	}
	if err := policy.Validate(); err != nil {
		return commonRepCleanupImpact{}, err
	}
	if len(resourceIDs) == 0 {
		return newCommonRepCleanupImpactWithRootIDs(0, nil)
	}
	if err := ctx.Err(); err != nil {
		return commonRepCleanupImpact{}, err
	}

	typeFilter := ""
	arguments := []any{resourceIDs}
	if resourceType, hasFilter := policy.ResourceType(); hasFilter {
		typeFilter = " AND r.type = ?"
		arguments = append(arguments, resourceType.Serialize())
	}
	arguments = append(arguments, policy.KeepVersions())

	deleteQuery := `
		WITH scoped AS (
			SELECT r.id AS resource_id, r.common_version
			FROM resource r
			WHERE r.id IN (?) AND EXISTS (
				SELECT 1 FROM reporter_resources rr
				WHERE rr.resource_id = r.id AND rr.tombstone IS FALSE
			)` + typeFilter + `
		), authoritative AS (
			SELECT s.resource_id, s.common_version,
				COALESCE(s.common_version, (
					SELECT MAX(cr.version) FROM common_representations cr WHERE cr.resource_id = s.resource_id
				)) AS current_version
			FROM scoped s
		), classified AS (
			SELECT a.resource_id, a.current_version,
				CASE WHEN a.common_version IS NOT NULL AND NOT EXISTS (
					SELECT 1 FROM common_representations cr
					WHERE cr.resource_id = a.resource_id AND cr.version = a.common_version
				) THEN 'inconsistent_pointer' ELSE '' END AS exclusion
			FROM authoritative a
		), ranked AS (
			SELECT cr.resource_id, cr.version,
				ROW_NUMBER() OVER (PARTITION BY cr.resource_id ORDER BY cr.version DESC) AS version_rank
			FROM common_representations cr JOIN classified c ON c.resource_id = cr.resource_id
			WHERE c.exclusion = '' AND cr.version <= c.current_version
		), selected AS (
			SELECT resource_id, version FROM ranked WHERE version_rank > ?
		)
		DELETE FROM common_representations cr
		USING selected
		WHERE cr.resource_id = selected.resource_id AND cr.version = selected.version
		RETURNING cr.resource_id`

	var deletedResourceIDs []uuid.UUID
	if err := repository.db.WithContext(ctx).Raw(deleteQuery, arguments...).Scan(&deletedResourceIDs).Error; err != nil {
		return commonRepCleanupImpact{}, fmt.Errorf("delete prunable common representation roots: %w", err)
	}

	distinctRootIDs := make(map[uuid.UUID]struct{}, len(deletedResourceIDs))
	for _, deletedResourceID := range deletedResourceIDs {
		distinctRootIDs[deletedResourceID] = struct{}{}
	}
	resourceRootIDs := make([]bizmodel.ResourceId, 0, len(distinctRootIDs))
	for deletedResourceID := range distinctRootIDs {
		resourceRootID, err := bizmodel.NewResourceId(deletedResourceID)
		if err != nil {
			return commonRepCleanupImpact{}, fmt.Errorf("prunable common representation has invalid resource root identity: %w", err)
		}
		resourceRootIDs = append(resourceRootIDs, resourceRootID)
	}
	impact, err := newCommonRepCleanupImpactWithRootIDs(int64(len(deletedResourceIDs)), resourceRootIDs)
	if err != nil {
		return commonRepCleanupImpact{}, fmt.Errorf("validate committed common representation cleanup impact: %w", err)
	}
	return impact, nil
}

// DeleteLowestPrunableVersions re-derives the retention threshold for one root inside
// the caller's transaction and deletes at most rowLimit of its lowest prunable
// versions. Deep roots are pruned by repeated bounded statements instead of one
// unbounded delete, and the loop terminates when a statement deletes nothing.
func (repository *gormCommonRepCleanupRepository) DeleteLowestPrunableVersions(
	ctx context.Context,
	policy commonRepCleanupPolicy,
	resourceID uuid.UUID,
	rowLimit int64,
) (int64, error) {
	if repository == nil || repository.db == nil {
		return 0, fmt.Errorf("common representation cleanup repository is not configured")
	}
	if err := policy.Validate(); err != nil {
		return 0, err
	}
	if resourceID == uuid.Nil {
		return 0, fmt.Errorf("deep common representation deletion requires a resource id")
	}
	if rowLimit <= 0 {
		return 0, fmt.Errorf("deep common representation deletion requires a positive row limit")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	typeFilter := ""
	arguments := []any{resourceID}
	if resourceType, hasFilter := policy.ResourceType(); hasFilter {
		typeFilter = " AND r.type = ?"
		arguments = append(arguments, resourceType.Serialize())
	}
	arguments = append(arguments, resourceID, resourceID, policy.KeepVersions()-1, resourceID, rowLimit)

	deleteQuery := `
		WITH scoped AS (
			SELECT r.common_version
			FROM resource r
			WHERE r.id = ? AND EXISTS (
				SELECT 1 FROM reporter_resources rr
				WHERE rr.resource_id = r.id AND rr.tombstone IS FALSE
			)` + typeFilter + `
		), authoritative AS (
			SELECT s.common_version,
				COALESCE(s.common_version, (
					SELECT MAX(cr.version) FROM common_representations cr WHERE cr.resource_id = ?
				)) AS current_version
			FROM scoped s
		), threshold AS (
			SELECT cr.version AS threshold_version
			FROM common_representations cr, authoritative a
			WHERE cr.resource_id = ? AND cr.version <= a.current_version
			ORDER BY cr.version DESC
			LIMIT 1 OFFSET ?
		), batch AS (
			SELECT cr.resource_id, cr.version
			FROM common_representations cr, threshold t
			WHERE cr.resource_id = ? AND cr.version < t.threshold_version
			ORDER BY cr.version
			LIMIT ?
		)
		DELETE FROM common_representations cr
		USING batch
		WHERE cr.resource_id = batch.resource_id AND cr.version = batch.version`

	deleted := repository.db.WithContext(ctx).Exec(deleteQuery, arguments...)
	if deleted.Error != nil {
		return 0, fmt.Errorf("delete lowest prunable common representation versions: %w", deleted.Error)
	}
	return deleted.RowsAffected, nil
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
