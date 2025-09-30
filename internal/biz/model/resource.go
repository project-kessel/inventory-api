package model

import (
	"fmt"
	"strings"
	"time"
)

const initialCommonVersion = 0

// Create Entities with unexported fields for encapsulation
type Resource struct {
	id                   ResourceId
	resourceType         ResourceType
	commonVersion        Version
	consistencyToken     ConsistencyToken
	reporterResources    []ReporterResource
	resourceReportEvents []ResourceReportEvent
	resourceDeleteEvents []ResourceDeleteEvent
}

// Factory methods
func NewResource(id ResourceId, localResourceId LocalResourceId, resourceType ResourceType, reporterType ReporterType, reporterInstanceId ReporterInstanceId, transactionId TransactionId, reporterResourceId ReporterResourceId, apiHref ApiHref, consoleHref ConsoleHref, reporterRepresentationData Representation, commonRepresentationData Representation, reporterVersion *ReporterVersion) (Resource, error) {

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
		transactionId,
		localResourceId,
		reporterResource.Id(),
		apiHref,
		consoleHref,
		reporterRepresentationData,
		commonRepresentationData,
		reporterVersion,
		reporterResource.representationVersion,
		reporterResource.generation,
		commonVersion)
	if err != nil {
		return Resource{}, fmt.Errorf("resource invalid ResourceReportEvent: %w", err)
	}

	reporterResources := []ReporterResource{reporterResource}

	resource := Resource{
		id:                   id,
		resourceType:         resourceType,
		commonVersion:        commonVersion,
		reporterResources:    reporterResources,
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
	transactionId TransactionId,
) error {
	r.commonVersion = r.commonVersion.Increment()

	reporterResource, err := r.findReporterResourceToUpdateByKey(key)
	if err != nil {
		return err
	}

	reporterResource.Update(apiHref, consoleHref)

	resourceEvent, err := resourceEventAndRepresentations(
		reporterResource.resourceID,
		key.ResourceType(),
		key.ReporterType(),
		key.ReporterInstanceId(),
		transactionId,
		key.LocalResourceId(),
		reporterResource.Id(),
		apiHref,
		consoleHref,
		reporterRepresentationData,
		commonRepresentationData,
		reporterVersion,
		reporterResource.representationVersion,
		reporterResource.generation,
		r.commonVersion)
	if err != nil {
		return fmt.Errorf("failed to create updated ResourceReportEvent: %w", err)
	}

	r.resourceReportEvents = []ResourceReportEvent{resourceEvent}
	return nil
}

func (r *Resource) Delete(key ReporterResourceKey) error {
	reporterResource, err := r.findReporterResourceToUpdateByKey(key)
	if err != nil {
		return err
	}

	reporterResource.Delete()
	resourceDeleteEvent, err := deleteEventAndRepresentations(
		reporterResource.resourceID,
		key.ResourceType(),
		key.ReporterType(),
		key.ReporterInstanceId(),
		key.LocalResourceId(),
		reporterResource.Id(),
		reporterResource.representationVersion,
		reporterResource.generation)

	if err != nil {
		return fmt.Errorf("failed to create ResourceDeleteEvent: %w", err)
	}

	r.resourceDeleteEvents = []ResourceDeleteEvent{resourceDeleteEvent}
	return nil
}

func resourceEventAndRepresentations(
	resourceId ResourceId,
	resourceType ResourceType,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
	transactionId TransactionId,
	localResourceId LocalResourceId,
	reporterResourceId ReporterResourceId,
	apiHref ApiHref,
	consoleHref ConsoleHref,
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
		transactionId,
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
		transactionId,
	)
	if err != nil {
		return ResourceReportEvent{}, fmt.Errorf("invalid CommonRepresentation: %w", err)
	}
	resourceEvent, err := NewResourceReportEvent(
		resourceId,
		resourceType,
		reporterType,
		reporterInstanceId,
		localResourceId,
		apiHref,
		consoleHref,
		reporterRepresentation,
		commonRepresentation,
	)
	if err != nil {
		return ResourceReportEvent{}, fmt.Errorf("invalid ResourceReportEvent: %w", err)
	}

	return resourceEvent, nil
}

func deleteEventAndRepresentations(resourceId ResourceId,
	resourceType ResourceType,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
	localResourceId LocalResourceId,
	reporterResourceId ReporterResourceId,
	representationVersion Version,
	generation Generation) (ResourceDeleteEvent, error) {

	reporterDeleteRepresentation, err := NewReporterDeleteRepresentation(
		reporterResourceId,
		representationVersion,
		generation,
	)
	if err != nil {
		return ResourceDeleteEvent{}, fmt.Errorf("invalid ReporterRepresentation: %w", err)
	}

	resourceDeleteEvent, err := NewResourceDeleteEvent(
		resourceId,
		resourceType,
		reporterType,
		reporterInstanceId,
		localResourceId,
		reporterDeleteRepresentation)

	if err != nil {
		return ResourceDeleteEvent{}, fmt.Errorf("invalid ResourceReportEvent: %w", err)
	}

	return resourceDeleteEvent, nil
}

func (r *Resource) findReporterResourceToUpdateByKey(key ReporterResourceKey) (*ReporterResource, error) {
	for i := range r.reporterResources {
		reporter := &r.reporterResources[i]
		storedKey := reporter.Key()

		if strings.EqualFold(storedKey.LocalResourceId().Serialize(), key.LocalResourceId().Serialize()) &&
			strings.EqualFold(storedKey.ResourceType().Serialize(), key.ResourceType().Serialize()) &&
			strings.EqualFold(storedKey.ReporterType().Serialize(), key.ReporterType().Serialize()) {

			searchReporterInstanceId := key.ReporterInstanceId().Serialize()
			storedReporterInstanceId := storedKey.ReporterInstanceId().Serialize()

			if searchReporterInstanceId == "" || strings.EqualFold(storedReporterInstanceId, searchReporterInstanceId) {
				return reporter, nil
			}
		}
	}

	return nil, fmt.Errorf("reporter resource with key (localResourceId=%s, resourceType=%s, reporterType=%s, reporterInstanceId=%s) not found in resource",
		key.LocalResourceId(), key.ResourceType(), key.ReporterType(), key.ReporterInstanceId())
}

// Add getters only where needed
func (r Resource) ResourceReportEvents() []ResourceReportEvent {
	return r.resourceReportEvents
}

func (r Resource) ResourceDeleteEvents() []ResourceDeleteEvent {
	return r.resourceDeleteEvents
}

func (r Resource) ReporterResources() []ReporterResource {
	return r.reporterResources
}

func (r Resource) ConsistencyToken() ConsistencyToken {
	return r.consistencyToken
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
	if len(r.resourceDeleteEvents) > 0 {
		//TODO: Fix this to serialize all ResourceEvents
		reporterRepresentationSnapshot = r.resourceDeleteEvents[0].reporterRepresentation.Serialize()
	}

	return resourceSnapshot, reporterResourceSnapshot, reporterRepresentationSnapshot, commonRepresentationSnapshot, nil
}

// TODO: When a Resource is deserialized, does it get a list of events?
func DeserializeResource(
	resourceSnapshot *ResourceSnapshot,
	reporterResourceSnapshots []ReporterResourceSnapshot,
	reporterRepresentationSnapshot *ReporterRepresentationSnapshot,
	commonRepresentationSnapshot *CommonRepresentationSnapshot,
) *Resource {

	if resourceSnapshot == nil {
		return nil
	}

	var reporterResources []ReporterResource
	for _, reporterResourceSnapshot := range reporterResourceSnapshots {
		reporterResource := DeserializeReporterResource(reporterResourceSnapshot)
		reporterResources = append(reporterResources, reporterResource)
	}

	resourceEvent := DeserializeResourceEvent(reporterRepresentationSnapshot, commonRepresentationSnapshot)

	return &Resource{
		id:                   DeserializeResourceId(resourceSnapshot.ID),
		resourceType:         DeserializeResourceType(resourceSnapshot.Type),
		commonVersion:        DeserializeVersion(resourceSnapshot.CommonVersion),
		consistencyToken:     DeserializeConsistencyToken(resourceSnapshot.ConsistencyToken),
		reporterResources:    reporterResources,
		resourceReportEvents: []ResourceReportEvent{resourceEvent},
	}
}
