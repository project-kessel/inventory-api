package model

// MinimizeLatencyToken as an empty token string.
const MinimizeLatencyToken ConsistencyToken = ""

// Consistency represents the consistency requirement for authorization checks.
// It holds all options: Unspecified, MinimizeLatency, AtLeastAsAcknowledged, and AtLeastAsFresh (with token).
type Consistency struct {
	Preference ConsistencyPreference // Unspecified, MinimizeLatency, AtLeastAsAcknowledged, or AtLeastAsFresh
	Token      *ConsistencyToken     // Set only when Preference is AtLeastAsFresh
}

// NewConsistencyUnspecified creates a Consistency with Preference set to
// ConsistencyUnspecified and no token.
// It is a placeholder value only; final consistency behavior is resolved
// elsewhere (for example, via configuration or feature flags).
func NewConsistencyUnspecified() Consistency {
	return Consistency{Preference: ConsistencyUnspecified, Token: nil}
}

// NewConsistencyMinimizeLatency creates a Consistency that minimizes latency.
func NewConsistencyMinimizeLatency() Consistency {
	return Consistency{Preference: ConsistencyMinimizeLatency, Token: nil}
}

// NewConsistencyAtLeastAsAcknowledged creates a Consistency for at_least_as_acknowledged.
func NewConsistencyAtLeastAsAcknowledged() Consistency {
	return Consistency{Preference: ConsistencyAtLeastAsAcknowledged, Token: nil}
}

// NewConsistencyAtLeastAsFresh creates a Consistency requiring at-least-as-fresh semantics.
func NewConsistencyAtLeastAsFresh(token ConsistencyToken) Consistency {
	return Consistency{Preference: ConsistencyAtLeastAsFresh, Token: &token}
}

// MinimizeLatency returns true if this consistency prefers minimal latency over freshness.
func (c Consistency) MinimizeLatency() bool {
	return c.Preference == ConsistencyMinimizeLatency
}

// AtLeastAsFresh returns the consistency token only when Preference is
// ConsistencyAtLeastAsFresh and a token is present.
// For all other preferences, it returns nil.
func (c Consistency) AtLeastAsFresh() *ConsistencyToken {
	if c.Preference == ConsistencyAtLeastAsFresh {
		return c.Token
	}
	return nil
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
