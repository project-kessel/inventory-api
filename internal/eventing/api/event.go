package api

import (
	"time"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/common"
)

const (
	ADD    string = "add"
	UPDATE        = "update"
	REMOVE        = "remove"
)

// TODO this is a bit of a hack for now to convert the models into the shape expected by event consumers
type EventResource[Detail any] struct {
	Metadata     *biz.Metadata `json:"metadata"`
	ReporterData *biz.Reporter `json:"reporter_data"`
	ResourceData *Detail       `json:"resource_data"`
}

type Event struct {
	EventType string `json:"event_type"`

	EventTime time.Time `json:"event_time"`

	// TODO: events may be sent for relationships as well as resource types.
	ResourceType string      `json:"resource_type"`
	Resource     interface{} `json:"resource"`
}

func NewAddEvent[Detail any](resourceType string, last_reported_time time.Time, obj *EventResource[Detail]) *Event {
	return &Event{
		EventType:    ADD,
		EventTime:    last_reported_time,
		ResourceType: resourceType,
		Resource:     obj,
	}
}

func NewUpdateEvent[Detail any](resourceType string, last_reported_time time.Time, obj *EventResource[Detail]) *Event {
	return &Event{
		EventType:    UPDATE,
		EventTime:    last_reported_time,
		ResourceType: resourceType,
		Resource:     obj,
	}
}

func NewDeleteEvent(resourceType string, resourceId string, last_reported_time time.Time, requester *authnapi.Identity) *Event {
	return &Event{
		EventType:    REMOVE,
		EventTime:    last_reported_time,
		ResourceType: resourceType,
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
