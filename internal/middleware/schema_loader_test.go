package middleware_test

import (
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateResourceReporterCombination_Valid(t *testing.T) {
	viper.Set("resources.use_cache", true)
	resourceType := "testres"
	reporterType := "repA"
	config := map[string]interface{}{"resource_reporters": []string{"repA", "repB"}}
	middleware.SchemaCache.Store("config:"+resourceType, config)

	err := middleware.ValidateResourceReporterCombination(resourceType, reporterType)
	assert.NoError(t, err)
}

func TestValidateResourceReporterCombination_Invalid(t *testing.T) {
	viper.Set("resources.use_cache", true)
	resourceType := "testres"
	reporterType := "repC"
	config := map[string]interface{}{"resource_reporters": []string{"repA", "repB"}}
	middleware.SchemaCache.Store("config:"+resourceType, config)

	err := middleware.ValidateResourceReporterCombination(resourceType, reporterType)
	assert.Contains(t, err.Error(), "invalid reporter_type: repC for resource_type: testres")
}

func TestValidateResourceReporterCombination_ConfigNotFound(t *testing.T) {
	viper.Set("resources.use_cache", true)
	resourceType := "notfound"
	reporterType := "repA"
	middleware.SchemaCache.Delete("config:" + resourceType)

	err := middleware.ValidateResourceReporterCombination(resourceType, reporterType)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load valid reporters")
}

func TestLoadResourceSchema_Valid(t *testing.T) {
	dir := t.TempDir()
	resourceType := "foo"
	reporterType := "bar"
	path := filepath.Join(dir, resourceType, "reporters", reporterType)
	err := os.MkdirAll(path, 0755)
	assert.NoError(t, err)
	schemaFile := filepath.Join(path, resourceType+".json")
	expected := `{"type":"object"}`
	err = os.WriteFile(schemaFile, []byte(expected), 0644)
	assert.NoError(t, err)

	schema, exists, err := middleware.LoadResourceSchema(resourceType, reporterType, dir)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, expected, schema)
}

func TestLoadResourceSchema_NotFound(t *testing.T) {
	dir := t.TempDir()
	resourceType := "foo"
	reporterType := "nope"

	schema, exists, err := middleware.LoadResourceSchema(resourceType, reporterType, dir)
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Equal(t, "", schema)
}

func TestLoadCommonResourceDataSchema_Valid(t *testing.T) {
	dir := t.TempDir()
	resourceType := "foo"
	err := os.MkdirAll(filepath.Join(dir, resourceType), 0755)
	assert.NoError(t, err)
	schemaPath := filepath.Join(dir, resourceType, "common_representation.json")
	expected := `{"type":"object"}`
	err = os.WriteFile(schemaPath, []byte(expected), 0644)
	assert.NoError(t, err)

	schema, err := middleware.LoadCommonResourceDataSchema(resourceType, dir)
	assert.NoError(t, err)
	assert.Equal(t, expected, schema)
}

func TestLoadCommonResourceDataSchema_NotFound(t *testing.T) {
	dir := t.TempDir()
	resourceType := "bar"

	schema, err := middleware.LoadCommonResourceDataSchema(resourceType, dir)
	assert.Error(t, err)
	assert.Empty(t, schema)
	assert.Contains(t, err.Error(), "failed to read common resource schema")
}

func TestLoadValidReporters_FromCache_JSON(t *testing.T) {
	viper.Set("resources.use_cache", true)
	resourceType := "jsonres"
	config := map[string]interface{}{"resource_reporters": []string{"repA"}}
	middleware.SchemaCache.Store("config:"+resourceType, config)
	defer middleware.SchemaCache.Delete("config:" + resourceType)

	reporters, err := middleware.LoadValidReporters(resourceType)
	assert.NoError(t, err)
	assert.Equal(t, []string{"repA"}, reporters)
}

func TestLoadValidReporters_FromFilesystem_ConfigNotFound(t *testing.T) {
	viper.Set("resources.use_cache", false)
	resourceType := "notfound"
	middleware.SchemaCache.Delete("config:" + resourceType)

	reporters, err := middleware.LoadValidReporters(resourceType)
	assert.Error(t, err)
	assert.Nil(t, reporters)
}
