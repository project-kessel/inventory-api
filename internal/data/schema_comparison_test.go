package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchemaComparison_ValidationEquivalence verifies that YAML and JSON schemas
// produce identical validation results
func TestSchemaComparison_ValidationEquivalence(t *testing.T) {
	// Load host schema from YAML
	yamlSchemas, err := LoadUnifiedSchemasFromDirectory("../../data/schema/resources")
	require.NoError(t, err)

	var hostYAML *model.UnifiedSchema
	for i := range yamlSchemas {
		if yamlSchemas[i].Name == "host" {
			hostYAML = &yamlSchemas[i]
			break
		}
	}
	require.NotNil(t, hostYAML, "host schema should exist in YAML")

	yamlSchema := NewUnifiedSchemaImpl(hostYAML.Common.Schema, hostYAML.Common.Relations)

	// Create equivalent JSON schema (current DefaultSchema)
	jsonSchemaStr := `{
		"type": "object",
		"properties": {
			"workspace_id": {"type": "string"}
		},
		"required": ["workspace_id"]
	}`
	jsonSchema := NewJsonSchemaWithWorkspacesFromString(jsonSchemaStr)

	testCases := []struct {
		name       string
		data       map[string]interface{}
		shouldPass bool
	}{
		{
			name:       "valid host with workspace_id",
			data:       map[string]interface{}{"workspace_id": "ws-1"},
			shouldPass: true,
		},
		{
			name:       "missing workspace_id",
			data:       map[string]interface{}{},
			shouldPass: false,
		},
		{
			name:       "wrong type for workspace_id",
			data:       map[string]interface{}{"workspace_id": 123},
			shouldPass: false,
		},
		{
			name:       "null workspace_id",
			data:       map[string]interface{}{"workspace_id": nil},
			shouldPass: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate with JSON schema
			jsonValid, jsonErr := jsonSchema.Validate(tc.data)

			// Validate with YAML schema
			yamlValid, yamlErr := yamlSchema.Validate(tc.data)

			// Results must match
			assert.Equal(t, jsonValid, yamlValid,
				"validation results must match for %s", tc.name)
			assert.Equal(t, jsonErr != nil, yamlErr != nil,
				"error presence must match for %s", tc.name)

			// Verify expected outcome
			if tc.shouldPass {
				assert.True(t, yamlValid, "should pass validation: %s", tc.name)
				assert.NoError(t, yamlErr, "should not error: %s", tc.name)
			} else {
				assert.False(t, yamlValid,
					"should fail validation: %s", tc.name)
			}
		})
	}
}

// TestSchemaComparison_TupleGenerationEquivalence extends existing comparison tests
// with additional scenarios
func TestSchemaComparison_TupleGenerationEquivalence(t *testing.T) {
	// Setup both schemas
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

	t.Run("multiple workspace changes in sequence", func(t *testing.T) {
		// Sequence: nil → ws-1 → ws-2 → ws-3
		workspaces := []string{"ws-1", "ws-2", "ws-3"}
		var previous *model.Representations

		for i, workspace := range workspaces {
			ver := model.NewVersion(uint(i))
			current, err := model.NewRepresentations(
				model.Representation(map[string]interface{}{"workspace_id": workspace}),
				&ver, nil, nil,
			)
			require.NoError(t, err)

			// Calculate with both
			unifiedTuples, err := unifiedSchema.CalculateTuples(current, previous, key)
			require.NoError(t, err)

			defaultTuples, err := defaultSchema.CalculateTuples(current, previous, key)
			require.NoError(t, err)

			// Verify equivalence
			assert.Equal(t, defaultTuples.HasTuplesToCreate(), unifiedTuples.HasTuplesToCreate(),
				"create status must match for transition to %s", workspace)
			assert.Equal(t, defaultTuples.HasTuplesToDelete(), unifiedTuples.HasTuplesToDelete(),
				"delete status must match for transition to %s", workspace)

			if unifiedTuples.HasTuplesToCreate() {
				unifiedCreated := unifiedTuples.TuplesToCreate()
				defaultCreated := defaultTuples.TuplesToCreate()
				assert.Equal(t, len(*defaultCreated), len(*unifiedCreated),
					"tuple count must match for %s", workspace)
			}

			if unifiedTuples.HasTuplesToDelete() {
				unifiedDeleted := unifiedTuples.TuplesToDelete()
				defaultDeleted := defaultTuples.TuplesToDelete()
				assert.Equal(t, len(*defaultDeleted), len(*unifiedDeleted),
					"delete count must match for %s", workspace)
			}

			previous = current
		}
	})

	t.Run("empty previous representation (new resource)", func(t *testing.T) {
		ver := model.NewVersion(1)
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-new"}),
			&ver, nil, nil,
		)
		require.NoError(t, err)

		unifiedTuples, err := unifiedSchema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		defaultTuples, err := defaultSchema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		// Both should create, none should delete
		assert.Equal(t, defaultTuples.HasTuplesToCreate(), unifiedTuples.HasTuplesToCreate())
		assert.Equal(t, defaultTuples.HasTuplesToDelete(), unifiedTuples.HasTuplesToDelete())
		assert.True(t, unifiedTuples.HasTuplesToCreate())
		assert.False(t, unifiedTuples.HasTuplesToDelete())
	})

	t.Run("both previous and current nil (edge case)", func(t *testing.T) {
		unifiedTuples, err := unifiedSchema.CalculateTuples(nil, nil, key)
		require.NoError(t, err)

		defaultTuples, err := defaultSchema.CalculateTuples(nil, nil, key)
		require.NoError(t, err)

		// Both should produce no tuples
		assert.Equal(t, defaultTuples.HasTuplesToCreate(), unifiedTuples.HasTuplesToCreate())
		assert.Equal(t, defaultTuples.HasTuplesToDelete(), unifiedTuples.HasTuplesToDelete())
		assert.False(t, unifiedTuples.HasTuplesToCreate())
		assert.False(t, unifiedTuples.HasTuplesToDelete())
	})

	t.Run("delete resource (current nil, previous has data)", func(t *testing.T) {
		ver := model.NewVersion(1)
		previous, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{"workspace_id": "ws-old"}),
			&ver, nil, nil,
		)
		require.NoError(t, err)

		unifiedTuples, err := unifiedSchema.CalculateTuples(nil, previous, key)
		require.NoError(t, err)

		defaultTuples, err := defaultSchema.CalculateTuples(nil, previous, key)
		require.NoError(t, err)

		// Both should delete, none should create
		assert.Equal(t, defaultTuples.HasTuplesToCreate(), unifiedTuples.HasTuplesToCreate())
		assert.Equal(t, defaultTuples.HasTuplesToDelete(), unifiedTuples.HasTuplesToDelete())
		assert.False(t, unifiedTuples.HasTuplesToCreate())
		assert.True(t, unifiedTuples.HasTuplesToDelete())
	})
}

// BenchmarkSchemaComparison_TupleCalculation compares performance between
// DefaultSchema and UnifiedSchemaImpl
func BenchmarkSchemaComparison_TupleCalculation(b *testing.B) {
	// Setup test data
	resourceType, _ := model.NewResourceType("host")
	reporterType, _ := model.NewReporterType("hbi")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)
	current, _ := model.NewRepresentations(
		model.Representation(map[string]interface{}{"workspace_id": "ws-1"}),
		&ver, nil, nil,
	)
	previous, _ := model.NewRepresentations(
		model.Representation(map[string]interface{}{"workspace_id": "ws-2"}),
		&ver, nil, nil,
	)

	b.Run("DefaultSchema", func(b *testing.B) {
		schema := model.NewDefaultSchema()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = schema.CalculateTuples(current, previous, key)
		}
	})

	b.Run("UnifiedSchemaImpl", func(b *testing.B) {
		relations := []model.RelationDefinition{
			{
				Name:        "workspace",
				Target:      "rbac/workspace",
				Field:       "workspace_id",
				Cardinality: "one",
			},
		}
		schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = schema.CalculateTuples(current, previous, key)
		}
	})

	b.Run("UnifiedSchemaImpl_MultipleRelations", func(b *testing.B) {
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
		schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

		currentMulti, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"workspace_id": "ws-1",
				"tenant_id":    "tenant-1",
			}),
			&ver, nil, nil,
		)
		previousMulti, _ := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"workspace_id": "ws-2",
				"tenant_id":    "tenant-2",
			}),
			&ver, nil, nil,
		)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = schema.CalculateTuples(currentMulti, previousMulti, key)
		}
	})
}

// BenchmarkSchemaComparison_Validation compares validation performance
func BenchmarkSchemaComparison_Validation(b *testing.B) {
	testData := map[string]interface{}{
		"workspace_id": "ws-test",
	}

	b.Run("JSONSchema", func(b *testing.B) {
		schema := NewJsonSchemaWithWorkspacesFromString(`{
			"type": "object",
			"properties": {"workspace_id": {"type": "string"}},
			"required": ["workspace_id"]
		}`)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = schema.Validate(testData)
		}
	})

	b.Run("UnifiedSchemaImpl", func(b *testing.B) {
		jsonSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"workspace_id": map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"workspace_id"},
		}
		schema := NewUnifiedSchemaImpl(jsonSchema, []model.RelationDefinition{})
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, _ = schema.Validate(testData)
		}
	})
}
