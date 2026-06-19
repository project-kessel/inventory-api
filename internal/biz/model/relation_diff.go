package model

// DiffRelationValues compares current and previous values for a given relation
// and returns the tuples to create and delete based on set difference.
// Works for both multi-valued relations (e.g., allowed_workspaces) and
// single-valued relations (e.g., parent) passed as single-element slices.
func DiffRelationValues(
	key ReporterResourceKey,
	relationName string,
	subjectNamespace string,
	subjectResourceType string,
	currentValues []string,
	previousValues []string,
) (tuplesToCreate []RelationsTuple, tuplesToDelete []RelationsTuple) {
	previousSet := make(map[string]struct{}, len(previousValues))
	for _, v := range previousValues {
		previousSet[v] = struct{}{}
	}

	currentSet := make(map[string]struct{}, len(currentValues))
	for _, v := range currentValues {
		currentSet[v] = struct{}{}
	}

	for _, v := range currentValues {
		if _, exists := previousSet[v]; !exists {
			tuplesToCreate = append(tuplesToCreate, NewRelationTupleForSubject(
				key, relationName, subjectNamespace, subjectResourceType, v,
			))
		}
	}

	for _, v := range previousValues {
		if _, exists := currentSet[v]; !exists {
			tuplesToDelete = append(tuplesToDelete, NewRelationTupleForSubject(
				key, relationName, subjectNamespace, subjectResourceType, v,
			))
		}
	}

	return tuplesToCreate, tuplesToDelete
}
