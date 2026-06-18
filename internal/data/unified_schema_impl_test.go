package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnifiedSchemaImpl_Validate(t *testing.T) {
	t.Run("valid data passes validation", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace_id": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []interface{}{"workspace_id"},
		}

		impl := NewUnifiedSchemaImpl(schema, nil)

		data := map[string]interface{}{
			"workspace_id": "ws-123",
		}

		valid, err := impl.Validate(data)
		assert.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("missing required field fails validation", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace_id": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []interface{}{"workspace_id"},
		}

		impl := NewUnifiedSchemaImpl(schema, nil)

		data := map[string]interface{}{
			"other_field": "value",
		}

		valid, err := impl.Validate(data)
		assert.Error(t, err)
		assert.False(t, valid)
		assert.Contains(t, err.Error(), "workspace_id is required")
	})

	t.Run("wrong type fails validation", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace_id": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []interface{}{"workspace_id"},
		}

		impl := NewUnifiedSchemaImpl(schema, nil)

		data := map[string]interface{}{
			"workspace_id": 123, // Should be string
		}

		valid, err := impl.Validate(data)
		assert.Error(t, err)
		assert.False(t, valid)
		assert.Contains(t, err.Error(), "Invalid type")
	})

	t.Run("enum validation works", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"status": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"READY", "FAILED", "OFFLINE"},
				},
			},
			"required": []interface{}{"status"},
		}

		impl := NewUnifiedSchemaImpl(schema, nil)

		// Valid enum value
		validData := map[string]interface{}{
			"status": "READY",
		}
		valid, err := impl.Validate(validData)
		assert.NoError(t, err)
		assert.True(t, valid)

		// Invalid enum value
		invalidData := map[string]interface{}{
			"status": "INVALID",
		}
		valid, err = impl.Validate(invalidData)
		assert.Error(t, err)
		assert.False(t, valid)
	})
}

func TestUnifiedSchemaImpl_CalculateTuples(t *testing.T) {
	t.Run("delegates to DefaultSchema in Phase 1", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace_id": map[string]interface{}{"type": "string"},
			},
		}

		relations := []model.RelationDefinition{
			{
				Name:        "workspace",
				Target:      "rbac/workspace",
				Field:       "workspace_id",
				Cardinality: "one",
			},
		}

		impl := NewUnifiedSchemaImpl(schema, relations)

		resourceType, _ := model.NewResourceType("host")
		reporterType, _ := model.NewReporterType("hbi")
		reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
		key, _ := model.NewReporterResourceKey(
			model.LocalResourceId("test-resource"),
			resourceType, reporterType, reporterInstanceId,
		)

		ver := model.NewVersion(0)
		current, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-1"}),
			&ver, nil, nil,
		)

		tuples, err := impl.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		// Should delegate to DefaultSchema which creates workspace tuple
		assert.True(t, tuples.HasTuplesToCreate())
		assert.False(t, tuples.HasTuplesToDelete())
	})
}
