package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateJSONSchema(t *testing.T) {
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
			expectedError: "validation failed: workspace_id: Invalid type. Expected: string, given: integer", // Error due to wrong data type
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

	tests := []struct {
		name                 string
		version              uint
		currentWorkspaceID   string
		previousWorkspaceID  string
		expectTuplesToCreate bool
		expectTuplesToDelete bool
	}{
		{
			name:                 "version 0 creates initial tuple",
			version:              0,
			currentWorkspaceID:   "workspace-initial",
			previousWorkspaceID:  "",
			expectTuplesToCreate: true,
			expectTuplesToDelete: false,
		},
		{
			name:                 "workspace change creates and deletes tuples",
			version:              2,
			currentWorkspaceID:   "workspace-new",
			previousWorkspaceID:  "workspace-old",
			expectTuplesToCreate: true,
			expectTuplesToDelete: true,
		},
		{
			name:                 "same workspace does not create or delete tuples",
			version:              2,
			currentWorkspaceID:   "workspace-same",
			previousWorkspaceID:  "workspace-same",
			expectTuplesToCreate: false,
			expectTuplesToDelete: false,
		},
		{
			name:                 "delete operation (nil current) only deletes tuples",
			version:              1,
			currentWorkspaceID:   "",
			previousWorkspaceID:  "workspace-old",
			expectTuplesToCreate: false,
			expectTuplesToDelete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := model.NewReporterResourceKey(
				model.LocalResourceId("test-resource"),
				model.ResourceType("host"),
				model.ReporterType("HBI"),
				model.ReporterInstanceId("test-instance"),
			)
			require.NoError(t, err)

			// Build representations input
			var current, previous *model.Representations

			if tt.currentWorkspaceID != "" {
				currentData := map[string]interface{}{"workspace_id": tt.currentWorkspaceID}
				current, err = model.NewRepresentations(
					model.Representation(currentData),
					&tt.version,
					nil,
					nil,
				)
				require.NoError(t, err)
			}

			if tt.previousWorkspaceID != "" {
				prevVer := uint(0)
				if tt.version > 0 {
					prevVer = tt.version - 1
				}
				previous, err = model.NewRepresentations(
					model.Representation(map[string]interface{}{"workspace_id": tt.previousWorkspaceID}),
					&prevVer,
					nil,
					nil,
				)
				require.NoError(t, err)
			}

			result, err := schema.CalculateTuples(current, previous, key)
			require.NoError(t, err)
			assert.Equal(t, tt.expectTuplesToCreate, result.HasTuplesToCreate())
			assert.Equal(t, tt.expectTuplesToDelete, result.HasTuplesToDelete())
		})
	}
}
