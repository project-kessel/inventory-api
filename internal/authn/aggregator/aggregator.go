package aggregator

import (
	"github.com/project-kessel/inventory-api/internal/authn/api"
)

// AggregatingAuthenticator is the interface for authenticators that aggregate
// multiple underlying authenticators using a specific strategy.
type AggregatingAuthenticator interface {
	api.Authenticator
	// Add adds an authenticator to the chain
	Add(authenticator api.Authenticator)
}

// StrategyType represents the aggregation strategy to use
type StrategyType string

const (
	// FirstMatch returns the first non-Ignore decision
	FirstMatch StrategyType = "first_match"
)
