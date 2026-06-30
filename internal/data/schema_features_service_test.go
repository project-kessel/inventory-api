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
		"allowed_workspace_ids": { "type": "array", "items": { "type": "string" } },
		"billing_account_ids": { "type": "array", "items": { "type": "string" } },
		"parent_service_id": { "type": "string" }
	},
	"required": ["allowed_workspace_ids", "billing_account_ids"]
}`

func newServiceKey(t *testing.T) model.ReporterResourceKey {
	t.Helper()
	resourceType, err := model.NewResourceType("service")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("features")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		model.DeserializeLocalResourceId("svc-001"),
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)
	return key
}

func TestFeaturesServiceSchema_Validate(t *testing.T) {
	schema := NewFeaturesServiceSchemaFromString(serviceJsonSchema)

	t.Run("valid data passes", func(t *testing.T) {
		valid, err := schema.Validate(map[string]interface{}{
			"allowed_workspace_ids": []interface{}{"ws-1", "ws-2"},
			"billing_account_ids":   []interface{}{"ba-1"},
			"parent_service_id":     "parent-svc-1",
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
			"allowed_workspace_ids": "not-an-array",
			"billing_account_ids":   []interface{}{"ba-1"},
		})
		assert.False(t, valid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

func TestFeaturesServiceSchema_CalculateTuples_Create(t *testing.T) {
	schema := NewFeaturesServiceSchemaFromString(serviceJsonSchema)
	key := newServiceKey(t)

	ver := model.NewVersion(0)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspace_ids": []interface{}{"ws-1", "ws-2"},
			"billing_account_ids":   []interface{}{"ba-1"},
			"parent_service_id":     "parent-svc",
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)
	assert.True(t, result.HasTuplesToCreate())
	assert.False(t, result.HasTuplesToDelete())

	creates := *result.TuplesToCreate()
	assert.Len(t, creates, 4) // 2 workspaces + 1 billing + 1 parent

	relations := make(map[string][]string)
	for _, tuple := range creates {
		rel := tuple.Relation().Serialize()
		subjectId := tuple.Subject().Resource().ResourceId().Serialize()
		relations[rel] = append(relations[rel], subjectId)
	}
	assert.ElementsMatch(t, []string{"ws-1", "ws-2"}, relations["allowed_workspaces"])
	assert.ElementsMatch(t, []string{"ba-1"}, relations["billing_account"])
	assert.ElementsMatch(t, []string{"parent-svc"}, relations["parent"])
}

func TestFeaturesServiceSchema_CalculateTuples_Update(t *testing.T) {
	schema := NewFeaturesServiceSchemaFromString(serviceJsonSchema)
	key := newServiceKey(t)

	curVer := model.NewVersion(1)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspace_ids": []interface{}{"ws-2", "ws-3"},
			"billing_account_ids":   []interface{}{"ba-1"},
		}),
		&curVer, nil, nil,
	)
	require.NoError(t, err)

	prevVer := curVer.Decrement()
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspace_ids": []interface{}{"ws-1", "ws-2"},
			"billing_account_ids":   []interface{}{"ba-1"},
		}),
		&prevVer, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(current, previous, key)
	require.NoError(t, err)
	assert.True(t, result.HasTuplesToCreate())
	assert.True(t, result.HasTuplesToDelete())

	creates := *result.TuplesToCreate()
	assert.Len(t, creates, 1) // ws-3 added
	assert.Equal(t, "allowed_workspaces", creates[0].Relation().Serialize())
	assert.Equal(t, "ws-3", creates[0].Subject().Resource().ResourceId().Serialize())

	deletes := *result.TuplesToDelete()
	assert.Len(t, deletes, 1) // ws-1 removed
	assert.Equal(t, "allowed_workspaces", deletes[0].Relation().Serialize())
	assert.Equal(t, "ws-1", deletes[0].Subject().Resource().ResourceId().Serialize())
}

func TestFeaturesServiceSchema_CalculateTuples_ScalarUpdate(t *testing.T) {
	schema := NewFeaturesServiceSchemaFromString(serviceJsonSchema)
	key := newServiceKey(t)

	curVer := model.NewVersion(1)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspace_ids": []interface{}{"ws-1"},
			"billing_account_ids":   []interface{}{"ba-new"},
			"parent_service_id":     "parent-new",
		}),
		&curVer, nil, nil,
	)
	require.NoError(t, err)

	prevVer := curVer.Decrement()
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspace_ids": []interface{}{"ws-1"},
			"billing_account_ids":   []interface{}{"ba-old"},
			"parent_service_id":     "parent-old",
		}),
		&prevVer, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(current, previous, key)
	require.NoError(t, err)
	assert.True(t, result.HasTuplesToCreate())
	assert.True(t, result.HasTuplesToDelete())

	creates := *result.TuplesToCreate()
	assert.Len(t, creates, 2) // ba-new + parent-new

	deletes := *result.TuplesToDelete()
	assert.Len(t, deletes, 2) // ba-old + parent-old
}

func TestFeaturesServiceSchema_CalculateTuples_NoChange(t *testing.T) {
	schema := NewFeaturesServiceSchemaFromString(serviceJsonSchema)
	key := newServiceKey(t)

	curVer := model.NewVersion(1)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspace_ids": []interface{}{"ws-1"},
			"billing_account_ids":   []interface{}{"ba-1"},
		}),
		&curVer, nil, nil,
	)
	require.NoError(t, err)

	prevVer := curVer.Decrement()
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspace_ids": []interface{}{"ws-1"},
			"billing_account_ids":   []interface{}{"ba-1"},
		}),
		&prevVer, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(current, previous, key)
	require.NoError(t, err)
	assert.True(t, result.IsEmpty())
}

func TestFeaturesServiceSchema_CalculateTuples_Delete(t *testing.T) {
	schema := NewFeaturesServiceSchemaFromString(serviceJsonSchema)
	key := newServiceKey(t)

	prevVer := model.NewVersion(0)
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspace_ids": []interface{}{"ws-1"},
			"billing_account_ids":   []interface{}{"ba-1"},
			"parent_service_id":     "parent-svc",
		}),
		&prevVer, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(nil, previous, key)
	require.NoError(t, err)
	assert.False(t, result.HasTuplesToCreate())
	assert.True(t, result.HasTuplesToDelete())

	deletes := *result.TuplesToDelete()
	assert.Len(t, deletes, 3) // 1 workspace + 1 billing + 1 parent
}

func TestFeaturesServiceSchema_CalculateTuples_SubjectTypes(t *testing.T) {
	schema := NewFeaturesServiceSchemaFromString(serviceJsonSchema)
	key := newServiceKey(t)

	ver := model.NewVersion(0)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspace_ids": []interface{}{"ws-1"},
			"billing_account_ids":   []interface{}{"ba-1"},
			"parent_service_id":     "parent-svc",
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	creates := *result.TuplesToCreate()
	subjectByRelation := make(map[string]struct{ namespace, resourceType string })
	for _, tuple := range creates {
		rel := tuple.Relation().Serialize()
		subjectByRelation[rel] = struct{ namespace, resourceType string }{
			namespace:    tuple.Subject().Resource().Reporter().ReporterType().Serialize(),
			resourceType: tuple.Subject().Resource().ResourceType().Serialize(),
		}
	}

	assert.Equal(t, "rbac", subjectByRelation["allowed_workspaces"].namespace)
	assert.Equal(t, "workspace", subjectByRelation["allowed_workspaces"].resourceType)

	assert.Equal(t, "features", subjectByRelation["billing_account"].namespace)
	assert.Equal(t, "billing_account", subjectByRelation["billing_account"].resourceType)

	assert.Equal(t, "features", subjectByRelation["parent"].namespace)
	assert.Equal(t, "service", subjectByRelation["parent"].resourceType)
}
