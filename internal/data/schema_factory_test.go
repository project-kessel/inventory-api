package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// TestSchemaFactorySelection pins the rollout gate for reporter-derived
// tuples: the legacy factory keeps the pre-RHCLOUD-51869 dispatch, and the
// reporter-aware factory only applies Features schemas to reporter
// representations.
func TestSchemaFactorySelection(t *testing.T) {
	jsonSchema := `{"type": "object"}`

	t.Run("legacy keeps Features schemas for common representations", func(t *testing.T) {
		_, isRelations := LegacyFeaturesAwareSchemaFactory(serviceType, false, jsonSchema).(JsonSchemaWithRelations)
		assert.True(t, isRelations, "service should keep its Features relations")

		_, isRelations = LegacyFeaturesAwareSchemaFactory(billingAccountType, false, jsonSchema).(JsonSchemaWithRelations)
		assert.True(t, isRelations, "billing account should keep its Features relations")

		_, isWorkspaces := LegacyFeaturesAwareSchemaFactory(workspaceType, true, jsonSchema).(JsonSchemaWithWorkspaces)
		assert.True(t, isWorkspaces, "workspace should stay on the default workspace schema")
	})

	t.Run("reporter-aware only applies Features schemas to reporter representations", func(t *testing.T) {
		_, isWorkspaces := FeaturesAwareSchemaFactory(serviceType, false, jsonSchema).(JsonSchemaWithWorkspaces)
		assert.True(t, isWorkspaces, "common service tuple logic is replaced by the workspace model")

		_, isWorkspaces = FeaturesAwareSchemaFactory(billingAccountType, false, jsonSchema).(JsonSchemaWithWorkspaces)
		assert.True(t, isWorkspaces, "common billing account uses the default workspace schema")

		_, isRelations := FeaturesAwareSchemaFactory(workspaceType, true, jsonSchema).(JsonSchemaWithRelations)
		assert.True(t, isRelations, "reporter workspace carries the Features relations")

		_, isRelations = FeaturesAwareSchemaFactory(billingAccountType, true, jsonSchema).(JsonSchemaWithRelations)
		assert.True(t, isRelations, "reporter billing account carries the Features relations")
	})
}

// TestBillingAccountTupleModelSwitch pins the tuple difference the flag
// controls: legacy billing accounts relate to workspaces, reporter-aware ones
// relate to services.
func TestBillingAccountTupleModelSwitch(t *testing.T) {
	key := featuresBillingAccountKey(t)
	ver := model.NewVersion(0)

	legacyRepresentation, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{"workspaces": []interface{}{"ws-1"}}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := NewLegacyFeaturesBillingAccountSchemaFromString(`{"type": "object"}`).
		CalculateTuples(legacyRepresentation, nil, key)
	require.NoError(t, err)
	require.True(t, result.HasTuplesToCreate())
	assert.Equal(t, "workspace", (*result.TuplesToCreate())[0].Relation().Serialize())

	reporterRepresentation, err := model.NewRepresentations(
		nil, nil,
		model.Representation(map[string]interface{}{"services": []interface{}{"svc-1"}}),
		&ver,
	)
	require.NoError(t, err)

	result, err = NewFeaturesBillingAccountSchemaFromString(`{"type": "object"}`).
		CalculateTuples(reporterRepresentation, nil, key)
	require.NoError(t, err)
	require.True(t, result.HasTuplesToCreate())
	assert.Equal(t, "services", (*result.TuplesToCreate())[0].Relation().Serialize())
}
