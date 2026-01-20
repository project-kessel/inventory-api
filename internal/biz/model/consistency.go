package model

// ConsistencyPreference represents how consistency should be handled for authz checks.
type ConsistencyPreference int

const (
	// ConsistencyMinimizeLatency uses the fastest snapshot available (default).
	// No consistency guarantee - uses whatever data is fastest to retrieve.
	ConsistencyMinimizeLatency ConsistencyPreference = iota

	// ConsistencyInventoryManaged looks up the consistency token from inventory's database.
	// Provides read-after-write consistency for inventory-managed resources.
	ConsistencyInventoryManaged

	// ConsistencyAtLeastAsFresh uses a consistency token provided by the caller.
	// All data used in the API call must be at least as fresh as the token.
	ConsistencyAtLeastAsFresh
)

// String returns a human-readable representation of the consistency preference.
func (c ConsistencyPreference) String() string {
	switch c {
	case ConsistencyMinimizeLatency:
		return "minimize_latency"
	case ConsistencyInventoryManaged:
		return "inventory_managed"
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

// NewMinimizeLatencyConsistency creates a consistency config for minimize_latency.
func NewMinimizeLatencyConsistency() ConsistencyConfig {
	return ConsistencyConfig{
		Preference: ConsistencyMinimizeLatency,
	}
}

// NewInventoryManagedConsistency creates a consistency config for inventory_managed.
func NewInventoryManagedConsistency() ConsistencyConfig {
	return ConsistencyConfig{
		Preference: ConsistencyInventoryManaged,
	}
}

// NewAtLeastAsFreshConsistency creates a consistency config with a provided token.
func NewAtLeastAsFreshConsistency(token string) ConsistencyConfig {
	return ConsistencyConfig{
		Preference: ConsistencyAtLeastAsFresh,
		Token:      token,
	}
}
