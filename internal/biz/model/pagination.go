package model

// Pagination represents pagination parameters for streaming queries.
// Fields are pointers to distinguish between "not specified" (nil) and "specified with value".
type Pagination struct {
	// Limit is the maximum number of results to return. nil if not specified.
	Limit *uint32
	// Continuation is the token to continue from a previous query. nil if not specified.
	Continuation *string
}
