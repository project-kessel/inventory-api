package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnifiedSchemaImpl_Validate(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"workspace_id": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"workspace_id"},
	}
	implementation := NewUnifiedSchemaImpl(schema, nil, nil)

	t.Run("accepts valid representation", func(t *testing.T) {
		valid, err := implementation.Validate(map[string]interface{}{"workspace_id": "workspace-1"})

		assert.True(t, valid)
		assert.NoError(t, err)
	})

	t.Run("rejects invalid representation", func(t *testing.T) {
		valid, err := implementation.Validate(map[string]interface{}{})

		assert.False(t, valid)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workspace_id is required")
	})
}

func TestUnifiedSchemaImpl_CalculateTuplesDelegatesToLegacySchema(t *testing.T) {
	implementation := NewUnifiedSchemaImpl(map[string]interface{}{}, nil, nil)
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	reporterInstanceID, err := model.NewReporterInstanceId("instance-1")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("host-1"),
		resourceType,
		reporterType,
		reporterInstanceID,
	)
	require.NoError(t, err)

	version := model.NewVersion(1)
	current, err := model.NewRepresentations(
		model.Representation{"workspace_id": "workspace-1"},
		&version,
		nil,
		nil,
	)
	require.NoError(t, err)

	expected, err := model.NewDefaultSchema().CalculateTuples(current, nil, key)
	require.NoError(t, err)
	actual, err := implementation.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	assert.Equal(t, expected, actual)
}
