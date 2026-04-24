package model

// CheckResult contains the outcome of a single permission check.
type CheckResult struct {
	allowed          bool
	consistencyToken ConsistencyToken
}

func NewCheckResult(allowed bool, consistencyToken ConsistencyToken) CheckResult {
	return CheckResult{allowed: allowed, consistencyToken: consistencyToken}
}

func (r CheckResult) Allowed() bool                     { return r.allowed }
func (r CheckResult) ConsistencyToken() ConsistencyToken { return r.consistencyToken }
