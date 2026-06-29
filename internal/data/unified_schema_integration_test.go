package data

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

// TestUnifiedSchema_FullResourceLifecycle tests the complete lifecycle of a resource
// with YAML schema: create → update → delete
func TestUnifiedSchema_FullResourceLifecycle(t *testing.T) {
	_ = context.Background() // For potential future use

	// Load actual YAML schemas from filesystem
	schemas, err := LoadUnifiedSchemasFromDirectory("../../data/schema/resources")
	require.NoError(t, err)
	require.NotEmpty(t, schemas, "should load at least one schema")

	// Find host schema
	var hostSchema *bizmodel.UnifiedSchema
	for i := range schemas {
		if schemas[i].Name == "host" {
			hostSchema = &schemas[i]
			break
		}
	}
	require.NotNil(t, hostSchema, "host schema should exist")

	// Create schema implementation
	schema := NewUnifiedSchemaImpl(hostSchema.Common.Schema, hostSchema.Common.Relations)

	// Create resource key
	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := bizmodel.NewReporterType("hbi")
	require.NoError(t, err)
	reporterInstanceId, err := bizmodel.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	key, err := bizmodel.NewReporterResourceKey(
		bizmodel.LocalResourceId("test-host-1"),
		resourceType,
		reporterType,
		reporterInstanceId,
	)
	require.NoError(t, err)

	// Step 1: Create resource (workspace_id = "ws-1")
	t.Run("create resource", func(t *testing.T) {
		data := map[string]interface{}{
			"workspace_id": "ws-1",
		}

		// Validate
		valid, err := schema.Validate(data)
		assert.NoError(t, err)
		assert.True(t, valid, "valid data should pass validation")

		// Calculate tuples (no previous representation)
		ver := bizmodel.NewVersion(1)
		current, err := bizmodel.NewRepresentations(
			bizmodel.Representation(data),
			&ver,
			nil,
			nil,
		)
		require.NoError(t, err)

		tuples, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		// Verify tuple creation
		assert.True(t, tuples.HasTuplesToCreate(), "should have tuples to create")
		assert.False(t, tuples.HasTuplesToDelete(), "should not have tuples to delete")

		created := tuples.TuplesToCreate()
		require.NotNil(t, created)
		assert.Len(t, *created, 1, "should create one workspace tuple")

		// Verify tuple structure
		tuple := (*created)[0]
		assert.Equal(t, "workspace", tuple.Relation().String())
		// Just verify the relation is correct - detailed tuple structure is tested in comparison tests
		assert.NotNil(t, tuple.Subject())
		assert.NotNil(t, tuple.Object())
	})

	// Step 2: Update resource (workspace_id = "ws-2")
	t.Run("update resource", func(t *testing.T) {
		previousData := map[string]interface{}{
			"workspace_id": "ws-1",
		}
		currentData := map[string]interface{}{
			"workspace_id": "ws-2",
		}

		// Validate new data
		valid, err := schema.Validate(currentData)
		assert.NoError(t, err)
		assert.True(t, valid)

		// Calculate tuples
		ver := bizmodel.NewVersion(2)
		previous, err := bizmodel.NewRepresentations(
			bizmodel.Representation(previousData),
			&ver,
			nil,
			nil,
		)
		require.NoError(t, err)

		current, err := bizmodel.NewRepresentations(
			bizmodel.Representation(currentData),
			&ver,
			nil,
			nil,
		)
		require.NoError(t, err)

		tuples, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		// Verify tuple update (delete old, create new)
		assert.True(t, tuples.HasTuplesToCreate(), "should have tuples to create")
		assert.True(t, tuples.HasTuplesToDelete(), "should have tuples to delete")

		created := tuples.TuplesToCreate()
		deleted := tuples.TuplesToDelete()
		require.NotNil(t, created)
		require.NotNil(t, deleted)
		assert.Len(t, *created, 1, "should create one new workspace tuple")
		assert.Len(t, *deleted, 1, "should delete one old workspace tuple")

		// Verify tuples created and deleted
		newTuple := (*created)[0]
		assert.Equal(t, "workspace", newTuple.Relation().String())

		oldTuple := (*deleted)[0]
		assert.Equal(t, "workspace", oldTuple.Relation().String())
	})

	// Step 3: Delete resource
	t.Run("delete resource", func(t *testing.T) {
		previousData := map[string]interface{}{
			"workspace_id": "ws-2",
		}

		// Calculate tuples for deletion (current = nil)
		ver := bizmodel.NewVersion(3)
		previous, err := bizmodel.NewRepresentations(
			bizmodel.Representation(previousData),
			&ver,
			nil,
			nil,
		)
		require.NoError(t, err)

		tuples, err := schema.CalculateTuples(nil, previous, key)
		require.NoError(t, err)

		// Verify tuple deletion
		assert.False(t, tuples.HasTuplesToCreate(), "should not create tuples")
		assert.True(t, tuples.HasTuplesToDelete(), "should delete tuples")

		deleted := tuples.TuplesToDelete()
		require.NotNil(t, deleted)
		assert.Len(t, *deleted, 1, "should delete one workspace tuple")

		// Verify deleted tuple
		tuple := (*deleted)[0]
		assert.Equal(t, "workspace", tuple.Relation().String())
	})
}

// TestUnifiedSchema_MultipleRelations tests a schema with multiple relations
func TestUnifiedSchema_MultipleRelations(t *testing.T) {
	_ = context.Background() // For potential future use

	// Create a test schema with 2 relations
	relations := []bizmodel.RelationDefinition{
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

	jsonSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"workspace_id": map[string]interface{}{"type": "string"},
			"tenant_id":    map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"workspace_id", "tenant_id"},
	}

	schema := NewUnifiedSchemaImpl(jsonSchema, relations)

	resourceType, err := bizmodel.NewResourceType("test_resource")
	require.NoError(t, err)
	reporterType, err := bizmodel.NewReporterType("test_reporter")
	require.NoError(t, err)
	reporterInstanceId, err := bizmodel.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	key, err := bizmodel.NewReporterResourceKey(
		bizmodel.LocalResourceId("test-1"),
		resourceType,
		reporterType,
		reporterInstanceId,
	)
	require.NoError(t, err)

	t.Run("create with both relations", func(t *testing.T) {
		data := map[string]interface{}{
			"workspace_id": "ws-1",
			"tenant_id":    "tenant-1",
		}

		ver := bizmodel.NewVersion(1)
		current, err := bizmodel.NewRepresentations(
			bizmodel.Representation(data),
			&ver,
			nil,
			nil,
		)
		require.NoError(t, err)

		tuples, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		// Verify 2 tuples created (workspace + tenant)
		assert.True(t, tuples.HasTuplesToCreate())
		created := tuples.TuplesToCreate()
		require.NotNil(t, created)
		assert.Len(t, *created, 2, "should create tuples for both relations")

		// Verify tuple relations
		relations := make(map[string]bool)
		for _, tuple := range *created {
			relations[tuple.Relation().String()] = true
		}
		assert.True(t, relations["workspace"], "should have workspace relation")
		assert.True(t, relations["tenant"], "should have tenant relation")
	})

	t.Run("update one field", func(t *testing.T) {
		previousData := map[string]interface{}{
			"workspace_id": "ws-1",
			"tenant_id":    "tenant-1",
		}
		currentData := map[string]interface{}{
			"workspace_id": "ws-2",     // Changed
			"tenant_id":    "tenant-1", // Unchanged
		}

		ver := bizmodel.NewVersion(2)
		previous, err := bizmodel.NewRepresentations(
			bizmodel.Representation(previousData),
			&ver,
			nil,
			nil,
		)
		require.NoError(t, err)

		current, err := bizmodel.NewRepresentations(
			bizmodel.Representation(currentData),
			&ver,
			nil,
			nil,
		)
		require.NoError(t, err)

		tuples, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		// Verify only workspace tuple changed
		assert.True(t, tuples.HasTuplesToCreate())
		assert.True(t, tuples.HasTuplesToDelete())

		created := tuples.TuplesToCreate()
		deleted := tuples.TuplesToDelete()
		require.NotNil(t, created)
		require.NotNil(t, deleted)
		assert.Len(t, *created, 1, "should create one new workspace tuple")
		assert.Len(t, *deleted, 1, "should delete one old workspace tuple")

		// Verify changed tuples are for workspace relation
		assert.Equal(t, "workspace", (*created)[0].Relation().String())
		assert.Equal(t, "workspace", (*deleted)[0].Relation().String())
	})
}

// TestUnifiedSchema_RepositoryIntegration tests loading YAML schemas into the repository
func TestUnifiedSchema_RepositoryIntegration(t *testing.T) {
	ctx := context.Background()

	// Load YAML schemas from directory
	repo, err := NewInMemorySchemaRepositoryFromUnifiedYAMLDir(ctx, "../../data/schema/resources")
	require.NoError(t, err, "should load YAML schemas from directory")

	// Expected resources and reporters
	expectedResources := map[string][]string{
		"host":                      {"hbi"},
		"k8s_cluster":               {"acm", "acs", "ocm"},
		"k8s_policy":                {"acm"},
		"notifications_integration": {"notifications"},
	}

	for resourceName, expectedReporters := range expectedResources {
		t.Run(resourceName, func(t *testing.T) {
			resourceType, err := bizmodel.NewResourceType(resourceName)
			require.NoError(t, err)

			// Test GetResourceSchema
			resourceSchema, err := repo.GetResourceSchema(ctx, resourceType)
			require.NoError(t, err, "should get resource schema for %s", resourceName)
			assert.Equal(t, resourceType, resourceSchema.ResourceType())
			assert.NotNil(t, resourceSchema.Schema())

			// Verify schema has CalculateTuples method
			testData := map[string]interface{}{"workspace_id": "ws-test"}
			ver := bizmodel.NewVersion(1)
			current, err := bizmodel.NewRepresentations(
				bizmodel.Representation(testData),
				&ver,
				nil,
				nil,
			)
			require.NoError(t, err)

			reporterInstanceId, err := bizmodel.NewReporterInstanceId("test-instance")
			require.NoError(t, err)

			key, err := bizmodel.NewReporterResourceKey(
				bizmodel.LocalResourceId("test-1"),
				resourceType,
				bizmodel.DeserializeReporterType(expectedReporters[0]),
				reporterInstanceId,
			)
			require.NoError(t, err)

			tuples, err := resourceSchema.Schema().CalculateTuples(current, nil, key)
			require.NoError(t, err, "should calculate tuples")
			assert.NotNil(t, tuples)

			// Test GetReporterSchema for each reporter
			for _, reporterName := range expectedReporters {
				t.Run(reporterName, func(t *testing.T) {
					reporterType, err := bizmodel.NewReporterType(reporterName)
					require.NoError(t, err)

					reporterSchema, err := repo.GetReporterSchema(ctx, resourceType, reporterType)
					require.NoError(t, err, "should get reporter schema for %s/%s", resourceName, reporterName)
					assert.Equal(t, resourceType, reporterSchema.ResourceType())
					assert.Equal(t, reporterType, reporterSchema.ReporterType())
					assert.NotNil(t, reporterSchema.Schema())
				})
			}
		})
	}
}

// TestUnifiedSchema_ValidationPasses tests that validation works with YAML schemas
func TestUnifiedSchema_ValidationPasses(t *testing.T) {
	schemas, err := LoadUnifiedSchemasFromDirectory("../../data/schema/resources")
	require.NoError(t, err)

	// Find host schema
	var hostSchema *bizmodel.UnifiedSchema
	for i := range schemas {
		if schemas[i].Name == "host" {
			hostSchema = &schemas[i]
			break
		}
	}
	require.NotNil(t, hostSchema)

	schema := NewUnifiedSchemaImpl(hostSchema.Common.Schema, hostSchema.Common.Relations)

	testCases := []struct {
		name       string
		data       map[string]interface{}
		shouldPass bool
	}{
		{
			name:       "valid data",
			data:       map[string]interface{}{"workspace_id": "ws-1"},
			shouldPass: true,
		},
		{
			name:       "missing required field",
			data:       map[string]interface{}{},
			shouldPass: false,
		},
		{
			name:       "wrong type",
			data:       map[string]interface{}{"workspace_id": 123},
			shouldPass: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			valid, err := schema.Validate(tc.data)

			if tc.shouldPass {
				assert.NoError(t, err, "valid data should not produce error")
				assert.True(t, valid, "valid data should pass validation")
			} else {
				// Validation should return false (and typically an error with details)
				assert.False(t, valid, "invalid data should fail validation")
			}
		})
	}
}
