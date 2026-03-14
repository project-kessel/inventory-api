package model

import "context"

// RelationsRepository defines the interface for authorization and relationship
// management operations. Unlike the old Authorizer interface, all method
// signatures use domain model types only -- no protobuf or gRPC types leak
// through this boundary. Implementations are responsible for converting to
// and from whatever wire format the underlying service requires.
type RelationsRepository interface {
	Health(ctx context.Context) error

	Check(ctx context.Context, resource ReporterResourceKey, relation Relation,
		subject SubjectReference, consistency Consistency) (bool, ConsistencyToken, error)

	CheckForUpdate(ctx context.Context, resource ReporterResourceKey, relation Relation,
		subject SubjectReference) (bool, ConsistencyToken, error)

	CheckBulk(ctx context.Context, items []CheckItem,
		consistency Consistency) ([]CheckBulkResultItem, ConsistencyToken, error)

	LookupResources(ctx context.Context, query LookupResourcesQuery) (LookupResourcesIterator, error)

	CreateTuples(ctx context.Context, tuples []RelationsTuple, upsert bool,
		lockId, lockToken string) (ConsistencyToken, error)

	DeleteTuples(ctx context.Context, tuples []RelationsTuple,
		lockId, lockToken string) (ConsistencyToken, error)

	AcquireLock(ctx context.Context, lockId string) (string, error)
}

// CheckItem is a single item in a bulk permission check request.
type CheckItem struct {
	Resource ReporterResourceKey
	Relation Relation
	Subject  SubjectReference
}

// CheckBulkResultItem holds the result for a single item in a bulk check.
// Results are positionally matched to the input []CheckItem slice.
type CheckBulkResultItem struct {
	Allowed bool
	Error   error
}

// LookupResourcesQuery contains the parameters for a resource lookup.
// Grouped into a struct because the method would otherwise have 8+ parameters.
type LookupResourcesQuery struct {
	ResourceType ResourceType
	ReporterType ReporterType
	Relation     Relation
	Subject      SubjectReference
	Limit        uint32
	Continuation string
	Consistency  Consistency
}

// LookupResourcesIterator provides streaming access to lookup results.
// Callers should call Next() in a loop until it returns nil, io.EOF.
type LookupResourcesIterator interface {
	Next() (*LookupResourceResult, error)
}

// LookupResourceResult represents a single resource returned from a lookup.
type LookupResourceResult struct {
	ResourceId        LocalResourceId
	ResourceType      ResourceType
	Namespace         ReporterType
	ContinuationToken string
}
