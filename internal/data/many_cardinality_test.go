package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManyCardinality_CreatesMultipleTuples verifies that cardinality "many" creates one tuple per array element
func TestManyCardinality_CreatesMultipleTuples(t *testing.T) {
	relations := []model.RelationDefinition{
		{
			Name:        "tag",
			Target:      "rbac/tag",
			Field:       "tag_ids",
			Cardinality: "many",
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("acm")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("cluster-1"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Create resource with array of tag IDs
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"tag_ids": []interface{}{"tag1", "tag2", "tag3"},
		}),
		&ver,
		nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	// Should create 3 tuples - one for each tag
	assert.True(t, tuples.HasTuplesToCreate())
	created := tuples.TuplesToCreate()
	require.NotNil(t, created)
	assert.Len(t, *created, 3, "should create one tuple per array element")

	// Verify all three tag relations exist
	tagTargets := make(map[string]bool)
	for _, tuple := range *created {
		assert.Equal(t, "tag", tuple.Relation().String())
		tagTargets[tuple.Subject().Resource().ResourceId().String()] = true
	}
	assert.True(t, tagTargets["tag1"])
	assert.True(t, tagTargets["tag2"])
	assert.True(t, tagTargets["tag3"])
}

// TestManyCardinality_HandlesEmptyArray verifies that empty arrays create no tuples
func TestManyCardinality_HandlesEmptyArray(t *testing.T) {
	relations := []model.RelationDefinition{
		{
			Name:        "tag",
			Target:      "rbac/tag",
			Field:       "tag_ids",
			Cardinality: "many",
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("acm")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("cluster-1"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Create resource with empty array
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"tag_ids": []interface{}{},
		}),
		&ver,
		nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	// Should create no tuples
	assert.False(t, tuples.HasTuplesToCreate())
}

// TestManyCardinality_HandlesNilArray verifies that nil/missing arrays create no tuples
func TestManyCardinality_HandlesNilArray(t *testing.T) {
	relations := []model.RelationDefinition{
		{
			Name:        "tag",
			Target:      "rbac/tag",
			Field:       "tag_ids",
			Cardinality: "many",
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("acm")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("cluster-1"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Create resource without the tag_ids field
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"other_field": "value",
		}),
		&ver,
		nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	// Should create no tuples
	assert.False(t, tuples.HasTuplesToCreate())
}

// TestManyCardinality_UpdateAddsNewElements verifies adding elements to array creates new tuples
func TestManyCardinality_UpdateAddsNewElements(t *testing.T) {
	relations := []model.RelationDefinition{
		{
			Name:        "tag",
			Target:      "rbac/tag",
			Field:       "tag_ids",
			Cardinality: "many",
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("acm")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("cluster-1"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Previous state: 2 tags
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"tag_ids": []interface{}{"tag1", "tag2"},
		}),
		&ver,
		nil, nil,
	)
	require.NoError(t, err)

	// Current state: 3 tags (added tag3)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"tag_ids": []interface{}{"tag1", "tag2", "tag3"},
		}),
		&ver,
		nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, previous, key)
	require.NoError(t, err)

	// Should create 1 new tuple for tag3
	assert.True(t, tuples.HasTuplesToCreate())
	assert.False(t, tuples.HasTuplesToDelete())

	created := tuples.TuplesToCreate()
	require.NotNil(t, created)
	assert.Len(t, *created, 1, "should create tuple for new element")
	assert.Equal(t, "tag3", (*created)[0].Subject().Resource().ResourceId().String())
}

// TestManyCardinality_UpdateRemovesElements verifies removing elements deletes tuples
func TestManyCardinality_UpdateRemovesElements(t *testing.T) {
	relations := []model.RelationDefinition{
		{
			Name:        "tag",
			Target:      "rbac/tag",
			Field:       "tag_ids",
			Cardinality: "many",
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("acm")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("cluster-1"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Previous state: 3 tags
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"tag_ids": []interface{}{"tag1", "tag2", "tag3"},
		}),
		&ver,
		nil, nil,
	)
	require.NoError(t, err)

	// Current state: 2 tags (removed tag2)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"tag_ids": []interface{}{"tag1", "tag3"},
		}),
		&ver,
		nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, previous, key)
	require.NoError(t, err)

	// Should delete 1 tuple for tag2
	assert.False(t, tuples.HasTuplesToCreate())
	assert.True(t, tuples.HasTuplesToDelete())

	deleted := tuples.TuplesToDelete()
	require.NotNil(t, deleted)
	assert.Len(t, *deleted, 1, "should delete tuple for removed element")
	assert.Equal(t, "tag2", (*deleted)[0].Subject().Resource().ResourceId().String())
}

// TestManyCardinality_UpdateAddRemoveMixed verifies mixed add/remove operations
func TestManyCardinality_UpdateAddRemoveMixed(t *testing.T) {
	relations := []model.RelationDefinition{
		{
			Name:        "tag",
			Target:      "rbac/tag",
			Field:       "tag_ids",
			Cardinality: "many",
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("acm")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("cluster-1"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Previous state: tag1, tag2, tag3
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"tag_ids": []interface{}{"tag1", "tag2", "tag3"},
		}),
		&ver,
		nil, nil,
	)
	require.NoError(t, err)

	// Current state: tag1, tag3, tag4 (removed tag2, added tag4, kept tag1 and tag3)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"tag_ids": []interface{}{"tag1", "tag3", "tag4"},
		}),
		&ver,
		nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, previous, key)
	require.NoError(t, err)

	// Should create 1 tuple (tag4) and delete 1 tuple (tag2)
	assert.True(t, tuples.HasTuplesToCreate())
	assert.True(t, tuples.HasTuplesToDelete())

	created := tuples.TuplesToCreate()
	deleted := tuples.TuplesToDelete()

	require.NotNil(t, created)
	require.NotNil(t, deleted)
	assert.Len(t, *created, 1, "should create tuple for tag4")
	assert.Len(t, *deleted, 1, "should delete tuple for tag2")

	assert.Equal(t, "tag4", (*created)[0].Subject().Resource().ResourceId().String())
	assert.Equal(t, "tag2", (*deleted)[0].Subject().Resource().ResourceId().String())
}

// TestManyCardinality_MixedWithOneCardinality verifies many and one cardinality work together
func TestManyCardinality_MixedWithOneCardinality(t *testing.T) {
	relations := []model.RelationDefinition{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "workspace_id",
			Cardinality: "one",
		},
		{
			Name:        "tag",
			Target:      "rbac/tag",
			Field:       "tag_ids",
			Cardinality: "many",
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("acm")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("cluster-1"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Create resource with both one and many cardinality fields
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-1",
			"tag_ids":      []interface{}{"tag1", "tag2"},
		}),
		&ver,
		nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	// Should create 3 tuples: 1 workspace + 2 tags
	assert.True(t, tuples.HasTuplesToCreate())
	created := tuples.TuplesToCreate()
	require.NotNil(t, created)
	assert.Len(t, *created, 3, "should create 1 workspace tuple + 2 tag tuples")

	// Verify relations
	relationCounts := make(map[string]int)
	for _, tuple := range *created {
		relationCounts[tuple.Relation().String()]++
	}
	assert.Equal(t, 1, relationCounts["workspace"])
	assert.Equal(t, 2, relationCounts["tag"])
}

// TestManyCardinality_EmptyStringElements verifies empty strings in array are skipped
func TestManyCardinality_EmptyStringElements(t *testing.T) {
	relations := []model.RelationDefinition{
		{
			Name:        "tag",
			Target:      "rbac/tag",
			Field:       "tag_ids",
			Cardinality: "many",
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("k8s_cluster")
	reporterType, _ := model.NewReporterType("acm")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("cluster-1"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Array with empty strings (should be skipped)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"tag_ids": []interface{}{"tag1", "", "tag2", ""},
		}),
		&ver,
		nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	// Should create only 2 tuples (empty strings skipped)
	assert.True(t, tuples.HasTuplesToCreate())
	created := tuples.TuplesToCreate()
	require.NotNil(t, created)
	assert.Len(t, *created, 2, "should skip empty string elements")

	// Verify only non-empty tags
	tagTargets := make(map[string]bool)
	for _, tuple := range *created {
		tagTargets[tuple.Subject().Resource().ResourceId().String()] = true
	}
	assert.True(t, tagTargets["tag1"])
	assert.True(t, tagTargets["tag2"])
}
