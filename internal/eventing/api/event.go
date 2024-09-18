package api

import (
	"time"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/common"
)

// TODO this is a bit of a hack for now to convert the models into the shape expected by event consumers
type EventResource[Detail any] struct {
	Metadata     *biz.Metadata `json:"metadata"`
	ReporterData *biz.Reporter `json:"reporter_data"`
	ResourceData *Detail       `json:"resource_data"`
}

type EventRelationship[Detail any] struct {
	Metadata         *biz.RelationshipMetadata `json:"metadata"`
	ReporterData     *biz.RelationshipReporter `json:"reporter_data"`
	RelationshipData *Detail                   `json:"relationship_data,omitempty"`
}

const (
	EventTypeResource              = "resources"
	EventTypeResourcesRelationship = "resources_relationship"
)

const (
	operationTypeCreated = "created"
	operationTypeUpdated = "updated"
	operationTypeDeleted = "deleted"
)

type Event struct {
	EventType     string `json:"event_type"`
	OperationType string `json:"operation_type"`

	EventTime    time.Time   `json:"event_time"`
	ResourceType string      `json:"resource_type"`
	Resource     interface{} `json:"resource"`
	ResourceId   string      `json:"resource_id"`
}

func NewCreatedResourceEvent[Detail any](resourceType, resourceId string, lastReportedTime time.Time, obj *EventResource[Detail]) *Event {
	return &Event{
		EventType:     EventTypeResource,
		OperationType: operationTypeCreated,
		EventTime:     lastReportedTime,
		ResourceType:  resourceType,
		ResourceId:    resourceId,
		Resource:      obj,
	}
}

func NewUpdatedResourceEvent[Detail any](resourceType, resourceId string, lastReportedTime time.Time, obj *EventResource[Detail]) *Event {
	return &Event{
		EventType:     EventTypeResource,
		OperationType: operationTypeUpdated,
		EventTime:     lastReportedTime,
		ResourceType:  resourceType,
		ResourceId:    resourceId,
		Resource:      obj,
	}
}

func NewDeletedResourceEvent(resourceType, resourceId string, lastReportedTime time.Time, requester *authnapi.Identity) *Event {
	return &Event{
		EventType:     EventTypeResource,
		OperationType: operationTypeDeleted,
		EventTime:     lastReportedTime,
		ResourceType:  resourceType,
		ResourceId:    resourceId,
		Resource: &EventResource[struct{}]{
			Metadata: &biz.Metadata{},
			ReporterData: &biz.Reporter{
				ReporterID:      requester.Principal,
				ReporterType:    requester.Type,
				LocalResourceId: resourceId,
			},
			ResourceData: nil,
		},
	}
}

func NewCreatedResourcesRelationshipEvent[Detail any](relationshipType, subjectResourceId string, lastReportedTime time.Time, obj *EventRelationship[Detail]) *Event {
	return &Event{
		EventType:     EventTypeResourcesRelationship,
		OperationType: operationTypeCreated,
		EventTime:     lastReportedTime,
		ResourceType:  relationshipType,
		ResourceId:    subjectResourceId,
		Resource:      obj,
	}
}

func NewUpdatedResourcesRelationshipEvent[Detail any](relationshipType, subjectResourceId string, lastReportedTime time.Time, obj *EventRelationship[Detail]) *Event {
	return &Event{
		EventType:     EventTypeResourcesRelationship,
		OperationType: operationTypeUpdated,
		EventTime:     lastReportedTime,
		ResourceType:  relationshipType,
		ResourceId:    subjectResourceId,
		Resource:      obj,
	}
}

func NewDeletedResourcesRelationshipEvent[Detail any](relationshipType, subjectResourceId string, lastReportedTime time.Time, obj *EventRelationship[Detail]) *Event {
	return &Event{
		EventType:     EventTypeResourcesRelationship,
		OperationType: operationTypeDeleted,
		EventTime:     lastReportedTime,
		ResourceType:  relationshipType,
		ResourceId:    subjectResourceId,
		Resource:      obj,
	}
}
