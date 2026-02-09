package model

// MinimizeLatencyToken is the token value for minimize-latency consistency (empty string).
const MinimizeLatencyToken ConsistencyToken = ""

// Consistency represents the consistency requirement for authorization checks.
// It holds all options: Unspecified, MinimizeLatency, AtLeastAsAcknowledged, and AtLeastAsFresh (with token).
type Consistency struct {
	Preference ConsistencyPreference // Unspecified, MinimizeLatency, AtLeastAsAcknowledged, or AtLeastAsFresh
	Token      ConsistencyToken      // MinimizeLatencyToken ("") for minimize latency; set when Preference is AtLeastAsFresh
}

// NewConsistencyUnspecified creates a Consistency when no preference was specified. Uses MinimiezLatency by default.
func NewConsistencyUnspecified() Consistency {
	return Consistency{Preference: ConsistencyUnspecified, Token: MinimizeLatencyToken}
}

// NewConsistencyMinimizeLatency creates a Consistency that minimizes latency (empty string).
func NewConsistencyMinimizeLatency() Consistency {
	return Consistency{Preference: ConsistencyMinimizeLatency, Token: MinimizeLatencyToken}
}

// NewConsistencyAtLeastAsAcknowledged creates a Consistency for at_least_as_acknowledged.
func NewConsistencyAtLeastAsAcknowledged() Consistency {
	return Consistency{Preference: ConsistencyAtLeastAsAcknowledged, Token: MinimizeLatencyToken}
}

// NewConsistencyAtLeastAsFresh creates a Consistency requiring at-least-as-fresh semantics with the given token.
func NewConsistencyAtLeastAsFresh(token ConsistencyToken) Consistency {
	return Consistency{Preference: ConsistencyAtLeastAsFresh, Token: token}
}

// MinimizeLatency returns true if this consistency prefers minimal latency over freshness.
func (c Consistency) MinimizeLatency() bool {
	return c.Preference == ConsistencyMinimizeLatency
}

// AtLeastAsFresh returns the consistency token when Preference is AtLeastAsFresh; otherwise empty.
func (c Consistency) AtLeastAsFresh() ConsistencyToken {
	if c.Preference == ConsistencyAtLeastAsFresh {
		return c.Token
	}
	return MinimizeLatencyToken
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
