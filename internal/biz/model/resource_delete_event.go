package model

import (
	"time"

	"github.com/google/uuid"
)

type ResourceDeleteEvent struct {
	id                     ResourceId
	resourceType           ResourceType
	reporterId             ReporterId
	localResourceId        LocalResourceId
	reporterRepresentation ReporterDeleteRepresentation
	commonVersion          *Version // Last live common version (for tuple generation)
	createdAt              time.Time
	updatedAt              time.Time
}

func NewResourceDeleteEvent(
	resourceId ResourceId,
	resourceType ResourceType,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
	localResourceId LocalResourceId,
	reporterRepresentation ReporterDeleteRepresentation,
	commonVersion *Version,
) (ResourceDeleteEvent, error) {
	reporterId := NewReporterId(reporterType, reporterInstanceId)

	return ResourceDeleteEvent{
		id:                     resourceId,
		resourceType:           resourceType,
		reporterId:             reporterId,
		localResourceId:        localResourceId,
		reporterRepresentation: reporterRepresentation,
		commonVersion:          commonVersion,
	}, nil
}

func (re ResourceDeleteEvent) CreatedAt() *time.Time {
	return &re.createdAt
}

func (re ResourceDeleteEvent) UpdatedAt() *time.Time {
	return &re.updatedAt
}

func (re ResourceDeleteEvent) ResourceType() ResourceType {
	return re.resourceType
}

func (re ResourceDeleteEvent) ReporterType() ReporterType {
	return re.reporterId.reporterType
}

func (re ResourceDeleteEvent) ReporterInstanceId() string {
	return re.reporterId.reporterInstanceId.String()
}

func (re ResourceDeleteEvent) Id() ResourceId {
	return re.id
}

func (re ResourceDeleteEvent) LocalResourceId() string {
	return re.localResourceId.String()
}

func (re ResourceDeleteEvent) ResourceId() uuid.UUID {
	return uuid.UUID(re.id)
}

func (re ResourceDeleteEvent) WorkspaceId() *string {
	return nil
}

// CurrentCommonVersion returns the common version at deletion time.
// Since common doesn't get tombstoned, this IS the last live version.
func (re ResourceDeleteEvent) CurrentCommonVersion() *Version {
	return re.commonVersion
}

// CurrentReporterRepresentationVersion returns the tombstone version.
// Consumer should fetch (version - 1) to get last live reporter data.
func (re ResourceDeleteEvent) CurrentReporterRepresentationVersion() *Version {
	return &re.reporterRepresentation.version
}

// CurrentReporterGeneration returns the generation from the ReporterRepresentation.
func (re ResourceDeleteEvent) CurrentReporterGeneration() *Generation {
	gen := re.reporterRepresentation.Generation()
	return &gen
}

// ReporterResourceKey constructs and returns the ReporterResourceKey from the event fields
func (re ResourceDeleteEvent) ReporterResourceKey() ReporterResourceKey {
	return ReporterResourceKey{
		localResourceID: re.localResourceId,
		resourceType:    re.resourceType,
		reporter:        re.reporterId,
	}
}
