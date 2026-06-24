package data

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNullableRelation_SkipsWhenNull verifies that nullable relations are skipped when field is empty
func TestNullableRelation_SkipsWhenNull(t *testing.T) {
	// Schema with nullable tenant relation
	relations := []model.RelationDefinition{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "workspace_id",
			Cardinality: "one",
			Nullable:    false, // Required
		},
		{
			Name:        "tenant",
			Target:      "rbac/tenant",
			Field:       "tenant_id",
			Cardinality: "one",
			Nullable:    true, // Optional
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("host")
	reporterType, _ := model.NewReporterType("hbi")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
	)

	// Create resource with workspace but no tenant
	ver := model.NewVersion(1)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-1",
			// tenant_id is absent
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	// Should create workspace tuple, but skip tenant tuple
	assert.True(t, tuples.HasTuplesToCreate())
	created := tuples.TuplesToCreate()
	require.NotNil(t, created)
	assert.Len(t, *created, 1, "should only create workspace tuple")
	assert.Equal(t, "workspace", (*created)[0].Relation().String())
}

// TestNullableRelation_CreatesWhenPresent verifies that nullable relations create tuples when present
func TestNullableRelation_CreatesWhenPresent(t *testing.T) {
	relations := []model.RelationDefinition{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "workspace_id",
			Cardinality: "one",
			Nullable:    false,
		},
		{
			Name:        "tenant",
			Target:      "rbac/tenant",
			Field:       "tenant_id",
			Cardinality: "one",
			Nullable:    true,
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("host")
	reporterType, _ := model.NewReporterType("hbi")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
	)

	// Create resource with both workspace and tenant
	ver := model.NewVersion(1)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-1",
			"tenant_id":    "tenant-1",
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	// Should create both workspace and tenant tuples
	assert.True(t, tuples.HasTuplesToCreate())
	created := tuples.TuplesToCreate()
	require.NotNil(t, created)
	assert.Len(t, *created, 2, "should create both workspace and tenant tuples")

	// Verify relations
	relationNames := make(map[string]bool)
	for _, tuple := range *created {
		relationNames[tuple.Relation().String()] = true
	}
	assert.True(t, relationNames["workspace"])
	assert.True(t, relationNames["tenant"])
}

// TestNullableRelation_DeletesWhenBecomesNull verifies that tuples are deleted when nullable field becomes null
func TestNullableRelation_DeletesWhenBecomesNull(t *testing.T) {
	relations := []model.RelationDefinition{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "workspace_id",
			Cardinality: "one",
			Nullable:    false,
		},
		{
			Name:        "tenant",
			Target:      "rbac/tenant",
			Field:       "tenant_id",
			Cardinality: "one",
			Nullable:    true,
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("host")
	reporterType, _ := model.NewReporterType("hbi")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Previous state: both workspace and tenant present
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-1",
			"tenant_id":    "tenant-1",
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	// Current state: workspace present, tenant removed
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-1",
			// tenant_id removed
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, previous, key)
	require.NoError(t, err)

	// Should delete tenant tuple, workspace unchanged
	assert.False(t, tuples.HasTuplesToCreate(), "no new tuples to create")
	assert.True(t, tuples.HasTuplesToDelete(), "should delete tenant tuple")

	deleted := tuples.TuplesToDelete()
	require.NotNil(t, deleted)
	assert.Len(t, *deleted, 1, "should delete tenant tuple")
	assert.Equal(t, "tenant", (*deleted)[0].Relation().String())
}

// TestNullableRelation_CreatesWhenBecomesNonNull verifies that tuples are created when nullable field gets a value
func TestNullableRelation_CreatesWhenBecomesNonNull(t *testing.T) {
	relations := []model.RelationDefinition{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "workspace_id",
			Cardinality: "one",
			Nullable:    false,
		},
		{
			Name:        "tenant",
			Target:      "rbac/tenant",
			Field:       "tenant_id",
			Cardinality: "one",
			Nullable:    true,
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("host")
	reporterType, _ := model.NewReporterType("hbi")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Previous state: only workspace
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-1",
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	// Current state: workspace and tenant
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-1",
			"tenant_id":    "tenant-1", // Added
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, previous, key)
	require.NoError(t, err)

	// Should create tenant tuple, workspace unchanged
	assert.True(t, tuples.HasTuplesToCreate(), "should create tenant tuple")
	assert.False(t, tuples.HasTuplesToDelete(), "no tuples to delete")

	created := tuples.TuplesToCreate()
	require.NotNil(t, created)
	assert.Len(t, *created, 1, "should create tenant tuple")
	assert.Equal(t, "tenant", (*created)[0].Relation().String())
}

// TestNullableRelation_RequiredFieldMustBePresent verifies that non-nullable relations behave correctly
func TestNullableRelation_RequiredFieldMustBePresent(t *testing.T) {
	// This test verifies that non-nullable fields create tuples when present
	// and skip when empty (validation should happen at JSON Schema level)
	relations := []model.RelationDefinition{
		{
			Name:        "workspace",
			Target:      "rbac/workspace",
			Field:       "workspace_id",
			Cardinality: "one",
			Nullable:    false, // Required
		},
	}

	schema := NewUnifiedSchemaImpl(map[string]interface{}{}, relations)

	resourceType, _ := model.NewResourceType("host")
	reporterType, _ := model.NewReporterType("hbi")
	reporterInstanceId, _ := model.NewReporterInstanceId("test-instance")
	key, _ := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		resourceType, reporterType, reporterInstanceId,
	)

	ver := model.NewVersion(1)

	// Create resource with some data but workspace_id is empty string
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "",      // Explicitly empty
			"other_field":  "value", // Need at least one field for valid representation
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	tuples, err := schema.CalculateTuples(current, nil, key)
	require.NoError(t, err)

	// Current behavior: empty field means no tuple created (same as nullable)
	// This is acceptable - validation should happen at the JSON Schema level
	assert.False(t, tuples.HasTuplesToCreate())
}
