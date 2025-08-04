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

func (r Resource) ReporterResources() []ReporterResource {
	return r.reporterResources
}

func Deserialize(
	resourceID uuid.UUID,
	resourceType string,
	commonVersion uint,
	reporterResourceID uuid.UUID,
	localResourceID string,
	reporterType string,
	reporterInstanceID string,
	representationVersion uint,
	generation uint,
	tombstone bool,
	apiHref string,
	consoleHref string,
) (*Resource, error) {
	domainResourceId, err := NewResourceId(resourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to create ResourceId: %w", err)
	}

	domainResourceType, err := NewResourceType(resourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to create ResourceType: %w", err)
	}

	reporterResource, err := DeserializeReporterResource(
		reporterResourceID,
		localResourceID,
		reporterType,
		resourceType,
		reporterInstanceID,
		resourceID,
		apiHref,
		consoleHref,
		representationVersion,
		generation,
		tombstone,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReporterResource: %w", err)
	}

	resource := &Resource{
		id:                domainResourceId,
		resourceType:      domainResourceType,
		commonVersion:     NewVersion(commonVersion),
		reporterResources: []ReporterResource{*reporterResource},
		resourceEvents:    []ResourceEvent{},
	}

	return resource, nil
}

func (r Resource) findReporterResourceById(reporterResourceId uuid.UUID) (ReporterResource, error) {
	for _, reporterResource := range r.reporterResources {
		if reporterResource.Id().UUID() == reporterResourceId {
			return reporterResource, nil
		}
	}
	return ReporterResource{}, fmt.Errorf("reporter resource with ID %s not found in resource", reporterResourceId)
}

func (r Resource) Update(
	reporterResourceId uuid.UUID,
	apiHref string,
	consoleHref string,
	reporterVersion *string,
	commonData internal.JsonObject,
	reporterData internal.JsonObject,
) (Resource, error) {
	newCommonVersion := r.commonVersion.Increment()

	existingReporterResource, err := r.findReporterResourceById(reporterResourceId)
	if err != nil {
		return Resource{}, err
	}

	key := existingReporterResource.Key()
	updatedReporterResource, err := existingReporterResource.Update(
		apiHref,
		consoleHref,
	)
	if err != nil {
		return Resource{}, fmt.Errorf("failed to update ReporterResource: %w", err)
	}

	resourceEvent, err := NewResourceEvent(
		reporterResourceId,
		key.ResourceType(),
		key.ReporterType(),
		key.ReporterInstanceId(),
		reporterData,
		r.id.String(),
		updatedReporterResource.representationVersion.Uint(),
		existingReporterResource.generation.Uint(),
		commonData,
		newCommonVersion.Uint(),
		reporterVersion,
	)
	if err != nil {
		return Resource{}, fmt.Errorf("failed to create updated ResourceEvent: %w", err)
	}

	return Resource{
		id:                r.id,
		resourceType:      r.resourceType,
		commonVersion:     newCommonVersion,
		consistencyToken:  r.consistencyToken, // TODO: Issue here is that this is not reported in ReportResourceRequest, so we may need to return it from select.
		reporterResources: []ReporterResource{updatedReporterResource},
		resourceEvents:    append(r.resourceEvents, resourceEvent),
	}, nil
}
