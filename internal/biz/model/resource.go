package model

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

const initialCommonVersion = 0

// Create Entities with unexported fields for encapsulation
type Resource struct {
	id                   ResourceId
	resourceType         ResourceType
	commonVersion        Version
	consistencyToken     ConsistencyToken //nolint:unused
	reporterResources    []ReporterResource
	resourceReportEvents []ResourceReportEvent
}

// Factory methods
func NewResource(id ResourceId, localResourceId LocalResourceId, resourceType ResourceType, reporterType ReporterType, reporterInstanceId ReporterInstanceId, reporterResourceId ReporterResourceId, apiHref ApiHref, consoleHref ConsoleHref, reporterRepresentationData Representation, commonRepresentationData Representation, reporterVersion *ReporterVersion) (Resource, error) {

	commonVersion := NewVersion(initialCommonVersion)

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

	resourceEvent, err := resourceEventAndRepresentations(
		reporterResource.resourceID,
		resourceType,
		reporterType,
		reporterInstanceId,
		reporterResource.Id(),
		reporterRepresentationData,
		commonRepresentationData,
		reporterVersion,
		reporterResource.representationVersion,
		reporterResource.generation,
		commonVersion)
	if err != nil {
		return Resource{}, fmt.Errorf("resource invalid ResourceReportEvent: %w", err)
	}

	resource := Resource{
		id:                   id,
		resourceType:         resourceType,
		commonVersion:        commonVersion,
		reporterResources:    []ReporterResource{reporterResource},
		resourceReportEvents: []ResourceReportEvent{resourceEvent},
	}
	return resource, nil
}

// Model Behavior
func (r *Resource) Update(
	key ReporterResourceKey,
	apiHref ApiHref,
	consoleHref ConsoleHref,
	reporterVersion *ReporterVersion,
	reporterRepresentationData Representation,
	commonRepresentationData Representation,
) error {
	r.commonVersion = r.commonVersion.Increment()

	reporterResource, err := r.findReporterResourceToUpdateByKey(key)
	if err != nil {
		return err
	}

	reporterResource.Update(apiHref, consoleHref)

	log.Infof("Reporter Resource: %+v", reporterResource.representationVersion)

	resourceEvent, err := resourceEventAndRepresentations(
		reporterResource.resourceID,
		key.ResourceType(),
		key.ReporterType(),
		key.ReporterInstanceId(),
		reporterResource.Id(),
		reporterRepresentationData,
		commonRepresentationData,
		reporterVersion,
		reporterResource.representationVersion,
		reporterResource.generation,
		r.commonVersion)
	if err != nil {
		return fmt.Errorf("failed to create updated ResourceReportEvent: %w", err)
	}

	r.resourceReportEvents = append(r.resourceReportEvents, resourceEvent)
	return nil
}

func resourceEventAndRepresentations(
	resourceId ResourceId,
	resourceType ResourceType,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
	reporterResourceId ReporterResourceId,
	reporterData Representation,
	commonData Representation,
	reporterVersion *ReporterVersion,
	representationVersion Version,
	generation Generation,
	commonVersion Version,
) (ResourceReportEvent, error) {
	reporterRepresentation, err := NewReporterDataRepresentation(
		reporterResourceId,
		representationVersion,
		generation,
		reporterData,
		commonVersion,
		reporterVersion,
	)
	if err != nil {
		return ResourceReportEvent{}, fmt.Errorf("invalid ReporterRepresentation: %w", err)
	}

	commonRepresentation, err := NewCommonRepresentation(
		resourceId,
		commonData,
		commonVersion,
		reporterType,
		reporterInstanceId,
	)
	if err != nil {
		return ResourceReportEvent{}, fmt.Errorf("invalid CommonRepresentation: %w", err)
	}

	resourceEvent, err := NewResourceReportEvent(
		resourceId,
		resourceType,
		reporterType,
		reporterInstanceId,
		reporterRepresentation,
		commonRepresentation,
	)
	if err != nil {
		return ResourceReportEvent{}, fmt.Errorf("invalid ResourceReportEvent: %w", err)
	}

	return resourceEvent, nil
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
func (r Resource) ResourceEvents() []ResourceReportEvent {
	return r.resourceReportEvents
}

func (r Resource) ReporterResources() []ReporterResource {
	return r.reporterResources
}

// Serialization + Deserialization functions, direct initialization without validation
func (r Resource) Serialize() (ResourceSnapshot, ReporterResourceSnapshot, ReporterRepresentationSnapshot, CommonRepresentationSnapshot, error) {
	var createdAt, updatedAt time.Time
	if len(r.resourceReportEvents) > 0 {
		createdAt = r.resourceReportEvents[0].createdAt
		updatedAt = r.resourceReportEvents[0].updatedAt
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
	if len(r.resourceReportEvents) > 0 {
		//TODO: Fix this to serialize all ResourceEvents
		reporterRepresentationSnapshot = r.resourceReportEvents[0].reporterRepresentation.Serialize()
		commonRepresentationSnapshot = r.resourceReportEvents[0].commonRepresentation.Serialize()
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
		id:                   DeserializeResourceId(resourceSnapshot.ID),
		resourceType:         DeserializeResourceType(resourceSnapshot.Type),
		commonVersion:        DeserializeVersion(resourceSnapshot.CommonVersion),
		consistencyToken:     DeserializeConsistencyToken(resourceSnapshot.ConsistencyToken),
		reporterResources:    reporterResources,
		resourceReportEvents: []ResourceReportEvent{resourceEvent},
	}
}
