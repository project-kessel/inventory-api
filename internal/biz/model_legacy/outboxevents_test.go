package model_legacy

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
)

const txid = "txid"

func createTestResourceReportEvent() bizmodel.ResourceReportEvent {
	resourceIdUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	reporterResourceIdUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440002")

	resourceId, _ := bizmodel.NewResourceId(resourceIdUUID)
	resourceType, _ := bizmodel.NewResourceType("my-resource")
	reporterType, _ := bizmodel.NewReporterType("reporter_type")
	reporterInstanceId, _ := bizmodel.NewReporterInstanceId("reporter_id")
	localResourceId, _ := bizmodel.NewLocalResourceId("foo-resource")
	apiHref, _ := bizmodel.NewApiHref("/etc/api")
	consoleHref, _ := bizmodel.NewConsoleHref("/etc/console")
	reporterResourceId, _ := bizmodel.NewReporterResourceId(reporterResourceIdUUID)
	transactionId := bizmodel.NewTransactionId("test-txid")

	reporterData, _ := bizmodel.NewRepresentation(internal.JsonObject{
		"foo": "bar",
	})
	commonData, _ := bizmodel.NewRepresentation(internal.JsonObject{
		"workspace_id": "my-workspace",
	})

	reporterRepresentation, _ := bizmodel.NewReporterDataRepresentation(
		reporterResourceId,
		bizmodel.NewVersion(1),
		bizmodel.NewGeneration(0),
		reporterData,
		bizmodel.NewVersion(1),
		nil,
		transactionId,
	)

	commonRepresentation, _ := bizmodel.NewCommonRepresentation(
		resourceId,
		commonData,
		bizmodel.NewVersion(1),
		reporterType,
		reporterInstanceId,
		transactionId,
	)

	event, _ := bizmodel.NewResourceReportEvent(
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

	return event
}

func TestNewOutboxEventsFromResourceEventCreated(t *testing.T) {
	resourceEvent := createTestResourceReportEvent()
	outboxResourceEvent, tupleEvent, err := NewOutboxEventsFromResourceEvent(resourceEvent, bizmodel.OperationTypeCreated, txid)
	assert.Nil(t, err)
	assert.NotNil(t, outboxResourceEvent)
	assert.NotNil(t, tupleEvent)
	assertResourceEventFromDomainEvent(t, bizmodel.OperationTypeCreated, resourceEvent, outboxResourceEvent)
	assertTupleEventFromDomainEvent(t, resourceEvent, tupleEvent)
}

func TestNewOutboxEventsFromResourceEventUpdated(t *testing.T) {
	resourceEvent := createTestResourceReportEvent()
	outboxResourceEvent, tupleEvent, err := NewOutboxEventsFromResourceEvent(resourceEvent, bizmodel.OperationTypeUpdated, txid)
	assert.Nil(t, err)
	assert.NotNil(t, outboxResourceEvent)
	assert.NotNil(t, tupleEvent)
	assertResourceEventFromDomainEvent(t, bizmodel.OperationTypeUpdated, resourceEvent, outboxResourceEvent)
	assertTupleEventFromDomainEvent(t, resourceEvent, tupleEvent)
}

func TestNewOutboxEventsFromResourceEventDeleted(t *testing.T) {
	resourceEvent := createTestResourceReportEvent()
	outboxResourceEvent, tupleEvent, err := NewOutboxEventsFromResourceEvent(resourceEvent, bizmodel.OperationTypeDeleted, txid)
	assert.Nil(t, err)
	assert.NotNil(t, outboxResourceEvent)
	assert.NotNil(t, tupleEvent)
	// For deleted events, payload is empty
	assert.Equal(t, bizmodel.OperationTypeDeleted, outboxResourceEvent.Operation)
	assertTupleEventFromDomainEvent(t, resourceEvent, tupleEvent)
}

func assertTupleEventFromDomainEvent(t *testing.T, resourceEvent bizmodel.ResourceReportEvent, event *OutboxEvent) {
	assert.NotNil(t, event)
	assert.Equal(t, txid, event.TxId)
	assert.Equal(t, TupleAggregateType, event.AggregateType)
	assert.Equal(t, resourceEvent.Id().String(), event.AggregateID)

	payloadJson, err := json.Marshal(event.Payload)
	assert.Nil(t, err)

	var tupleEvent bizmodel.TupleEvent
	err = json.Unmarshal(payloadJson, &tupleEvent)
	assert.Nil(t, err)

	key := tupleEvent.ReporterResourceKey()
	assert.Equal(t, resourceEvent.ResourceType(), key.ResourceType().String())
	assert.Equal(t, resourceEvent.ReporterType(), key.ReporterType().String())
	assert.Equal(t, resourceEvent.ReporterInstanceId(), key.ReporterInstanceId().String())
	assert.Equal(t, resourceEvent.LocalResourceId(), key.LocalResourceId().String())

	assert.NotNil(t, tupleEvent.CommonVersion())
	assert.NotNil(t, tupleEvent.ReporterRepresentationVersion())
}

func assertResourceEventFromDomainEvent(t *testing.T, operation bizmodel.EventOperationType, resourceEvent bizmodel.ResourceReportEvent, event *OutboxEvent) {
	assert.NotNil(t, event)
	assert.Equal(t, operation, event.Operation)
	assert.Equal(t, ResourceAggregateType, event.AggregateType)
	assert.Equal(t, resourceEvent.Id().String(), event.AggregateID)

	payloadJson, err := json.Marshal(event.Payload)
	assert.Nil(t, err)
	cloudEvent := ResourceEvent{}
	err = json.Unmarshal(payloadJson, &cloudEvent)
	assert.Nil(t, err)

	assert.Equal(t, "1.0", cloudEvent.Specversion)
	assert.Contains(t, cloudEvent.Type, string(operation.OperationType()))
	assert.NotEmpty(t, cloudEvent.Id)
	assert.Equal(t, "application/json", cloudEvent.DataContentType)

	// Data attributes
	assert.NotNil(t, cloudEvent.Data)
	dataBytes, err := json.Marshal(cloudEvent.Data)
	assert.Nil(t, err)
	var data EventResourceData
	err = json.Unmarshal(dataBytes, &data)
	assert.Nil(t, err)
	assert.Equal(t, resourceEvent.ResourceType(), data.Metadata.ResourceType)
	assert.Equal(t, resourceEvent.WorkspaceId(), data.Metadata.WorkspaceId)
	assert.Equal(t, resourceEvent.ReporterInstanceId(), data.ReporterData.ReporterInstanceId)
	assert.Equal(t, resourceEvent.ReporterType(), data.ReporterData.ReporterType)
	assert.Equal(t, resourceEvent.ConsoleHref(), data.ReporterData.ConsoleHref)
	assert.Equal(t, resourceEvent.ApiHref(), data.ReporterData.ApiHref)
	assert.Equal(t, resourceEvent.LocalResourceId(), data.ReporterData.LocalResourceId)
}

// TestEventResourceReporter_ReporterVersion_Optional documents that EventResourceReporter
// uses *string for ReporterVersion (nil = not set). Phase 2 will align EventRelationshipReporter.
func TestEventResourceReporter_ReporterVersion_Optional(t *testing.T) {
	// Nil ReporterVersion round-trips as omitted/zero in JSON
	er := EventResourceReporter{
		ReporterInstanceId: "inst",
		ReporterType:       "type",
		ConsoleHref:        "",
		ApiHref:            "",
		LocalResourceId:    "local",
		ReporterVersion:    nil,
	}
	b, err := json.Marshal(er)
	assert.NoError(t, err)
	var decoded EventResourceReporter
	err = json.Unmarshal(b, &decoded)
	assert.NoError(t, err)
	assert.Nil(t, decoded.ReporterVersion)

	// Non-nil ReporterVersion round-trips
	ver := "1.0"
	er.ReporterVersion = &ver
	b, err = json.Marshal(er)
	assert.NoError(t, err)
	err = json.Unmarshal(b, &decoded)
	assert.NoError(t, err)
	assert.NotNil(t, decoded.ReporterVersion)
	assert.Equal(t, "1.0", *decoded.ReporterVersion)
}

// TestEventRelationshipReporter_ReporterVersion_Optional documents that EventRelationshipReporter
// uses *string for ReporterVersion (nil = not set), aligned with EventResourceReporter.
func TestEventRelationshipReporter_ReporterVersion_Optional(t *testing.T) {
	er := EventRelationshipReporter{
		ReporterType:           "type",
		SubjectLocalResourceId: "sub",
		ObjectLocalResourceId:  "obj",
		ReporterVersion:        nil, // nil = not set
		ReporterInstanceId:     "inst",
	}
	b, err := json.Marshal(er)
	assert.NoError(t, err)
	var decoded EventRelationshipReporter
	err = json.Unmarshal(b, &decoded)
	assert.NoError(t, err)
	assert.Nil(t, decoded.ReporterVersion)

	ver := "1.0"
	er.ReporterVersion = &ver
	b, err = json.Marshal(er)
	assert.NoError(t, err)
	err = json.Unmarshal(b, &decoded)
	assert.NoError(t, err)
	assert.NotNil(t, decoded.ReporterVersion)
	assert.Equal(t, "1.0", *decoded.ReporterVersion)
}
