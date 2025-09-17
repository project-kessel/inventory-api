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
) (ResourceDeleteEvent, error) {
	reporterId := NewReporterId(reporterType, reporterInstanceId)

	return ResourceDeleteEvent{
		id:                     resourceId,
		resourceType:           resourceType,
		reporterId:             reporterId,
		localResourceId:        localResourceId,
		reporterRepresentation: reporterRepresentation,
	}, nil
}

func (re ResourceDeleteEvent) CreatedAt() *time.Time {
	return &re.createdAt
}

func (re ResourceDeleteEvent) UpdatedAt() *time.Time {
	return &re.updatedAt
}

func (re ResourceDeleteEvent) ResourceType() string {
	return re.resourceType.String()
}

func (re ResourceDeleteEvent) ReporterType() string {
	return re.reporterId.reporterType.String()
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

// TODO: These fields do not really belong on the delete event, need to figure out a better way to model these
func (re ResourceDeleteEvent) WorkspaceId() string {
	return ""
}

func (re ResourceDeleteEvent) CommonVersion() Version {
	return re.reporterRepresentation.CommonVersion()
}

func (re ResourceDeleteEvent) ReporterResourceKey() (ReporterResourceKey, error) {
	localResourceId, err := NewLocalResourceId(re.LocalResourceId())
	if err != nil {
		return ReporterResourceKey{}, err
	}

	resourceType, err := NewResourceType(re.ResourceType())
	if err != nil {
		return ReporterResourceKey{}, err
	}

	reporterType, err := NewReporterType(re.ReporterType())
	if err != nil {
		return ReporterResourceKey{}, err
	}

	reporterInstanceId, err := NewReporterInstanceId(re.ReporterInstanceId())
	if err != nil {
		return ReporterResourceKey{}, err
	}

	return NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
}
