package api

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"strings"
	"time"
)

// Todo: get rid of this Event and have an Event (as event output) with all the assignments going on the New* functions
type Event struct {
	Specversion     string      `json:"specversion"`
	Type            string      `json:"type"`
	Source          string      `json:"source"`
	Id              string      `json:"id"`
	Subject         string      `json:"subject"`
	Time            time.Time   `json:"time"`
	DataContentType string      `json:"datacontenttype"`
	Data            interface{} `json:"data"`
}

type ResourceData struct {
	Metadata     ResourceMetadata `json:"metadata"`
	ReporterData ResourceReporter `json:"reporter_data"`
	ResourceData model.JsonObject `json:"resource_data,omitempty"`
}

type RelationshipData struct {
	Metadata     RelationshipMetadata `json:"metadata"`
	ReporterData RelationshipReporter `json:"reporter_data"`
	ResourceData model.JsonObject     `json:"resource_data,omitempty"`
}

type ResourceMetadata struct {
	Id           string          `json:"id"`
	ResourceType string          `json:"resource_type"`
	OrgId        string          `json:"org_id"`
	CreatedAt    *time.Time      `json:"created_at,omitempty"`
	UpdatedAt    *time.Time      `json:"updated_at,omitempty"`
	DeletedAt    *time.Time      `json:"deleted_at,omitempty"`
	WorkspaceId  string          `json:"workspace_id"`
	Labels       []ResourceLabel `json:"labels,omitempty"`
}

type ResourceLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ResourceReporter struct {
	ReporterInstanceId string `json:"reporter_instance_id"`
	ReporterType       string `json:"reporter_type"`
	ConsoleHref        string `json:"console_href"`
	ApiHref            string `json:"api_href"`
	LocalResourceId    string `json:"local_resource_id"`
	ReporterVersion    string `json:"reporter_version"`
}

type RelationshipMetadata struct {
	Id               string     `json:"id"`
	RelationshipType string     `json:"relationship_type"`
	CreatedAt        *time.Time `json:"created_at,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

type RelationshipReporter struct {
	ReporterType           string `json:"reporter_type"`
	SubjectLocalResourceId string `json:"subject_local_resource_id"`
	ObjectLocalResourceId  string `json:"object_local_resource_id"`
	ReporterVersion        string `json:"reporter_version"`
	ReporterInstanceId     string `json:"reporter_instance_id"`
}

type OperationType interface {
	OperationType() operationType
}

type operationType string

const (
	OperationTypeCreated operationType = "created"
	OperationTypeUpdated operationType = "updated"
	OperationTypeDeleted operationType = "deleted"
)

func (o operationType) OperationType() operationType {
	return o
}

func NewResourceEvent(operationType OperationType, resource *model.Resource, reportedTime time.Time) (*Event, error) {
	const eventType = "resources"

	eventId, err := uuid.NewUUID() // Todo: we need to have an stable id if we implement some re-trying logic
	if err != nil {
		return nil, err
	}

	var labels []ResourceLabel
	for _, val := range resource.Labels {
		labels = append(labels, ResourceLabel{
			Key:   val.Key,
			Value: val.Value,
		})
	}

	var createdAt *time.Time
	var updatedAt *time.Time
	var deletedAt *time.Time

	switch operationType {
	case OperationTypeCreated:
		createdAt = &reportedTime
	case OperationTypeUpdated:
		updatedAt = &reportedTime
	case OperationTypeDeleted:
		deletedAt = &reportedTime
	}

	return &Event{
		Specversion:     "1.0",
		Type:            makeEventType(eventType, resource.ResourceType, string(operationType.OperationType())),
		Source:          "", // Todo: inventory uri
		Id:              eventId.String(),
		Subject:         makeEventSubject(eventType, resource.ResourceType, resource.ID.String()),
		Time:            reportedTime,
		DataContentType: "application/json",
		Data: ResourceData{
			Metadata: ResourceMetadata{
				Id:           resource.ID.String(),
				OrgId:        resource.OrgId,
				ResourceType: resource.ResourceType,
				CreatedAt:    createdAt,
				UpdatedAt:    updatedAt,
				DeletedAt:    deletedAt,
				WorkspaceId:  resource.WorkspaceId,
				Labels:       labels,
			},
			ReporterData: ResourceReporter{
				ReporterInstanceId: resource.Reporter.ReporterId,
				ReporterType:       resource.Reporter.ReporterType,
				ConsoleHref:        resource.ConsoleHref,
				ApiHref:            resource.ApiHref,
				LocalResourceId:    resource.Reporter.LocalResourceId,
				ReporterVersion:    resource.Reporter.ReporterVersion,
			},
			ResourceData: resource.ResourceData,
		},
	}, nil
}

func NewRelationshipEvent(operationType OperationType, relationship *model.Relationship, reportedTime time.Time) (*Event, error) {
	const eventType = "resources-relationship"

	eventId, err := uuid.NewUUID() // Todo: we need to have an stable id if we implement some re-trying logic
	if err != nil {
		return nil, err
	}

	var createdAt *time.Time
	var updatedAt *time.Time
	var deletedAt *time.Time

	switch operationType {
	case OperationTypeCreated:
		createdAt = &reportedTime
	case OperationTypeUpdated:
		updatedAt = &reportedTime
	case OperationTypeDeleted:
		deletedAt = &reportedTime
	}

	return &Event{
		Specversion:     "1.0",
		Type:            makeEventType(eventType, relationship.RelationshipType, string(operationType.OperationType())),
		Source:          "", // Todo: inventory uri
		Id:              eventId.String(),
		Subject:         makeEventSubject(eventType, relationship.RelationshipType, relationship.ID.String()),
		Time:            reportedTime,
		DataContentType: "application/json",
		Data: RelationshipData{
			Metadata: RelationshipMetadata{
				Id:               relationship.ID.String(),
				RelationshipType: relationship.RelationshipType,
				CreatedAt:        createdAt,
				UpdatedAt:        updatedAt,
				DeletedAt:        deletedAt,
			},
			ReporterData: RelationshipReporter{
				ReporterType:           relationship.Reporter.ReporterType,
				SubjectLocalResourceId: relationship.Reporter.SubjectLocalResourceId,
				ObjectLocalResourceId:  relationship.Reporter.ObjectLocalResourceId,
				ReporterVersion:        relationship.Reporter.ReporterVersion,
				ReporterInstanceId:     relationship.Reporter.ReporterId,
			},
			// Todo Looks like we need to add the inventory ids for the related resources (see kafka-event examples)
			ResourceData: relationship.RelationshipData,
		},
	}, nil
}

func makeEventType(eventType, resourceType, operation string) string {
	return fmt.Sprintf("redhat.inventory.%s.%s.%s", eventType, resourceType, operation)
}

func makeEventSubject(eventType, resourceType, resourceId string) string {
	return "/" + strings.Join([]string{eventType, resourceType, resourceId}, "/")
}
