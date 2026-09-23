package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnifiedSchemaImpl_Validate(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"workspace_id": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"workspace_id"},
	}
	implementation := NewUnifiedSchemaImpl(schema, nil, nil)

	t.Run("accepts valid representation", func(t *testing.T) {
		valid, err := implementation.Validate(map[string]interface{}{"workspace_id": "workspace-1"})

		assert.True(t, valid)
		assert.NoError(t, err)
	})

	t.Run("rejects invalid representation", func(t *testing.T) {
		valid, err := implementation.Validate(map[string]interface{}{})

		assert.False(t, valid)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workspace_id is required")
	})
}

func TestUnifiedSchemaImpl_CalculateTuples_CommonOneRelation(t *testing.T) {
	key := newUnifiedSchemaTestKey(t, "hbi")
	implementation := NewUnifiedSchemaImpl(map[string]interface{}{}, []UnifiedSchemaRelation{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "workspace_id",
			Cardinality: "one",
		},
	}, nil)
	current := newUnifiedSchemaRepresentations(t, map[string]interface{}{"workspace_id": "workspace-new"}, nil)
	previous := newUnifiedSchemaRepresentations(t, map[string]interface{}{"workspace_id": "workspace-old"}, nil)

	tuples, err := implementation.CalculateTuples(current, previous, key)

	require.NoError(t, err)
	require.Len(t, *tuples.TuplesToCreate(), 1)
	require.Len(t, *tuples.TuplesToDelete(), 1)
	assert.Equal(t, model.NewRelationTupleForSubject(key, "workspace", "rbac", "workspace", "workspace-new"), (*tuples.TuplesToCreate())[0])
	assert.Equal(t, model.NewRelationTupleForSubject(key, "workspace", "rbac", "workspace", "workspace-old"), (*tuples.TuplesToDelete())[0])
}

func TestUnifiedSchemaImpl_CalculateTuples_CommonManyRelation(t *testing.T) {
	key := newUnifiedSchemaTestKey(t, "hbi")
	implementation := NewUnifiedSchemaImpl(map[string]interface{}{}, []UnifiedSchemaRelation{
		{
			Name:        "tag",
			Target:      "rbac/tag",
			Field:       "tag_ids",
			Cardinality: "many",
		},
	}, nil)
	current := newUnifiedSchemaRepresentations(t, map[string]interface{}{"tag_ids": []interface{}{"tag-2", "tag-3"}}, nil)
	previous := newUnifiedSchemaRepresentations(t, map[string]interface{}{"tag_ids": []interface{}{"tag-1", "tag-2"}}, nil)

	tuples, err := implementation.CalculateTuples(current, previous, key)

	require.NoError(t, err)
	require.Len(t, *tuples.TuplesToCreate(), 1)
	require.Len(t, *tuples.TuplesToDelete(), 1)
	assert.Equal(t, "tag-3", (*tuples.TuplesToCreate())[0].Subject().Resource().ResourceId().String())
	assert.Equal(t, "tag-1", (*tuples.TuplesToDelete())[0].Subject().Resource().ResourceId().String())
}

func TestUnifiedSchemaImpl_CalculateTuples_ReporterRelation(t *testing.T) {
	key := newUnifiedSchemaTestKey(t, "hbi")
	implementation := NewUnifiedSchemaImpl(map[string]interface{}{}, []UnifiedSchemaRelation{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "workspace_id",
			Cardinality: "one",
		},
	}, map[string][]UnifiedSchemaRelation{
		"hbi": {
			{
				Name:        "host",
				Target:      "hbi/host",
				Field:       "host_id",
				Cardinality: "one",
			},
		},
	})
	current := newUnifiedSchemaRepresentations(t, map[string]interface{}{"workspace_id": "workspace-1"}, map[string]interface{}{"host_id": "host-1"})

	tuples, err := implementation.CalculateTuples(current, nil, key)

	require.NoError(t, err)
	require.Len(t, *tuples.TuplesToCreate(), 2)
	assert.Equal(t, "workspace", (*tuples.TuplesToCreate())[0].Relation().String())
	assert.Equal(t, "host", (*tuples.TuplesToCreate())[1].Relation().String())
}

func TestUnifiedSchemaImpl_CalculateTuples_OptionalRelationRemoval(t *testing.T) {
	key := newUnifiedSchemaTestKey(t, "hbi")
	implementation := NewUnifiedSchemaImpl(map[string]interface{}{}, []UnifiedSchemaRelation{
		{
			Name:        "tenant",
			Target:      "rbac/tenant",
			Field:       "tenant_id",
			Cardinality: "one",
		},
	}, nil)
	current := newUnifiedSchemaRepresentations(t, map[string]interface{}{}, nil)
	previous := newUnifiedSchemaRepresentations(t, map[string]interface{}{"tenant_id": "tenant-1"}, nil)

	tuples, err := implementation.CalculateTuples(current, previous, key)

	require.NoError(t, err)
	assert.False(t, tuples.HasTuplesToCreate())
	require.Len(t, *tuples.TuplesToDelete(), 1)
	assert.Equal(t, "tenant-1", (*tuples.TuplesToDelete())[0].Subject().Resource().ResourceId().String())
}

func TestUnifiedSchemaImpl_CalculateTuples_ResourceDeletion(t *testing.T) {
	key := newUnifiedSchemaTestKey(t, "hbi")
	implementation := NewUnifiedSchemaImpl(map[string]interface{}{}, []UnifiedSchemaRelation{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "workspace_id",
			Cardinality: "one",
		},
	}, map[string][]UnifiedSchemaRelation{
		"hbi": {
			{
				Name:        "host",
				Target:      "hbi/host",
				Field:       "host_id",
				Cardinality: "one",
			},
		},
	})
	previous := newUnifiedSchemaRepresentations(t, map[string]interface{}{"workspace_id": "workspace-1"}, map[string]interface{}{"host_id": "host-1"})

	tuples, err := implementation.CalculateTuples(nil, previous, key)

	require.NoError(t, err)
	assert.False(t, tuples.HasTuplesToCreate())
	require.Len(t, *tuples.TuplesToDelete(), 2)
}

func newUnifiedSchemaTestKey(t *testing.T, reporterName string) model.ReporterResourceKey {
	t.Helper()
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType(reporterName)
	require.NoError(t, err)
	reporterInstanceID, err := model.NewReporterInstanceId("instance-1")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("host-1"),
		resourceType,
		reporterType,
		reporterInstanceID,
	)
	require.NoError(t, err)
	return key
}

func newUnifiedSchemaRepresentations(t *testing.T, common, reporter map[string]interface{}) *model.Representations {
	t.Helper()
	commonVersion := model.NewVersion(1)
	reporterVersion := model.NewVersion(1)
	current, err := model.NewRepresentations(common, &commonVersion, reporter, &reporterVersion)
	require.NoError(t, err)
	return current
}
