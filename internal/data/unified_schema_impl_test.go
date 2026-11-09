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
	current := (*model.Representations)(nil)
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
	var commonVersion *model.Version
	if len(common) > 0 {
		version := model.NewVersion(1)
		commonVersion = &version
	}
	var reporterVersion *model.Version
	if len(reporter) > 0 {
		version := model.NewVersion(1)
		reporterVersion = &version
	}
	current, err := model.NewRepresentations(
		model.Representation(common),
		commonVersion,
		model.Representation(reporter),
		reporterVersion,
	)
	require.NoError(t, err)
	return current
}

// TestUnifiedSchemaImpl_StreamRelationFields verifies that the stream-specific
// relation field accessors keep common and reporter metadata separate, with
// reporter fields scoped to the given reporter type.
func TestUnifiedSchemaImpl_StreamRelationFields(t *testing.T) {
	implementation := NewUnifiedSchemaImpl(map[string]interface{}{},
		[]UnifiedSchemaRelation{
			{Name: "workspace", Target: "rbac/workspace", Field: "workspace_id", Cardinality: "one"},
		},
		map[string][]UnifiedSchemaRelation{
			"hbi": {
				{Name: "service_preference", Target: "features/service", Field: "service_preferences", Cardinality: "many"},
			},
			"features": {
				{Name: "billing_account", Target: "features/billing_account", Field: "direct_billing_account", Cardinality: "one"},
				{Name: "service_preference", Target: "features/service", Field: "direct_service_preferences", Cardinality: "many"},
			},
		})

	t.Run("common relation fields are independent of the reporter type", func(t *testing.T) {
		assert.Equal(t, []string{"workspace_id"}, implementation.CommonRelationFields())
	})

	t.Run("reporter relation fields are scoped to the requested reporter type", func(t *testing.T) {
		features, err := model.NewReporterType("features")
		require.NoError(t, err)
		hbi, err := model.NewReporterType("hbi")
		require.NoError(t, err)

		assert.ElementsMatch(t, []string{"direct_billing_account", "direct_service_preferences"}, implementation.ReporterRelationFields(features))
		assert.Equal(t, []string{"service_preferences"}, implementation.ReporterRelationFields(hbi))
	})

	t.Run("unknown reporter type has no reporter relation fields", func(t *testing.T) {
		reporterType, err := model.NewReporterType("unknown-reporter")
		require.NoError(t, err)

		assert.Empty(t, implementation.ReporterRelationFields(reporterType))
	})
}

// TestUnifiedSchemaImpl_StreamRelationFields_Empty verifies that a schema
// without relation metadata reports no relation fields for either stream,
// which callers treat as "unknown" and skip strict field enforcement.
func TestUnifiedSchemaImpl_StreamRelationFields_Empty(t *testing.T) {
	implementation := NewUnifiedSchemaImpl(map[string]interface{}{}, nil, nil)
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)

	assert.Empty(t, implementation.CommonRelationFields())
	assert.Empty(t, implementation.ReporterRelationFields(reporterType))
}

// TestJSONSchemaImplementations_StreamRelationFields_Empty verifies that
// JSON-schema-only implementations report no relation fields for either
// stream: strict field enforcement is a unified-schema capability.
func TestJSONSchemaImplementations_StreamRelationFields_Empty(t *testing.T) {
	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)

	relations, err := model.NewRelationDef("workspace_id", "workspace", "rbac", "workspace", false)
	require.NoError(t, err)

	schemas := map[string]model.Schema{
		"default":                     model.NewDefaultSchema(),
		"json schema with workspaces": NewJsonSchemaWithWorkspacesFromString(`{"type": "object"}`),
		"json schema with relations":  NewJsonSchemaWithRelations(`{"type": "object"}`, []model.RelationDef{relations}),
		"features workspace schema":   NewFeaturesWorkspaceSchemaFromString(`{"type": "object"}`),
	}

	for name, schema := range schemas {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, schema.CommonRelationFields())
			assert.Empty(t, schema.ReporterRelationFields(reporterType))
		})
	}
}
