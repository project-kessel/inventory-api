package data

import (
	"testing"

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
			model.Representation(map[string]interface{}{
				"direct_billing_account":     "ba-100",
				"direct_service_preferences": []interface{}{"svc-1", "svc-2"},
			}),
			&ver, nil, nil,
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
			model.Representation(map[string]interface{}{
				"direct_billing_account":     "ba-100",
				"direct_service_preferences": []interface{}{"svc-1", "svc-2"},
			}),
			&ver1, nil, nil,
		)
		require.NoError(t, err)

		ver2 := model.NewVersion(2)
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"direct_billing_account":     "ba-200",
				"direct_service_preferences": []interface{}{"svc-2", "svc-3"},
			}),
			&ver2, nil, nil,
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
			model.Representation(map[string]interface{}{
				"direct_billing_account":     "ba-100",
				"direct_service_preferences": []interface{}{"svc-1"},
			}),
			&ver, nil, nil,
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

	t.Run("handles nil direct_billing_account", func(t *testing.T) {
		ver := model.NewVersion(0)
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"direct_service_preferences": []interface{}{"svc-1"},
			}),
			&ver, nil, nil,
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
			model.Representation(map[string]interface{}{
				"direct_billing_account": "ba-100",
			}),
			&ver, nil, nil,
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
			model.Representation(map[string]interface{}{
				"services": []interface{}{"svc-1", "svc-2"},
			}),
			&ver, nil, nil,
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
			model.Representation(map[string]interface{}{
				"services": []interface{}{"svc-1", "svc-2"},
			}),
			&ver1, nil, nil,
		)
		require.NoError(t, err)

		ver2 := model.NewVersion(2)
		current, err := model.NewRepresentations(
			model.Representation(map[string]interface{}{
				"services": []interface{}{"svc-2", "svc-3"},
			}),
			&ver2, nil, nil,
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
