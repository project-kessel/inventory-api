package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSampleApp_AllFeatures tests the sample_app resource that uses all Phase 4 features
func TestSampleApp_AllFeatures(t *testing.T) {
	// Load the sample_app schema
	schemas, err := LoadUnifiedSchemasFromDirectory("../../data/schema/resources")
	require.NoError(t, err)

	var sampleAppSchema *model.UnifiedSchema
	for _, schema := range schemas {
		if schema.Name == "sample_app" {
			sampleAppSchema = &schema
			break
		}
	}
	require.NotNil(t, sampleAppSchema, "sample_app.yaml should be loaded")

	// Verify schema metadata
	assert.Equal(t, "1.0", sampleAppSchema.SchemaVersion)
	assert.Equal(t, "sample_app", sampleAppSchema.Name)
	assert.Contains(t, sampleAppSchema.Description, "all relation features")

	// Verify common relations
	require.Len(t, sampleAppSchema.Common.Relations, 4, "should have 4 common relations")

	// Check workspace relation (required, one)
	workspaceRel := sampleAppSchema.Common.Relations[0]
	assert.Equal(t, "workspace", workspaceRel.Name)
	assert.Equal(t, "rbac/workspace", workspaceRel.Target)
	assert.Equal(t, "workspace_id", workspaceRel.Field)
	assert.Equal(t, "one", workspaceRel.Cardinality)
	assert.False(t, workspaceRel.Nullable)

	// Check tenant relation (nullable, one)
	tenantRel := sampleAppSchema.Common.Relations[1]
	assert.Equal(t, "tenant", tenantRel.Name)
	assert.Equal(t, "rbac/tenant", tenantRel.Target)
	assert.Equal(t, "tenant_id", tenantRel.Field)
	assert.Equal(t, "one", tenantRel.Cardinality)
	assert.True(t, tenantRel.Nullable, "tenant should be nullable")

	// Check tag relation (many)
	tagRel := sampleAppSchema.Common.Relations[2]
	assert.Equal(t, "tag", tagRel.Name)
	assert.Equal(t, "rbac/tag", tagRel.Target)
	assert.Equal(t, "tag_ids", tagRel.Field)
	assert.Equal(t, "many", tagRel.Cardinality)

	// Check owner relation (many)
	ownerRel := sampleAppSchema.Common.Relations[3]
	assert.Equal(t, "owner", ownerRel.Name)
	assert.Equal(t, "rbac/user", ownerRel.Target)
	assert.Equal(t, "owner_ids", ownerRel.Field)
	assert.Equal(t, "many", ownerRel.Cardinality)

	// Verify reporters
	require.Len(t, sampleAppSchema.Reporters, 3, "should have 3 reporters")

	// Check OCM reporter relations
	ocmReporter := sampleAppSchema.Reporters[0]
	assert.Equal(t, "ocm", ocmReporter.Name)
	require.Len(t, ocmReporter.Relations, 2, "OCM should have 2 relations")

	ocmSubscriptionRel := ocmReporter.Relations[0]
	assert.Equal(t, "subscription", ocmSubscriptionRel.Name)
	assert.Equal(t, "ocm/subscription", ocmSubscriptionRel.Target)
	assert.Equal(t, "one", ocmSubscriptionRel.Cardinality)

	ocmClusterRel := ocmReporter.Relations[1]
	assert.Equal(t, "cluster", ocmClusterRel.Name)
	assert.Equal(t, "ocm/cluster", ocmClusterRel.Target)
	assert.Equal(t, "many", ocmClusterRel.Cardinality)

	// Check HBI reporter relations
	hbiReporter := sampleAppSchema.Reporters[1]
	assert.Equal(t, "hbi", hbiReporter.Name)
	require.Len(t, hbiReporter.Relations, 1, "HBI should have 1 relation")

	hbiHostRel := hbiReporter.Relations[0]
	assert.Equal(t, "host", hbiHostRel.Name)
	assert.Equal(t, "hbi/host", hbiHostRel.Target)
	assert.Equal(t, "many", hbiHostRel.Cardinality)

	// Check ACM reporter (no relations)
	acmReporter := sampleAppSchema.Reporters[2]
	assert.Equal(t, "acm", acmReporter.Name)
	assert.Len(t, acmReporter.Relations, 0, "ACM should have no relations")
}

// TestSampleApp_FullLifecycleWithOCM tests full lifecycle with OCM reporter
func TestSampleApp_FullLifecycleWithOCM(t *testing.T) {
	// Load schema
	schemas, err := LoadUnifiedSchemasFromDirectory("../../data/schema/resources")
	require.NoError(t, err)

	var sampleAppSchema *model.UnifiedSchema
	for _, schema := range schemas {
		if schema.Name == "sample_app" {
			sampleAppSchema = &schema
			break
		}
	}
	require.NotNil(t, sampleAppSchema)

	// Create schema implementation with OCM reporter
	ocmReporter := sampleAppSchema.Reporters[0]
	schema := NewUnifiedSchemaImplWithReporterRelations(
		ocmReporter.Schema,
		sampleAppSchema.Common.Relations,
		"ocm",
		ocmReporter.Relations,
	)

	resourceType, _ := model.NewResourceType("sample_app")
	reporterType, _ := model.NewReporterType("ocm")
	reporterInstanceId, _ := model.NewReporterInstanceId("ocm-prod")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("app-12345"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Step 1: Create application with all features
	t.Run("create with all features", func(t *testing.T) {
		current, err := model.NewRepresentations(
			// Common data
			model.Representation(map[string]interface{}{
				"workspace_id": "ws-engineering",
				"tenant_id":    "tenant-acme", // Nullable - present
				"tag_ids":      []interface{}{"production", "critical", "monitored"},
				"owner_ids":    []interface{}{"user-alice", "user-bob"},
				"app_name":     "payment-service",
				"environment":  "production",
			}),
			&ver,
			// OCM reporter data
			model.Representation(map[string]interface{}{
				"subscription_id":  "sub-premium-001",
				"cluster_ids":      []interface{}{"cluster-east-1", "cluster-west-1", "cluster-central"},
				"deployment_count": 3,
			}),
			&ver,
		)
		require.NoError(t, err)

		tuples, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		// Should create:
		// Common: 1 workspace + 1 tenant + 3 tags + 2 owners = 7 tuples
		// OCM: 1 subscription + 3 clusters = 4 tuples
		// Total: 11 tuples
		assert.True(t, tuples.HasTuplesToCreate())
		created := tuples.TuplesToCreate()
		require.NotNil(t, created)
		assert.Len(t, *created, 11, "should create 11 tuples total")

		// Count by relation
		relationCounts := make(map[string]int)
		for _, tuple := range *created {
			relationCounts[tuple.Relation().String()]++
		}

		assert.Equal(t, 1, relationCounts["workspace"], "1 workspace")
		assert.Equal(t, 1, relationCounts["tenant"], "1 tenant")
		assert.Equal(t, 3, relationCounts["tag"], "3 tags")
		assert.Equal(t, 2, relationCounts["owner"], "2 owners")
		assert.Equal(t, 1, relationCounts["subscription"], "1 subscription")
		assert.Equal(t, 3, relationCounts["cluster"], "3 clusters")
	})

	// Step 2: Update - modify arrays and remove nullable tenant
	t.Run("update arrays and remove tenant", func(t *testing.T) {
		previous, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"workspace_id": "ws-engineering",
				"tenant_id":    "tenant-acme",
				"tag_ids":      []interface{}{"production", "critical", "monitored"},
				"owner_ids":    []interface{}{"user-alice", "user-bob"},
				"app_name":     "payment-service",
				"environment":  "production",
			}),
			&ver,
			model.Representation(map[string]interface{}{
				"subscription_id":  "sub-premium-001",
				"cluster_ids":      []interface{}{"cluster-east-1", "cluster-west-1", "cluster-central"},
				"deployment_count": 3,
			}),
			&ver,
		)
		require.NoError(t, err)

		// Update:
		// - Remove tenant_id (nullable becomes null)
		// - Remove "critical" tag, add "audit" tag
		// - Remove "user-bob" owner, add "user-charlie" and "user-dave"
		// - Remove "cluster-central", add "cluster-south"
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"workspace_id": "ws-engineering",
				// tenant_id removed (nullable!)
				"tag_ids":     []interface{}{"production", "monitored", "audit"},
				"owner_ids":   []interface{}{"user-alice", "user-charlie", "user-dave"},
				"app_name":    "payment-service",
				"environment": "production",
			}),
			&ver,
			model.Representation(map[string]interface{}{
				"subscription_id":  "sub-premium-001",
				"cluster_ids":      []interface{}{"cluster-east-1", "cluster-west-1", "cluster-south"},
				"deployment_count": 3,
			}),
			&ver,
		)
		require.NoError(t, err)

		tuples, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		// Creates:
		// - 1 tag (audit)
		// - 2 owners (charlie, dave)
		// - 1 cluster (south)
		// Total: 4 creates
		created := tuples.TuplesToCreate()
		require.NotNil(t, created)
		assert.Len(t, *created, 4, "should create 4 new tuples")

		// Deletes:
		// - 1 tenant (became null)
		// - 1 tag (critical)
		// - 1 owner (bob)
		// - 1 cluster (central)
		// Total: 4 deletes
		deleted := tuples.TuplesToDelete()
		require.NotNil(t, deleted)
		assert.Len(t, *deleted, 4, "should delete 4 old tuples")

		// Verify deleted tuples
		deletedRelations := make(map[string]int)
		for _, tuple := range *deleted {
			deletedRelations[tuple.Relation().String()]++
		}
		assert.Equal(t, 1, deletedRelations["tenant"], "tenant removed (nullable)")
		assert.Equal(t, 1, deletedRelations["tag"], "critical tag removed")
		assert.Equal(t, 1, deletedRelations["owner"], "bob removed")
		assert.Equal(t, 1, deletedRelations["cluster"], "central cluster removed")
	})

	// Step 3: Change workspace (one cardinality update)
	t.Run("change workspace", func(t *testing.T) {
		previous, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"workspace_id": "ws-engineering",
				"tag_ids":      []interface{}{"production"},
				"owner_ids":    []interface{}{"user-alice"},
				"app_name":     "payment-service",
				"environment":  "production",
			}),
			&ver,
			model.Representation(map[string]interface{}{
				"subscription_id": "sub-premium-001",
				"cluster_ids":     []interface{}{"cluster-east-1"},
			}),
			&ver,
		)
		require.NoError(t, err)

		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"workspace_id": "ws-platform", // Changed!
				"tag_ids":      []interface{}{"production"},
				"owner_ids":    []interface{}{"user-alice"},
				"app_name":     "payment-service",
				"environment":  "production",
			}),
			&ver,
			model.Representation(map[string]interface{}{
				"subscription_id": "sub-premium-001",
				"cluster_ids":     []interface{}{"cluster-east-1"},
			}),
			&ver,
		)
		require.NoError(t, err)

		tuples, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		// Should create 1 tuple for new workspace and delete 1 for old
		created := tuples.TuplesToCreate()
		deleted := tuples.TuplesToDelete()

		require.NotNil(t, created)
		require.NotNil(t, deleted)
		assert.Len(t, *created, 1, "create new workspace tuple")
		assert.Len(t, *deleted, 1, "delete old workspace tuple")

		assert.Equal(t, "workspace", (*created)[0].Relation().String())
		assert.Equal(t, "workspace", (*deleted)[0].Relation().String())
	})

	// Step 4: Empty all arrays
	t.Run("empty all arrays", func(t *testing.T) {
		previous, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"workspace_id": "ws-engineering",
				"tag_ids":      []interface{}{"tag1", "tag2", "tag3"},
				"owner_ids":    []interface{}{"user-alice", "user-bob"},
				"app_name":     "payment-service",
				"environment":  "production",
			}),
			&ver,
			model.Representation(map[string]interface{}{
				"subscription_id": "sub-premium-001",
				"cluster_ids":     []interface{}{"cluster-east-1", "cluster-west-1"},
			}),
			&ver,
		)
		require.NoError(t, err)

		// Empty all arrays
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"workspace_id": "ws-engineering",
				"tag_ids":      []interface{}{},
				"owner_ids":    []interface{}{},
				"app_name":     "payment-service",
				"environment":  "production",
			}),
			&ver,
			model.Representation(map[string]interface{}{
				"subscription_id": "sub-premium-001",
				"cluster_ids":     []interface{}{},
			}),
			&ver,
		)
		require.NoError(t, err)

		tuples, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		// Should delete all array tuples: 3 tags + 2 owners + 2 clusters = 7 deletes
		assert.False(t, tuples.HasTuplesToCreate())
		assert.True(t, tuples.HasTuplesToDelete())

		deleted := tuples.TuplesToDelete()
		require.NotNil(t, deleted)
		assert.Len(t, *deleted, 7, "should delete all array tuples")
	})
}

// TestSampleApp_Validation tests schema validation
func TestSampleApp_Validation(t *testing.T) {
	schemas, err := LoadUnifiedSchemasFromDirectory("../../data/schema/resources")
	require.NoError(t, err)

	var sampleAppSchema *model.UnifiedSchema
	for _, schema := range schemas {
		if schema.Name == "sample_app" {
			sampleAppSchema = &schema
			break
		}
	}
	require.NotNil(t, sampleAppSchema)

	schema := NewUnifiedSchemaImpl(sampleAppSchema.Common.Schema, sampleAppSchema.Common.Relations)

	t.Run("valid data passes", func(t *testing.T) {
		data := map[string]interface{}{
			"workspace_id": "ws-1",
			"app_name":     "test-app",
			"environment":  "development",
			"tag_ids":      []interface{}{"tag1", "tag2"},
			"owner_ids":    []interface{}{"user1"},
		}

		valid, err := schema.Validate(data)
		assert.True(t, valid)
		assert.NoError(t, err)
	})

	t.Run("missing required field fails", func(t *testing.T) {
		data := map[string]interface{}{
			"app_name":    "test-app",
			"environment": "development",
		}

		valid, err := schema.Validate(data)
		assert.False(t, valid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workspace_id")
	})

	t.Run("invalid enum value fails", func(t *testing.T) {
		data := map[string]interface{}{
			"workspace_id": "ws-1",
			"app_name":     "test-app",
			"environment":  "invalid-env",
		}

		valid, err := schema.Validate(data)
		assert.False(t, valid)
		assert.Error(t, err)
	})

	t.Run("nullable field can be omitted", func(t *testing.T) {
		data := map[string]interface{}{
			"workspace_id": "ws-1",
			"app_name":     "test-app",
			"environment":  "production",
			// tenant_id omitted - should be valid
		}

		valid, err := schema.Validate(data)
		assert.True(t, valid)
		assert.NoError(t, err)
	})
}
