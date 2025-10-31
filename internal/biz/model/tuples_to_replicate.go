package model

type TuplesToReplicate struct {
	tuplesToCreate *[]RelationsTuple
	tuplesToDelete *[]RelationsTuple
}

// NewTuplesToReplicate creates a new TuplesToReplicate instance.
// Both slices can be empty, which represents a no-op scenario where no tuple
// operations are needed (e.g., when a resource is updated but the workspace_id hasn't changed).
// See KSL-34 design document, scenario: "Resource Update (Workspace Same)".
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

// IsEmpty returns true if there are no tuples to create or delete (represents a no-op scenario)
func (ttr TuplesToReplicate) IsEmpty() bool {
	hasCreate := ttr.tuplesToCreate != nil && len(*ttr.tuplesToCreate) > 0
	hasDelete := ttr.tuplesToDelete != nil && len(*ttr.tuplesToDelete) > 0
	return !hasCreate && !hasDelete
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
