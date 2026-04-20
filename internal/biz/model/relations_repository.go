package model

import (
	"context"
)

// RelationsRepository defines the interface for managing relations (tuples, checks, and lookups).
// It provides methods for reading and writing relationships, permission checks, and health checks.
// All parameters and return types are model types; protobuf conversion is the responsibility
// of the implementation (data layer).
type RelationsRepository interface {
	Health(ctx context.Context) (HealthResult, error)

	Check(ctx context.Context, resource ReporterResourceKey, relation Relation,
		subject SubjectReference, consistency Consistency,
	) (CheckResult, error)

	CheckForUpdate(ctx context.Context, resource ReporterResourceKey, relation Relation,
		subject SubjectReference,
	) (CheckResult, error)

	CheckBulk(ctx context.Context, items []CheckBulkItem, consistency Consistency,
	) (CheckBulkResult, error)

	CheckForUpdateBulk(ctx context.Context, items []CheckBulkItem,
	) (CheckBulkResult, error)

	LookupResources(ctx context.Context, resourceType ResourceType, reporterType ReporterType,
		relation Relation, subject SubjectReference, pagination *Pagination, consistency Consistency,
	) (ResultStream[LookupResourcesItem], error)

	LookupSubjects(ctx context.Context, resource ReporterResourceKey, relation Relation,
		subjectType ResourceType, subjectReporter ReporterType, subjectRelation *Relation,
		pagination *Pagination, consistency Consistency,
	) (ResultStream[LookupSubjectsItem], error)

	CreateTuples(ctx context.Context, tuples []RelationsTuple, upsert bool, fencing *FencingCheck,
	) (TuplesResult, error)

	DeleteTuples(ctx context.Context, tuples []RelationsTuple, fencing *FencingCheck,
	) (TuplesResult, error)

	DeleteTuplesByFilter(ctx context.Context, filter TupleFilter, fencing *FencingCheck,
	) (TuplesResult, error)

	ReadTuples(ctx context.Context, filter TupleFilter, pagination *Pagination, consistency Consistency,
	) (ResultStream[ReadTuplesItem], error)

	AcquireLock(ctx context.Context, lockId string) (AcquireLockResult, error)
}
