package jobs

import (
	"fmt"

	"gorm.io/gorm"
)

// commonRepCleanupCandidateQuery is shared by preview and worklist creation.
// Only version order determines retention; timestamps are not consulted.
// Above-pointer rows are never eligible for pruning.
func commonRepCleanupCandidateQuery(options commonRepCleanupOptions) (string, []any) {
	typeFilter := ""
	arguments := []any{}
	if options.resourceType != "" {
		typeFilter = " AND resource.type = ?"
		arguments = append(arguments, options.resourceType)
	}
	query := `
  WITH resource_history AS (
   -- Resolve and verify the current pointer once per active resource.
   SELECT resource.id AS resource_id,
    COALESCE(resource.common_version, MAX(history.version)) AS current_version
   FROM resource
   JOIN common_representations history ON history.resource_id = resource.id
   WHERE EXISTS (
    SELECT 1 FROM reporter_resources reporter
    WHERE reporter.resource_id = resource.id AND reporter.tombstone IS FALSE
   )` + typeFilter + `
   GROUP BY resource.id, resource.common_version
   HAVING resource.common_version IS NULL OR BOOL_OR(history.version = resource.common_version)
  ), ranked_history AS (
   -- Keep the current representation and its newest existing predecessors.
   SELECT history.resource_id, history.version,
    ROW_NUMBER() OVER (PARTITION BY history.resource_id ORDER BY history.version DESC) AS version_rank
   FROM common_representations history
   JOIN resource_history ON resource_history.resource_id = history.resource_id
   WHERE history.version <= resource_history.current_version
  )
  SELECT resource_id, version FROM ranked_history
  WHERE version_rank > ?`
	return query, append(arguments, options.keepVersions)
}

// deleteCommonRepCleanupBatch drains exactly the selected worklist entries and
// their history in one transaction. A mismatch aborts rather than skipping work.
func deleteCommonRepCleanupBatch(tx *gorm.DB, batchSize int) (int64, error) {
	var counts struct{ Selected, Deleted, Drained int64 }
	err := tx.Raw(`
  WITH batch AS MATERIALIZED (
   SELECT resource_id, version FROM pg_temp.common_rep_cleanup_candidates
   ORDER BY resource_id, version LIMIT ?
  ), deleted AS (
   DELETE FROM common_representations history USING batch
   WHERE history.resource_id = batch.resource_id AND history.version = batch.version
   RETURNING history.resource_id, history.version
  ), drained AS (
   DELETE FROM pg_temp.common_rep_cleanup_candidates candidates USING deleted
   WHERE candidates.resource_id = deleted.resource_id AND candidates.version = deleted.version
   RETURNING candidates.resource_id
  )
  SELECT (SELECT COUNT(*) FROM batch) AS selected,
   (SELECT COUNT(*) FROM deleted) AS deleted,
   (SELECT COUNT(*) FROM drained) AS drained`, batchSize).Scan(&counts).Error
	if err != nil {
		return 0, err
	}
	if counts.Selected == 0 || counts.Selected != counts.Deleted || counts.Deleted != counts.Drained {
		return 0, fmt.Errorf("cleanup worklist mismatch: selected=%d deleted=%d drained=%d", counts.Selected, counts.Deleted, counts.Drained)
	}
	return counts.Deleted, nil
}
