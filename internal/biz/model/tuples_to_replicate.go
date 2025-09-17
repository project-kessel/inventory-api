package model

import (
	"fmt"
)

type TuplesToReplicate struct {
	tuplesToCreate *[]RelationsTuple
	tuplesToDelete *[]RelationsTuple
}

func NewTuplesToReplicate(tuplesToCreate, tuplesToDelete *[]RelationsTuple) (TuplesToReplicate, error) {
	// Validate that at least one attribute is provided
	if tuplesToCreate == nil && tuplesToDelete == nil {
		return TuplesToReplicate{}, fmt.Errorf("at least one of tuplesToCreate or tuplesToDelete must be provided")
	}

	// Validate that if an attribute is not null, it is not empty
	if tuplesToCreate != nil && len(*tuplesToCreate) == 0 {
		return TuplesToReplicate{}, fmt.Errorf("tuplesToCreate cannot be empty when provided")
	}

	if tuplesToDelete != nil && len(*tuplesToDelete) == 0 {
		return TuplesToReplicate{}, fmt.Errorf("tuplesToDelete cannot be empty when provided")
	}

	return TuplesToReplicate{
		tuplesToCreate: tuplesToCreate,
		tuplesToDelete: tuplesToDelete,
	}, nil
}

func (ttr TuplesToReplicate) TuplesToCreate() *[]RelationsTuple {
	return ttr.tuplesToCreate
}

func (ttr TuplesToReplicate) TuplesToDelete() *[]RelationsTuple {
	return ttr.tuplesToDelete
}

func (ttr TuplesToReplicate) HasTuplesToCreate() bool {
	return ttr.tuplesToCreate != nil
}

func (ttr TuplesToReplicate) HasTuplesToDelete() bool {
	return ttr.tuplesToDelete != nil
}
