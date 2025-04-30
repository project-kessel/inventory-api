package model

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/stretchr/testify/assert"
)

const txid = "txid"

func createTestResource(isv1beta2 bool) *Resource {
	now := time.Now()
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}

	inventoryId, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}

	resource := &Resource{
		ID:          id,
		InventoryId: &inventoryId,
		CreatedAt:   &now,
		UpdatedAt:   &now,
		OrgId:       "my-org",
		ResourceData: map[string]any{
			"foo": "bar",
		},
		ResourceType: "my-resource",
		WorkspaceId:  "my-workspace",
		Reporter: ResourceReporter{
			Reporter: Reporter{
				ReporterId:      "reporter_id",
				ReporterType:    "reporter_type",
				ReporterVersion: "1.0.2",
			},
			LocalResourceId: "foo-resource",
		},
		ConsoleHref: "/etc/console",
		ApiHref:     "/etc/api",
		Labels: Labels{
			{
				Key:   "label-1",
				Value: "value-1",
			},
			{
				Key:   "label-1",
				Value: "value-2",
			},
			{
				Key:   "label-xyz",
				Value: "value-xyz",
			},
		},
	}

	if isv1beta2 {
		// Should replace namespace
		resource.ReporterType = "hbi"
	}

	return resource
}

func TestNewOutboxEventsFromResourceCreated(t *testing.T) {
	resource := createTestResource(false)
	namespace := "foobar-namespace"
	resourceEvent, tupleEvent, err := NewOutboxEventsFromResource(*resource, namespace, OperationTypeCreated, txid)
	assert.Nil(t, err)
	assert.NotNil(t, resourceEvent)
	assert.NotNil(t, tupleEvent)
	assertResourceEvent(t, OperationTypeCreated, resource, resourceEvent)
	assertSetTupleEvent(t, resource, tupleEvent, namespace)
}

func TestNewOutboxEventsFromResourceUpdated(t *testing.T) {
	resource := createTestResource(false)
	namespace := "foobar-namespace"
	resourceEvent, tupleEvent, err := NewOutboxEventsFromResource(*resource, namespace, OperationTypeUpdated, txid)
	assert.Nil(t, err)
	assert.NotNil(t, resourceEvent)
	assert.NotNil(t, tupleEvent)
	assertResourceEvent(t, OperationTypeUpdated, resource, resourceEvent)
	assertSetTupleEvent(t, resource, tupleEvent, namespace)
}

func TestNewOutboxEventsFromResourceDeleted(t *testing.T) {
	resource := createTestResource(false)
	namespace := "foobar-namespace"
	resourceEvent, tupleEvent, err := NewOutboxEventsFromResource(*resource, namespace, OperationTypeDeleted, txid)
	assert.Nil(t, err)
	assert.NotNil(t, resourceEvent)
	assert.NotNil(t, tupleEvent)
	assertResourceEvent(t, OperationTypeDeleted, resource, resourceEvent)
	assertUnsetTupleEvent(t, resource, tupleEvent, namespace)
}

func TestNewOutboxEventsFromResourceCreated_v1beta2(t *testing.T) {
	resource := createTestResource(true)
	namespace := "foobar-namespace"
	resourceEvent, tupleEvent, err := NewOutboxEventsFromResource(*resource, namespace, OperationTypeCreated, txid)
	assert.Nil(t, err)
	assert.NotNil(t, resourceEvent)
	assert.NotNil(t, tupleEvent)
	assertResourceEvent(t, OperationTypeCreated, resource, resourceEvent)
	assertSetTupleEvent(t, resource, tupleEvent, resource.ReporterType) // Use reporter type as namespace for v1beta2
}

func assertSetTupleEvent(t *testing.T, resource *Resource, event *OutboxEvent, namespace string) {
	assert.NotNil(t, event)
	payloadJson, err := json.Marshal(event.Payload)
	assert.Nil(t, err)
	tupleEvent := kessel.Relationship{}
	err = json.Unmarshal(payloadJson, &tupleEvent)
	assert.Nil(t, err)

	assert.Equal(t, event.TxId, txid)
	assert.Equal(t, resource.ResourceType, tupleEvent.Resource.Type.Name)
	assert.Equal(t, namespace, tupleEvent.Resource.Type.Namespace)
	assert.Equal(t, resource.ReporterResourceId, tupleEvent.Resource.Id)
	assert.Equal(t, "workspace", tupleEvent.Relation)
	assert.Equal(t, "workspace", tupleEvent.Subject.Subject.Type.Name)
	assert.Equal(t, "rbac", tupleEvent.Subject.Subject.Type.Namespace)
	assert.Equal(t, resource.WorkspaceId, tupleEvent.Subject.Subject.Id)
}

func assertUnsetTupleEvent(t *testing.T, resource *Resource, event *OutboxEvent, namespace string) {
	assert.NotNil(t, event)
	payloadJson, err := json.Marshal(event.Payload)
	assert.Nil(t, err)
	tupleEvent := kessel.RelationTupleFilter{}
	err = json.Unmarshal(payloadJson, &tupleEvent)
	assert.Nil(t, err)

	assert.Equal(t, resource.ResourceType, *tupleEvent.ResourceType)
	assert.Equal(t, namespace, *tupleEvent.ResourceNamespace)
	assert.Equal(t, resource.ReporterResourceId, *tupleEvent.ResourceId)
	assert.Equal(t, "workspace", *tupleEvent.Relation)
}

func assertResourceEvent(t *testing.T, operation eventOperationType, resource *Resource, event *OutboxEvent) {
	assert.NotNil(t, event)
	payloadJson, err := json.Marshal(event.Payload)
	assert.Nil(t, err)
	resourceEvent := ResourceEvent{}
	err = json.Unmarshal(payloadJson, &resourceEvent)
	assert.Nil(t, err)

	assert.Equal(t, "1.0", resourceEvent.Specversion)
	assert.Contains(t, resourceEvent.Type, string(operation))
	assert.Empty(t, resourceEvent.Source)
	assert.NotEmpty(t, resourceEvent.Id)
	assert.Equal(t, resourceEvent.Subject, fmt.Sprintf("/resources/%s/%s", resource.ResourceType, resource.ID))
	switch operation {
	case OperationTypeCreated:
		assert.Equal(t, resource.CreatedAt.Format(time.UnixDate), resourceEvent.Time.Format(time.UnixDate))
	case OperationTypeUpdated:
		assert.Equal(t, resource.UpdatedAt.Format(time.UnixDate), resourceEvent.Time.Format(time.UnixDate))
	case OperationTypeDeleted:
		assert.NotNil(t, resourceEvent.Time)
	}
	assert.Equal(t, "application/json", resourceEvent.DataContentType)

	// Data attributes
	assert.NotNil(t, resourceEvent.Data)
	dataBytes, err := json.Marshal(resourceEvent.Data)
	assert.Nil(t, err)
	var data EventResourceData
	err = json.Unmarshal(dataBytes, &data)
	assert.Nil(t, err)
	assert.Equal(t, resource.ID.String(), data.Metadata.Id)
	assert.Equal(t, resource.OrgId, data.Metadata.OrgId)
	assert.Equal(t, resource.ResourceType, data.Metadata.ResourceType)
	switch operation {
	case OperationTypeCreated:
		assert.Equal(t, resource.CreatedAt.Format(time.UnixDate), data.Metadata.CreatedAt.Format(time.UnixDate))
	case OperationTypeUpdated:
		assert.Equal(t, resource.UpdatedAt.Format(time.UnixDate), data.Metadata.UpdatedAt.Format(time.UnixDate))
	case OperationTypeDeleted:
		assert.NotNil(t, data.Metadata.DeletedAt)
	}
	assert.Equal(t, resource.WorkspaceId, data.Metadata.WorkspaceId)
	assert.Len(t, data.Metadata.Labels, len(resource.Labels))
	for i, label := range resource.Labels {
		assert.Equal(t, label.Key, data.Metadata.Labels[i].Key)
		assert.Equal(t, label.Value, data.Metadata.Labels[i].Value)
	}
	assert.Equal(t, resource.ReporterId, data.ReporterData.ReporterInstanceId)
	assert.Equal(t, resource.Reporter.ReporterType, data.ReporterData.ReporterType)
	assert.Equal(t, resource.ConsoleHref, data.ReporterData.ConsoleHref)
	assert.Equal(t, resource.ApiHref, data.ReporterData.ApiHref)
	assert.Equal(t, resource.ReporterResourceId, data.ReporterData.LocalResourceId)
	assert.Equal(t, resource.Reporter.ReporterVersion, data.ReporterData.ReporterVersion)
	assert.Equal(t, resource.ResourceData, data.ResourceData)

}
