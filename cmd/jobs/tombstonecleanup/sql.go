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

const impactLimitedSQL = `WITH eligible AS (SELECT id,resource_id,updated_at,generation,representation_version
 FROM reporter_resources WHERE tombstone IS TRUE AND updated_at < ?), selected AS (
 SELECT * FROM eligible ORDER BY updated_at ASC, id ASC LIMIT ?
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

// impact counts reporter cascades and only roots whose every reporter is selected.
// The prefix limit bounds parents, not the number of cascading history rows.
func impact(tx *gorm.DB, cutoff time.Time, limit int64) (r result, err error) {
	var counts struct{ Candidates, ReporterResources, Affected, ReporterHistory, CommonHistory, Roots int64 }
	query := impactAllSQL
	arguments := []any{cutoff}
	if limit > 0 {
		query = impactLimitedSQL
		arguments = append(arguments, limit)
	}
	err = tx.Raw(query, arguments...).Scan(&counts).Error
	r.candidates = counts.Candidates
	r.reporterResources = counts.ReporterResources
	r.affected = counts.Affected
	r.reporterHistory = counts.ReporterHistory
	r.commonHistory = counts.CommonHistory
	r.roots = counts.Roots
	return
}

// deleteBatch rechecks frozen identities before deletion. Separate statements
// ensure orphan checks see the reporter deletions within this transaction.
func deleteBatch(tx *gorm.DB, batchSize int, cutoff time.Time) (r result, err error) {
	var batch []candidate
	if err = tx.Raw(selectBatchSQL, batchSize).Scan(&batch).Error; err != nil {
		return
	}
	r.selected = int64(len(batch))
	if len(batch) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(batch))
	for _, c := range batch {
		ids = append(ids, c.ID)
	}
	var eligible []candidate
	err = tx.Raw(`SELECT parent.id,parent.resource_id FROM reporter_resources parent
 JOIN pg_temp.tombstone_cleanup_candidates planned ON planned.id=parent.id
 WHERE parent.id IN ? AND parent.tombstone IS TRUE AND parent.updated_at < ?
 AND parent.resource_id=planned.resource_id AND parent.updated_at=planned.updated_at
 AND parent.generation=planned.generation AND parent.representation_version=planned.representation_version`, ids, cutoff).Scan(&eligible).Error
	if err != nil {
		return
	}
	if len(eligible) > 0 {
		eligibleIDs := make([]uuid.UUID, 0, len(eligible))
		rootIDs := make([]uuid.UUID, 0, len(eligible))
		for _, c := range eligible {
			eligibleIDs = append(eligibleIDs, c.ID)
			rootIDs = append(rootIDs, c.ResourceID)
		}
		if err = tx.Raw("SELECT COUNT(*) FROM reporter_representations WHERE reporter_resource_id IN ?", eligibleIDs).Scan(&r.reporterHistory).Error; err != nil {
			return
		}
		deleted := tx.Exec(`DELETE FROM reporter_resources parent USING pg_temp.tombstone_cleanup_candidates planned
   WHERE parent.id=planned.id AND parent.id IN ? AND parent.tombstone IS TRUE AND parent.updated_at < ?
   AND parent.resource_id=planned.resource_id AND parent.updated_at=planned.updated_at
   AND parent.generation=planned.generation AND parent.representation_version=planned.representation_version`, eligibleIDs, cutoff)
		if err = deleted.Error; err != nil {
			return
		}
		r.reporterResources = deleted.RowsAffected
		if r.reporterResources != int64(len(eligible)) {
			return r, fmt.Errorf("eligible reporters changed within transaction")
		}
		affected := tx.Exec("INSERT INTO pg_temp.tombstone_cleanup_affected SELECT DISTINCT resource_id FROM pg_temp.tombstone_cleanup_candidates WHERE id IN ? ON CONFLICT DO NOTHING", eligibleIDs)
		if err = affected.Error; err != nil {
			return
		}
		r.affected = affected.RowsAffected
		common := tx.Exec(`DELETE FROM common_representations h WHERE h.resource_id IN ?
   AND NOT EXISTS (SELECT 1 FROM reporter_resources parent WHERE parent.resource_id=h.resource_id)`, rootIDs)
		if err = common.Error; err != nil {
			return
		}
		r.commonHistory = common.RowsAffected
		roots := tx.Exec(`DELETE FROM resource r WHERE r.id IN ? AND NOT EXISTS
   (SELECT 1 FROM reporter_resources parent WHERE parent.resource_id=r.id)`, rootIDs)
		if err = roots.Error; err != nil {
			return
		}
		r.roots = roots.RowsAffected
	}
	r.skipped = r.selected - r.reporterResources
	err = tx.Exec("DELETE FROM pg_temp.tombstone_cleanup_candidates WHERE id IN ?", ids).Error
	return
}
