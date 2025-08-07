package model

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

const initialCommonVersion = 0

// Create Entities with unexported fields for encapsulation
type Resource struct {
	id                ResourceId
	resourceType      ResourceType
	commonVersion     Version
	consistencyToken  ConsistencyToken //nolint:unused
	reporterResources []ReporterResource
	resourceEvents    []ResourceEvent
}

// Factory methods
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

// Model Behavior
func (r *Resource) Update(
	key ReporterResourceKey,
	apiHref ApiHref,
	consoleHref ConsoleHref,
	reporterVersion *ReporterVersion,
	commonData Representation,
	reporterData Representation,
) error {
	r.commonVersion = r.commonVersion.Increment()

	reporterResource, err := r.findReporterResourceToUpdateByKey(key)
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

	log.Infof("Reporter Resource: %+v", reporterResource.representationVersion)
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

func (r *Resource) findReporterResourceToUpdateByKey(key ReporterResourceKey) (*ReporterResource, error) {
	for i := range r.reporterResources {
		if r.reporterResources[i].Key() == key {
			return &r.reporterResources[i], nil
		}
	}
	return nil, fmt.Errorf("reporter resource with key (localResourceId=%s, resourceType=%s, reporterType=%s, reporterInstanceId=%s) not found in resource",
		key.LocalResourceId(), key.ResourceType(), key.ReporterType(), key.ReporterInstanceId())
}

// Add getters only where needed
func (r Resource) ResourceEvents() []ResourceEvent {
	return r.resourceEvents
}

func (r Resource) ReporterResources() []ReporterResource {
	return r.reporterResources
}

// Serialization + Deserialization functions, direct initialization without validation
func (r Resource) Serialize() (ResourceSnapshot, ReporterResourceSnapshot, ReporterRepresentationSnapshot, CommonRepresentationSnapshot, error) {
	var createdAt, updatedAt time.Time
	if len(r.resourceEvents) > 0 {
		createdAt = r.resourceEvents[0].createdAt
		updatedAt = r.resourceEvents[0].updatedAt
	}

	resourceSnapshot := ResourceSnapshot{
		ID:               r.id.Serialize(),
		Type:             r.resourceType.Serialize(),
		CommonVersion:    r.commonVersion.Serialize(),
		ConsistencyToken: r.consistencyToken.Serialize(),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}

	var reporterResourceSnapshot ReporterResourceSnapshot
	if len(r.reporterResources) > 0 {
		//TODO: Fix this to serialize all ReporterResources
		reporterResourceSnapshot = r.reporterResources[0].Serialize()
	}

	var reporterRepresentationSnapshot ReporterRepresentationSnapshot
	var commonRepresentationSnapshot CommonRepresentationSnapshot
	if len(r.resourceEvents) > 0 {
		//TODO: Fix this to serialize all ResourceEvents
		reporterRepresentationSnapshot = r.resourceEvents[0].reporterRepresentation.Serialize()
		commonRepresentationSnapshot = r.resourceEvents[0].commonRepresentation.Serialize()
	}

	return resourceSnapshot, reporterResourceSnapshot, reporterRepresentationSnapshot, commonRepresentationSnapshot, nil
}

// TODO: When a Resource is deserialized, does it get a list of events?
func DeserializeResource(
	resourceSnapshot ResourceSnapshot,
	reporterResourceSnapshots []ReporterResourceSnapshot,
	reporterRepresentationSnapshot *ReporterRepresentationSnapshot,
	commonRepresentationSnapshot *CommonRepresentationSnapshot,
) Resource {

	var reporterResources []ReporterResource
	for _, reporterResourceSnapshot := range reporterResourceSnapshots {
		reporterResources = append(reporterResources, DeserializeReporterResource(reporterResourceSnapshot))
	}

	resourceEvent := DeserializeResourceEvent(reporterRepresentationSnapshot, commonRepresentationSnapshot)

	return Resource{
		id:                DeserializeResourceId(resourceSnapshot.ID),
		resourceType:      DeserializeResourceType(resourceSnapshot.Type),
		commonVersion:     DeserializeVersion(resourceSnapshot.CommonVersion),
		consistencyToken:  DeserializeConsistencyToken(resourceSnapshot.ConsistencyToken),
		reporterResources: reporterResources,
		resourceEvents:    []ResourceEvent{resourceEvent},
	}
}
