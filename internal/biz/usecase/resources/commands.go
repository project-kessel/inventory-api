package resources

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// WriteVisibility represents the write visibility option for resource operations.
type WriteVisibility int

const (
	// WriteVisibilityUnspecified means no specific visibility was requested.
	WriteVisibilityUnspecified WriteVisibility = iota
	// WriteVisibilityMinimizeLatency prioritizes speed over consistency.
	WriteVisibilityMinimizeLatency
	// WriteVisibilityConsistent waits for the write to be visible.
	WriteVisibilityConsistent
)

// ReportResourceCommand contains the data needed to report a resource.
// This is the domain command used by the usecase layer, decoupled from protobuf types.
type ReportResourceCommand struct {
	// Resource identification
	LocalResourceId    model.LocalResourceId
	ResourceType       model.ResourceType
	ReporterType       model.ReporterType
	ReporterInstanceId model.ReporterInstanceId

	// Metadata
	ApiHref         model.ApiHref
	ConsoleHref     *model.ConsoleHref
	ReporterVersion *model.ReporterVersion
	TransactionId   *model.TransactionId

	// Representations (optional)
	ReporterRepresentation *model.Representation
	CommonRepresentation   *model.Representation

	// Write behavior
	WriteVisibility WriteVisibility
}

// CheckBulkItem represents a single item in a bulk check request.
type CheckBulkItem struct {
	Resource model.ResourceReference
	Relation model.Relation
	Subject  model.SubjectReference
}

// CheckBulkCommand contains the request for a bulk permission check.
type CheckBulkCommand struct {
	Items       []CheckBulkItem
	Consistency model.Consistency
}

// CheckBulkResultItem contains the result for a single bulk check item.
type CheckBulkResultItem struct {
	Allowed   bool
	Error     error // non-nil if this specific item failed
	ErrorCode int32 // gRPC status code if Error is non-nil (0 if unknown)
}

// CheckBulkResultPair pairs a request item with its result.
type CheckBulkResultPair struct {
	Request CheckBulkItem
	Result  CheckBulkResultItem
}

// CheckBulkResult contains the response from a bulk permission check.
type CheckBulkResult struct {
	Pairs            []CheckBulkResultPair
	ConsistencyToken model.ConsistencyToken
}

// CheckForUpdateBulkCommand contains the request for a bulk check-for-update (strongly consistent).
// Reuses CheckBulkItem; no consistency token since each check is strongly consistent.
type CheckForUpdateBulkCommand struct {
	Items []CheckBulkItem
}

// CheckSelfBulkItem represents a single item in a self-check bulk request.
// Unlike CheckBulkItem, no explicit subject is provided - the subject
// is derived from the authenticated context.
type CheckSelfBulkItem struct {
	Resource model.ResourceReference
	Relation model.Relation
}

// CheckSelfBulkCommand contains the request for a bulk self-permission check.
type CheckSelfBulkCommand struct {
	Items       []CheckSelfBulkItem
	Consistency model.Consistency
}

// LookupObjectsCommand contains the request for looking up objects.
type LookupObjectsCommand struct {
	ObjectType  model.RepresentationType
	Relation    model.Relation
	Subject     model.SubjectReference
	Pagination  *model.Pagination
	Consistency model.Consistency
}

// LookupSubjectsCommand contains the request for looking up subjects.
type LookupSubjectsCommand struct {
	Resource        model.ResourceReference
	Relation        model.Relation
	SubjectType     model.RepresentationType
	SubjectRelation *model.Relation
	Pagination      *model.Pagination
	Consistency     model.Consistency
}
