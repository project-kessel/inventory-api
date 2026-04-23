package model

// CheckBulkResultItem contains the result for a single bulk check item.
type CheckBulkResultItem struct {
	allowed   bool
	err       error
	errorCode int32
}

func NewCheckBulkResultItem(allowed bool, err error, errorCode int32) CheckBulkResultItem {
	return CheckBulkResultItem{allowed: allowed, err: err, errorCode: errorCode}
}

func (i CheckBulkResultItem) Allowed() bool    { return i.allowed }
func (i CheckBulkResultItem) Err() error       { return i.err }
func (i CheckBulkResultItem) ErrorCode() int32 { return i.errorCode }

// CheckBulkResultPair pairs a request Relationship with its result.
type CheckBulkResultPair struct {
	request Relationship
	result  CheckBulkResultItem
}

func NewCheckBulkResultPair(request Relationship, result CheckBulkResultItem) CheckBulkResultPair {
	return CheckBulkResultPair{request: request, result: result}
}

func (p CheckBulkResultPair) Request() Relationship       { return p.request }
func (p CheckBulkResultPair) Result() CheckBulkResultItem { return p.result }

// CheckBulkResult contains the response from a bulk permission check.
type CheckBulkResult struct {
	pairs            []CheckBulkResultPair
	consistencyToken ConsistencyToken
}

func NewCheckBulkResult(pairs []CheckBulkResultPair, consistencyToken ConsistencyToken) CheckBulkResult {
	return CheckBulkResult{pairs: pairs, consistencyToken: consistencyToken}
}

func (r CheckBulkResult) Pairs() []CheckBulkResultPair      { return r.pairs }
func (r CheckBulkResult) ConsistencyToken() ConsistencyToken { return r.consistencyToken }
