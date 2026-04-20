package model

// CheckResult contains the outcome of a permission check.
type CheckResult struct {
	Allowed          bool
	ConsistencyToken ConsistencyToken
}

// CheckBulkItem represents a single item in a bulk check request.
type CheckBulkItem struct {
	Resource ReporterResourceKey
	Relation Relation
	Subject  SubjectReference
}

// CheckBulkResultItem contains the result for a single bulk check item.
type CheckBulkResultItem struct {
	Allowed   bool
	Error     error
	ErrorCode int32
}

// CheckBulkResultPair pairs a request item with its result.
type CheckBulkResultPair struct {
	Request CheckBulkItem
	Result  CheckBulkResultItem
}

// CheckBulkResult contains the response from a bulk permission check.
type CheckBulkResult struct {
	Pairs            []CheckBulkResultPair
	ConsistencyToken ConsistencyToken
}

// TuplesResult contains the outcome of a tuple create or delete operation.
type TuplesResult struct {
	ConsistencyToken ConsistencyToken
}

// AcquireLockResult contains the outcome of a lock acquisition.
type AcquireLockResult struct {
	LockToken string
}

// HealthResult contains the outcome of a health check.
type HealthResult struct {
	Status string
	Code   int
}

// LookupResourcesItem represents a single resource from a lookup stream.
type LookupResourcesItem struct {
	ResourceId        LocalResourceId
	ResourceType      ResourceType
	ReporterType      ReporterType
	ContinuationToken string
}

// LookupSubjectsItem represents a single subject from a lookup stream.
// Uses raw typed fields rather than SubjectReference because the Relations API
// response does not include ReporterInstanceId (required by ReporterResourceKey).
type LookupSubjectsItem struct {
	SubjectId         LocalResourceId
	SubjectType       ResourceType
	SubjectReporter   ReporterType
	SubjectRelation   *Relation
	ContinuationToken string
}

// FencingCheck contains fencing token information for tuple operations.
type FencingCheck struct {
	LockId    string
	LockToken string
}

// TupleFilter represents filtering criteria for tuple queries.
// Used by the deprecated RBAC compatibility CRUD endpoints.
type TupleFilter struct {
	ResourceNamespace *string
	ResourceType      *string
	ResourceId        *string
	Relation          *string
	SubjectFilter     *TupleSubjectFilter
}

// TupleSubjectFilter represents subject filtering criteria within a TupleFilter.
type TupleSubjectFilter struct {
	SubjectNamespace *string
	SubjectType      *string
	SubjectId        *string
	Relation         *string
}

// ReadTuplesItem represents a single tuple returned from a ReadTuples stream.
// Uses raw string fields because this is a passthrough for the deprecated RBAC
// compatibility endpoint — the data comes directly from the Relations API.
type ReadTuplesItem struct {
	ResourceNamespace string
	ResourceType      string
	ResourceId        string
	Relation          string
	SubjectNamespace  string
	SubjectType       string
	SubjectId         string
	SubjectRelation   *string
	ContinuationToken string
	ConsistencyToken  ConsistencyToken
}

// ResultStream is a domain-level streaming interface replacing grpc.ServerStreamingClient.
// Implementations return io.EOF when the stream is exhausted.
type ResultStream[T any] interface {
	Recv() (T, error)
}
