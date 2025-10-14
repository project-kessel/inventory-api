package model_legacy

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
