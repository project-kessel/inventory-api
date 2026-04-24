package model

// TuplesResult contains the outcome of a tuple mutation (create or delete).
type TuplesResult struct {
	consistencyToken ConsistencyToken
}

func NewTuplesResult(consistencyToken ConsistencyToken) TuplesResult {
	return TuplesResult{consistencyToken: consistencyToken}
}

func (r TuplesResult) ConsistencyToken() ConsistencyToken { return r.consistencyToken }
