package model

import (
	"fmt"
)

type TuplesToReplicate struct {
	tuplesToCreate *[]RelationsTuple
	tuplesToDelete *[]RelationsTuple
}

func NewTuplesToReplicate(tuplesToCreate, tuplesToDelete []RelationsTuple) (TuplesToReplicate, error) {
	// Validate that at least one slice has content
	if len(tuplesToCreate) == 0 && len(tuplesToDelete) == 0 {
		return TuplesToReplicate{}, fmt.Errorf("at least one of tuplesToCreate or tuplesToDelete must be provided")
	}

	var createPtr, deletePtr *[]RelationsTuple

	if len(tuplesToCreate) > 0 {
		createPtr = &tuplesToCreate
	}
	if len(tuplesToDelete) > 0 {
		deletePtr = &tuplesToDelete
	}

	return TuplesToReplicate{
		tuplesToCreate: createPtr,
		tuplesToDelete: deletePtr,
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
