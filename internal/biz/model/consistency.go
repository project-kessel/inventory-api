package model

// Consistency represents the consistency requirement for authorization checks.
// Zero value means minimize latency.
type Consistency struct {
	atLeastAsFresh ConsistencyToken // empty string means minimize latency
}

// NewConsistencyMinimizeLatency creates a Consistency that minimizes latency.
func NewConsistencyMinimizeLatency() Consistency {
	return Consistency{}
}

// NewConsistencyAtLeastAsFresh creates a Consistency requiring at-least-as-fresh semantics.
func NewConsistencyAtLeastAsFresh(token ConsistencyToken) Consistency {
	return Consistency{atLeastAsFresh: token}
}

// MinimizeLatency returns true if this consistency prefers minimal latency over freshness.
func (c Consistency) MinimizeLatency() bool {
	return c.atLeastAsFresh == ""
}

// AtLeastAsFresh returns the consistency token, or empty if minimize latency.
func (c Consistency) AtLeastAsFresh() ConsistencyToken {
	return c.atLeastAsFresh
}
