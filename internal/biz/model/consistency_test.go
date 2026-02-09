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

func TestNewConsistencyUnspecified(t *testing.T) {
	c := NewConsistencyUnspecified()
	assert.Equal(t, ConsistencyUnspecified, c.Preference)
	assert.Empty(t, c.Token)
}

func TestNewConsistencyMinimizeLatency(t *testing.T) {
	c := NewConsistencyMinimizeLatency()
	assert.Equal(t, ConsistencyMinimizeLatency, c.Preference)
	assert.Empty(t, c.Token)
}

func TestNewConsistencyAtLeastAsAcknowledged(t *testing.T) {
	c := NewConsistencyAtLeastAsAcknowledged()
	assert.Equal(t, ConsistencyAtLeastAsAcknowledged, c.Preference)
	assert.Empty(t, c.Token)
}

func TestNewConsistencyAtLeastAsFresh(t *testing.T) {
	token := "test-token-123"
	c := NewConsistencyAtLeastAsFresh(ConsistencyToken(token))
	assert.Equal(t, ConsistencyAtLeastAsFresh, c.Preference)
	assert.Equal(t, token, string(c.Token))
}

func TestConsistency_Defaults(t *testing.T) {
	// Zero value should be unspecified
	var c Consistency
	assert.Equal(t, ConsistencyUnspecified, c.Preference)
	assert.Empty(t, c.Token)
}
