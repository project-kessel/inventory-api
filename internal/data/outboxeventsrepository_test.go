package data

import (
	"testing"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapOutboxEventToWALMessage(t *testing.T) {
	fixedID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

	tests := []struct {
		name        string
		event       *model_legacy.OutboxEvent
		expectedMsg walOutboxMessage
		expectIDSet bool
	}{
		{
			name: "all fields properly map, including existing ID",
			event: &model_legacy.OutboxEvent{
				ID:            fixedID,
				AggregateType: model_legacy.ResourceAggregateType,
				AggregateID:   "resource-123",
				Operation:     bizmodel.OperationTypeCreated,
				TxId:          "123456",
				Payload:       internal.JsonObject{"key": "value"},
			},
			expectedMsg: walOutboxMessage{
				ID:            fixedID.String(),
				AggregateType: "kessel.resources",
				AggregateID:   "resource-123",
				Operation:     "created",
				TxID:          "123456",
				Payload:       internal.JsonObject{"key": "value"},
			},
		},
		{
			name: "generates ID when event ID is nil, all fields properly map",
			event: &model_legacy.OutboxEvent{
				ID:            uuid.Nil,
				AggregateType: model_legacy.TupleAggregateType,
				AggregateID:   "tuple-123",
				Operation:     bizmodel.OperationTypeDeleted,
				TxId:          "123456",
				Payload:       internal.JsonObject{},
			},
			expectIDSet: true,
			expectedMsg: walOutboxMessage{
				AggregateType: "kessel.tuples",
				AggregateID:   "tuple-123",
				Operation:     "deleted",
				TxID:          "123456",
				Payload:       internal.JsonObject{},
			},
		},
		{
			name: "ensures 'updated' operation type maps (already tested created/deleted)",
			event: &model_legacy.OutboxEvent{
				ID:            fixedID,
				AggregateType: model_legacy.TupleAggregateType,
				AggregateID:   "tuple-123",
				Operation:     bizmodel.OperationTypeUpdated,
				TxId:          "",
				Payload:       internal.JsonObject{"key": "value"},
			},
			expectedMsg: walOutboxMessage{
				ID:            fixedID.String(),
				AggregateType: "kessel.tuples",
				AggregateID:   "tuple-123",
				Operation:     "updated",
				TxID:          "",
				Payload:       internal.JsonObject{"key": "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := mapOutboxEventToWALMessage(tt.event)
			require.NoError(t, err)

			if tt.expectIDSet {
				assert.NotEmpty(t, msg.ID, "expected a generated ID")
				assert.NotEqual(t, uuid.Nil.String(), msg.ID)
				assert.NotEqual(t, uuid.Nil, tt.event.ID)
				assert.Equal(t, tt.expectedMsg.AggregateType, msg.AggregateType)
				assert.Equal(t, tt.expectedMsg.AggregateID, msg.AggregateID)
				assert.Equal(t, tt.expectedMsg.Operation, msg.Operation)
				assert.Equal(t, tt.expectedMsg.TxID, msg.TxID)
				assert.Equal(t, tt.expectedMsg.Payload, msg.Payload)
			} else {
				assert.Equal(t, tt.expectedMsg, msg)
			}
		})
	}
}
