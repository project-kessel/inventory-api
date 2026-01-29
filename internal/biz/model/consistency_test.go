package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConsistencyPreference_String(t *testing.T) {
	tests := []struct {
		name     string
		pref     ConsistencyPreference
		expected string
	}{
		{
			name:     "unspecified",
			pref:     ConsistencyUnspecified,
			expected: "unspecified",
		},
		{
			name:     "minimize_latency",
			pref:     ConsistencyMinimizeLatency,
			expected: "minimize_latency",
		},
		{
			name:     "at_least_as_acknowledged",
			pref:     ConsistencyAtLeastAsAcknowledged,
			expected: "at_least_as_acknowledged",
		},
		{
			name:     "at_least_as_fresh",
			pref:     ConsistencyAtLeastAsFresh,
			expected: "at_least_as_fresh",
		},
		{
			name:     "unknown",
			pref:     ConsistencyPreference(99),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.pref.String())
		})
	}
}

func TestNewUnspecifiedConsistency(t *testing.T) {
	config := NewUnspecifiedConsistency()
	assert.Equal(t, ConsistencyUnspecified, config.Preference)
	assert.Empty(t, config.Token)
}

func TestNewMinimizeLatencyConsistency(t *testing.T) {
	config := NewMinimizeLatencyConsistency()
	assert.Equal(t, ConsistencyMinimizeLatency, config.Preference)
	assert.Empty(t, config.Token)
}

func TestNewAtLeastAsAcknowledgedConsistency(t *testing.T) {
	config := NewAtLeastAsAcknowledgedConsistency()
	assert.Equal(t, ConsistencyAtLeastAsAcknowledged, config.Preference)
	assert.Empty(t, config.Token)
}

func TestNewAtLeastAsFreshConsistency(t *testing.T) {
	token := "test-token-123"
	config := NewAtLeastAsFreshConsistency(token)
	assert.Equal(t, ConsistencyAtLeastAsFresh, config.Preference)
	assert.Equal(t, token, config.Token)
}

func TestConsistencyConfig_Defaults(t *testing.T) {
	// Verify default values - zero value should be unspecified
	var config ConsistencyConfig
	assert.Equal(t, ConsistencyUnspecified, config.Preference)
	assert.Empty(t, config.Token)
}
