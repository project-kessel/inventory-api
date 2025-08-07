package model

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal"

	"github.com/google/uuid"
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
	representationVersion Version,
	reporterGeneration Generation,
	commonData Representation,
	commonVersion Version,
	reporterVersion *ReporterVersion,
) (ResourceEvent, error) {
	reporterId := NewReporterId(reporterType, reporterInstanceId)

	reporterRep, err := NewReporterDataRepresentation(
		reporterResourceID,
		representationVersion,
		reporterGeneration,
		reporterData,
		commonVersion,
		reporterVersion,
	)
	if err != nil {
		return ResourceEvent{}, fmt.Errorf("ResourceEvent invalid reporter representation: %w", err)
	}

	reporterRepresentation := reporterRep.ReporterRepresentation

	log.Infof("Reporter Representation : %+v", reporterRepresentation)

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

// DeserializeResourceEvent creates a ResourceEvent from representation snapshots - direct initialization without validation
func DeserializeResourceEvent(
	reporterRepresentationSnapshot *ReporterRepresentationSnapshot,
	commonRepresentationSnapshot *CommonRepresentationSnapshot,
) ResourceEvent {
	// Create domain tiny types directly from snapshot values
	resourceId := ResourceId(commonRepresentationSnapshot.ResourceId)
	resourceType := ResourceType(commonRepresentationSnapshot.ReportedByReporterType) // TODO: This might need adjustment
	reporterType := ReporterType(commonRepresentationSnapshot.ReportedByReporterType)
	reporterInstanceId := ReporterInstanceId(commonRepresentationSnapshot.ReportedByReporterInstance)

	// Create reporter ID
	reporterId := ReporterId{
		reporterType:       reporterType,
		reporterInstanceId: reporterInstanceId,
	}

	// Deserialize representations
	reporterRepresentation := DeserializeReporterRepresentation(reporterRepresentationSnapshot)
	commonRepresentation := DeserializeCommonRepresentation(commonRepresentationSnapshot)

	// Create a placeholder ReporterResource since it's needed for the event
	reporterResource := ReporterResource{} // TODO: This might need proper initialization

	return ResourceEvent{
		id:                     resourceId,
		resourceType:           resourceType,
		reporterId:             reporterId,
		reporterResource:       reporterResource,
		reporterRepresentation: *reporterRepresentation,
		commonRepresentation:   commonRepresentation,
		createdAt:              commonRepresentationSnapshot.CreatedAt,
		updatedAt:              commonRepresentationSnapshot.CreatedAt, // TODO: Add UpdatedAt to snapshots if needed
	}
}
