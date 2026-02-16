package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/project-kessel/inventory-api/internal/biz"
)

const initialCommonVersion = 0

// OutboxEvent represents an event from the outbox as received from Kafka.
// This matches the shape of messages produced by Debezium CDC.
type OutboxEvent struct {
	operation   OperationType
	txID        TransactionId
	inventoryID ResourceId
	tupleEvent  TupleEvent
}

// OperationType represents the type of operation for an outbox event.
type OperationType string

const (
	OperationCreated OperationType = "created"
	OperationUpdated OperationType = "updated"
	OperationDeleted OperationType = "deleted"
)

// NewOutboxEvent creates a new OutboxEvent with validation.
func NewOutboxEvent(operation OperationType, txID TransactionId, inventoryID ResourceId, tupleEvent TupleEvent) OutboxEvent {
	return OutboxEvent{
		operation:   operation,
		txID:        txID,
		inventoryID: inventoryID,
		tupleEvent:  tupleEvent,
	}
}

func (e OutboxEvent) Operation() OperationType { return e.operation }
func (e OutboxEvent) TxID() TransactionId      { return e.txID }
func (e OutboxEvent) InventoryID() ResourceId  { return e.inventoryID }
func (e OutboxEvent) TupleEvent() TupleEvent   { return e.tupleEvent }

// Store provides transactional access to resource persistence and an event stream
// for committed changes.
type Store interface {
	// Begin starts a new transaction.
	Begin() (Tx, error)

	// EventSource returns the event source for consuming outbox events.
	// May return nil if the store does not support event streaming.
	EventSource() EventSource
}

// EventSource consumes events and processes them with a handler.
// It encapsulates how events are received (e.g., from Kafka) and provides
// at-least-once delivery guarantees with strong ordering.
type EventSource interface {
	// Listen starts the event source and processes each delivery with the handler.
	// Processing is synchronous - each delivery is fully processed before the next.
	// If handler returns nil, the delivery is acknowledged.
	// If handler returns ErrFencingFailed, the delivery is nacked for redelivery.
	// If handler returns any other error, Listen stops and returns the error.
	// It blocks until ctx is cancelled or an error occurs.
	Listen(ctx context.Context, handler DeliveryHandler) error
}

// DeliveryHandler processes a delivery synchronously.
// It returns nil on success (delivery will be acked) or an error on failure.
// If the error is ErrFencingFailed, the delivery will be nacked for redelivery.
// Other errors cause the consumer to stop.
type DeliveryHandler func(ctx context.Context, delivery Delivery) error

// Tx represents a database transaction with access to repositories.
type Tx interface {
	// ResourceRepository returns the repository for resource operations
	// within this transaction.
	ResourceRepository() ResourceRepository

	// Commit commits the transaction.
	Commit() error

	// Rollback aborts the transaction. It is safe to call after Commit
	// (it will be a no-op), allowing the pattern: defer tx.Rollback()
	Rollback() error
}

type ResourceRepository interface {
	NextResourceId() (ResourceId, error)
	NextReporterResourceId() (ReporterResourceId, error)
	Save(resource Resource, operationType biz.EventOperationType, txid string) error
	FindResourceByKeys(key ReporterResourceKey) (*Resource, error)
	FindCurrentAndPreviousVersionedRepresentations(key ReporterResourceKey, currentVersion *uint, operationType biz.EventOperationType) (*Representations, *Representations, error)
	FindLatestRepresentations(key ReporterResourceKey) (*Representations, error)
	ContainsEventForTransactionId(transactionId string) (bool, error)
}

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
	var err error
	if transactionId == "" {
		// generate transaction IDs when not provided
		transactionId, err = GenerateTransactionId()
		if err != nil {
			return Resource{}, err
		}
	}

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

	if transactionId == "" {
		// generate transaction IDs when not provided
		transactionId, err = GenerateTransactionId()
		if err != nil {
			return err
		}
	}

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

	// If the reporter resource is already tombstoned, drop the delete operation entirely
	if reporterResource.tombstone.Serialize() {
		return nil
	}

	// Only process delete for non-tombstoned resources
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

// SetConsistencyToken updates the resource's consistency token.
func (r *Resource) SetConsistencyToken(token ConsistencyToken) {
	r.consistencyToken = token
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
