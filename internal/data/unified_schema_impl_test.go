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
	t.Run("creates tuple for new workspace", func(t *testing.T) {
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

		// Should create workspace tuple
		assert.True(t, tuples.HasTuplesToCreate())
		assert.False(t, tuples.HasTuplesToDelete())

		// Verify tuple structure
		created := tuples.TuplesToCreate()
		require.NotNil(t, created)
		require.Len(t, *created, 1)
		assert.Equal(t, "workspace", (*created)[0].Relation().Serialize())
	})

	t.Run("deletes old tuple when workspace changes", func(t *testing.T) {
		relations := []model.RelationDefinition{
			{
				Name:        "workspace",
				Target:      "rbac/workspace",
				Field:       "workspace_id",
				Cardinality: "one",
			},
		}

		impl := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

		resourceType, _ := model.NewResourceType("host")
		reporterType, _ := model.NewReporterType("hbi")
		reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
		key, _ := model.NewReporterResourceKey(
			model.LocalResourceId("test-resource"),
			resourceType, reporterType, reporterInstanceId,
		)

		ver := model.NewVersion(0)
		previous, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-old"}),
			&ver, nil, nil,
		)

		ver2 := model.NewVersion(1)
		current, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-new"}),
			&ver2, nil, nil,
		)

		tuples, err := impl.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		// Should create tuple for new workspace and delete old one
		assert.True(t, tuples.HasTuplesToCreate())
		assert.True(t, tuples.HasTuplesToDelete())

		created := tuples.TuplesToCreate()
		deleted := tuples.TuplesToDelete()
		require.NotNil(t, created)
		require.NotNil(t, deleted)
		require.Len(t, *created, 1)
		require.Len(t, *deleted, 1)
	})

	t.Run("no tuples when workspace unchanged", func(t *testing.T) {
		relations := []model.RelationDefinition{
			{
				Name:        "workspace",
				Target:      "rbac/workspace",
				Field:       "workspace_id",
				Cardinality: "one",
			},
		}

		impl := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

		resourceType, _ := model.NewResourceType("host")
		reporterType, _ := model.NewReporterType("hbi")
		reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
		key, _ := model.NewReporterResourceKey(
			model.LocalResourceId("test-resource"),
			resourceType, reporterType, reporterInstanceId,
		)

		ver := model.NewVersion(0)
		previous, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-1"}),
			&ver, nil, nil,
		)

		ver2 := model.NewVersion(1)
		current, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-1"}),
			&ver2, nil, nil,
		)

		tuples, err := impl.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		// Workspace unchanged - no tuples needed
		assert.False(t, tuples.HasTuplesToCreate())
		assert.False(t, tuples.HasTuplesToDelete())
	})

	t.Run("handles multiple relations", func(t *testing.T) {
		relations := []model.RelationDefinition{
			{
				Name:        "workspace",
				Target:      "rbac/workspace",
				Field:       "workspace_id",
				Cardinality: "one",
			},
			{
				Name:        "tenant",
				Target:      "rbac/tenant",
				Field:       "tenant_id",
				Cardinality: "one",
			},
		}

		impl := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

		resourceType, _ := model.NewResourceType("host")
		reporterType, _ := model.NewReporterType("hbi")
		reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
		key, _ := model.NewReporterResourceKey(
			model.LocalResourceId("test-resource"),
			resourceType, reporterType, reporterInstanceId,
		)

		ver := model.NewVersion(0)
		current, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"workspace_id": "ws-1",
				"tenant_id":    "tenant-1",
			}),
			&ver, nil, nil,
		)

		tuples, err := impl.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		// Should create tuples for both relations
		assert.True(t, tuples.HasTuplesToCreate())
		created := tuples.TuplesToCreate()
		require.NotNil(t, created)
		require.Len(t, *created, 2)

		// Verify both relation types are present
		relationNames := make(map[string]bool)
		for _, tuple := range *created {
			relationNames[tuple.Relation().Serialize()] = true
		}
		assert.True(t, relationNames["workspace"])
		assert.True(t, relationNames["tenant"])
	})

	t.Run("parses target correctly", func(t *testing.T) {
		namespace, resourceType, err := parseTarget("rbac/workspace")
		require.NoError(t, err)
		assert.Equal(t, "rbac", namespace)
		assert.Equal(t, "workspace", resourceType)
	})

	t.Run("rejects invalid target format", func(t *testing.T) {
		_, _, err := parseTarget("invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "namespace/resource_type")
	})
}
