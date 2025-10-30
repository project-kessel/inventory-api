package model

import (
	"time"

	"github.com/project-kessel/inventory-api/internal"

	"github.com/google/uuid"
)

type ResourceReportEvent struct {
	id                     ResourceId
	resourceType           ResourceType
	reporterId             ReporterId
	localResourceId        LocalResourceId
	apiHref                ApiHref
	consoleHref            ConsoleHref
	reporterRepresentation ReporterDataRepresentation
	commonRepresentation   CommonRepresentation
	createdAt              time.Time
	updatedAt              time.Time
}

func NewResourceReportEvent(
	resourceId ResourceId,
	resourceType ResourceType,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
	localResourceId LocalResourceId,
	apiHref ApiHref,
	consoleHref ConsoleHref,
	reporterDataRepresentation ReporterDataRepresentation,
	commonRepresentation CommonRepresentation,
) (ResourceReportEvent, error) {
	reporterId := NewReporterId(reporterType, reporterInstanceId)

	return ResourceReportEvent{
		id:                     resourceId,
		resourceType:           resourceType,
		reporterId:             reporterId,
		localResourceId:        localResourceId,
		apiHref:                apiHref,
		consoleHref:            consoleHref,
		reporterRepresentation: reporterDataRepresentation,
		commonRepresentation:   commonRepresentation,
	}, nil
}

func (re ResourceReportEvent) CreatedAt() *time.Time {
	return &re.createdAt
}

func (re ResourceReportEvent) UpdatedAt() *time.Time {
	return &re.updatedAt
}

func (re ResourceReportEvent) ResourceType() string {
	return re.resourceType.String()
}

func (re ResourceReportEvent) ReporterType() string {
	return re.reporterId.reporterType.String()
}

func (re ResourceReportEvent) ReporterInstanceId() string {
	return re.reporterId.reporterInstanceId.String()
}

func (re ResourceReportEvent) ReporterVersion() *string {
	if re.reporterRepresentation.reporterVersion == nil {
		return nil
	}
	versionStr := re.reporterRepresentation.reporterVersion.String()
	return &versionStr
}

// CurrentCommonVersion returns the version from the CommonRepresentation
func (re ResourceReportEvent) CurrentCommonVersion() *Version {
	return &re.commonRepresentation.version
}

// CurrentReporterRepresentationVersion returns the version from the ReporterRepresentation
func (re ResourceReportEvent) CurrentReporterRepresentationVersion() *Version {
	return &re.reporterRepresentation.version
}

func (re ResourceReportEvent) Id() ResourceId {
	return re.id
}

func (re ResourceReportEvent) LocalResourceId() string {
	return re.localResourceId.String()
}

func (re ResourceReportEvent) ResourceId() uuid.UUID {
	return uuid.UUID(re.id)
}

func (re ResourceReportEvent) ConsoleHref() string {
	return re.consoleHref.String()
}

func (re ResourceReportEvent) ApiHref() string {
	return re.apiHref.String()
}

func (re ResourceReportEvent) Data() internal.JsonObject {
	return re.reporterRepresentation.Data()
}

func (re ResourceReportEvent) WorkspaceId() string {
	if workspaceId, ok := re.commonRepresentation.Data()["workspace_id"]; ok {
		if workspaceIdStr, ok := workspaceId.(string); ok {
			return workspaceIdStr
		}
	}
	return ""
}

// ReporterResourceKey constructs and returns the ReporterResourceKey from the event fields
func (re ResourceReportEvent) ReporterResourceKey() ReporterResourceKey {
	return ReporterResourceKey{
		localResourceID: re.localResourceId,
		resourceType:    re.resourceType,
		reporter:        re.reporterId,
	}
}

// SetTimestamps sets the createdAt and updatedAt timestamps on the event
func (re *ResourceReportEvent) SetTimestamps(createdAt time.Time, updatedAt time.Time) {
	re.createdAt = createdAt
	re.updatedAt = updatedAt
}

// DeserializeResourceEvent creates a ResourceReportEvent from representation snapshots - direct initialization without validation
func DeserializeResourceEvent(
	reporterRepresentationSnapshot *ReporterRepresentationSnapshot,
	commonRepresentationSnapshot *CommonRepresentationSnapshot,
	createdAt time.Time,
	updatedAt time.Time,
) ResourceReportEvent {
	var event ResourceReportEvent

	if commonRepresentationSnapshot != nil {
		event.commonRepresentation = DeserializeCommonRepresentation(commonRepresentationSnapshot)
	}

	if reporterRepresentationSnapshot != nil {
		event.reporterRepresentation = *DeserializeReporterDataRepresentation(reporterRepresentationSnapshot)
	}

	event.createdAt = createdAt
	event.updatedAt = updatedAt

	return event
}
