package model

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
)

const initialCommonVersion = 0

type Resource struct {
	id                ResourceId
	resourceType      ResourceType
	commonVersion     Version
	consistencyToken  ConsistencyToken //nolint:unused
	reporterResources []ReporterResource
	resourceEvents    []ResourceEvent
}

func NewResource(
	idVal uuid.UUID,
	localResourceIdVal string,
	resourceTypeVal string,
	reporterTypeVal string,
	reporterInstanceIdVal string,
	resourceIdVal uuid.UUID,
	apiHrefVal string,
	consoleHrefVal string,
	reporterRepresentationData internal.JsonObject,
	commonRepresentationData internal.JsonObject,
) (Resource, error) {

	reporterResource, err := NewReporterResource(
		resourceIdVal,
		localResourceIdVal,
		resourceTypeVal,
		reporterTypeVal,
		reporterInstanceIdVal,
		idVal,
		apiHrefVal,
		consoleHrefVal,
	)
	if err != nil {
		return Resource{}, fmt.Errorf("resource invalid ReporterResource: %w", err)
	}

	resourceTypeObj, err := NewResourceType(resourceTypeVal)
	if err != nil {
		return Resource{}, fmt.Errorf("resource invalid type: %w", err)
	}

	resourceId, err := NewResourceId(idVal)
	if err != nil {
		return Resource{}, fmt.Errorf("resource invalid ID: %w", err)
	}

	resourceEvent, err := NewResourceEvent(
		idVal,
		resourceTypeVal,
		reporterTypeVal,
		reporterInstanceIdVal,
		reporterRepresentationData,
		resourceIdVal.String(),
		0,
		0,
		commonRepresentationData,
		initialCommonVersion,
		nil,
	)
	if err != nil {
		return Resource{}, fmt.Errorf("resource invalid ResourceEvent: %w", err)
	}

	resource := Resource{
		id:                resourceId,
		resourceType:      resourceTypeObj,
		commonVersion:     NewVersion(initialCommonVersion),
		reporterResources: []ReporterResource{reporterResource},
		resourceEvents:    []ResourceEvent{resourceEvent},
	}
	return resource, nil
}

func (r Resource) Serialize() (*datamodel.Resource, *datamodel.ReporterResource, *datamodel.ReporterRepresentation, *datamodel.CommonRepresentation, error) {
	// Serialize the main Resource
	resource, err := datamodel.NewResource(
		uuid.UUID(r.id),
		r.resourceType.String(),
		uint(r.commonVersion),
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to serialize resource: %w", err)
	}

	// Serialize the first ReporterResource (assuming there's at least one)
	if len(r.reporterResources) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("resource has no reporter resources to serialize")
	}

	reporterResource, err := r.reporterResources[0].Serialize()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to serialize reporter resource: %w", err)
	}

	// Serialize the first ResourceEvent (assuming there's at least one)
	if len(r.resourceEvents) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("resource has no resource events to serialize")
	}

	reporterRepresentation, err := r.resourceEvents[0].SerializeReporterRepresentation()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to serialize reporter representation: %w", err)
	}

	commonRepresentation, err := r.resourceEvents[0].SerializeCommonRepresentation()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to serialize common representation: %w", err)
	}

	return resource, reporterResource, reporterRepresentation, commonRepresentation, nil
}

func (r Resource) ResourceEvents() []ResourceEvent {
	return r.resourceEvents
}
