package model

// MinimizeLatencyToken as an empty token string.
const MinimizeLatencyToken ConsistencyToken = ""

// Consistency models consistency requirements as a closed set of variants.
// Implementations are intentionally sealed to this package.
type Consistency interface {
	sealedConsistency()
	Type() ConsistencyType
}

// AtLeastAsFreshConsistency is the only consistency subtype that carries a token.
type AtLeastAsFreshConsistency interface {
	Consistency
	ConsistencyToken() ConsistencyToken
}

type unspecifiedConsistency struct{}
type minimizeLatencyConsistency struct{}
type atLeastAsAcknowledgedConsistency struct{}
type atLeastAsFreshConsistency struct {
	token ConsistencyToken
}

func (unspecifiedConsistency) sealedConsistency()           {}
func (minimizeLatencyConsistency) sealedConsistency()       {}
func (atLeastAsAcknowledgedConsistency) sealedConsistency() {}
func (atLeastAsFreshConsistency) sealedConsistency()        {}

func (unspecifiedConsistency) Type() ConsistencyType {
	return ConsistencyUnspecified
}

func (minimizeLatencyConsistency) Type() ConsistencyType {
	return ConsistencyMinimizeLatency
}

func (atLeastAsAcknowledgedConsistency) Type() ConsistencyType {
	return ConsistencyAtLeastAsAcknowledged
}

func (atLeastAsFreshConsistency) Type() ConsistencyType {
	return ConsistencyAtLeastAsFresh
}

func (c atLeastAsFreshConsistency) ConsistencyToken() ConsistencyToken {
	return c.token
}

// NewConsistencyUnspecified creates a Consistency with Preference set to
// ConsistencyUnspecified and no token.
// It is a placeholder value only; final consistency behavior is resolved
// elsewhere (for example, via configuration or feature flags).
func NewConsistencyUnspecified() Consistency {
	return unspecifiedConsistency{}
}

// NewConsistencyMinimizeLatency creates a Consistency that minimizes latency.
func NewConsistencyMinimizeLatency() Consistency {
	return minimizeLatencyConsistency{}
}

// NewConsistencyAtLeastAsAcknowledged creates a Consistency for at_least_as_acknowledged.
func NewConsistencyAtLeastAsAcknowledged() Consistency {
	return atLeastAsAcknowledgedConsistency{}
}

// NewConsistencyAtLeastAsFresh creates a Consistency requiring at-least-as-fresh semantics.
func NewConsistencyAtLeastAsFresh(token ConsistencyToken) Consistency {
	return atLeastAsFreshConsistency{token: token}
}

// normalizeConsistency maps nil Consistency to the unspecified variant.
func normalizeConsistency(c Consistency) Consistency {
	if c == nil {
		return NewConsistencyUnspecified()
	}
	return c
}

// ConsistencyTypeOf returns the type for the provided consistency.
// Nil consistency is treated as unspecified.
func ConsistencyTypeOf(c Consistency) ConsistencyType {
	return normalizeConsistency(c).Type()
}

// AsAtLeastAsFresh narrows Consistency to AtLeastAsFreshConsistency when possible.
func AsAtLeastAsFresh(c Consistency) (AtLeastAsFreshConsistency, bool) {
	fresh, ok := normalizeConsistency(c).(AtLeastAsFreshConsistency)
	return fresh, ok
}

// ConsistencyAtLeastAsFreshToken returns the token only for at-least-as-fresh consistency.
// Nil consistency is treated as unspecified.
func ConsistencyAtLeastAsFreshToken(c Consistency) *ConsistencyToken {
	fresh, ok := AsAtLeastAsFresh(c)
	if !ok {
		return nil
	}
	token := fresh.ConsistencyToken()
	return &token
}

// ConsistencyType represents how consistency should be handled for authz checks.
type ConsistencyType int

// ConsistencyPreference is a backward-compatible alias for ConsistencyType.
type ConsistencyPreference = ConsistencyType

const (
	// ConsistencyUnspecified indicates no preference was specified by the client.
	// The behavior depends on the feature flag authz.kessel.default-to-at-least-as-acknowledged.
	ConsistencyUnspecified ConsistencyType = 0

	// ConsistencyMinimizeLatency uses the fastest snapshot available.
	// No consistency guarantee - uses whatever data is fastest to retrieve.
	ConsistencyMinimizeLatency ConsistencyType = 1

	// ConsistencyAtLeastAsAcknowledged looks up the consistency token from inventory's database.
	// Provides read-after-write consistency for inventory-managed resources.
	ConsistencyAtLeastAsAcknowledged ConsistencyType = 2

	// ConsistencyAtLeastAsFresh uses a consistency token provided by the caller.
	// All data used in the API call must be at least as fresh as the token.
	ConsistencyAtLeastAsFresh ConsistencyType = 3
)

// String returns a human-readable representation of the consistency type.
func (c ConsistencyType) String() string {
	switch c {
	case ConsistencyUnspecified:
		return "unspecified"
	case ConsistencyMinimizeLatency:
		return "minimize_latency"
	case ConsistencyAtLeastAsAcknowledged:
		return "at_least_as_acknowledged"
	case ConsistencyAtLeastAsFresh:
		return "at_least_as_fresh"
	default:
		return "unknown"
	}
}
