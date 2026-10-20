package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
	"gorm.io/gorm"
)

type gormTombstoneCleanupRepository struct {
	db *gorm.DB
}

type tombstoneCleanupCandidate struct {
	ReporterResourceID    uuid.UUID `gorm:"column:reporter_resource_id"`
	ResourceID            uuid.UUID `gorm:"column:resource_id"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
	RepresentationVersion uint      `gorm:"column:representation_version"`
	Generation            uint      `gorm:"column:generation"`
}

type tombstoneCleanupExclusionCounts struct {
	AgeCandidates                      int64 `gorm:"column:age_candidates"`
	MissingRoot                        int64 `gorm:"column:missing_root"`
	RootTypeMismatch                   int64 `gorm:"column:root_type_mismatch"`
	InvalidReporterKey                 int64 `gorm:"column:invalid_reporter_key"`
	MissingTerminalRepresentation      int64 `gorm:"column:missing_terminal_representation"`
	InconsistentTerminalRepresentation int64 `gorm:"column:inconsistent_terminal_representation"`
	NewerHistory                       int64 `gorm:"column:newer_history"`
}

type tombstoneCleanupImpactCounts struct {
	ReporterEntities        int64 `gorm:"column:reporter_entities"`
	ReporterResources       int64 `gorm:"column:reporter_resources"`
	ReporterRepresentations int64 `gorm:"column:reporter_representations"`
	CommonRepresentations   int64 `gorm:"column:common_representations"`
	ResourceRootsImpacted   int64 `gorm:"column:resource_roots_impacted"`
	ResourcesRemovable      int64 `gorm:"column:resources_removable"`
}

// PreviewTombstones reports total eligible and bounded impact without modifying stored rows.
func (repository *gormTombstoneCleanupRepository) PreviewTombstones(
	ctx context.Context,
	policy TombstoneCleanupPolicy,
	boundedReporterRowLimit int64,
) (TombstoneCleanupReport, error) {
	if repository == nil || repository.db == nil {
		return TombstoneCleanupReport{}, fmt.Errorf("tombstone cleanup repository is not configured")
	}
	if err := policy.Validate(); err != nil {
		return TombstoneCleanupReport{}, err
	}
	if boundedReporterRowLimit <= 0 {
		return TombstoneCleanupReport{}, fmt.Errorf("bounded reporter row limit must be positive")
	}
	if err := ctx.Err(); err != nil {
		return TombstoneCleanupReport{}, err
	}

	var snapshotAt time.Time
	if err := repository.db.WithContext(ctx).Raw("SELECT transaction_timestamp()").Scan(&snapshotAt).Error; err != nil {
		return TombstoneCleanupReport{}, fmt.Errorf("read tombstone cleanup snapshot time: %w", err)
	}

	classifiedCTE, arguments := tombstoneCleanupClassifiedCTE(policy)
	var exclusions tombstoneCleanupExclusionCounts
	exclusionQuery := classifiedCTE + `
		SELECT COUNT(*) AS age_candidates,
		       COUNT(*) FILTER (WHERE exclusion_reason = 'missing_root') AS missing_root,
		       COUNT(*) FILTER (WHERE exclusion_reason = 'root_type_mismatch') AS root_type_mismatch,
	       COUNT(*) FILTER (WHERE exclusion_reason = 'invalid_reporter_key') AS invalid_reporter_key,
	       COUNT(*) FILTER (WHERE exclusion_reason = 'missing_terminal_representation') AS missing_terminal_representation,
	       COUNT(*) FILTER (WHERE exclusion_reason = 'inconsistent_terminal_representation') AS inconsistent_terminal_representation,
	       COUNT(*) FILTER (WHERE exclusion_reason = 'newer_history') AS newer_history
	FROM classified_candidates`
	if err := repository.db.WithContext(ctx).Raw(exclusionQuery, arguments...).Scan(&exclusions).Error; err != nil {
		return TombstoneCleanupReport{}, fmt.Errorf("count tombstone candidates and protections: %w", err)
	}
	unknownTimestampQuery := `SELECT COUNT(*) FROM reporter_resources AS rr
		WHERE rr.tombstone IS TRUE AND rr.updated_at IS NULL`
	unknownTimestampArguments := []any{}
	if reporterType, hasFilter := policy.ReporterType(); hasFilter {
		unknownTimestampQuery += " AND rr.reporter_type = ?"
		unknownTimestampArguments = append(unknownTimestampArguments, reporterType.Serialize())
	}
	var unknownTimestampRows int64
	if err := repository.db.WithContext(ctx).Raw(unknownTimestampQuery, unknownTimestampArguments...).Scan(&unknownTimestampRows).Error; err != nil {
		return TombstoneCleanupReport{}, fmt.Errorf("count tombstones with unknown timestamps: %w", err)
	}

	totalImpact, err := repository.readImpact(ctx, classifiedCTE, arguments, 0)
	if err != nil {
		return TombstoneCleanupReport{}, fmt.Errorf("count total tombstone cleanup impact: %w", err)
	}
	boundedImpact, err := repository.readImpact(ctx, classifiedCTE, arguments, boundedReporterRowLimit)
	if err != nil {
		return TombstoneCleanupReport{}, fmt.Errorf("count bounded tombstone cleanup impact: %w", err)
	}
	protected, err := NewTombstoneCleanupExclusions(
		exclusions.MissingRoot,
		exclusions.RootTypeMismatch,
		exclusions.InvalidReporterKey,
		exclusions.MissingTerminalRepresentation,
		exclusions.InconsistentTerminalRepresentation,
		exclusions.NewerHistory,
	)
	if err != nil {
		return TombstoneCleanupReport{}, fmt.Errorf("validate tombstone cleanup exclusions: %w", err)
	}
	total, err := NewTombstoneCleanupImpact(
		totalImpact.ReporterEntities,
		totalImpact.ReporterResources,
		totalImpact.ReporterRepresentations,
		totalImpact.CommonRepresentations,
		totalImpact.ResourceRootsImpacted,
		totalImpact.ResourcesRemovable,
	)
	if err != nil {
		return TombstoneCleanupReport{}, fmt.Errorf("validate total tombstone cleanup impact: %w", err)
	}
	bounded, err := NewTombstoneCleanupImpact(
		boundedImpact.ReporterEntities,
		boundedImpact.ReporterResources,
		boundedImpact.ReporterRepresentations,
		boundedImpact.CommonRepresentations,
		boundedImpact.ResourceRootsImpacted,
		boundedImpact.ResourcesRemovable,
	)
	if err != nil {
		return TombstoneCleanupReport{}, fmt.Errorf("validate bounded tombstone cleanup impact: %w", err)
	}
	report, err := NewTombstoneCleanupReport(
		policy.EvaluationTime(), snapshotAt, policy.Cutoff(), boundedReporterRowLimit,
		exclusions.AgeCandidates, unknownTimestampRows, total, bounded, protected,
	)
	if err != nil {
		return TombstoneCleanupReport{}, fmt.Errorf("build tombstone cleanup report: %w", err)
	}
	return report, nil
}

// readImpact counts one deterministic candidate prefix and roots removable only after its selected reporters.
func (repository *gormTombstoneCleanupRepository) readImpact(
	ctx context.Context,
	classifiedCTE string,
	arguments []any,
	limit int64,
) (tombstoneCleanupImpactCounts, error) {
	selection := "SELECT * FROM eligible_candidates ORDER BY updated_at ASC, reporter_resource_id ASC"
	queryArguments := append([]any(nil), arguments...)
	if limit > 0 {
		selection += " LIMIT ?"
		queryArguments = append(queryArguments, limit)
	}
	query := classifiedCTE + `,
	eligible_candidates AS (
		SELECT * FROM classified_candidates WHERE exclusion_reason IS NULL
	),
	selected_candidates AS (` + selection + `
	),
	removable_roots AS (
		SELECT DISTINCT selected.resource_id
		FROM selected_candidates AS selected
		WHERE NOT EXISTS (
			SELECT 1
			FROM reporter_resources AS remaining
			WHERE remaining.resource_id = selected.resource_id
			  AND NOT EXISTS (
				SELECT 1 FROM selected_candidates AS selected_reporter
				WHERE selected_reporter.reporter_resource_id = remaining.id
			  )
		)
	)
	SELECT
		(SELECT COUNT(DISTINCT reporter_resource_id) FROM selected_candidates) AS reporter_entities,
		(SELECT COUNT(*) FROM selected_candidates) AS reporter_resources,
		(SELECT COUNT(*) FROM reporter_representations AS history
		 JOIN selected_candidates AS selected USING (reporter_resource_id)) AS reporter_representations,
		(SELECT COUNT(*) FROM common_representations AS common
		 JOIN removable_roots AS removable ON removable.resource_id = common.resource_id) AS common_representations,
		(SELECT COUNT(DISTINCT resource_id) FROM selected_candidates) AS resource_roots_impacted,
		(SELECT COUNT(*) FROM resource AS root
		 JOIN removable_roots AS removable ON removable.resource_id = root.id) AS resources_removable`
	var impact tombstoneCleanupImpactCounts
	if err := repository.db.WithContext(ctx).Raw(query, queryArguments...).Scan(&impact).Error; err != nil {
		return tombstoneCleanupImpactCounts{}, err
	}
	return impact, nil
}

// DeleteTombstoneBatch reselects and deletes one batch, then removes common history and only touched empty roots.
func (repository *gormTombstoneCleanupRepository) DeleteTombstoneBatch(
	ctx context.Context,
	policy TombstoneCleanupPolicy,
	batchSize int,
) (TombstoneCleanupImpact, error) {
	if repository == nil || repository.db == nil {
		return TombstoneCleanupImpact{}, fmt.Errorf("tombstone cleanup repository is not configured")
	}
	if err := policy.Validate(); err != nil {
		return TombstoneCleanupImpact{}, err
	}
	if batchSize <= 0 {
		return TombstoneCleanupImpact{}, fmt.Errorf("tombstone cleanup batch size must be positive")
	}
	if err := ctx.Err(); err != nil {
		return TombstoneCleanupImpact{}, err
	}

	classifiedCTE, arguments := tombstoneCleanupClassifiedCTE(policy)
	candidateQuery := classifiedCTE + `
		SELECT reporter_resource_id, resource_id, updated_at, representation_version, generation
		FROM classified_candidates
		WHERE exclusion_reason IS NULL
		ORDER BY updated_at ASC, reporter_resource_id ASC
		LIMIT ?`
	arguments = append(arguments, batchSize)
	var candidates []tombstoneCleanupCandidate
	if err := repository.db.WithContext(ctx).Raw(candidateQuery, arguments...).Scan(&candidates).Error; err != nil {
		return TombstoneCleanupImpact{}, fmt.Errorf("select structurally valid tombstone batch: %w", err)
	}
	if len(candidates) == 0 {
		return NewTombstoneCleanupImpact(0, 0, 0, 0, 0, 0)
	}

	reporterResourceIDs := make([]uuid.UUID, 0, len(candidates))
	resourceIDs := make([]uuid.UUID, 0, len(candidates))
	resourceRootIDs := make([]bizmodel.ResourceId, 0, len(candidates))
	seenResourceIDs := make(map[uuid.UUID]struct{}, len(candidates))
	for _, candidate := range candidates {
		reporterResourceIDs = append(reporterResourceIDs, candidate.ReporterResourceID)
		if _, seen := seenResourceIDs[candidate.ResourceID]; !seen {
			resourceRootID, err := bizmodel.NewResourceId(candidate.ResourceID)
			if err != nil {
				return TombstoneCleanupImpact{}, fmt.Errorf("selected tombstone has invalid resource root identity: %w", err)
			}
			seenResourceIDs[candidate.ResourceID] = struct{}{}
			resourceIDs = append(resourceIDs, candidate.ResourceID)
			resourceRootIDs = append(resourceRootIDs, resourceRootID)
		}
	}

	var reporterRepresentationRows int64
	if err := repository.db.WithContext(ctx).Table("reporter_representations").
		Where("reporter_resource_id IN ?", reporterResourceIDs).Count(&reporterRepresentationRows).Error; err != nil {
		return TombstoneCleanupImpact{}, fmt.Errorf("count reporter history scheduled for cascade deletion: %w", err)
	}
	deleteReporters := repository.db.WithContext(ctx).Where("id IN ?", reporterResourceIDs).
		Where("tombstone IS TRUE AND updated_at < ?", policy.Cutoff()).
		Where(`NULLIF(BTRIM(local_resource_id), '') IS NOT NULL AND
		       NULLIF(BTRIM(reporter_type), '') IS NOT NULL AND
		       NULLIF(BTRIM(resource_type), '') IS NOT NULL AND
		       NULLIF(BTRIM(reporter_instance_id), '') IS NOT NULL`).
		Where(`EXISTS (
			SELECT 1 FROM resource AS root
			WHERE root.id = reporter_resources.resource_id
			  AND root.type = reporter_resources.resource_type
		)`).
		Where(`EXISTS (
			SELECT 1 FROM reporter_representations AS terminal
			WHERE terminal.reporter_resource_id = reporter_resources.id
			  AND terminal.version = reporter_resources.representation_version
			  AND terminal.generation = reporter_resources.generation
			  AND terminal.tombstone IS TRUE
			  AND terminal.common_version IS NULL
			  AND terminal.reporter_version IS NULL
			  AND (terminal.data IS NULL OR terminal.data = 'null'::jsonb)
		)`).
		Where(`NOT EXISTS (
			SELECT 1 FROM reporter_representations AS future
			WHERE future.reporter_resource_id = reporter_resources.id
			  AND (future.generation > reporter_resources.generation OR
			       (future.generation = reporter_resources.generation AND
			        future.version > reporter_resources.representation_version))
		)`)
	if reporterType, hasFilter := policy.ReporterType(); hasFilter {
		deleteReporters = deleteReporters.Where("reporter_type = ?", reporterType.Serialize())
	}
	deletedReporters := deleteReporters.Delete(&datamodel.ReporterResource{})
	if deletedReporters.Error != nil {
		return TombstoneCleanupImpact{}, fmt.Errorf("delete selected tombstone reporter resources: %w", deletedReporters.Error)
	}
	if deletedReporters.RowsAffected != int64(len(candidates)) {
		return TombstoneCleanupImpact{}, fmt.Errorf("selected tombstone batch changed before deletion: selected %d rows, deleted %d", len(candidates), deletedReporters.RowsAffected)
	}

	var removableRoots []uuid.UUID
	if err := repository.db.WithContext(ctx).Table("resource AS root").Select("root.id").
		Where("root.id IN ?", resourceIDs).
		Where("NOT EXISTS (SELECT 1 FROM reporter_resources AS remaining WHERE remaining.resource_id = root.id)").
		Order("root.id ASC").Scan(&removableRoots).Error; err != nil {
		return TombstoneCleanupImpact{}, fmt.Errorf("identify touched roots with no surviving reporters: %w", err)
	}

	var commonRows int64
	if len(removableRoots) > 0 {
		if err := repository.db.WithContext(ctx).Table("common_representations").
			Where("resource_id IN ?", removableRoots).Count(&commonRows).Error; err != nil {
			return TombstoneCleanupImpact{}, fmt.Errorf("count common history on removable roots: %w", err)
		}
		deletedCommon := repository.db.WithContext(ctx).Where("resource_id IN ?", removableRoots).
			Delete(&datamodel.CommonRepresentation{})
		if deletedCommon.Error != nil {
			return TombstoneCleanupImpact{}, fmt.Errorf("delete common history on removable roots: %w", deletedCommon.Error)
		}
		if deletedCommon.RowsAffected != commonRows {
			return TombstoneCleanupImpact{}, fmt.Errorf("common history changed within cleanup transaction: counted %d rows, deleted %d", commonRows, deletedCommon.RowsAffected)
		}
	}

	var deletedRoots int64
	if len(removableRoots) > 0 {
		result := repository.db.WithContext(ctx).Exec(`
			DELETE FROM resource AS root
			WHERE root.id IN ?
			  AND NOT EXISTS (
				SELECT 1 FROM reporter_resources AS remaining
				WHERE remaining.resource_id = root.id
			  )`, removableRoots)
		if result.Error != nil {
			return TombstoneCleanupImpact{}, fmt.Errorf("delete touched orphan roots: %w", result.Error)
		}
		deletedRoots = result.RowsAffected
		if deletedRoots != int64(len(removableRoots)) {
			return TombstoneCleanupImpact{}, fmt.Errorf("touched roots changed within cleanup transaction: selected %d rows, deleted %d", len(removableRoots), deletedRoots)
		}
	}

	impact, err := NewTombstoneCleanupImpactWithRootIDs(
		deletedReporters.RowsAffected,
		deletedReporters.RowsAffected,
		reporterRepresentationRows,
		commonRows,
		resourceRootIDs,
		deletedRoots,
	)
	if err != nil {
		return TombstoneCleanupImpact{}, fmt.Errorf("validate committed tombstone cleanup impact: %w", err)
	}
	return impact, nil
}

// tombstoneCleanupClassifiedCTE applies the age/filter selector and assigns at most one structural exclusion reason.
func tombstoneCleanupClassifiedCTE(policy TombstoneCleanupPolicy) (string, []any) {
	predicate := "rr.tombstone IS TRUE AND rr.updated_at < ?"
	arguments := []any{policy.Cutoff()}
	if reporterType, hasFilter := policy.ReporterType(); hasFilter {
		predicate += " AND rr.reporter_type = ?"
		arguments = append(arguments, reporterType.Serialize())
	}
	query := `WITH age_candidates AS (
		SELECT rr.id AS reporter_resource_id, rr.resource_id, rr.local_resource_id,
		       rr.reporter_type, rr.resource_type, rr.reporter_instance_id,
		       rr.updated_at, rr.representation_version, rr.generation
		FROM reporter_resources AS rr
		WHERE ` + predicate + `
	), classified_candidates AS (
		SELECT candidate.*,
		       CASE
			WHEN root.id IS NULL THEN 'missing_root'
			WHEN root.type IS DISTINCT FROM candidate.resource_type THEN 'root_type_mismatch'
			WHEN NULLIF(BTRIM(candidate.local_resource_id), '') IS NULL OR
			     NULLIF(BTRIM(candidate.reporter_type), '') IS NULL OR
			     NULLIF(BTRIM(candidate.resource_type), '') IS NULL OR
			     NULLIF(BTRIM(candidate.reporter_instance_id), '') IS NULL THEN 'invalid_reporter_key'
			WHEN terminal.reporter_resource_id IS NULL THEN 'missing_terminal_representation'
			WHEN terminal.tombstone IS DISTINCT FROM TRUE OR
			     terminal.common_version IS NOT NULL OR
			     terminal.reporter_version IS NOT NULL OR
			     (terminal.data IS NOT NULL AND terminal.data <> 'null'::jsonb) THEN 'inconsistent_terminal_representation'
			WHEN EXISTS (
				SELECT 1 FROM reporter_representations AS future
				WHERE future.reporter_resource_id = candidate.reporter_resource_id
				  AND (future.generation > candidate.generation OR
				       (future.generation = candidate.generation AND future.version > candidate.representation_version))
			) THEN 'newer_history'
			ELSE NULL
		       END AS exclusion_reason
		FROM age_candidates AS candidate
		LEFT JOIN resource AS root ON root.id = candidate.resource_id
		LEFT JOIN reporter_representations AS terminal
		  ON terminal.reporter_resource_id = candidate.reporter_resource_id
		 AND terminal.version = candidate.representation_version
		 AND terminal.generation = candidate.generation
	)`
	return query, arguments
}

type gormTombstoneCleanupTransaction struct {
	db                 *gorm.DB
	transactionManager bizmodel.TransactionManager
}

// newTombstoneCleanupTransaction adapts the shared transaction manager to cleanup repository work.
func newTombstoneCleanupTransaction(db *gorm.DB, transactionManager bizmodel.TransactionManager) TombstoneCleanupTransaction {
	return &gormTombstoneCleanupTransaction{db: db, transactionManager: transactionManager}
}

// WithinSerializableTombstoneCleanup runs bounded repository work with transaction-local timeouts.
func (transaction *gormTombstoneCleanupTransaction) WithinSerializableTombstoneCleanup(
	ctx context.Context,
	operationName string,
	readOnly bool,
	statementTimeout time.Duration,
	lockTimeout time.Duration,
	work func(TombstoneCleanupRepository) error,
) error {
	if transaction == nil || transaction.db == nil || transaction.transactionManager == nil {
		return fmt.Errorf("tombstone cleanup transaction adapter is not configured")
	}
	if transaction.db.Name() != "postgres" {
		return fmt.Errorf("tombstone cleanup requires PostgreSQL")
	}
	if statementTimeout <= 0 || lockTimeout <= 0 || lockTimeout > statementTimeout {
		return fmt.Errorf("tombstone cleanup transaction timeouts must be positive and lock timeout cannot exceed statement timeout")
	}
	if work == nil {
		return fmt.Errorf("tombstone cleanup transaction work is required")
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
		if err := setLocalTombstoneCleanupTimeout(tx, "statement_timeout", statementTimeout); err != nil {
			return fmt.Errorf("set tombstone cleanup statement timeout: %w", err)
		}
		if err := setLocalTombstoneCleanupTimeout(tx, "lock_timeout", lockTimeout); err != nil {
			return fmt.Errorf("set tombstone cleanup lock timeout: %w", err)
		}
		return work(&gormTombstoneCleanupRepository{db: tx.WithContext(ctx)})
	})
}

func setLocalTombstoneCleanupTimeout(tx *gorm.DB, setting string, timeout time.Duration) error {
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
