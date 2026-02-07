package gorm

import (
	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

// FindResourceByKeysResult is the row shape returned by the join query in
// [resourceRepository.FindResourceByKeys].
type FindResourceByKeysResult struct {
	ReporterResourceID    uuid.UUID `gorm:"column:reporter_resource_id"`
	RepresentationVersion uint      `gorm:"column:representation_version"`
	Generation            uint      `gorm:"column:generation"`
	Tombstone             bool      `gorm:"column:tombstone"`
	CommonVersion         uint      `gorm:"column:common_version"`
	ResourceID            uuid.UUID `gorm:"column:resource_id"`
	ResourceType          string    `gorm:"column:resource_type"`
	LocalResourceID       string    `gorm:"column:local_resource_id"`
	ReporterType          string    `gorm:"column:reporter_type"`
	ReporterInstanceID    string    `gorm:"column:reporter_instance_id"`
	ConsistencyToken      string    `gorm:"column:consistency_token"`
}

// ToSnapshotsFromResults converts a slice of FindResourceByKeysResult into
// domain snapshot types suitable for [bizmodel.DeserializeResource].
func ToSnapshotsFromResults(results []FindResourceByKeysResult) (*bizmodel.ResourceSnapshot, []bizmodel.ReporterResourceSnapshot) {
	if len(results) == 0 {
		return nil, nil
	}

	var reporterSnapshots []bizmodel.ReporterResourceSnapshot
	var resourceSnapshot bizmodel.ResourceSnapshot

	for i, result := range results {
		resSnap, repSnap := result.ToSnapshots()

		if i == 0 {
			resourceSnapshot = resSnap
		}
		reporterSnapshots = append(reporterSnapshots, repSnap)
	}

	return &resourceSnapshot, reporterSnapshots
}

// ToSnapshots converts a single result row into domain snapshot types.
func (result FindResourceByKeysResult) ToSnapshots() (bizmodel.ResourceSnapshot, bizmodel.ReporterResourceSnapshot) {
	resourceSnapshot := bizmodel.ResourceSnapshot{
		ID:               result.ResourceID,
		Type:             result.ResourceType,
		CommonVersion:    result.CommonVersion,
		ConsistencyToken: result.ConsistencyToken,
	}

	keySnapshot := bizmodel.ReporterResourceKeySnapshot{
		LocalResourceID:    result.LocalResourceID,
		ReporterType:       result.ReporterType,
		ResourceType:       result.ResourceType,
		ReporterInstanceID: result.ReporterInstanceID,
	}

	reporterResourceSnapshot := bizmodel.ReporterResourceSnapshot{
		ID:                    result.ReporterResourceID,
		ReporterResourceKey:   keySnapshot,
		ResourceID:            result.ResourceID,
		RepresentationVersion: result.RepresentationVersion,
		Generation:            result.Generation,
		Tombstone:             result.Tombstone,
	}

	return resourceSnapshot, reporterResourceSnapshot
}
