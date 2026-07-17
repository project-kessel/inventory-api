package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJsonSchemaWithWorkspaces_Validate(t *testing.T) {
	schema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"workspace_id": { "type": "string" }
		},
		"required": ["workspace_id"]
	}`
	tests := []struct {
		name          string
		jsonInput     interface{}
		expectErr     bool
		expectedError string
	}{
		{
			name:      "Valid JSON",
			jsonInput: map[string]interface{}{"workspace_id": "workspace-123"},
			expectErr: false,
		},
		{
			name:          "Invalid JSON (missing required field)",
			jsonInput:     map[string]interface{}{"otherKey": "value"},
			expectErr:     true,
			expectedError: "validation failed: (root): workspace_id is required",
		},
		{
			name:          "Invalid JSON (wrong data type for workspace_id)",
			jsonInput:     map[string]interface{}{"workspace_id": 123},
			expectErr:     true,
			expectedError: "validation failed: workspace_id: Invalid type. Expected: string, given: integer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationSchema := NewJsonSchemaWithWorkspacesFromString(schema)
			isValid, err := validationSchema.Validate(tt.jsonInput)
			assert.NotEqual(t, tt.expectErr, isValid)
			if tt.expectErr {
				assert.Error(t, err, "Expected error but got nil")
				assert.Contains(t, err.Error(), tt.expectedError, "Error message mismatch")
			} else {
				assert.NoError(t, err, "Expected no error for valid JSON format")
			}
		})
	}
}

func TestJsonSchemaWithWorkspaces_CalculateTuples(t *testing.T) {
	schema := NewJsonSchemaWithWorkspacesFromString(`{"type": "object"}`)

	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)

	t.Run("new workspace creates tuple", func(t *testing.T) {
		currentVersion := model.NewVersion(0)
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-1"}),
			&currentVersion, nil, nil,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)
		assert.True(t, result.HasTuplesToCreate())
		assert.False(t, result.HasTuplesToDelete())

		require.Len(t, *result.TuplesToCreate(), 1)
		assert.Equal(t, model.NewWorkspaceRelationsTuple("ws-1", key), (*result.TuplesToCreate())[0])
	})

	t.Run("workspace change creates and deletes", func(t *testing.T) {
		currentVersion := model.NewVersion(1)
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-new"}),
			&currentVersion, nil, nil,
		)
		require.NoError(t, err)

		previousVersion := currentVersion.Decrement()
		previous, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-old"}),
			&previousVersion, nil, nil,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)
		assert.True(t, result.HasTuplesToCreate())
		assert.True(t, result.HasTuplesToDelete())

		require.Len(t, *result.TuplesToCreate(), 1)
		assert.Equal(t, model.NewWorkspaceRelationsTuple("ws-new", key), (*result.TuplesToCreate())[0])
		require.Len(t, *result.TuplesToDelete(), 1)
		assert.Equal(t, model.NewWorkspaceRelationsTuple("ws-old", key), (*result.TuplesToDelete())[0])
	})

	t.Run("same workspace is no-op", func(t *testing.T) {
		currentVersion := model.NewVersion(1)
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-same"}),
			&currentVersion, nil, nil,
		)
		require.NoError(t, err)

		previousVersion := currentVersion.Decrement()
		previous, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-same"}),
			&previousVersion, nil, nil,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)
		assert.False(t, result.HasTuplesToCreate())
		assert.False(t, result.HasTuplesToDelete())
		assert.Nil(t, result.TuplesToCreate())
		assert.Nil(t, result.TuplesToDelete())
	})
}
