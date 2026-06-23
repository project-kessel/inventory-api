package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReporterRelation_ExtractsFromReporterData verifies that reporter relations extract from reporter data
func TestReporterRelation_ExtractsFromReporterData(t *testing.T) {
	// Define common relations
	commonRelations := []model.RelationDefinition{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "workspace_id",
			Cardinality: "one",
		},
	}

	// Define reporter-specific relations
	reporterRelations := []model.RelationDefinition{
		{
			Name:        "subscription",
			Target:      "ocm/subscription",
			Field:       "subscription_id",
			Cardinality: "one",
		},
	}

	// Create schema with reporter relations
	schema := NewUnifiedSchemaImplWithReporterRelations(
		map[string]interface{}{},
		commonRelations,
		"ocm", // Reporter type
		reporterRelations,
	)

	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("ocm")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("test-cluster"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Create resource with both common and reporter data
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-1",
		}),
		&ver,
		model.Representation(map[string]interface{}{
			"subscription_id": "sub-123",
		}),
		&ver,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	// Should create both workspace tuple (from common) and subscription tuple (from reporter)
	assert.True(t, tuples.HasTuplesToCreate())
	created := tuples.TuplesToCreate()
	require.NotNil(t, created)
	assert.Len(t, *created, 2, "should create workspace and subscription tuples")

	// Verify both relations
	relationNames := make(map[string]bool)
	for _, tuple := range *created {
		relationNames[tuple.Relation().String()] = true
	}
	assert.True(t, relationNames["workspace"], "should have workspace relation from common")
	assert.True(t, relationNames["subscription"], "should have subscription relation from reporter")
}

// TestReporterRelation_IndependentFromCommon verifies reporter and common relations are independent
func TestReporterRelation_IndependentFromCommon(t *testing.T) {
	commonRelations := []model.RelationDefinition{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "field_a",
			Cardinality: "one",
		},
	}

	reporterRelations := []model.RelationDefinition{
		{
			Name:        "cluster",
			Target:      "ocm/cluster",
			Field:       "field_a", // Same field name, but different source
			Cardinality: "one",
		},
	}

	schema := NewUnifiedSchemaImplWithReporterRelations(
		map[string]interface{}{},
		commonRelations,
		"ocm",
		reporterRelations,
	)

	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("ocm")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("test-cluster"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Create with same field name in both common and reporter
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"field_a": "value-from-common",
		}),
		&ver,
		model.Representation(map[string]interface{}{
			"field_a": "value-from-reporter",
		}),
		&ver,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	// Should create 2 tuples, one from each source
	created := tuples.TuplesToCreate()
	require.NotNil(t, created)
	assert.Len(t, *created, 2, "should create tuples from both common and reporter")

	// Verify both relations exist
	relationNames := make(map[string]bool)
	for _, tuple := range *created {
		relationNames[tuple.Relation().String()] = true
	}
	assert.True(t, relationNames["workspace"])
	assert.True(t, relationNames["cluster"])
}

// TestReporterRelation_OnlyForMatchingReporter verifies reporter relations only apply to the correct reporter
func TestReporterRelation_OnlyForMatchingReporter(t *testing.T) {
	commonRelations := []model.RelationDefinition{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "workspace_id",
			Cardinality: "one",
		},
	}

	// Reporter relations for OCM
	ocmRelations := []model.RelationDefinition{
		{
			Name:        "subscription",
			Target:      "ocm/subscription",
			Field:       "subscription_id",
			Cardinality: "one",
		},
	}

	// Create schema with OCM relations
	schema := NewUnifiedSchemaImplWithReporterRelations(
		map[string]interface{}{},
		commonRelations,
		"ocm", // OCM reporter type
		ocmRelations,
	)

	// But use ACM reporter type in the key
	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("acm") // Different reporter!
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("test-cluster"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-1",
		}),
		&ver,
		model.Representation(map[string]interface{}{
			"subscription_id": "sub-123",
		}),
		&ver,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	// Should only create workspace tuple (common)
	// OCM relations should NOT be processed for ACM reporter
	created := tuples.TuplesToCreate()
	require.NotNil(t, created)
	assert.Len(t, *created, 1, "should only create workspace tuple, not subscription")
	assert.Equal(t, "workspace", (*created)[0].Relation().String())
}

// TestReporterRelation_UpdatesCorrectly verifies reporter relations update properly
func TestReporterRelation_UpdatesCorrectly(t *testing.T) {
	reporterRelations := []model.RelationDefinition{
		{
			Name:        "subscription",
			Target:      "ocm/subscription",
			Field:       "subscription_id",
			Cardinality: "one",
		},
	}

	schema := NewUnifiedSchemaImplWithReporterRelations(
		map[string]interface{}{},
		[]model.RelationDefinition{}, // No common relations
		"ocm",
		reporterRelations,
	)

	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("ocm")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("test-cluster"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Previous state
	previous, err := model.NewRepresentations(
		nil, nil,
		model.Representation(map[string]interface{}{
			"subscription_id": "sub-old",
		}),
		&ver,
	)
	require.NoError(t, err)

	// Current state - subscription changed
	current, err := model.NewRepresentations(
		nil, nil,
		model.Representation(map[string]interface{}{
			"subscription_id": "sub-new",
		}),
		&ver,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, previous, key)
	require.NoError(t, err)

	// Should delete old subscription and create new subscription
	assert.True(t, tuples.HasTuplesToCreate())
	assert.True(t, tuples.HasTuplesToDelete())

	created := tuples.TuplesToCreate()
	deleted := tuples.TuplesToDelete()

	require.NotNil(t, created)
	require.NotNil(t, deleted)
	assert.Len(t, *created, 1)
	assert.Len(t, *deleted, 1)

	assert.Equal(t, "subscription", (*created)[0].Relation().String())
	assert.Equal(t, "subscription", (*deleted)[0].Relation().String())
}
