package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
