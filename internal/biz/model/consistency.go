package model

// MinimizeLatencyToken as an empty token string.
const MinimizeLatencyToken ConsistencyToken = ""

// Consistency models consistency requirements as a closed set of variants.
// Implementations are intentionally sealed to this package.
type Consistency interface {
	sealedConsistency()
	Preference() ConsistencyPreference
	MinimizeLatency() bool
	AtLeastAsFresh() *ConsistencyToken
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

func (unspecifiedConsistency) Preference() ConsistencyPreference {
	return ConsistencyUnspecified
}

func (minimizeLatencyConsistency) Preference() ConsistencyPreference {
	return ConsistencyMinimizeLatency
}

func (atLeastAsAcknowledgedConsistency) Preference() ConsistencyPreference {
	return ConsistencyAtLeastAsAcknowledged
}

func (atLeastAsFreshConsistency) Preference() ConsistencyPreference {
	return ConsistencyAtLeastAsFresh
}

func (unspecifiedConsistency) MinimizeLatency() bool {
	return false
}

func (minimizeLatencyConsistency) MinimizeLatency() bool {
	return true
}

func (atLeastAsAcknowledgedConsistency) MinimizeLatency() bool {
	return false
}

func (atLeastAsFreshConsistency) MinimizeLatency() bool {
	return false
}

func (unspecifiedConsistency) AtLeastAsFresh() *ConsistencyToken {
	return nil
}

func (minimizeLatencyConsistency) AtLeastAsFresh() *ConsistencyToken {
	return nil
}

func (atLeastAsAcknowledgedConsistency) AtLeastAsFresh() *ConsistencyToken {
	return nil
}

func (c atLeastAsFreshConsistency) AtLeastAsFresh() *ConsistencyToken {
	return &c.token
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

// ConsistencyPreferenceOf returns the preference for the provided consistency.
// Nil consistency is treated as unspecified.
func ConsistencyPreferenceOf(c Consistency) ConsistencyPreference {
	return normalizeConsistency(c).Preference()
}

// ConsistencyAtLeastAsFreshToken returns the token only for at-least-as-fresh consistency.
// Nil consistency is treated as unspecified.
func ConsistencyAtLeastAsFreshToken(c Consistency) *ConsistencyToken {
	return normalizeConsistency(c).AtLeastAsFresh()
}

// ConsistencyPreference represents how consistency should be handled for authz checks.
type ConsistencyPreference int

const (
	// ConsistencyUnspecified indicates no preference was specified by the client.
	// The behavior depends on the feature flag authz.kessel.default-to-at-least-as-acknowledged.
	ConsistencyUnspecified ConsistencyPreference = 0

	// ConsistencyMinimizeLatency uses the fastest snapshot available.
	// No consistency guarantee - uses whatever data is fastest to retrieve.
	ConsistencyMinimizeLatency ConsistencyPreference = 1

	// ConsistencyAtLeastAsAcknowledged looks up the consistency token from inventory's database.
	// Provides read-after-write consistency for inventory-managed resources.
	ConsistencyAtLeastAsAcknowledged ConsistencyPreference = 2

	// ConsistencyAtLeastAsFresh uses a consistency token provided by the caller.
	// All data used in the API call must be at least as fresh as the token.
	ConsistencyAtLeastAsFresh ConsistencyPreference = 3
)

// String returns a human-readable representation of the consistency preference.
func (c ConsistencyPreference) String() string {
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
