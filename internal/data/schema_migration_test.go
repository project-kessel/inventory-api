package data

import (
	"context"
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchemaMigration_CoexistingSchemas verifies that JSON and YAML schemas
// can coexist in the same repository
func TestSchemaMigration_CoexistingSchemas(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemorySchemaRepository()

	// Add host with YAML schema
	t.Run("add host with YAML schema", func(t *testing.T) {
		// Load YAML schema
		yamlSchemas, err := LoadUnifiedSchemasFromDirectory("../../data/schema/resources")
		require.NoError(t, err)

		var hostYAML *model.UnifiedSchema
		for i := range yamlSchemas {
			if yamlSchemas[i].Name == "host" {
				hostYAML = &yamlSchemas[i]
				break
			}
		}
		require.NotNil(t, hostYAML)

		// Create schema implementation
		yamlSchema := NewUnifiedSchemaImpl(hostYAML.Common.Schema, hostYAML.Common.Relations)

		// Add to repository
		resourceType, err := model.NewResourceType("host")
		require.NoError(t, err)

		resourceSchema, err := model.NewResourceSchemaRepresentation(resourceType, yamlSchema)
		require.NoError(t, err)

		err = repo.CreateResourceSchema(ctx, resourceSchema)
		assert.NoError(t, err)

		// Add reporter schema
		reporterType, err := model.NewReporterType("hbi")
		require.NoError(t, err)

		reporterSchema, err := model.NewReporterSchemaRepresentation(
			resourceType,
			reporterType,
			yamlSchema,
		)
		require.NoError(t, err)

		err = repo.CreateReporterSchema(ctx, reporterSchema)
		assert.NoError(t, err)
	})

	// Add k8s_cluster with JSON schema (old way)
	t.Run("add k8s_cluster with JSON schema", func(t *testing.T) {
		jsonSchemaStr := `{
			"type": "object",
			"properties": {
				"workspace_id": {"type": "string"},
				"external_cluster_id": {"type": "string"}
			},
			"required": ["workspace_id"]
		}`
		jsonSchema := NewJsonSchemaWithWorkspacesFromString(jsonSchemaStr)

		resourceType, err := model.NewResourceType("k8s_cluster")
		require.NoError(t, err)

		resourceSchema, err := model.NewResourceSchemaRepresentation(resourceType, jsonSchema)
		require.NoError(t, err)

		err = repo.CreateResourceSchema(ctx, resourceSchema)
		assert.NoError(t, err)

		// Add reporter schema
		reporterType, err := model.NewReporterType("acm")
		require.NoError(t, err)

		reporterSchema, err := model.NewReporterSchemaRepresentation(
			resourceType,
			reporterType,
			jsonSchema,
		)
		require.NoError(t, err)

		err = repo.CreateReporterSchema(ctx, reporterSchema)
		assert.NoError(t, err)
	})

	// Verify both work side-by-side
	t.Run("both schemas work correctly", func(t *testing.T) {
		testData := map[string]interface{}{"workspace_id": "ws-test"}

		// Test host (YAML schema)
		hostType, err := model.NewResourceType("host")
		require.NoError(t, err)

		hostSchema, err := repo.GetResourceSchema(ctx, hostType)
		require.NoError(t, err, "should get host schema")
		assert.Equal(t, hostType, hostSchema.ResourceType())

		hostValid, err := hostSchema.Schema().Validate(testData)
		assert.NoError(t, err)
		assert.True(t, hostValid, "host validation should pass")

		// Test k8s_cluster (JSON schema)
		k8sType, err := model.NewResourceType("k8s_cluster")
		require.NoError(t, err)

		k8sSchema, err := repo.GetResourceSchema(ctx, k8sType)
		require.NoError(t, err, "should get k8s_cluster schema")
		assert.Equal(t, k8sType, k8sSchema.ResourceType())

		k8sValid, err := k8sSchema.Schema().Validate(testData)
		assert.NoError(t, err)
		assert.True(t, k8sValid, "k8s_cluster validation should pass")
	})

	// Verify tuple generation works for both
	t.Run("tuple generation works for both", func(t *testing.T) {
		// Setup common data
		testData := map[string]interface{}{"workspace_id": "ws-tuples"}
		ver := model.NewVersion(1)
		current, err := model.NewRepresentations(
			model.Representation(testData),
			&ver, nil, nil,
		)
		require.NoError(t, err)

		// Test host tuple generation (YAML)
		hostType, _ := model.NewResourceType("host")
		hostReporter, _ := model.NewReporterType("hbi")
		hostReporterInstanceId, _ := model.NewReporterInstanceId("test-hbi")
		hostKey, _ := model.NewReporterResourceKey(
			model.LocalResourceId("test-host"),
			hostType, hostReporter, hostReporterInstanceId,
		)

		hostSchema, err := repo.GetResourceSchema(ctx, hostType)
		require.NoError(t, err)

		hostTuples, err := hostSchema.Schema().CalculateTuples(current, nil, hostKey)
		assert.NoError(t, err)
		assert.True(t, hostTuples.HasTuplesToCreate(), "host should create tuples")

		// Test k8s_cluster tuple generation (JSON)
		k8sType, _ := model.NewResourceType("k8s_cluster")
		k8sReporter, _ := model.NewReporterType("acm")
		k8sReporterInstanceId, _ := model.NewReporterInstanceId("test-acm")
		k8sKey, _ := model.NewReporterResourceKey(
			model.LocalResourceId("test-cluster"),
			k8sType, k8sReporter, k8sReporterInstanceId,
		)

		k8sSchema, err := repo.GetResourceSchema(ctx, k8sType)
		require.NoError(t, err)

		k8sTuples, err := k8sSchema.Schema().CalculateTuples(current, nil, k8sKey)
		assert.NoError(t, err)
		assert.True(t, k8sTuples.HasTuplesToCreate(), "k8s_cluster should create tuples")
	})
}

// TestSchemaMigration_IncrementalMigration tests migrating resources one at a time
func TestSchemaMigration_IncrementalMigration(t *testing.T) {
	ctx := context.Background()

	// Start with all JSON schemas
	jsonRepo := NewInMemorySchemaRepository()

	resources := []struct {
		name     string
		reporter string
	}{
		{"host", "hbi"},
		{"k8s_cluster", "acm"},
		{"k8s_policy", "acm"},
	}

	jsonSchemaStr := `{
		"type": "object",
		"properties": {"workspace_id": {"type": "string"}},
		"required": ["workspace_id"]
	}`

	for _, res := range resources {
		resourceType, _ := model.NewResourceType(res.name)
		reporterType, _ := model.NewReporterType(res.reporter)

		jsonSchema := NewJsonSchemaWithWorkspacesFromString(jsonSchemaStr)

		resourceSchema, _ := model.NewResourceSchemaRepresentation(resourceType, jsonSchema)
		_ = jsonRepo.CreateResourceSchema(ctx, resourceSchema)

		reporterSchema, _ := model.NewReporterSchemaRepresentation(resourceType, reporterType, jsonSchema)
		_ = jsonRepo.CreateReporterSchema(ctx, reporterSchema)
	}

	// Verify all JSON schemas work
	t.Run("all JSON schemas work", func(t *testing.T) {
		for _, res := range resources {
			resourceType, _ := model.NewResourceType(res.name)
			schema, err := jsonRepo.GetResourceSchema(ctx, resourceType)
			assert.NoError(t, err, "should get %s schema", res.name)
			assert.NotNil(t, schema)
		}
	})

	// Migrate host to YAML
	t.Run("migrate host to YAML", func(t *testing.T) {
		// Load YAML schemas
		yamlSchemas, err := LoadUnifiedSchemasFromDirectory("../../data/schema/resources")
		require.NoError(t, err)

		var hostYAML *model.UnifiedSchema
		for i := range yamlSchemas {
			if yamlSchemas[i].Name == "host" {
				hostYAML = &yamlSchemas[i]
				break
			}
		}
		require.NotNil(t, hostYAML)

		// Create new repository with mixed schemas
		mixedRepo := NewInMemorySchemaRepository()

		// Add host with YAML
		yamlSchema := NewUnifiedSchemaImpl(hostYAML.Common.Schema, hostYAML.Common.Relations)
		hostType, _ := model.NewResourceType("host")
		resourceSchema, _ := model.NewResourceSchemaRepresentation(hostType, yamlSchema)
		err = mixedRepo.CreateResourceSchema(ctx, resourceSchema)
		assert.NoError(t, err)

		hbiType, _ := model.NewReporterType("hbi")
		reporterSchema, _ := model.NewReporterSchemaRepresentation(hostType, hbiType, yamlSchema)
		err = mixedRepo.CreateReporterSchema(ctx, reporterSchema)
		assert.NoError(t, err)

		// Add others with JSON
		jsonSchema := NewJsonSchemaWithWorkspacesFromString(jsonSchemaStr)

		for _, res := range resources[1:] { // Skip host
			resourceType, _ := model.NewResourceType(res.name)
			reporterType, _ := model.NewReporterType(res.reporter)

			resSchema, _ := model.NewResourceSchemaRepresentation(resourceType, jsonSchema)
			_ = mixedRepo.CreateResourceSchema(ctx, resSchema)

			repSchema, _ := model.NewReporterSchemaRepresentation(resourceType, reporterType, jsonSchema)
			_ = mixedRepo.CreateReporterSchema(ctx, repSchema)
		}

		// Verify all still work
		for _, res := range resources {
			resourceType, _ := model.NewResourceType(res.name)
			schema, err := mixedRepo.GetResourceSchema(ctx, resourceType)
			assert.NoError(t, err, "should get %s schema after migration", res.name)
			assert.NotNil(t, schema)

			// Verify validation still works
			testData := map[string]interface{}{"workspace_id": "ws-test"}
			valid, err := schema.Schema().Validate(testData)
			assert.NoError(t, err)
			assert.True(t, valid, "%s validation should pass after migration", res.name)
		}
	})
}

// TestSchemaMigration_BehaviorEquivalence verifies that migrating to YAML
// doesn't change observable behavior
func TestSchemaMigration_BehaviorEquivalence(t *testing.T) {
	ctx := context.Background()

	// Setup test data
	testData := map[string]interface{}{"workspace_id": "ws-migration-test"}
	ver := model.NewVersion(1)
	current, err := model.NewRepresentations(
		model.Representation(testData),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	resourceType, _ := model.NewResourceType("host")
	reporterType, _ := model.NewReporterType("hbi")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
	)

	// Create JSON schema repository
	jsonRepo := NewInMemorySchemaRepository()
	jsonSchemaStr := `{
		"type": "object",
		"properties": {"workspace_id": {"type": "string"}},
		"required": ["workspace_id"]
	}`
	jsonSchema := NewJsonSchemaWithWorkspacesFromString(jsonSchemaStr)
	resourceSchema, _ := model.NewResourceSchemaRepresentation(resourceType, jsonSchema)
	_ = jsonRepo.CreateResourceSchema(ctx, resourceSchema)

	// Create YAML schema repository
	yamlRepo, err := NewInMemorySchemaRepositoryFromUnifiedYAMLDir(ctx, "../../data/schema/resources")
	require.NoError(t, err)

	// Get schemas
	jsonResourceSchema, err := jsonRepo.GetResourceSchema(ctx, resourceType)
	require.NoError(t, err)

	yamlResourceSchema, err := yamlRepo.GetResourceSchema(ctx, resourceType)
	require.NoError(t, err)

	t.Run("validation produces same results", func(t *testing.T) {
		jsonValid, jsonErr := jsonResourceSchema.Schema().Validate(testData)
		yamlValid, yamlErr := yamlResourceSchema.Schema().Validate(testData)

		assert.Equal(t, jsonValid, yamlValid, "validation results must match")
		assert.Equal(t, jsonErr != nil, yamlErr != nil, "error presence must match")
	})

	t.Run("tuple generation produces same results", func(t *testing.T) {
		jsonTuples, jsonErr := jsonResourceSchema.Schema().CalculateTuples(current, nil, key)
		require.NoError(t, jsonErr)

		yamlTuples, yamlErr := yamlResourceSchema.Schema().CalculateTuples(current, nil, key)
		require.NoError(t, yamlErr)

		// Same number of tuples to create
		assert.Equal(t, jsonTuples.HasTuplesToCreate(), yamlTuples.HasTuplesToCreate())
		assert.Equal(t, jsonTuples.HasTuplesToDelete(), yamlTuples.HasTuplesToDelete())

		if jsonTuples.HasTuplesToCreate() {
			jsonCreated := jsonTuples.TuplesToCreate()
			yamlCreated := yamlTuples.TuplesToCreate()
			assert.Equal(t, len(*jsonCreated), len(*yamlCreated), "tuple count must match")

			// Same relation
			assert.Equal(t, (*jsonCreated)[0].Relation(), (*yamlCreated)[0].Relation())
		}
	})
}
