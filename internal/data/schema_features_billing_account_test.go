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
		"workspace_ids": { "type": "array", "items": { "type": "string" } }
	},
	"required": ["workspace_ids"]
}`

func newBillingAccountKey(t *testing.T) model.ReporterResourceKey {
	t.Helper()
	resourceType, err := model.NewResourceType("billing_account")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("features")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		model.DeserializeLocalResourceId("ba-001"),
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)
	return key
}

func TestFeaturesBillingAccountSchema_Validate(t *testing.T) {
	schema := NewFeaturesBillingAccountSchemaFromString(billingAccountJsonSchema)

	t.Run("valid data passes", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{
			"workspace_ids": []interface{}{"ws-1", "ws-2"},
		})
		assert.True(t, valid)
		assert.NoError(t, err)
	})

	t.Run("missing required fields fails", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{})
		assert.False(t, valid)
		assert.Error(t, err)
	})

	t.Run("wrong type fails", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{
			"workspace_ids": "not-an-array",
		})
		assert.False(t, valid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

func TestFeaturesBillingAccountSchema_CalculateTuples_Create(t *testing.T) {
	schema := NewFeaturesBillingAccountSchemaFromString(billingAccountJsonSchema)
	key := newBillingAccountKey(t)

	ver := model.NewVersion(0)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_ids": []interface{}{"ws-1", "ws-2"},
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)
	assert.True(t, result.HasTuplesToCreate())
	assert.False(t, result.HasTuplesToDelete())

	creates := *result.TuplesToCreate()
	assert.Len(t, creates, 2)

	relations := make(map[string][]string)
	for _, tuple := range creates {
		rel := tuple.Relation().Serialize()
		subjectId := tuple.Subject().Resource().ResourceId().Serialize()
		relations[rel] = append(relations[rel], subjectId)
	}
	assert.ElementsMatch(t, []string{"ws-1", "ws-2"}, relations["workspace"])
}

func TestFeaturesBillingAccountSchema_CalculateTuples_Update(t *testing.T) {
	schema := NewFeaturesBillingAccountSchemaFromString(billingAccountJsonSchema)
	key := newBillingAccountKey(t)

	curVer := model.NewVersion(1)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_ids": []interface{}{"ws-2", "ws-3"},
		}),
		&curVer, nil, nil,
	)
	require.NoError(t, err)

	prevVer := curVer.Decrement()
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_ids": []interface{}{"ws-1", "ws-2"},
		}),
		&prevVer, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(current, previous, key)
	require.NoError(t, err)
	assert.True(t, result.HasTuplesToCreate())
	assert.True(t, result.HasTuplesToDelete())

	creates := *result.TuplesToCreate()
	assert.Len(t, creates, 1)
	assert.Equal(t, "workspace", creates[0].Relation().Serialize())
	assert.Equal(t, "ws-3", creates[0].Subject().Resource().ResourceId().Serialize())

	deletes := *result.TuplesToDelete()
	assert.Len(t, deletes, 1)
	assert.Equal(t, "workspace", deletes[0].Relation().Serialize())
	assert.Equal(t, "ws-1", deletes[0].Subject().Resource().ResourceId().Serialize())
}

func TestFeaturesBillingAccountSchema_CalculateTuples_NoChange(t *testing.T) {
	schema := NewFeaturesBillingAccountSchemaFromString(billingAccountJsonSchema)
	key := newBillingAccountKey(t)

	curVer := model.NewVersion(1)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_ids": []interface{}{"ws-1"},
		}),
		&curVer, nil, nil,
	)
	require.NoError(t, err)

	prevVer := curVer.Decrement()
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_ids": []interface{}{"ws-1"},
		}),
		&prevVer, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(current, previous, key)
	require.NoError(t, err)
	assert.True(t, result.IsEmpty())
}

func TestFeaturesBillingAccountSchema_CalculateTuples_Delete(t *testing.T) {
	schema := NewFeaturesBillingAccountSchemaFromString(billingAccountJsonSchema)
	key := newBillingAccountKey(t)

	prevVer := model.NewVersion(0)
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_ids": []interface{}{"ws-1", "ws-2"},
		}),
		&prevVer, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(nil, previous, key)
	require.NoError(t, err)
	assert.False(t, result.HasTuplesToCreate())
	assert.True(t, result.HasTuplesToDelete())

	deletes := *result.TuplesToDelete()
	assert.Len(t, deletes, 2)
}

func TestFeaturesBillingAccountSchema_CalculateTuples_SubjectTypes(t *testing.T) {
	schema := NewFeaturesBillingAccountSchemaFromString(billingAccountJsonSchema)
	key := newBillingAccountKey(t)

	ver := model.NewVersion(0)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_ids": []interface{}{"ws-1"},
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	creates := *result.TuplesToCreate()
	assert.Len(t, creates, 1)
	assert.Equal(t, "workspace", creates[0].Relation().Serialize())
	assert.Equal(t, "rbac", creates[0].Subject().Resource().Reporter().ReporterType().Serialize())
	assert.Equal(t, "workspace", creates[0].Subject().Resource().ResourceType().Serialize())
}
