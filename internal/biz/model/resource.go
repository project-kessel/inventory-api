package model

import (
	"fmt"
	"time"
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
		id,
		resourceType,
		reporterType,
		reporterInstanceId,
		reporterRepresentationData,
		reporterResourceId,
		reporterResource.representationVersion,
		reporterResource.generation,
		commonRepresentationData,
		NewVersion(initialCommonVersion),
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

func (r Resource) Serialize() (ResourceSnapshot, ReporterResourceSnapshot, ReporterRepresentationSnapshot, CommonRepresentationSnapshot, error) {
	var createdAt, updatedAt time.Time
	if len(r.resourceEvents) > 0 {
		createdAt = r.resourceEvents[0].createdAt
		updatedAt = r.resourceEvents[0].updatedAt
	}

	resourceSnapshot := ResourceSnapshot{
		ID:               r.id.UUID(),
		Type:             r.resourceType.String(),
		CommonVersion:    r.commonVersion.Serialize(),
		ConsistencyToken: r.consistencyToken.Serialize(),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}

	var reporterResourceSnapshot ReporterResourceSnapshot
	if len(r.reporterResources) > 0 {
		reporterResourceSnapshot = r.reporterResources[0].Serialize()
	}

	var reporterRepresentationSnapshot ReporterRepresentationSnapshot
	var commonRepresentationSnapshot CommonRepresentationSnapshot
	if len(r.resourceEvents) > 0 {
		reporterRepresentationSnapshot = r.resourceEvents[0].SerializeReporterRepresentation()
		commonRepresentationSnapshot = r.resourceEvents[0].SerializeCommonRepresentation()
	}

	return resourceSnapshot, reporterResourceSnapshot, reporterRepresentationSnapshot, commonRepresentationSnapshot, nil
}

func DeserializeResource(
	resourceSnapshot ResourceSnapshot,
	reporterResourceSnapshot ReporterResourceSnapshot,
	reporterRepresentationSnapshot ReporterRepresentationSnapshot,
	commonRepresentationSnapshot CommonRepresentationSnapshot,
) Resource {

	resourceId := ResourceId(resourceSnapshot.ID)
	resourceType := ResourceType(resourceSnapshot.Type)
	commonVersion := DeserializeVersion(resourceSnapshot.CommonVersion)

	// Create reporter resource
	reporterResource := DeserializeReporterResource(reporterResourceSnapshot)

	// Create resource event from representations
	resourceEvent := DeserializeResourceEvent(reporterRepresentationSnapshot, commonRepresentationSnapshot)

	return Resource{
		id:                resourceId,
		resourceType:      resourceType,
		commonVersion:     commonVersion,
		reporterResources: []ReporterResource{reporterResource},
		resourceEvents:    []ResourceEvent{resourceEvent},
	}
}

func (r Resource) ResourceEvents() []ResourceEvent {
	return r.resourceEvents
}

// CreateSnapshot creates a complete snapshot of the Resource and all its related entities
func (r Resource) createSnapshot() (ResourceSnapshot, ReporterResourceSnapshot, CommonRepresentationSnapshot, ReporterRepresentationSnapshot, error) {
	resourceSnapshot, reporterResourceSnapshot, reporterRepresentationSnapshot, commonRepresentationSnapshot, err := r.Serialize()
	return resourceSnapshot, reporterResourceSnapshot, commonRepresentationSnapshot, reporterRepresentationSnapshot, err
}

func (r Resource) ReporterResources() []ReporterResource {
	return r.reporterResources
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

func (r *Resource) findReporterResource(key ReporterResourceKey) (*ReporterResource, error) {
	for i := range r.reporterResources {
		if r.reporterResources[i].Key() == key {
			return &r.reporterResources[i], nil
		}
	}
	return nil, fmt.Errorf("reporter resource with key (localResourceId=%s, resourceType=%s, reporterType=%s, reporterInstanceId=%s) not found in resource",
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

	reporterResource, err := r.findReporterResource(key)
	if err != nil {
		return err
	}

	reporterResource.Update(apiHref, consoleHref)

	// Extract domain types from key
	keyResourceType, err := NewResourceType(key.ResourceType())
	if err != nil {
		return fmt.Errorf("invalid resource type from key: %w", err)
	}
	keyReporterType, err := NewReporterType(key.ReporterType())
	if err != nil {
		return fmt.Errorf("invalid reporter type from key: %w", err)
	}
	keyReporterInstanceId, err := NewReporterInstanceId(key.ReporterInstanceId())
	if err != nil {
		return fmt.Errorf("invalid reporter instance ID from key: %w", err)
	}

	resourceEvent, err := NewResourceEvent(
		r.id,
		keyResourceType,
		keyReporterType,
		keyReporterInstanceId,
		reporterData,
		reporterResource.Id(),
		reporterResource.representationVersion,
		reporterResource.generation,
		commonData,
		r.commonVersion,
		reporterVersion,
	)
	if err != nil {
		return fmt.Errorf("failed to create updated ResourceEvent: %w", err)
	}

	r.resourceEvents = append(r.resourceEvents, resourceEvent)
	return nil
}
