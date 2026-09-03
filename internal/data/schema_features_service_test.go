package data

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var billingAccountJsonSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"type": "object",
	"properties": {
		"services": { "type": "array", "items": { "type": "string" } }
	},
	"required": []
}`

var workspaceJsonSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"type": "object",
	"properties": {
		"direct_billing_account": { "type": "string" },
		"direct_service_preferences": { "type": "array", "items": { "type": "string" } }
	},
	"required": []
}`

func TestFeaturesWorkspaceSchema_Validate(t *testing.T) {
	schema := NewFeaturesWorkspaceSchemaFromString(workspaceJsonSchema)

	t.Run("valid data passes", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{
			"direct_billing_account":     "ba-1",
			"direct_service_preferences": []interface{}{"svc-1", "svc-2"},
		})
		assert.True(t, valid)
		assert.NoError(t, err)
	})

	t.Run("empty object passes with no required fields", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{})
		assert.True(t, valid)
		assert.NoError(t, err)
	})

	t.Run("wrong type for direct_billing_account fails", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{
			"direct_billing_account": []interface{}{"not-a-string"},
		})
		assert.False(t, valid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("wrong type for direct_service_preferences fails", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{
			"direct_service_preferences": "not-an-array",
		})
		assert.False(t, valid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

func TestFeaturesBillingAccountSchema_Validate(t *testing.T) {
	schema := NewFeaturesBillingAccountSchemaFromString(billingAccountJsonSchema)

	t.Run("valid data passes", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{
			"services": []interface{}{"svc-1", "svc-2"},
		})
		assert.True(t, valid)
		assert.NoError(t, err)
	})

	t.Run("empty object passes with no required fields", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{})
		assert.True(t, valid)
		assert.NoError(t, err)
	})

	t.Run("wrong type fails", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{
			"services": "not-an-array",
		})
		assert.False(t, valid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

func featuresWorkspaceKey(t *testing.T) model.ReporterResourceKey {
	t.Helper()
	resourceType, err := model.NewResourceType("workspace")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("features")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("features-instance")
	require.NoError(t, err)
	localResourceId, err := model.NewLocalResourceId("ws-001")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		localResourceId,
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)
	return key
}

func featuresBillingAccountKey(t *testing.T) model.ReporterResourceKey {
	t.Helper()
	resourceType, err := model.NewResourceType("billing_account")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("features")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("features-instance")
	require.NoError(t, err)
	localResourceId, err := model.NewLocalResourceId("ba-001")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		localResourceId,
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)
	return key
}

func TestFeaturesWorkspaceSchema_CalculateTuples(t *testing.T) {
	schema := NewFeaturesWorkspaceSchemaFromString(workspaceJsonSchema)
	key := featuresWorkspaceKey(t)

	t.Run("create produces tuples for all relations", func(t *testing.T) {
		ver := model.NewVersion(0)
		current, err := model.NewRepresentations(
			nil, nil,
			model.Representation(map[string]interface{}{
				"direct_billing_account":     "ba-100",
				"direct_service_preferences": []interface{}{"svc-1", "svc-2"},
			}),
			&ver,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		assert.True(t, result.HasTuplesToCreate())
		assert.False(t, result.HasTuplesToDelete())

		expected := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "direct_billing_account", "features", "billing_account", "ba-100"),
			model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-1"),
			model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-2"),
		}
		assert.ElementsMatch(t, expected, *result.TuplesToCreate())
	})

	t.Run("update creates and deletes changed values", func(t *testing.T) {
		ver1 := model.NewVersion(1)
		previous, err := model.NewRepresentations(
			nil, nil,
			model.Representation(map[string]interface{}{
				"direct_billing_account":     "ba-100",
				"direct_service_preferences": []interface{}{"svc-1", "svc-2"},
			}),
			&ver1,
		)
		require.NoError(t, err)

		ver2 := model.NewVersion(2)
		current, err := model.NewRepresentations(
			nil, nil,
			model.Representation(map[string]interface{}{
				"direct_billing_account":     "ba-200",
				"direct_service_preferences": []interface{}{"svc-2", "svc-3"},
			}),
			&ver2,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		assert.True(t, result.HasTuplesToCreate())
		assert.True(t, result.HasTuplesToDelete())

		expectedCreates := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "direct_billing_account", "features", "billing_account", "ba-200"),
			model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-3"),
		}
		assert.ElementsMatch(t, expectedCreates, *result.TuplesToCreate())

		expectedDeletes := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "direct_billing_account", "features", "billing_account", "ba-100"),
			model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-1"),
		}
		assert.ElementsMatch(t, expectedDeletes, *result.TuplesToDelete())
	})

	t.Run("delete produces only deletes", func(t *testing.T) {
		ver := model.NewVersion(1)
		previous, err := model.NewRepresentations(
			nil, nil,
			model.Representation(map[string]interface{}{
				"direct_billing_account":     "ba-100",
				"direct_service_preferences": []interface{}{"svc-1"},
			}),
			&ver,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(nil, previous, key)
		require.NoError(t, err)

		assert.False(t, result.HasTuplesToCreate())
		assert.True(t, result.HasTuplesToDelete())

		deletes := *result.TuplesToDelete()
		assert.Len(t, deletes, 2) // 1 direct_billing_account + 1 direct_service_preferences
	})

	t.Run("same data produces no tuples", func(t *testing.T) {
		sameData := map[string]interface{}{
			"direct_billing_account":     "ba-100",
			"direct_service_preferences": []interface{}{"svc-1"},
		}

		ver1 := model.NewVersion(1)
		previous, err := model.NewRepresentations(
			nil, nil, model.Representation(sameData), &ver1,
		)
		require.NoError(t, err)

		ver2 := model.NewVersion(2)
		current, err := model.NewRepresentations(
			nil, nil, model.Representation(sameData), &ver2,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		assert.False(t, result.HasTuplesToCreate())
		assert.False(t, result.HasTuplesToDelete())
	})

	t.Run("handles nil direct_billing_account", func(t *testing.T) {
		ver := model.NewVersion(0)
		current, err := model.NewRepresentations(
			nil, nil,
			model.Representation(map[string]interface{}{
				"direct_service_preferences": []interface{}{"svc-1"},
			}),
			&ver,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		assert.True(t, result.HasTuplesToCreate())
		creates := *result.TuplesToCreate()
		assert.Len(t, creates, 1) // Only direct_service_preferences tuple
		assert.Equal(t, model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-1"), creates[0])
	})

	t.Run("handles empty direct_service_preferences", func(t *testing.T) {
		ver := model.NewVersion(0)
		current, err := model.NewRepresentations(
			nil, nil,
			model.Representation(map[string]interface{}{
				"direct_billing_account": "ba-100",
			}),
			&ver,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		assert.True(t, result.HasTuplesToCreate())
		creates := *result.TuplesToCreate()
		assert.Len(t, creates, 1) // Only direct_billing_account tuple
		assert.Equal(t, model.NewRelationTupleForSubject(key, "direct_billing_account", "features", "billing_account", "ba-100"), creates[0])
	})
}

func TestFeaturesBillingAccountSchema_CalculateTuples(t *testing.T) {
	schema := NewFeaturesBillingAccountSchemaFromString(billingAccountJsonSchema)
	key := featuresBillingAccountKey(t)

	t.Run("create produces tuples for services relation", func(t *testing.T) {
		ver := model.NewVersion(0)
		current, err := model.NewRepresentations(
			nil, nil,
			model.Representation(map[string]interface{}{
				"services": []interface{}{"svc-1", "svc-2"},
			}),
			&ver,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		assert.True(t, result.HasTuplesToCreate())
		assert.False(t, result.HasTuplesToDelete())

		expected := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "services", "features", "service", "svc-1"),
			model.NewRelationTupleForSubject(key, "services", "features", "service", "svc-2"),
		}
		assert.ElementsMatch(t, expected, *result.TuplesToCreate())
	})

	t.Run("update creates and deletes changed services", func(t *testing.T) {
		ver1 := model.NewVersion(1)
		previous, err := model.NewRepresentations(
			nil, nil,
			model.Representation(map[string]interface{}{
				"services": []interface{}{"svc-1", "svc-2"},
			}),
			&ver1,
		)
		require.NoError(t, err)

		ver2 := model.NewVersion(2)
		current, err := model.NewRepresentations(
			nil, nil,
			model.Representation(map[string]interface{}{
				"services": []interface{}{"svc-2", "svc-3"},
			}),
			&ver2,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		assert.True(t, result.HasTuplesToCreate())
		assert.True(t, result.HasTuplesToDelete())

		expectedCreates := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "services", "features", "service", "svc-3"),
		}
		assert.ElementsMatch(t, expectedCreates, *result.TuplesToCreate())

		expectedDeletes := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "services", "features", "service", "svc-1"),
		}
		assert.ElementsMatch(t, expectedDeletes, *result.TuplesToDelete())
	})
}

func TestFeaturesAwareSchemaFactory_FallsBackForOtherTypes(t *testing.T) {
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)

	schema := FeaturesAwareSchemaFactory(resourceType, false, `{"type": "object"}`)

	reporterType, err := model.NewReporterType("HBI")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	localResourceId, err := model.NewLocalResourceId("test-host")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		localResourceId,
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)

	ver := model.NewVersion(0)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-host",
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate())
	creates := *result.TuplesToCreate()
	require.Len(t, creates, 1)
	assert.Equal(t, model.NewWorkspaceRelationsTuple("ws-host", key), creates[0])
}

// TestSchemaService_CalculateTuplesForResource_FeaturesWorkspace verifies that
// SchemaService correctly uses reporter schema for Features workspace resources
// and reads from reporter representation data.
func TestSchemaService_CalculateTuplesForResource_FeaturesWorkspace(t *testing.T) {
	ctx := context.Background()

	// Create schema repository with Features workspace schema
	repo := NewInMemorySchemaRepository()

	resourceType, _ := model.NewResourceType("workspace")
	reporterType, _ := model.NewReporterType("features")

	// Create resource schema first (empty common representation)
	emptyCommonSchema := `{"type": "object", "properties": {}, "required": []}`
	resourceSchema := NewJsonSchemaWithRelations(emptyCommonSchema, nil)
	resourceSchemaRepr, err := model.NewResourceSchemaRepresentation(
		resourceType, resourceSchema,
	)
	require.NoError(t, err)
	err = repo.CreateResourceSchema(ctx, resourceSchemaRepr)
	require.NoError(t, err)

	// Register reporter schema (this should be used for tuple calculation)
	reporterSchema := NewFeaturesWorkspaceSchemaFromString(workspaceJsonSchema)
	reporterSchemaRepr, err := model.NewReporterSchemaRepresentation(
		resourceType, reporterType, reporterSchema,
	)
	require.NoError(t, err)
	err = repo.CreateReporterSchema(ctx, reporterSchemaRepr)
	require.NoError(t, err)

	// Create schema service
	logger := log.NewHelper(log.DefaultLogger)
	schemaService := model.NewSchemaService(repo, logger)

	// Create resource key
	key := featuresWorkspaceKey(t)

	// Create representations with data in REPORTER representation (not common)
	ver := model.NewVersion(1)
	current, err := model.NewRepresentations(
		nil, nil, // Empty common representation
		model.Representation(map[string]interface{}{
			"direct_billing_account":     "ba-100",
			"direct_service_preferences": []interface{}{"svc-1", "svc-2"},
		}),
		&ver, // Reporter representation
	)
	require.NoError(t, err)

	// Calculate tuples using SchemaService
	result, err := schemaService.CalculateTuplesForResource(ctx, current, nil, key)
	require.NoError(t, err)

	// Verify tuples were created from reporter data
	assert.True(t, result.HasTuplesToCreate())
	creates := *result.TuplesToCreate()
	assert.Len(t, creates, 3) // 1 billing_account + 2 service_preferences

	expected := []model.RelationsTuple{
		model.NewRelationTupleForSubject(key, "direct_billing_account", "features", "billing_account", "ba-100"),
		model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-1"),
		model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-2"),
	}
	assert.ElementsMatch(t, expected, creates)
}

// TestFeaturesSchemas_FromDirectory loads schemas from data/schema directory
// and verifies tuple calculation works with real schema files.
func TestFeaturesSchemas_FromDirectory(t *testing.T) {
	ctx := context.Background()

	// Load schemas from actual directory
	repo, err := NewInMemorySchemaRepositoryFromDir(ctx, "../../data/schema/resources", FeaturesAwareSchemaFactory)
	require.NoError(t, err)

	logger := log.NewHelper(log.DefaultLogger)
	schemaService := model.NewSchemaService(repo, logger)

	t.Run("workspace with reporter data", func(t *testing.T) {
		key := featuresWorkspaceKey(t)

		ver := model.NewVersion(1)
		current, err := model.NewRepresentations(
			nil, nil,
			model.Representation(map[string]interface{}{
				"direct_billing_account":     "ba-100",
				"direct_service_preferences": []interface{}{"svc-1", "svc-2"},
			}),
			&ver,
		)
		require.NoError(t, err)

		result, err := schemaService.CalculateTuplesForResource(ctx, current, nil, key)
		require.NoError(t, err)

		assert.True(t, result.HasTuplesToCreate())
		creates := *result.TuplesToCreate()
		assert.Len(t, creates, 3)

		expected := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "direct_billing_account", "features", "billing_account", "ba-100"),
			model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-1"),
			model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-2"),
		}
		assert.ElementsMatch(t, expected, creates)
	})

	t.Run("billing_account with reporter data", func(t *testing.T) {
		key := featuresBillingAccountKey(t)

		ver := model.NewVersion(1)
		current, err := model.NewRepresentations(
			nil, nil,
			model.Representation(map[string]interface{}{
				"services": []interface{}{"svc-1", "svc-2"},
			}),
			&ver,
		)
		require.NoError(t, err)

		result, err := schemaService.CalculateTuplesForResource(ctx, current, nil, key)
		require.NoError(t, err)

		assert.True(t, result.HasTuplesToCreate())
		creates := *result.TuplesToCreate()
		assert.Len(t, creates, 2)

		expected := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "services", "features", "service", "svc-1"),
			model.NewRelationTupleForSubject(key, "services", "features", "service", "svc-2"),
		}
		assert.ElementsMatch(t, expected, creates)
	})
}

// TestFeaturesSchemas_MergeBehavior verifies that tuple calculation
// merges fields from both common and reporter representations.
func TestFeaturesSchemas_MergeBehavior(t *testing.T) {
	schema := NewFeaturesWorkspaceSchemaFromString(workspaceJsonSchema)
	key := featuresWorkspaceKey(t)

	t.Run("uses only common data when reporter is empty", func(t *testing.T) {
		ver := model.NewVersion(1)
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"direct_billing_account": "ba-from-common",
			}),
			&ver,
			nil, nil, // No reporter data
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		creates := *result.TuplesToCreate()
		require.Len(t, creates, 1)
		assert.Equal(t, "ba-from-common", creates[0].Subject().Resource().ResourceId().String())
	})

	t.Run("uses only reporter data when common is empty", func(t *testing.T) {
		ver := model.NewVersion(1)
		current, err := model.NewRepresentations(
			nil, nil, // No common data
			model.Representation(map[string]interface{}{
				"direct_billing_account": "ba-from-reporter",
			}),
			&ver,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		creates := *result.TuplesToCreate()
		require.Len(t, creates, 1)
		assert.Equal(t, "ba-from-reporter", creates[0].Subject().Resource().ResourceId().String())
	})

	t.Run("merges different fields from both representations", func(t *testing.T) {
		ver := model.NewVersion(1)
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"direct_billing_account": "ba-from-common",
			}),
			&ver,
			model.Representation(map[string]interface{}{
				"direct_service_preferences": []interface{}{"svc-1", "svc-2"},
			}),
			&ver,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		creates := *result.TuplesToCreate()
		require.Len(t, creates, 3)

		// billing_account from common, service_preferences from reporter
		expected := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "direct_billing_account", "features", "billing_account", "ba-from-common"),
			model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-1"),
			model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-2"),
		}
		assert.ElementsMatch(t, expected, creates)
	})

}

// TestSchemaService_MergesReporterAndCommonSchemas verifies that
// CalculateTuplesForResource merges tuples from both reporter schema and common schema.
func TestSchemaService_MergesReporterAndCommonSchemas(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemorySchemaRepository()

	resourceType, _ := model.NewResourceType("workspace")
	reporterType, _ := model.NewReporterType("features")

	// Create common/resource schema with workspace_id relation
	workspaceIdRelations := []model.RelationDef{
		mustRelationDef("workspace_id", "workspace", "rbac", "workspace", false),
	}
	commonSchema := NewJsonSchemaWithRelations(`{"type": "object"}`, workspaceIdRelations)
	resourceSchemaRepr, err := model.NewResourceSchemaRepresentation(resourceType, commonSchema)
	require.NoError(t, err)
	err = repo.CreateResourceSchema(ctx, resourceSchemaRepr)
	require.NoError(t, err)

	// Create reporter schema with Features-specific relations
	reporterSchema := NewFeaturesWorkspaceSchemaFromString(workspaceJsonSchema)
	reporterSchemaRepr, err := model.NewReporterSchemaRepresentation(resourceType, reporterType, reporterSchema)
	require.NoError(t, err)
	err = repo.CreateReporterSchema(ctx, reporterSchemaRepr)
	require.NoError(t, err)

	// Create schema service
	logger := log.NewHelper(log.DefaultLogger)
	schemaService := model.NewSchemaService(repo, logger)

	key := featuresWorkspaceKey(t)

	// Create representations with data in BOTH common and reporter
	ver := model.NewVersion(1)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-123", // Common schema field
		}),
		&ver,
		model.Representation(map[string]interface{}{
			"direct_billing_account":     "ba-100",               // Reporter schema field
			"direct_service_preferences": []interface{}{"svc-1"}, // Reporter schema field
		}),
		&ver,
	)
	require.NoError(t, err)

	// Calculate tuples - should get tuples from BOTH schemas
	result, err := schemaService.CalculateTuplesForResource(ctx, current, nil, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate())
	creates := *result.TuplesToCreate()
	assert.Len(t, creates, 3) // 1 from common schema + 2 from reporter schema

	expected := []model.RelationsTuple{
		// From common schema
		model.NewRelationTupleForSubject(key, "workspace", "rbac", "workspace", "ws-123"),
		// From reporter schema
		model.NewRelationTupleForSubject(key, "direct_billing_account", "features", "billing_account", "ba-100"),
		model.NewRelationTupleForSubject(key, "direct_service_preferences", "features", "service", "svc-1"),
	}
	assert.ElementsMatch(t, expected, creates)
}
