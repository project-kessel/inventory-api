package model

import (
	"fmt"

	"github.com/google/uuid"
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
	id ResourceId,
	localResourceId LocalResourceId,
	resourceType ResourceType,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
	reporterResourceId ReporterResourceId,
	apiHref ApiHref,
	consoleHref ConsoleHref,
	reporterRepresentationData Representation,
	commonRepresentationData Representation,
) (Resource, error) {

	reporterResource, err := NewReporterResource(
		reporterResourceId,
		localResourceId,
		resourceType,
		reporterType,
		reporterInstanceId,
		id,
		apiHref,
		consoleHref,
	)
	if err != nil {
		return Resource{}, fmt.Errorf("resource invalid ReporterResource: %w", err)
	}

	resourceEvent, err := NewResourceEvent(
		id.UUID(),
		resourceType.String(),
		reporterType.String(),
		reporterInstanceId.String(),
		reporterRepresentationData.Serialize(),
		reporterResourceId.String(),
		0,
		0,
		commonRepresentationData.Serialize(),
		initialCommonVersion,
		nil,
	)
	if err != nil {
		return Resource{}, fmt.Errorf("resource invalid ResourceEvent: %w", err)
	}

	resource := Resource{
		id:                id,
		resourceType:      resourceType,
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

	//TODO: This should serialize all the ReporterResources?
	reporterResource, err := r.reporterResources[0].Serialize()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to serialize reporter resource: %w", err)
	}

	// Serialize the first ResourceEvent (assuming there's at least one)
	if len(r.resourceEvents) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("resource has no resource events to serialize")
	}

	//TODO: This should serialize all the Representations? We need to consider the read vs the write models here
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
	domainResourceId := DeserializeResourceId(resourceID)
	domainResourceType := DeserializeResourceType(resourceType)

	//TODO: Deserialize all ReporterResources
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
		commonVersion:     DeserializeVersion(commonVersion),
		reporterResources: []ReporterResource{*reporterResource},
		resourceEvents:    []ResourceEvent{},
	}

	return resource, nil
}

func (r Resource) findReporterResourceByKey(key ReporterResourceKey) (ReporterResource, error) {
	for _, reporterResource := range r.reporterResources {
		reporterKey := reporterResource.Key()
		if reporterKey.LocalResourceId() == key.LocalResourceId() &&
			reporterKey.ResourceType() == key.ResourceType() &&
			reporterKey.ReporterType() == key.ReporterType() &&
			reporterKey.ReporterInstanceId() == key.ReporterInstanceId() {
			return reporterResource, nil
		}
	}
	return ReporterResource{}, fmt.Errorf("reporter resource with key (localResourceId=%s, resourceType=%s, reporterType=%s, reporterInstanceId=%s) not found in resource",
		key.LocalResourceId(), key.ResourceType(), key.ReporterType(), key.ReporterInstanceId())
}

func (r *Resource) Update(
	key ReporterResourceKey,
	apiHref ApiHref,
	consoleHref ConsoleHref,
	reporterVersion *ReporterVersion,
	commonData Representation,
	reporterData Representation,
) error {
	r.commonVersion = r.commonVersion.Increment()

	// Find the index of the reporter resource to update
	var reporterResourceToUpdate *ReporterResource
	for i := range r.reporterResources {
		if r.reporterResources[i].Key() == key {
			r.reporterResources[i].Update(apiHref, consoleHref)
			reporterResourceToUpdate = &r.reporterResources[i]
			break
		}
	}

	if reporterResourceToUpdate == nil {
		return fmt.Errorf("reporter resource with key (localResourceId=%s, resourceType=%s, reporterType=%s, reporterInstanceId=%s) not found in resource",
			key.LocalResourceId(), key.ResourceType(), key.ReporterType(), key.ReporterInstanceId())
	}

	var reporterVersionStr *string
	if reporterVersion != nil {
		versionStr := reporterVersion.String()
		reporterVersionStr = &versionStr
	}

	resourceEvent, err := NewResourceEvent(
		reporterResourceToUpdate.Id().UUID(),
		key.ResourceType(),
		key.ReporterType(),
		key.ReporterInstanceId(),
		reporterData.Serialize(),
		r.id.String(),
		reporterResourceToUpdate.representationVersion.Uint(),
		reporterResourceToUpdate.generation.Uint(),
		commonData.Serialize(),
		r.commonVersion.Uint(),
		reporterVersionStr,
	)
	if err != nil {
		return fmt.Errorf("failed to create updated ResourceEvent: %w", err)
	}

	r.resourceEvents = append(r.resourceEvents, resourceEvent)
	return nil
}
