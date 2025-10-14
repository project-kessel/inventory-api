package biz

import (
	"encoding/json"
	"fmt"
)

type EventOperationType interface {
	OperationType() eventOperationType
}

type eventOperationType string

const (
	OperationTypeCreated eventOperationType = "created"
	OperationTypeUpdated eventOperationType = "updated"
	OperationTypeDeleted eventOperationType = "deleted"
)

func (o eventOperationType) OperationType() eventOperationType {
	return o
}

// MarshalJSON implements json.Marshaler interface
func (o eventOperationType) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(o))
}

// UnmarshalJSON implements json.Unmarshaler interface
func (o *eventOperationType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	switch s {
	case "created":
		*o = OperationTypeCreated
	case "updated":
		*o = OperationTypeUpdated
	case "deleted":
		*o = OperationTypeDeleted
	default:
		return fmt.Errorf("invalid operation type: %s", s)
	}

	return nil
}

// Helper functions to create EventOperationType instances
func NewOperationTypeCreated() EventOperationType {
	op := OperationTypeCreated
	return &op
}

func NewOperationTypeUpdated() EventOperationType {
	op := OperationTypeUpdated
	return &op
}

func NewOperationTypeDeleted() EventOperationType {
	op := OperationTypeDeleted
	return &op
}
