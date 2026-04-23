package model

import (
	"context"
)

// RelationsRepository defines the interface for managing relations (tuples, checks, and lookups).
// All parameters and return types are model types; protobuf conversion is the responsibility
// of the implementation (data layer).
type RelationsRepository interface {
	Health(ctx context.Context) (HealthResult, error)

	// --- Check APIs: use Relationship ---

	Check(ctx context.Context, rel Relationship, consistency Consistency,
	) (bool, ConsistencyToken, error)

	CheckForUpdate(ctx context.Context, rel Relationship,
	) (bool, ConsistencyToken, error)

	CheckBulk(ctx context.Context, rels []Relationship, consistency Consistency,
	) (CheckBulkResult, error)

	CheckForUpdateBulk(ctx context.Context, rels []Relationship,
	) (CheckBulkResult, error)

	// --- Lookup APIs: use RepresentationType for type patterns ---

	LookupObjects(ctx context.Context,
		objectType RepresentationType,
		relation Relation, subject SubjectReference,
		pagination *Pagination, consistency Consistency,
	) (ResultStream[LookupObjectsItem], error)

	LookupSubjects(ctx context.Context,
		object ResourceReference, relation Relation,
		subjectType RepresentationType,
		subjectRelation *Relation,
		pagination *Pagination, consistency Consistency,
	) (ResultStream[LookupSubjectsItem], error)

	// --- Tuple APIs ---

	CreateTuples(ctx context.Context, tuples []RelationsTuple,
		upsert bool, fencing *FencingCheck,
	) (ConsistencyToken, error)

	DeleteTuples(ctx context.Context, filter TupleFilter,
		fencing *FencingCheck,
	) (ConsistencyToken, error)

	ReadTuples(ctx context.Context, filter TupleFilter,
		pagination *Pagination, consistency Consistency,
	) (ResultStream[ReadTuplesItem], error)

	AcquireLock(ctx context.Context, lockId LockId) (LockToken, error)
}
