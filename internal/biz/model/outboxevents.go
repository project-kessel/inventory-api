package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

type AggregateType string

const (
	TupleAggregateType    AggregateType = "kessel.tuples"
	ResourceAggregateType AggregateType = "kessel.resources"
)

type OutboxEvent struct {
	ID            uuid.UUID          `gorm:"type:uuid;primarykey;not null"`
	AggregateType AggregateType      `gorm:"column:aggregatetype;type:varchar(255);not null"`
	AggregateID   string             `gorm:"column:aggregateid;type:varchar(255);not null"`
	Operation     EventOperationType `gorm:"type:varchar(255);not null"`
	TxId          string             `gorm:"column:txid;type:varchar(255)"`
	Payload       JsonObject
}

func (r *OutboxEvent) BeforeCreate(db *gorm.DB) error {
	var err error
	if r.ID == uuid.Nil {
		r.ID, err = uuid.NewV7()
		if err != nil {
			return fmt.Errorf("failed to generate uuid: %w", err)
		}
	}
	return nil
}

type ResourceEvent struct {
	Specversion     string      `json:"specversion"`
	Type            string      `json:"type"`
	Source          string      `json:"source"`
	Id              string      `json:"id"`
	Subject         string      `json:"subject"`
	Time            time.Time   `json:"time"`
	DataContentType string      `json:"datacontenttype"`
	Data            interface{} `json:"data"`
}

type EventResourceData struct {
	Metadata     EventResourceMetadata `json:"metadata"`
	ReporterData EventResourceReporter `json:"reporter_data"`
	ResourceData JsonObject            `json:"resource_data,omitempty"`
}

type EventRelationshipData struct {
	Metadata     EventRelationshipMetadata `json:"metadata"`
	ReporterData EventRelationshipReporter `json:"reporter_data"`
	ResourceData JsonObject                `json:"resource_data,omitempty"`
}

type EventResourceMetadata struct {
	Id           string               `json:"id"`
	ResourceType string               `json:"resource_type"`
	OrgId        string               `json:"org_id"`
	CreatedAt    *time.Time           `json:"created_at,omitempty"`
	UpdatedAt    *time.Time           `json:"updated_at,omitempty"`
	DeletedAt    *time.Time           `json:"deleted_at,omitempty"`
	WorkspaceId  string               `json:"workspace_id"`
	Labels       []EventResourceLabel `json:"labels,omitempty"`
}

type EventResourceLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type EventResourceReporter struct {
	ReporterInstanceId string `json:"reporter_instance_id"`
	ReporterType       string `json:"reporter_type"`
	ConsoleHref        string `json:"console_href"`
	ApiHref            string `json:"api_href"`
	LocalResourceId    string `json:"local_resource_id"`
	ReporterVersion    string `json:"reporter_version"`
}

type EventRelationshipMetadata struct {
	Id               string     `json:"id"`
	RelationshipType string     `json:"relationship_type"`
	CreatedAt        *time.Time `json:"created_at,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

type EventRelationshipReporter struct {
	ReporterType           string `json:"reporter_type"`
	SubjectLocalResourceId string `json:"subject_local_resource_id"`
	ObjectLocalResourceId  string `json:"object_local_resource_id"`
	ReporterVersion        string `json:"reporter_version"`
	ReporterInstanceId     string `json:"reporter_instance_id"`
}

func newResourceEvent(operationType EventOperationType, resource *Resource) (*ResourceEvent, error) {
	const eventType = "resources"
	now := time.Now()

	eventId, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	var labels []EventResourceLabel
	for _, val := range resource.Labels {
		labels = append(labels, EventResourceLabel(val))
	}

	var reportedTime time.Time
	var createdAt *time.Time
	var updatedAt *time.Time
	var deletedAt *time.Time

	switch operationType {
	case OperationTypeCreated:
		createdAt = resource.CreatedAt
		reportedTime = *createdAt
	case OperationTypeUpdated:
		updatedAt = resource.UpdatedAt
		reportedTime = *updatedAt
	case OperationTypeDeleted:
		deletedAt = &now
		reportedTime = *deletedAt
	}

	return &ResourceEvent{
		Specversion:     "1.0",
		Type:            makeEventType(eventType, resource.ResourceType, string(operationType.OperationType())),
		Source:          "", // TODO: inventory uri
		Id:              eventId.String(),
		Subject:         makeEventSubject(eventType, resource.ResourceType, resource.ID.String()),
		Time:            reportedTime,
		DataContentType: "application/json",
		Data: EventResourceData{
			Metadata: EventResourceMetadata{
				Id:           resource.ID.String(),
				OrgId:        resource.OrgId,
				ResourceType: resource.ResourceType,
				CreatedAt:    createdAt,
				UpdatedAt:    updatedAt,
				DeletedAt:    deletedAt,
				WorkspaceId:  resource.WorkspaceId,
				Labels:       labels,
			},
			ReporterData: EventResourceReporter{
				ReporterInstanceId: resource.ReporterId,
				ReporterType:       resource.Reporter.ReporterType, //nolint:staticcheck
				ConsoleHref:        resource.ConsoleHref,
				ApiHref:            resource.ApiHref,
				LocalResourceId:    resource.ReporterResourceId,
				ReporterVersion:    resource.Reporter.ReporterVersion, //nolint:staticcheck
			},
			ResourceData: resource.ResourceData,
		},
	}, nil
}

func convertResourceToResourceEvent(resource Resource, operationType EventOperationType) (JsonObject, error) {
	payload := JsonObject{}

	resourceEvent, err := newResourceEvent(operationType, &resource)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource event: %w", err)
	}

	marshalledJson, err := json.Marshal(resourceEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource to json: %w", err)
	}

	err = json.Unmarshal(marshalledJson, &payload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal json to payload: %w", err)
	}

	return payload, nil
}
func convertResourceToSetTupleEvent(resource Resource, namespace string) (JsonObject, error) {
	payload := JsonObject{}

	if resource.ReporterType != "" {
		// v1beta2 compatibility for namespace override, see common/DefaultSetWorkspace for parity
		namespace = strings.ToLower(resource.ReporterType)
	}

	relationship := &kessel.Relationship{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{
				Name:      resource.ResourceType,
				Namespace: namespace,
			},
			Id: resource.ReporterResourceId,
		},
		Relation: "workspace",
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{
					Name:      "workspace",
					Namespace: "rbac",
				},
				Id: resource.WorkspaceId,
			},
		},
	}

	marshalledJson, err := json.Marshal(relationship)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource to json: %w", err)
	}
	err = json.Unmarshal(marshalledJson, &payload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal json to payload: %w", err)
	}

	return payload, nil
}

func convertResourceToUnsetTupleEvent(resource Resource, namespace string) (JsonObject, error) {
	payload := JsonObject{}

	if resource.ReporterType != "" {
		// v1beta2 compatibility for namespace override, see common/DefaultSetWorkspace for parity
		namespace = strings.ToLower(resource.ReporterType)
	}

	tuple := &kessel.RelationTupleFilter{
		ResourceNamespace: proto.String(namespace),
		ResourceType:      proto.String(resource.ResourceType),
		ResourceId:        proto.String(resource.ReporterResourceId),
		Relation:          proto.String("workspace"),
	}

	marshalledJson, err := json.Marshal(tuple)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource to json: %w", err)
	}
	err = json.Unmarshal(marshalledJson, &payload)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal json to payload: %w", err)
	}

	return payload, nil
}

func NewOutboxEventsFromResource(resource Resource, namespace string, operationType EventOperationType, txid string) (*OutboxEvent, *OutboxEvent, error) {
	var tuplePayload JsonObject
	var tupleEvent *OutboxEvent

	// Build resource event
	payload, err := convertResourceToResourceEvent(resource, operationType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert resource to payload: %w", err)
	}

	resourceEvent := &OutboxEvent{
		Operation:     operationType,
		AggregateType: ResourceAggregateType,
		AggregateID:   resource.InventoryId.String(),
		TxId:          "",
		Payload:       payload,
	}

	// Build tuple event
	switch operationType.OperationType() {
	case OperationTypeDeleted:
		tuplePayload, err = convertResourceToUnsetTupleEvent(resource, namespace)
	default:
		tuplePayload, err = convertResourceToSetTupleEvent(resource, namespace)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert resource to payload: %w", err)
	}

	tupleEvent = &OutboxEvent{
		Operation:     operationType,
		AggregateType: TupleAggregateType,
		AggregateID:   resource.InventoryId.String(),
		TxId:          txid,
		Payload:       tuplePayload,
	}

	return resourceEvent, tupleEvent, nil
}

func makeEventType(eventType, resourceType, operation string) string {
	return fmt.Sprintf("redhat.inventory.%s.%s.%s", eventType, resourceType, operation)
}

func makeEventSubject(eventType, resourceType, resourceId string) string {
	return "/" + strings.Join([]string{eventType, resourceType, resourceId}, "/")
}
