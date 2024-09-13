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
}

func NewCreatedResourceEvent[Detail any](resourceType string, last_reported_time time.Time, obj *EventResource[Detail]) *Event {
	return &Event{
		EventType:     EventTypeResource,
		OperationType: operationTypeCreated,
		EventTime:     last_reported_time,
		ResourceType:  resourceType,
		Resource:      obj,
	}
}

func NewUpdatedResourceEvent[Detail any](resourceType string, last_reported_time time.Time, obj *EventResource[Detail]) *Event {
	return &Event{
		EventType:     EventTypeResource,
		OperationType: operationTypeUpdated,
		EventTime:     last_reported_time,
		ResourceType:  resourceType,
		Resource:      obj,
	}
}

func NewDeletedResourceEvent(resourceType string, resourceId string, last_reported_time time.Time, requester *authnapi.Identity) *Event {
	return &Event{
		EventType:     EventTypeResource,
		OperationType: operationTypeDeleted,
		EventTime:     last_reported_time,
		ResourceType:  resourceType,
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

func NewCreatedResourcesRelationshipEvent[Detail any](relationshipType string, last_reported_time time.Time, obj *EventResource[Detail]) *Event {
	return &Event{
		EventType:     EventTypeResourcesRelationship,
		OperationType: operationTypeCreated,
		EventTime:     last_reported_time,
		ResourceType:  relationshipType,
		Resource:      obj,
	}
}

func NewUpdatedResourcesRelationshipEvent[Detail any](relationshipType string, last_reported_time time.Time, obj *EventResource[Detail]) *Event {
	return &Event{
		EventType:     EventTypeResourcesRelationship,
		OperationType: operationTypeUpdated,
		EventTime:     last_reported_time,
		ResourceType:  relationshipType,
		Resource:      obj,
	}
}

func NewDeletedResourcesRelationshipEvent(relationshipType, resourceFromId, resourceToId string, last_reported_time time.Time, requester *authnapi.Identity) *Event {
	return &Event{
		EventType:     EventTypeResourcesRelationship,
		OperationType: operationTypeDeleted,
		EventTime:     last_reported_time,
		ResourceType:  relationshipType,
		Resource:      "stub",
		// Todo: We need a separate type - Metadata format is different on Relationships
		//	Relation: &EventResource[struct{}]{
		//		//Metadata: &biz.Metadata{},
		//		//ReporterData: &biz.Reporter{
		//		//	ReporterID:      requester.Principal,
		//		//	ReporterType:    requester.Type,
		//		//	LocalResourceId: resourceId,
		//		//},
		//		//ResourceData: nil,
		//	},
	}
}
