package tombstonecleanup

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type candidate struct {
	ID                    uuid.UUID
	ResourceID            uuid.UUID
	UpdatedAt             time.Time
	Generation            uint
	RepresentationVersion uint
}

// SQL statements are fixed literals; runtime values use bind parameters only.
const insertCandidatesSQL = `INSERT INTO pg_temp.tombstone_cleanup_candidates SELECT id,resource_id,updated_at,generation,representation_version
 FROM reporter_resources WHERE tombstone IS TRUE AND updated_at < ?`
const selectBatchSQL = `SELECT * FROM pg_temp.tombstone_cleanup_candidates
 ORDER BY updated_at ASC, id ASC LIMIT ?`

const impactAllSQL = `WITH eligible AS (SELECT id,resource_id,updated_at,generation,representation_version
 FROM reporter_resources WHERE tombstone IS TRUE AND updated_at < ?), selected AS (
 SELECT * FROM eligible
 ), removable AS (
 SELECT DISTINCT selected.resource_id FROM selected WHERE NOT EXISTS (
 SELECT 1 FROM reporter_resources remaining WHERE remaining.resource_id=selected.resource_id
 AND NOT EXISTS (SELECT 1 FROM selected WHERE selected.id=remaining.id)))
 SELECT (SELECT COUNT(*) FROM eligible) AS candidates,
 (SELECT COUNT(*) FROM selected) AS reporter_resources,
 (SELECT COUNT(DISTINCT resource_id) FROM selected) AS affected,
 (SELECT COUNT(*) FROM reporter_representations h JOIN selected ON selected.id=h.reporter_resource_id) AS reporter_history,
 (SELECT COUNT(*) FROM common_representations h JOIN removable USING(resource_id)) AS common_history,
 (SELECT COUNT(*) FROM resource r JOIN removable ON removable.resource_id=r.id) AS roots`

// impact reports full eligible impact; capped executions report a separate row ceiling.
func impact(tx *gorm.DB, cutoff time.Time) (r result, err error) {
	var counts struct{ Candidates, ReporterResources, Affected, ReporterHistory, CommonHistory, Roots int64 }
	err = tx.Raw(impactAllSQL, cutoff).Scan(&counts).Error
	r.candidates, r.reporterResources, r.affected = counts.Candidates, counts.ReporterResources, counts.Affected
	r.reporterHistory, r.commonHistory, r.roots = counts.ReporterHistory, counts.CommonHistory, counts.Roots
	return
}

// deletedRows excludes temporary worklist housekeeping.
func (r result) deletedRows() int64 {
	return r.reporterResources + r.reporterHistory + r.commonHistory + r.roots
}

// deleteBatch drains histories explicitly, retaining the parent as a restart anchor.
// Only finalized or skipped parents leave the worklist; partial history is progress.
func deleteBatch(tx *gorm.DB, batchSize int, cutoff time.Time) (r result, err error) {
	var batch []candidate
	if err = tx.Raw(selectBatchSQL, batchSize).Scan(&batch).Error; err != nil {
		return
	}
	for _, c := range batch {
		budget := int64(batchSize) - r.deletedRows()
		if budget == 0 {
			break
		}
		ids := []uuid.UUID{c.ID}
		var eligible []candidate
		if err = tx.Raw(recheckCandidatesSQL, ids, cutoff).Scan(&eligible).Error; err != nil {
			return
		}
		if len(eligible) == 0 {
			if err = tx.Exec(drainCandidatesSQL, ids).Error; err != nil {
				return
			}
			r.selected++
			r.skipped++
			continue
		}
		before := r.deletedRows()
		finalized := false
		reporter := tx.Exec(deleteReporterChunkSQL, c.ID, c.Generation, c.RepresentationVersion, budget)
		if err = reporter.Error; err != nil {
			return
		}
		r.reporterHistory += reporter.RowsAffected
		budget = int64(batchSize) - r.deletedRows()
		var shared bool
		if err = tx.Raw(hasOtherReporterSQL, c.ResourceID, c.ID).Scan(&shared).Error; err != nil {
			return
		}
		if budget > 0 && !shared {
			common := tx.Exec(deleteCommonChunkSQL, c.ResourceID, c.ResourceID, c.ResourceID, budget)
			if err = common.Error; err != nil {
				return
			}
			r.commonHistory += common.RowsAffected
		}
		// Remaining histories are protected snapshots, unless a chunk used the budget.
		budget = int64(batchSize) - r.deletedRows()
		if budget > 0 {
			var tail struct{ Reporter, Common, Roots int64 }
			if err = tx.Raw(finalizationSizeSQL, c.ID, c.ResourceID, c.ResourceID).Scan(&tail).Error; err != nil {
				return
			}
			needed := tail.Reporter + 1
			if !shared {
				needed += tail.Common + tail.Roots
			}
			if needed <= budget {
				history := tx.Exec(deleteFinalReporterSQL, c.ID, tail.Reporter)
				if err = history.Error; err != nil {
					return
				}
				r.reporterHistory += history.RowsAffected
				if !shared {
					common := tx.Exec(deleteFinalCommonSQL, c.ResourceID, tail.Common)
					if err = common.Error; err != nil {
						return
					}
					r.commonHistory += common.RowsAffected
				}
				parent := tx.Exec(deleteParentsSQL, ids, cutoff)
				if err = parent.Error; err != nil {
					return
				}
				if parent.RowsAffected != 1 {
					return r, fmt.Errorf("parent changed or reporter history remains")
				}
				r.reporterResources++
				if !shared {
					root := tx.Exec(deleteRootsSQL, []uuid.UUID{c.ResourceID})
					if err = root.Error; err != nil {
						return
					}
					r.roots += root.RowsAffected
				}
				if err = tx.Exec(drainCandidatesSQL, ids).Error; err != nil {
					return
				}
				r.selected++
				finalized = true
			}
		}
		if r.deletedRows() > int64(batchSize) {
			return r, fmt.Errorf("deleted row budget exceeded")
		}
		if r.deletedRows() > before {
			// A finalized parent has already left the worklist, so record its root directly.
			affected := tx.Exec(recordAffectedRootSQL, c.ResourceID)
			if err = affected.Error; err != nil {
				return
			}
			r.affected += affected.RowsAffected
		}
		// Finish this oldest parent before advancing to younger candidates.
		if !finalized {
			break
		}
	}
	return
}

const deleteReporterChunkSQL = `WITH chunk AS MATERIALIZED (
 SELECT reporter_resource_id,version,generation FROM reporter_representations
 WHERE reporter_resource_id=? AND NOT (generation=? AND version=?)
 ORDER BY version,generation LIMIT ?)
 DELETE FROM reporter_representations h USING chunk c WHERE h.reporter_resource_id=c.reporter_resource_id AND h.version=c.version AND h.generation=c.generation`

const hasOtherReporterSQL = `SELECT EXISTS(SELECT 1 FROM reporter_resources WHERE resource_id=? AND id<>?)`

// Keep both the API's pointer and MAX(version), which is used for version allocation.
const deleteCommonChunkSQL = `WITH chunk AS MATERIALIZED (
 SELECT resource_id,version FROM common_representations WHERE resource_id=?
 AND version IS DISTINCT FROM (SELECT common_version FROM resource WHERE id=?)
 AND version<>(SELECT MAX(version) FROM common_representations WHERE resource_id=?)
 ORDER BY version LIMIT ?)
 DELETE FROM common_representations h USING chunk c WHERE h.resource_id=c.resource_id AND h.version=c.version`

// LIMIT keeps this check bounded even when the next chunk has much more history.
const finalizationSizeSQL = `SELECT
 (SELECT COUNT(*) FROM (SELECT 1 FROM reporter_representations WHERE reporter_resource_id=? LIMIT 6) h) AS reporter,
 (SELECT COUNT(*) FROM (SELECT 1 FROM common_representations WHERE resource_id=? LIMIT 6) h) AS common,
 (SELECT COUNT(*) FROM resource WHERE id=?) AS roots`
const deleteFinalReporterSQL = `WITH chunk AS MATERIALIZED (
 SELECT reporter_resource_id,version,generation FROM reporter_representations WHERE reporter_resource_id=? LIMIT ?)
 DELETE FROM reporter_representations h USING chunk c WHERE h.reporter_resource_id=c.reporter_resource_id AND h.version=c.version AND h.generation=c.generation`
const deleteFinalCommonSQL = `WITH chunk AS MATERIALIZED (
 SELECT resource_id,version FROM common_representations WHERE resource_id=? LIMIT ?)
 DELETE FROM common_representations h USING chunk c WHERE h.resource_id=c.resource_id AND h.version=c.version`
const recordAffectedRootSQL = `INSERT INTO pg_temp.tombstone_cleanup_affected VALUES (?) ON CONFLICT DO NOTHING`

const readOnlySQL = "SET TRANSACTION READ ONLY"

const disableStatementTimeoutSQL = "SET LOCAL statement_timeout=0"

const dropWorklistSQL = "DROP TABLE IF EXISTS pg_temp.tombstone_cleanup_candidates,pg_temp.tombstone_cleanup_affected"

const createWorklistSQL = `CREATE TEMP TABLE tombstone_cleanup_candidates (
    id uuid NOT NULL,resource_id uuid NOT NULL,updated_at timestamptz NOT NULL,
    generation bigint NOT NULL,representation_version bigint NOT NULL,
    PRIMARY KEY(updated_at,id),UNIQUE(id)) ON COMMIT PRESERVE ROWS`

const createAffectedSQL = "CREATE TEMP TABLE tombstone_cleanup_affected (resource_id uuid PRIMARY KEY) ON COMMIT PRESERVE ROWS"

const batchLockTimeoutSQL = "SET LOCAL lock_timeout='5s'"

const insertLimitedCandidatesSQL = `INSERT INTO pg_temp.tombstone_cleanup_candidates SELECT id,resource_id,updated_at,generation,representation_version FROM reporter_resources WHERE tombstone IS TRUE AND updated_at < ? ORDER BY updated_at ASC,id ASC LIMIT ?`

const recheckCandidatesSQL = `SELECT parent.id,parent.resource_id FROM reporter_resources parent
 JOIN pg_temp.tombstone_cleanup_candidates planned ON planned.id=parent.id
 WHERE parent.id IN ? AND parent.tombstone IS TRUE AND parent.updated_at < ?
 AND parent.resource_id=planned.resource_id AND parent.updated_at=planned.updated_at
 AND parent.generation=planned.generation AND parent.representation_version=planned.representation_version`

const deleteParentsSQL = `DELETE FROM reporter_resources parent USING pg_temp.tombstone_cleanup_candidates planned
   WHERE parent.id=planned.id AND parent.id IN ? AND parent.tombstone IS TRUE AND parent.updated_at < ?
   AND parent.resource_id=planned.resource_id AND parent.updated_at=planned.updated_at
   AND parent.generation=planned.generation AND parent.representation_version=planned.representation_version
   AND NOT EXISTS (SELECT 1 FROM reporter_representations h WHERE h.reporter_resource_id=parent.id)`

const deleteRootsSQL = `DELETE FROM resource r WHERE r.id IN ? AND NOT EXISTS
   (SELECT 1 FROM reporter_resources parent WHERE parent.resource_id=r.id)
 AND NOT EXISTS (SELECT 1 FROM common_representations h WHERE h.resource_id=r.id)`

const drainCandidatesSQL = "DELETE FROM pg_temp.tombstone_cleanup_candidates WHERE id IN ?"
