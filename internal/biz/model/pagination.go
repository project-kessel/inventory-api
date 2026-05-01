package model

// Pagination represents pagination parameters for streaming queries.
// The struct pointer itself is nil when pagination is not specified.
// If the struct exists, proto validation guarantees Limit > 0.
type Pagination struct {
	// Limit is the maximum number of results to return (always > 0 when Pagination is non-nil).
	Limit uint32
	// Continuation is the token to continue from a previous query. nil if not specified.
	Continuation *ContinuationToken
}

// NewPagination builds pagination parameters for streaming queries.
func NewPagination(limit uint32, continuation *ContinuationToken) *Pagination {
	return &Pagination{Limit: limit, Continuation: continuation}
}

// ContinuationToken returns the continuation cursor, or nil when absent.
func (p *Pagination) ContinuationToken() *ContinuationToken {
	if p == nil {
		return nil
	}
	return p.Continuation
}
