package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnifiedSchemaImpl_CompareWithDefaultSchema verifies that UnifiedSchemaImpl
// produces the same tuples as DefaultSchema for the workspace relation.
func TestUnifiedSchemaImpl_CompareWithDefaultSchema(t *testing.T) {
	t.Run("produces same tuples as DefaultSchema for new workspace", func(t *testing.T) {
		// Setup UnifiedSchemaImpl with workspace relation
		relations := []model.RelationDefinition{
			{
				Name:        "workspace",
				Target:      "rbac/workspace",
				Field:       "workspace_id",
				Cardinality: "one",
			},
		}
		unifiedSchema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)
		defaultSchema := model.NewDefaultSchema()

		// Create test data
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

		// Calculate tuples with both implementations
		unifiedTuples, err := unifiedSchema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		defaultTuples, err := defaultSchema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		// Both should create tuples
		assert.Equal(t, defaultTuples.HasTuplesToCreate(), unifiedTuples.HasTuplesToCreate())
		assert.Equal(t, defaultTuples.HasTuplesToDelete(), unifiedTuples.HasTuplesToDelete())

		// Verify tuple counts match
		if unifiedTuples.HasTuplesToCreate() {
			unifiedCreated := unifiedTuples.TuplesToCreate()
			defaultCreated := defaultTuples.TuplesToCreate()
			require.Equal(t, len(*defaultCreated), len(*unifiedCreated))

			// Verify tuple structure matches
			assert.Equal(t, (*defaultCreated)[0].Relation(), (*unifiedCreated)[0].Relation())
		}
	})

	t.Run("produces same tuples as DefaultSchema when workspace changes", func(t *testing.T) {
		// Setup
		relations := []model.RelationDefinition{
			{
				Name:        "workspace",
				Target:      "rbac/workspace",
				Field:       "workspace_id",
				Cardinality: "one",
			},
		}
		unifiedSchema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)
		defaultSchema := model.NewDefaultSchema()

		resourceType, _ := model.NewResourceType("host")
		reporterType, _ := model.NewReporterType("hbi")
		reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
		key, _ := model.NewReporterResourceKey(
			model.LocalResourceId("test-resource"),
			resourceType, reporterType, reporterInstanceId,
		)

		ver1 := model.NewVersion(0)
		previous, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-old"}),
			&ver1, nil, nil,
		)

		ver2 := model.NewVersion(1)
		current, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-new"}),
			&ver2, nil, nil,
		)

		// Calculate tuples
		unifiedTuples, err := unifiedSchema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		defaultTuples, err := defaultSchema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		// Both should create and delete tuples
		assert.Equal(t, defaultTuples.HasTuplesToCreate(), unifiedTuples.HasTuplesToCreate())
		assert.Equal(t, defaultTuples.HasTuplesToDelete(), unifiedTuples.HasTuplesToDelete())

		// Verify counts match
		assert.Equal(t, len(*defaultTuples.TuplesToCreate()), len(*unifiedTuples.TuplesToCreate()))
		assert.Equal(t, len(*defaultTuples.TuplesToDelete()), len(*unifiedTuples.TuplesToDelete()))
	})

	t.Run("produces same tuples as DefaultSchema when workspace unchanged", func(t *testing.T) {
		// Setup
		relations := []model.RelationDefinition{
			{
				Name:        "workspace",
				Target:      "rbac/workspace",
				Field:       "workspace_id",
				Cardinality: "one",
			},
		}
		unifiedSchema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)
		defaultSchema := model.NewDefaultSchema()

		resourceType, _ := model.NewResourceType("host")
		reporterType, _ := model.NewReporterType("hbi")
		reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
		key, _ := model.NewReporterResourceKey(
			model.LocalResourceId("test-resource"),
			resourceType, reporterType, reporterInstanceId,
		)

		ver1 := model.NewVersion(0)
		previous, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-1"}),
			&ver1, nil, nil,
		)

		ver2 := model.NewVersion(1)
		current, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-1"}),
			&ver2, nil, nil,
		)

		// Calculate tuples
		unifiedTuples, err := unifiedSchema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		defaultTuples, err := defaultSchema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		// Both should have no tuples (workspace unchanged)
		assert.False(t, unifiedTuples.HasTuplesToCreate())
		assert.False(t, unifiedTuples.HasTuplesToDelete())
		assert.Equal(t, defaultTuples.HasTuplesToCreate(), unifiedTuples.HasTuplesToCreate())
		assert.Equal(t, defaultTuples.HasTuplesToDelete(), unifiedTuples.HasTuplesToDelete())
	})
}
