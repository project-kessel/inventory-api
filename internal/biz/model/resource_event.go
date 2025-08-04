package model

import (
	"fmt"
	"time"

	"github.com/project-kessel/inventory-api/internal"

	"github.com/google/uuid"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
)

type ResourceEvent struct {
	id                     ResourceId
	resourceType           ResourceType
	reporterId             ReporterId
	reporterResource       ReporterResource
	reporterRepresentation ReporterRepresentation
	commonRepresentation   CommonRepresentation
	createdAt              time.Time
	updatedAt              time.Time
}

func NewResourceEvent(
	resourceId ResourceId,
	resourceType ResourceType,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
	reporterData Representation,
	reporterResourceID ReporterResourceId,
	reporterVersion Version,
	reporterGeneration Generation,
	commonData Representation,
	commonVersion Version,
	reporterVersionVal *ReporterVersion,
) (ResourceEvent, error) {
	reporterId := NewReporterId(reporterType, reporterInstanceId)

	// Create ReporterRepresentation
	reporterRep, err := NewReporterDataRepresentation(
		reporterResourceID,
		reporterVersion,
		reporterGeneration,
		reporterData,
		commonVersion,
		reporterVersionVal,
	)
	if err != nil {
		return ResourceEvent{}, fmt.Errorf("ResourceEvent invalid reporter representation: %w", err)
	}

	// Convert interface to concrete type
	reporterRepresentation, ok := reporterRep.(ReporterRepresentation)
	if !ok {
		return ResourceEvent{}, fmt.Errorf("ResourceEvent: failed to convert reporter representation to expected type")
	}

	// Create CommonRepresentation
	commonRepresentation, err := NewCommonRepresentation(
		resourceId,
		commonData,
		commonVersion,
		reporterType,
		reporterInstanceId,
	)
	if err != nil {
		return ResourceEvent{}, fmt.Errorf("ResourceEvent invalid common representation: %w", err)
	}

	return ResourceEvent{
		id:                     resourceId,
		resourceType:           resourceType,
		reporterId:             reporterId,
		reporterRepresentation: reporterRepresentation,
		commonRepresentation:   commonRepresentation,
	}, nil
}

func (re ResourceEvent) CreatedAt() *time.Time {
	return &re.createdAt
}

func (re ResourceEvent) UpdatedAt() *time.Time {
	return &re.updatedAt
}

func (re ResourceEvent) ResourceType() string {
	return re.resourceType.String()
}

func (re ResourceEvent) ReporterType() string {
	return re.reporterId.reporterType.String()
}

func (re ResourceEvent) ReporterInstanceId() string {
	return re.reporterId.reporterInstanceId.String()
}

func (re ResourceEvent) ReporterVersion() *string {
	if re.reporterRepresentation.reporterVersion == nil {
		return nil
	}
	versionStr := re.reporterRepresentation.reporterVersion.String()
	return &versionStr
}

func (re ResourceEvent) Id() ResourceId {
	return re.id
}

func (re ResourceEvent) LocalResourceId() string {
	return re.reporterResource.localResourceID.String()
}

func (re ResourceEvent) ResourceId() uuid.UUID {
	return uuid.UUID(re.id)
}

func (re ResourceEvent) ConsoleHref() string {
	return re.reporterResource.consoleHref.String()
}

func (re ResourceEvent) ApiHref() string {
	return re.reporterResource.apiHref.String()
}

func (re ResourceEvent) Data() internal.JsonObject {
	return re.reporterRepresentation.Data()
}

func (re ResourceEvent) WorkspaceId() string {
	if workspaceId, ok := re.commonRepresentation.Data()["workspace_id"]; ok {
		if workspaceIdStr, ok := workspaceId.(string); ok {
			return workspaceIdStr
		}
	}
	return ""
}

func (re ResourceEvent) SerializeReporterRepresentation() (*datamodel.ReporterRepresentation, error) {
	var reporterVersionStr *string
	if re.reporterRepresentation.reporterVersion != nil {
		versionStr := re.reporterRepresentation.reporterVersion.String()
		reporterVersionStr = &versionStr
	}

	return datamodel.NewReporterRepresentation(
		re.reporterRepresentation.Data(),
		re.reporterRepresentation.reporterResourceID.String(),
		uint(re.reporterRepresentation.version),
		uint(re.reporterRepresentation.generation),
		uint(re.reporterRepresentation.commonVersion),
		re.reporterRepresentation.tombstone.Bool(),
		reporterVersionStr,
	)
}

func (re ResourceEvent) SerializeCommonRepresentation() (*datamodel.CommonRepresentation, error) {
	return datamodel.NewCommonRepresentation(
		uuid.UUID(re.id),
		re.commonRepresentation.Data(),
		uint(re.commonRepresentation.version),
		re.reporterId.reporterType.String(),
		re.reporterId.reporterInstanceId.String(),
	)
}
