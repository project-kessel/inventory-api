package model

import (
	"testing"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommonRepCleanupPolicyValidation(t *testing.T) {
	policy, err := NewCommonRepCleanupPolicy(2, " host ")
	require.NoError(t, err)
	resourceType, filtered := policy.ResourceType()
	assert.True(t, filtered)
	assert.Equal(t, "host", resourceType.String())
	for _, keep := range []int{-1, 0, 1} {
		_, err := NewCommonRepCleanupPolicy(keep, "")
		assert.Error(t, err)
	}
	_, err = NewCommonRepCleanupPolicy(2, "   ")
	assert.Error(t, err)
	assert.Error(t, (CommonRepCleanupPolicy{}).Validate())
}

func TestCommonRepCleanupValuesProtectInvariants(t *testing.T) {
	resourceID, err := NewResourceId(uuid.New())
	require.NoError(t, err)
	for _, input := range []struct {
		rows  int64
		roots []ResourceId
	}{
		{-1, nil}, {1, nil}, {0, []ResourceId{resourceID}}, {1, []ResourceId{{}}},
	} {
		_, err := NewCommonRepCleanupImpact(input.rows, input.roots)
		assert.Error(t, err)
	}
	for _, input := range []struct {
		exclusion string
		rows      int64
	}{
		{"unknown", 0}, {CommonRepExclusionNoActiveReporter, 1}, {"", -1},
	} {
		_, err := NewCommonRepRootClassification(resourceID, input.exclusion, input.rows)
		assert.Error(t, err)
	}
	roots := []ResourceId{resourceID}
	impact, err := NewCommonRepCleanupImpact(1, roots)
	require.NoError(t, err)
	roots[0] = ResourceId{}
	assert.Equal(t, []ResourceId{resourceID}, impact.ResourceRootIDs())
}
