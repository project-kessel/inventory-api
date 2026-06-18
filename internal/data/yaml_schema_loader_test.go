package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadUnifiedSchemaFromFile(t *testing.T) {
	t.Run("loads valid host schema", func(t *testing.T) {
		// Use actual host.yaml from the repository
		schemaPath := filepath.Join("..", "..", "data", "schema", "resources", "host.yaml")

		schema, err := LoadUnifiedSchemaFromFile(schemaPath)
		require.NoError(t, err)

		assert.Equal(t, "1.0", schema.SchemaVersion)
		assert.Equal(t, "host", schema.Name)
		assert.NotNil(t, schema.Common.Schema)
		assert.NotEmpty(t, schema.Common.Relations)
		require.Len(t, schema.Reporters, 1)
		assert.Equal(t, "hbi", schema.Reporters[0].Name)
	})

	t.Run("validates schema structure with JSON Schema", func(t *testing.T) {
		// Create a temporary invalid schema
		tmpDir := t.TempDir()
		invalidSchemaPath := filepath.Join(tmpDir, "invalid.yaml")

		// Missing required 'name' field
		invalidYAML := `
schema_version: "1.0"
description: "Missing name field"
common:
  schema:
    type: object
reporters:
  - name: reporter1
    schema:
      type: object
`
		err := os.WriteFile(invalidSchemaPath, []byte(invalidYAML), 0644)
		require.NoError(t, err)

		_, err = LoadUnifiedSchemaFromFile(invalidSchemaPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "name")
	})

	t.Run("requires at least one reporter via JSON Schema", func(t *testing.T) {
		tmpDir := t.TempDir()
		noReportersPath := filepath.Join(tmpDir, "no_reporters.yaml")

		noReportersYAML := `
schema_version: "1.0"
name: test_resource
common:
  schema:
    type: object
reporters: []  # Invalid: no reporters
`
		err := os.WriteFile(noReportersPath, []byte(noReportersYAML), 0644)
		require.NoError(t, err)

		_, err = LoadUnifiedSchemaFromFile(noReportersPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Array must have at least 1 items")
	})

	t.Run("validates relation cardinality enum", func(t *testing.T) {
		tmpDir := t.TempDir()
		invalidCardinalityPath := filepath.Join(tmpDir, "invalid_cardinality.yaml")

		// Invalid cardinality value (not "one" or "many")
		invalidCardinalityYAML := `
schema_version: "1.0"
name: test_resource
common:
  schema:
    type: object
  relations:
    - name: workspace
      target: rbac/workspace
      field: workspace_id
      cardinality: invalid  # Not in enum
reporters:
  - name: test
    schema:
      type: object
`
		err := os.WriteFile(invalidCardinalityPath, []byte(invalidCardinalityYAML), 0644)
		require.NoError(t, err)

		_, err = LoadUnifiedSchemaFromFile(invalidCardinalityPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cardinality")
	})

	t.Run("validates Phase 1 constraint: no cardinality many", func(t *testing.T) {
		tmpDir := t.TempDir()
		manyCardinalityPath := filepath.Join(tmpDir, "many_cardinality.yaml")

		// Valid structure, but "many" not allowed in Phase 1
		manyCardinalityYAML := `
schema_version: "1.0"
name: test_resource
common:
  schema:
    type: object
  relations:
    - name: tags
      target: common/tag
      field: tags
      cardinality: many  # Valid in structure, but blocked in Phase 1
reporters:
  - name: test
    schema:
      type: object
`
		err := os.WriteFile(manyCardinalityPath, []byte(manyCardinalityYAML), 0644)
		require.NoError(t, err)

		_, err = LoadUnifiedSchemaFromFile(manyCardinalityPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cardinality 'many' is not yet supported")
	})

	t.Run("validates required reporter fields via JSON Schema", func(t *testing.T) {
		tmpDir := t.TempDir()
		missingReporterFieldPath := filepath.Join(tmpDir, "missing_reporter_field.yaml")

		// Reporter missing required 'schema' field
		missingFieldYAML := `
schema_version: "1.0"
name: test_resource
common:
  schema:
    type: object
reporters:
  - name: test
    # Missing 'schema' field
`
		err := os.WriteFile(missingReporterFieldPath, []byte(missingFieldYAML), 0644)
		require.NoError(t, err)

		_, err = LoadUnifiedSchemaFromFile(missingReporterFieldPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "schema")
	})
}

func TestLoadUnifiedSchemasFromDirectory(t *testing.T) {
	t.Run("loads all schemas from resources directory", func(t *testing.T) {
		// Use actual schemas directory
		schemasDir := filepath.Join("..", "..", "data", "schema", "resources")

		schemas, err := LoadUnifiedSchemasFromDirectory(schemasDir)
		require.NoError(t, err)

		// Should load at least the host schema
		assert.GreaterOrEqual(t, len(schemas), 1, "Should load at least 1 schema file")

		// Check that each schema has valid structure
		for _, schema := range schemas {
			assert.Equal(t, "1.0", schema.SchemaVersion)
			assert.NotEmpty(t, schema.Name)
			assert.NotNil(t, schema.Common.Schema)
			assert.NotEmpty(t, schema.Reporters)
		}
	})

	t.Run("returns error for non-existent directory", func(t *testing.T) {
		_, err := LoadUnifiedSchemasFromDirectory("/non/existent/path")
		assert.Error(t, err)
	})

	t.Run("returns error for empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := LoadUnifiedSchemasFromDirectory(tmpDir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no .yaml schema files found")
	})
}
