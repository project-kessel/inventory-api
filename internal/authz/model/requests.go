package model

// CheckRequest represents a permission check request
type CheckRequest struct {
	Resource    *ObjectReference
	Relation    string
	Subject     *SubjectReference
	Consistency *Consistency
}

// CheckResponse represents a permission check response
type CheckResponse struct {
	Allowed          Allowed
	ConsistencyToken *ConsistencyToken
}

// Allowed enum
type Allowed int

const (
	AllowedUnspecified Allowed = iota
	AllowedTrue
	AllowedFalse
)

// CheckForUpdateRequest for fully consistent checks
type CheckForUpdateRequest struct {
	Resource *ObjectReference
	Relation string
	Subject  *SubjectReference
}

// CheckForUpdateResponse
type CheckForUpdateResponse struct {
	Allowed          Allowed
	ConsistencyToken *ConsistencyToken
}

// CheckBulkRequestItem represents a single item in a bulk check
type CheckBulkRequestItem struct {
	Resource *ObjectReference
	Relation string
	Subject  *SubjectReference
}

// CheckBulkRequest for bulk permission checks
type CheckBulkRequest struct {
	Items       []*CheckBulkRequestItem
	Consistency *Consistency
}

// CheckBulkResponseItem represents the result of a single check
type CheckBulkResponseItem struct {
	Allowed Allowed
}

// CheckBulkResponsePair pairs a request with its response or error
type CheckBulkResponsePair struct {
	Request *CheckBulkRequestItem
	Item    *CheckBulkResponseItem // mutually exclusive with Error
	Error   error                  // mutually exclusive with Item
}

// CheckBulkResponse contains all check results
type CheckBulkResponse struct {
	Pairs            []*CheckBulkResponsePair
	ConsistencyToken *ConsistencyToken
}
