package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var serviceJsonSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"type": "object",
	"properties": {
		"allowed_workspaces": { "type": "array", "items": { "type": "string" } },
		"billing_account": { "type": "array", "items": { "type": "string" } },
		"parent": { "type": "string" }
	},
	"required": []
}`

var billingAccountJsonSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"type": "object",
	"properties": {
		"workspaces": { "type": "array", "items": { "type": "string" } }
	},
	"required": []
}`

func TestFeaturesServiceSchema_Validate(t *testing.T) {
	schema := NewFeaturesServiceSchemaFromString(serviceJsonSchema)

	t.Run("valid data passes", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{
			"allowed_workspaces": []interface{}{"ws-1", "ws-2"},
			"billing_account":    []interface{}{"ba-1"},
			"parent":             "parent-svc-1",
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
			"allowed_workspaces": "not-an-array",
			"billing_account":    []interface{}{"ba-1"},
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
			"workspaces": []interface{}{"ws-1", "ws-2"},
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
			"workspaces": "not-an-array",
		})
		assert.False(t, valid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

func featuresServiceKey(t *testing.T) model.ReporterResourceKey {
	t.Helper()
	resourceType, err := model.NewResourceType("service")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("features")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("features-instance")
	require.NoError(t, err)
	localResourceId, err := model.NewLocalResourceId("svc-001")
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

func TestFeaturesServiceSchema_CalculateTuples(t *testing.T) {
	schema := NewFeaturesServiceSchemaFromString(serviceJsonSchema)
	key := featuresServiceKey(t)

	t.Run("create produces tuples for all relations", func(t *testing.T) {
		ver := model.NewVersion(0)
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"allowed_workspaces": []interface{}{"ws-1", "ws-2"},
				"billing_account":    []interface{}{"ba-100"},
				"parent":             "parent-svc",
			}),
			&ver, nil, nil,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, nil, key)
		require.NoError(t, err)

		assert.True(t, result.HasTuplesToCreate())
		assert.False(t, result.HasTuplesToDelete())

		expected := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "allowed_workspaces", "rbac", "workspace", "ws-1"),
			model.NewRelationTupleForSubject(key, "allowed_workspaces", "rbac", "workspace", "ws-2"),
			model.NewRelationTupleForSubject(key, "billing_account", "features", "billing_account", "ba-100"),
			model.NewRelationTupleForSubject(key, "parent", "features", "service", "parent-svc"),
		}
		assert.ElementsMatch(t, expected, *result.TuplesToCreate())
	})

	t.Run("update creates and deletes changed values", func(t *testing.T) {
		ver1 := model.NewVersion(1)
		previous, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"allowed_workspaces": []interface{}{"ws-1", "ws-2"},
				"billing_account":    []interface{}{"ba-100"},
				"parent":             "parent-svc",
			}),
			&ver1, nil, nil,
		)
		require.NoError(t, err)

		ver2 := model.NewVersion(2)
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"allowed_workspaces": []interface{}{"ws-2", "ws-3"},
				"billing_account":    []interface{}{"ba-200"},
				"parent":             "parent-svc",
			}),
			&ver2, nil, nil,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		assert.True(t, result.HasTuplesToCreate())
		assert.True(t, result.HasTuplesToDelete())

		expectedCreates := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "allowed_workspaces", "rbac", "workspace", "ws-3"),
			model.NewRelationTupleForSubject(key, "billing_account", "features", "billing_account", "ba-200"),
		}
		assert.ElementsMatch(t, expectedCreates, *result.TuplesToCreate())

		expectedDeletes := []model.RelationsTuple{
			model.NewRelationTupleForSubject(key, "allowed_workspaces", "rbac", "workspace", "ws-1"),
			model.NewRelationTupleForSubject(key, "billing_account", "features", "billing_account", "ba-100"),
		}
		assert.ElementsMatch(t, expectedDeletes, *result.TuplesToDelete())
	})

	t.Run("delete produces only deletes", func(t *testing.T) {
		ver := model.NewVersion(1)
		previous, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"allowed_workspaces": []interface{}{"ws-1"},
				"billing_account":    []interface{}{"ba-100"},
				"parent":             "parent-svc",
			}),
			&ver, nil, nil,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(nil, previous, key)
		require.NoError(t, err)

		assert.False(t, result.HasTuplesToCreate())
		assert.True(t, result.HasTuplesToDelete())

		deletes := *result.TuplesToDelete()
		assert.Len(t, deletes, 3) // 1 allowed_workspaces + 1 billing_account + 1 parent
	})

	t.Run("same data produces no tuples", func(t *testing.T) {
		sameData := map[string]interface{}{
			"allowed_workspaces": []interface{}{"ws-1"},
			"billing_account":    []interface{}{"ba-100"},
			"parent":             "parent-svc",
		}

		ver1 := model.NewVersion(1)
		previous, err := model.NewRepresentations(
			model.Representation(sameData), &ver1, nil, nil,
		)
		require.NoError(t, err)

		ver2 := model.NewVersion(2)
		current, err := model.NewRepresentations(
			model.Representation(sameData), &ver2, nil, nil,
		)
		require.NoError(t, err)

		result, err := schema.CalculateTuples(current, previous, key)
		require.NoError(t, err)

		assert.False(t, result.HasTuplesToCreate())
		assert.False(t, result.HasTuplesToDelete())
	})
}

func TestFeaturesBillingAccountSchema_CalculateTuples(t *testing.T) {
	schema := NewFeaturesBillingAccountSchemaFromString(billingAccountJsonSchema)
	key := featuresBillingAccountKey(t)

	ver := model.NewVersion(0)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspaces": []interface{}{"ws-billing-1", "ws-billing-2"},
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate())
	assert.False(t, result.HasTuplesToDelete())

	expected := []model.RelationsTuple{
		model.NewRelationTupleForSubject(key, "workspace", "rbac", "workspace", "ws-billing-1"),
		model.NewRelationTupleForSubject(key, "workspace", "rbac", "workspace", "ws-billing-2"),
	}
	assert.ElementsMatch(t, expected, *result.TuplesToCreate())
}

func TestFeaturesAwareSchemaFactory_FallsBackForOtherTypes(t *testing.T) {
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)

	schema := FeaturesAwareSchemaFactory(resourceType, `{"type": "object"}`)

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
