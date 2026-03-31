package model

// Pagination represents pagination parameters for streaming queries.
// The struct pointer itself is nil when pagination is not specified.
// If the struct exists, proto validation guarantees Limit > 0.
type Pagination struct {
	// Limit is the maximum number of results to return (always > 0 when Pagination is non-nil).
	Limit uint32
	// Continuation is the token to continue from a previous query. nil if not specified.
	Continuation *string
}
