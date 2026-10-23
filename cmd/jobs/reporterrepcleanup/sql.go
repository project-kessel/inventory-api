package reporterrepcleanup

import (
	"fmt"

	"gorm.io/gorm"
)

// reporterRepCleanupCandidateQuery is shared by preview and worklist creation.
// Only version order determines retention; timestamps are not consulted.
// Above-pointer rows are never eligible for pruning. Parent pointers may have
// gaps after common-only updates, so an exact history match is not required.
func reporterRepCleanupCandidateQuery(options reporterRepCleanupOptions) (string, []any) {
	query := `
  WITH ranked_history AS (
   SELECT history.reporter_resource_id, history.generation, history.version,
    ROW_NUMBER() OVER (
     PARTITION BY history.reporter_resource_id
     ORDER BY history.generation DESC, history.version DESC
    ) AS version_rank
   FROM reporter_representations history
   JOIN reporter_resources reporter ON reporter.id = history.reporter_resource_id
   WHERE reporter.tombstone IS FALSE
    AND (history.generation, history.version) <= (reporter.generation, reporter.representation_version)
  )
  SELECT reporter_resource_id, generation, version FROM ranked_history
  WHERE version_rank > ?`
	return query, []any{options.keepVersions}
}

// deleteReporterRepCleanupBatch drains exactly the selected worklist entries and
// their history in one transaction. A mismatch aborts rather than skipping work.
func deleteReporterRepCleanupBatch(tx *gorm.DB, batchSize int) (int64, error) {
	var counts struct{ Selected, Deleted, Drained int64 }
	err := tx.Raw(`
  WITH batch AS MATERIALIZED (
   SELECT reporter_resource_id, generation, version FROM pg_temp.reporter_rep_cleanup_candidates
   ORDER BY reporter_resource_id, generation, version LIMIT ?
  ), deleted AS (
   DELETE FROM reporter_representations history USING batch
   WHERE history.reporter_resource_id = batch.reporter_resource_id AND history.generation = batch.generation AND history.version = batch.version
   RETURNING history.reporter_resource_id, history.generation, history.version
  ), drained AS (
   DELETE FROM pg_temp.reporter_rep_cleanup_candidates candidates USING deleted
   WHERE candidates.reporter_resource_id = deleted.reporter_resource_id AND candidates.generation = deleted.generation AND candidates.version = deleted.version
   RETURNING candidates.reporter_resource_id
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
