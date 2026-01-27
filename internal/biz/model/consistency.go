package model

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

// ConsistencyConfig holds the consistency configuration for an authz check.
type ConsistencyConfig struct {
	Preference ConsistencyPreference
	// Token is only used when Preference is ConsistencyAtLeastAsFresh
	Token string
}

// NewUnspecifiedConsistency creates a consistency config when no preference was specified.
func NewUnspecifiedConsistency() ConsistencyConfig {
	return ConsistencyConfig{
		Preference: ConsistencyUnspecified,
	}
}

// NewMinimizeLatencyConsistency creates a consistency config for minimize_latency.
func NewMinimizeLatencyConsistency() ConsistencyConfig {
	return ConsistencyConfig{
		Preference: ConsistencyMinimizeLatency,
	}
}

// NewAtLeastAsAcknowledgedConsistency creates a consistency config for at_least_as_acknowledged.
func NewAtLeastAsAcknowledgedConsistency() ConsistencyConfig {
	return ConsistencyConfig{
		Preference: ConsistencyAtLeastAsAcknowledged,
	}
}

// NewAtLeastAsFreshConsistency creates a consistency config with a provided token.
func NewAtLeastAsFreshConsistency(token string) ConsistencyConfig {
	return ConsistencyConfig{
		Preference: ConsistencyAtLeastAsFresh,
		Token:      token,
	}
}
