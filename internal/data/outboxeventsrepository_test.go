package data

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/project-kessel/inventory-api/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

// TestPublishOutboxEventWAL_VisibleToLogicalDecoding verifies the WAL outbox
// publisher end to end on PostgreSQL: the event is emitted as a transactional
// logical decoding message, becomes visible when the surrounding transaction
// commits, and is readable by a logical replication slot — the same mechanism
// the Debezium consumer pipeline uses in production. The test container runs
// wal_level=logical to make this possible.
func TestPublishOutboxEventWAL_VisibleToLogicalDecoding(t *testing.T) {
	if !testutil.UsingPostgres() {
		t.Skip("pg_logical_emit_message is PostgreSQL-only; SQLite uses the no-op publisher")
	}
	db := setupRepositoryDB(t)

	// A logical slot records messages emitted after its creation. The
	// test_decoding output plugin renders messages as text, which makes the
	// prefix and JSON content assertable.
	slot := fmt.Sprintf("kessel_slot_%s", uuid.NewString()[:8])
	require.NoError(t, db.Exec("SELECT pg_create_logical_replication_slot(?, 'test_decoding')", slot).Error)
	t.Cleanup(func() {
		// The slot must be dropped before the per-test database is dropped;
		// t.Cleanup runs in LIFO order, so this runs before the database drop
		// registered by setupRepositoryDB.
		if err := db.Exec("SELECT pg_drop_replication_slot(?)", slot).Error; err != nil {
			t.Logf("drop logical replication slot %s: %v", slot, err)
		}
	})

	publisher := SetOutboxPublisher(storage.OutboxModeWAL)
	event := &model_legacy.OutboxEvent{
		AggregateType: model_legacy.ResourceAggregateType,
		AggregateID:   "resource-wal-test",
		Operation:     bizmodel.OperationTypeCreated,
		TxId:          "42",
		Payload:       internal.JsonObject{"wal": true},
	}

	// The publisher runs inside the transaction, as the repository invokes it:
	// a transactional message is only delivered when the transaction commits.
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return publisher(tx, event)
	}))

	rows, err := db.Raw("SELECT data FROM pg_logical_slot_peek_changes(?, NULL, NULL)", slot).Rows()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, rows.Close(), "close logical decoding rows")
	}()

	var messages []string
	for rows.Next() {
		var data string
		require.NoError(t, rows.Scan(&data))
		messages = append(messages, data)
	}
	require.NoError(t, rows.Err())

	joined := strings.Join(messages, "\n")
	assert.Contains(t, joined, "message: transactional: 1", "expected a transactional logical decoding message")
	assert.Contains(t, joined, fmt.Sprintf("prefix: %s", model_legacy.ResourceAggregateType), "expected the outbox prefix")
	assert.Contains(t, joined, `"aggregateid":"resource-wal-test"`, "expected the outbox JSON content")
}
