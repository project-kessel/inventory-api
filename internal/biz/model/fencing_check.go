package model

// FencingCheck contains fencing token information for tuple operations.
type FencingCheck struct {
	lockId    LockId
	lockToken LockToken
}

func NewFencingCheck(lockId LockId, lockToken LockToken) FencingCheck {
	return FencingCheck{lockId: lockId, lockToken: lockToken}
}

func (fc FencingCheck) LockId() LockId       { return fc.lockId }
func (fc FencingCheck) LockToken() LockToken { return fc.lockToken }
