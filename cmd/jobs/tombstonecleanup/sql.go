package tombstonecleanup

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Eligibility is the same everywhere: a reporter parent that is still tombstoned
// and was last updated strictly before the frozen cutoff. NULL timestamps never match.
// A root's other reporter "survives" unless it is itself eligible and in the same batch.

// previewSQL reports the full eligible impact without selecting a worklist.
const previewSQL = `WITH eligible AS (
  SELECT id, resource_id FROM reporter_resources WHERE tombstone AND updated_at < ?
 ), removable AS (
  SELECT DISTINCT e.resource_id FROM eligible e WHERE NOT EXISTS (
   SELECT 1 FROM reporter_resources other WHERE other.resource_id = e.resource_id
    AND (NOT other.tombstone OR other.updated_at IS NULL OR other.updated_at >= ?))
 )
 SELECT (SELECT COUNT(*) FROM eligible) AS parents,
  (SELECT COUNT(DISTINCT resource_id) FROM eligible) AS affected,
  (SELECT COUNT(*) FROM reporter_representations h JOIN eligible e ON e.id = h.reporter_resource_id) AS reporter_history,
  (SELECT COUNT(*) FROM common_representations h JOIN removable USING (resource_id)) AS common_history,
  (SELECT COUNT(*) FROM resource r JOIN removable ON removable.resource_id = r.id) AS roots`

const createWorklistSQL = `CREATE TEMP TABLE tombstone_cleanup_worklist (
  id uuid NOT NULL, updated_at timestamptz NOT NULL, PRIMARY KEY (updated_at, id)
 ) ON COMMIT PRESERVE ROWS`

const insertWorklistSQL = `INSERT INTO pg_temp.tombstone_cleanup_worklist
 SELECT id, updated_at FROM reporter_resources WHERE tombstone AND updated_at < ?`

// insertLimitedWorklistSQL keeps only the oldest parents a capped run can reach.
const insertLimitedWorklistSQL = `INSERT INTO pg_temp.tombstone_cleanup_worklist
 SELECT id, updated_at FROM reporter_resources WHERE tombstone AND updated_at < ?
 ORDER BY updated_at, id LIMIT ?`

const selectBatchSQL = `SELECT id FROM pg_temp.tombstone_cleanup_worklist ORDER BY updated_at, id LIMIT ?`

const drainWorklistSQL = `DELETE FROM pg_temp.tombstone_cleanup_worklist WHERE id IN ?`

const dropWorklistSQL = `DROP TABLE IF EXISTS pg_temp.tombstone_cleanup_worklist`

// trimReporterHistorySQL deletes up to LIMIT reporter rows of eligible batch
// parents, keeping each parent's current snapshot until finalization.
const trimReporterHistorySQL = `WITH chunk AS MATERIALIZED (
  SELECT h.reporter_resource_id, h.version, h.generation FROM reporter_representations h
  JOIN reporter_resources p ON p.id = h.reporter_resource_id
  WHERE p.id IN ? AND p.tombstone AND p.updated_at < ?
   AND NOT (h.generation = p.generation AND h.version = p.representation_version)
  LIMIT ?)
 DELETE FROM reporter_representations h USING chunk c
 WHERE h.reporter_resource_id = c.reporter_resource_id AND h.version = c.version AND h.generation = c.generation`

// trimCommonHistorySQL deletes up to LIMIT common rows of roots whose every
// reporter is an eligible parent in this batch. It keeps the root's referenced
// version and MAX(version), which the API uses to allocate the next version, so a
// root that is interrupted mid-trim remains usable.
const trimCommonHistorySQL = `WITH roots AS (
  SELECT DISTINCT p.resource_id FROM reporter_resources p
  WHERE p.id IN ? AND p.tombstone AND p.updated_at < ?
   AND NOT EXISTS (
    SELECT 1 FROM reporter_resources other WHERE other.resource_id = p.resource_id
     AND (other.id NOT IN ? OR NOT other.tombstone OR other.updated_at IS NULL OR other.updated_at >= ?))
 ), chunk AS MATERIALIZED (
  SELECT h.resource_id, h.version FROM common_representations h
  JOIN roots ON roots.resource_id = h.resource_id
  LEFT JOIN resource r ON r.id = h.resource_id
  WHERE h.version IS DISTINCT FROM r.common_version
   AND h.version < (SELECT MAX(m.version) FROM common_representations m WHERE m.resource_id = h.resource_id)
  LIMIT ?)
 DELETE FROM common_representations h USING chunk c
 WHERE h.resource_id = c.resource_id AND h.version = c.version`

const finalReporterHistorySQL = `DELETE FROM reporter_representations h USING reporter_resources p
 WHERE h.reporter_resource_id = p.id AND p.id IN ? AND p.tombstone AND p.updated_at < ?`

const deleteParentsSQL = `DELETE FROM reporter_resources
 WHERE id IN ? AND tombstone AND updated_at < ? RETURNING resource_id`

const finalCommonHistorySQL = `DELETE FROM common_representations h WHERE h.resource_id IN ?
 AND NOT EXISTS (SELECT 1 FROM reporter_resources p WHERE p.resource_id = h.resource_id)`

const deleteRootsSQL = `DELETE FROM resource r WHERE r.id IN ?
 AND NOT EXISTS (SELECT 1 FROM reporter_resources p WHERE p.resource_id = r.id)`

const statementTimeoutSQL = "SET LOCAL statement_timeout = 0"

const lockTimeoutSQL = "SET LOCAL lock_timeout = '5s'"

// counts are rows deleted from each production table.
type counts struct {
	reporterHistory, commonHistory, parents, roots int64
}

func (c counts) total() int64 {
	return c.reporterHistory + c.commonHistory + c.parents + c.roots
}

func (c *counts) add(other counts) {
	c.reporterHistory += other.reporterHistory
	c.commonHistory += other.commonHistory
	c.parents += other.parents
	c.roots += other.roots
}

// trimHistory deletes at most limit history rows for the batch, reporter history
// first. Fewer than limit rows means the batch has no more trimmable history.
func trimHistory(tx *gorm.DB, ids []uuid.UUID, cutoff time.Time, limit int64) (c counts, err error) {
	reporter := tx.Exec(trimReporterHistorySQL, ids, cutoff, limit)
	if err = reporter.Error; err != nil {
		return
	}
	c.reporterHistory = reporter.RowsAffected
	if remaining := limit - c.reporterHistory; remaining > 0 {
		common := tx.Exec(trimCommonHistorySQL, ids, cutoff, ids, cutoff, remaining)
		if err = common.Error; err != nil {
			return
		}
		c.commonHistory = common.RowsAffected
	}
	return
}

// finalizeBatch atomically removes the remaining snapshots, the still-eligible
// parents, and roots left without any reporter, then drains the batch from the
// worklist. Parents that changed since selection are skipped. After trimming,
// each parent leaves at most five rows, so the limit only trips if history was
// added during the batch; the transaction then rolls back.
func finalizeBatch(tx *gorm.DB, ids []uuid.UUID, cutoff time.Time, limit int64) (c counts, skipped int64, err error) {
	reporter := tx.Exec(finalReporterHistorySQL, ids, cutoff)
	if err = reporter.Error; err != nil {
		return
	}
	c.reporterHistory = reporter.RowsAffected
	var roots []uuid.UUID
	if err = tx.Raw(deleteParentsSQL, ids, cutoff).Scan(&roots).Error; err != nil {
		return
	}
	c.parents = int64(len(roots))
	if len(roots) > 0 {
		common := tx.Exec(finalCommonHistorySQL, roots)
		if err = common.Error; err != nil {
			return
		}
		c.commonHistory = common.RowsAffected
		deleted := tx.Exec(deleteRootsSQL, roots)
		if err = deleted.Error; err != nil {
			return
		}
		c.roots = deleted.RowsAffected
	}
	if c.total() > limit {
		return counts{}, 0, fmt.Errorf("finalization would delete %d rows, above rows-per-transaction %d; history was added during the batch, rerun the job", c.total(), limit)
	}
	if err = tx.Exec(drainWorklistSQL, ids).Error; err != nil {
		return
	}
	return c, int64(len(ids)) - c.parents, nil
}
