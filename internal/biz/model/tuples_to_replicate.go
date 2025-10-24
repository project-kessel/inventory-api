package model

type TuplesToReplicate struct {
	tuplesToCreate *[]RelationsTuple
	tuplesToDelete *[]RelationsTuple
}

func NewTuplesToReplicate(tuplesToCreate, tuplesToDelete []RelationsTuple) (TuplesToReplicate, error) {
	// Allow empty tuples - this represents a no-op scenario
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
