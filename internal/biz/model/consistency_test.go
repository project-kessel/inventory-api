package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, ConsistencyUnspecified, ConsistencyPreferenceOf(c))
	assert.Nil(t, ConsistencyAtLeastAsFreshToken(c))
}

func TestNewConsistencyMinimizeLatency(t *testing.T) {
	c := NewConsistencyMinimizeLatency()
	assert.Equal(t, ConsistencyMinimizeLatency, ConsistencyPreferenceOf(c))
	assert.Nil(t, ConsistencyAtLeastAsFreshToken(c))
	assert.Equal(t, ConsistencyMinimizeLatency, ConsistencyTypeOf(c))
}

func TestNewConsistencyAtLeastAsAcknowledged(t *testing.T) {
	c := NewConsistencyAtLeastAsAcknowledged()
	assert.Equal(t, ConsistencyAtLeastAsAcknowledged, ConsistencyPreferenceOf(c))
	assert.Nil(t, ConsistencyAtLeastAsFreshToken(c))
	assert.Equal(t, ConsistencyAtLeastAsAcknowledged, ConsistencyTypeOf(c))
}

func TestNewConsistencyAtLeastAsFresh(t *testing.T) {
	token := "test-token-123"
	c := NewConsistencyAtLeastAsFresh(ConsistencyToken(token))
	assert.Equal(t, ConsistencyAtLeastAsFresh, ConsistencyPreferenceOf(c))
	atLeastAsFresh := ConsistencyAtLeastAsFreshToken(c)
	require.NotNil(t, atLeastAsFresh)
	assert.Equal(t, token, atLeastAsFresh.String())
	freshConsistency, ok := AsAtLeastAsFresh(c)
	require.True(t, ok)
	assert.Equal(t, ConsistencyToken(token), freshConsistency.ConsistencyToken())
}

func TestConsistency_Defaults(t *testing.T) {
	// Nil interface value should be treated as unspecified.
	var c Consistency
	assert.Equal(t, ConsistencyUnspecified, ConsistencyPreferenceOf(c))
	assert.Nil(t, ConsistencyAtLeastAsFreshToken(c))
}

func TestConsistencyAtLeastAsFreshToken_OnlyFreshCarriesToken(t *testing.T) {
	freshToken := ConsistencyToken("fresh-token-xyz")
	tests := []struct {
		name         string
		consistency  Consistency
		expectNil    bool
		expectedPref ConsistencyPreference
	}{
		{
			name:         "unspecified has no token",
			consistency:  NewConsistencyUnspecified(),
			expectNil:    true,
			expectedPref: ConsistencyUnspecified,
		},
		{
			name:         "minimize_latency has no token",
			consistency:  NewConsistencyMinimizeLatency(),
			expectNil:    true,
			expectedPref: ConsistencyMinimizeLatency,
		},
		{
			name:         "at_least_as_acknowledged has no token",
			consistency:  NewConsistencyAtLeastAsAcknowledged(),
			expectNil:    true,
			expectedPref: ConsistencyAtLeastAsAcknowledged,
		},
		{
			name:         "at_least_as_fresh carries token",
			consistency:  NewConsistencyAtLeastAsFresh(freshToken),
			expectNil:    false,
			expectedPref: ConsistencyAtLeastAsFresh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedPref, ConsistencyPreferenceOf(tt.consistency))
			token := ConsistencyAtLeastAsFreshToken(tt.consistency)
			if tt.expectNil {
				assert.Nil(t, token)
				return
			}
			require.NotNil(t, token)
			assert.Equal(t, freshToken, *token)
		})
	}
}
