package data

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadUnifiedSchemaFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host.yaml")
	require.NoError(t, os.WriteFile(path, []byte(validUnifiedSchemaYAML("host")), 0o644))

	schema, err := LoadUnifiedSchemaFromFile(path)

	require.NoError(t, err)
	assert.Equal(t, "source-commit", schema.SchemaVersion)
	assert.Equal(t, "host", schema.Name)
	assert.Equal(t, "Host resource", schema.Description)
	assert.Contains(t, schema.Common.Schema, "properties")
	require.Len(t, schema.Common.Relations, 1)
	assert.Equal(t, "workspace", schema.Common.Relations[0].Name)
	assert.Equal(t, "Workspace relation", schema.Common.Relations[0].Description)
	require.Len(t, schema.Reporters, 1)
	assert.Equal(t, "hbi", schema.Reporters[0].Name)
	assert.Equal(t, "host_id", schema.Reporters[0].Relations[0].Field)
	assert.Equal(t, "Host relation", schema.Reporters[0].Relations[0].Description)
}

func TestLoadUnifiedSchemaFromFile_AllowsReporterWithoutSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host.yaml")
	contents := validUnifiedSchemaYAML("host") + "\n  - name: rbac\n    description: RBAC reporter\n"
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))

	schema, err := LoadUnifiedSchemaFromFile(path)

	require.NoError(t, err)
	require.Len(t, schema.Reporters, 2)
	assert.Nil(t, schema.Reporters[1].Schema)
}

func TestLoadUnifiedSchemaFromFile_RejectsInvalidArtifacts(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		contents      string
		errorContains string
	}{
		{
			name:          "missing description",
			path:          "host.yaml",
			contents:      validUnifiedSchemaYAMLWithoutDescription("host"),
			errorContains: "description",
		},
		{
			name:          "invalid cardinality",
			path:          "host.yaml",
			contents:      replaceOnce(validUnifiedSchemaYAML("host"), "cardinality: one", "cardinality: manyish"),
			errorContains: "cardinality",
		},
		{
			name:          "invalid embedded JSON Schema",
			path:          "host.yaml",
			contents:      replaceOnce(validUnifiedSchemaYAML("host"), "        type: string\n", "        type: unknown\n"),
			errorContains: "invalid embedded JSON Schema",
		},
		{
			name:          "relation references undefined field",
			path:          "host.yaml",
			contents:      replaceOnce(validUnifiedSchemaYAML("host"), "field: workspace_id", "field: missing_id"),
			errorContains: "undefined field",
		},
		{
			name:          "filename does not match resource name",
			path:          "other.yaml",
			contents:      validUnifiedSchemaYAML("host"),
			errorContains: "does not match filename",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tt.path)
			require.NoError(t, os.WriteFile(path, []byte(tt.contents), 0o644))

			_, err := LoadUnifiedSchemaFromFile(path)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorContains)
		})
	}
}

func TestLoadUnifiedSchemasFromDirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "host.yaml"), []byte(validUnifiedSchemaYAML("host")), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cluster.yaml"), []byte(validUnifiedSchemaYAML("cluster")), 0o644))

	schemas, err := LoadUnifiedSchemasFromDirectory(dir)

	require.NoError(t, err)
	require.Len(t, schemas, 2)
	assert.Equal(t, "cluster", schemas[0].Name)
	assert.Equal(t, "host", schemas[1].Name)
}

func TestLoadUnifiedSchemasFromDirectory_RejectsMissingDirectoryContent(t *testing.T) {
	_, err := LoadUnifiedSchemasFromDirectory(t.TempDir())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no unified YAML schemas found")
}

func TestLoadUnifiedSchemasFromDirectory_RejectsExternalSchemaReference(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	dir := t.TempDir()
	contents := replaceOnce(
		validUnifiedSchemaYAML("host"),
		"        type: string\n",
		fmt.Sprintf("        $ref: %q\n", server.URL),
	)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "host.yaml"), []byte(contents), 0o644))

	_, err := LoadUnifiedSchemasFromDirectory(dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "external JSON Schema reference")
	assert.Zero(t, requests.Load(), "external references must be rejected before network resolution")
}

// validUnifiedSchemaYAML returns a valid schema fixture for loader tests.
func validUnifiedSchemaYAML(name string) string {
	return fmt.Sprintf(`schema_version: "source-commit"
name: %s
description: "Host resource"
common:
  schema:
    type: object
    properties:
      workspace_id:
        type: string
    required:
      - workspace_id
  relations:
    - name: workspace
      target: rbac/workspace
      field: workspace_id
      cardinality: one
      description: "Workspace relation"
reporters:
  - name: hbi
    description: "Host-based inventory reporter"
    schema:
      type: object
      properties:
        host_id:
          type: string
      required:
        - host_id
    relations:
      - name: host
        target: hbi/host
        field: host_id
        cardinality: one
        description: "Host relation"
`, name)
}

// validUnifiedSchemaYAMLWithoutDescription returns a fixture missing its description.
func validUnifiedSchemaYAMLWithoutDescription(name string) string {
	return replaceOnce(validUnifiedSchemaYAML(name), "description: \"Host resource\"\n", "")
}

// replaceOnce replaces the first occurrence of old in contents.
func replaceOnce(contents, old, new string) string {
	index := 0
	for index+len(old) <= len(contents) {
		if contents[index:index+len(old)] == old {
			return contents[:index] + new + contents[index+len(old):]
		}
		index++
	}
	return contents
}
