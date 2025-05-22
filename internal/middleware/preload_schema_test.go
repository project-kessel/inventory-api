package middleware_test

import (
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestPreloadAllSchemas_UsesCachePath(t *testing.T) {
	cacheContent := `{"test-key":{"schema":"{}"}}`
	cacheFile := "schema_cache.json"
	err := os.WriteFile(cacheFile, []byte(cacheContent), 0644)
	assert.NoError(t, err)
	defer func() { _ = os.Remove(cacheFile) }()

	// use cache
	viper.Set("resources.use_cache", true)

	err = middleware.PreloadAllSchemas("/bad/directory")
	assert.NoError(t, err)

	val, ok := middleware.SchemaCache.Load("test-key")
	assert.True(t, ok)
	assert.NotNil(t, val)
}

func TestPreloadAllSchemas_FailsOnMissingCache(t *testing.T) {
	viper.Set("resources.use_cache", true)
	_ = os.Remove("schema_cache.json") // ensure it's missing

	err := middleware.PreloadAllSchemas("/bad/directory")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read schema cache file")
}

func TestPreloadAllSchemas_FailsOnFilesystemError(t *testing.T) {
	viper.Set("resources.use_cache", false)
	err := middleware.PreloadAllSchemas("/bad/directory")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no directories inside schema directory")
}

func TestLoadSchemaCacheFromJSON_MissingFile(t *testing.T) {
	err := middleware.LoadSchemaCacheFromJSON("/bad/directory/schema_cache.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read schema cache file")
}

func TestLoadSchemaCacheFromJSON_ValidFile(t *testing.T) {
	cacheContent := `{"resourceA": {"schema": "{}"}}`
	tmpFile, err := os.CreateTemp("", "schema_cache.json")
	assert.NoError(t, err)

	_, err = tmpFile.WriteString(cacheContent)
	assert.NoError(t, err)
	err = tmpFile.Close()
	assert.NoError(t, err)

	err = middleware.LoadSchemaCacheFromJSON(tmpFile.Name())
	assert.NoError(t, err)

}

func TestLoadSchemaCacheFromJSON_InvalidJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "schema_cache_*.json")
	assert.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString("{not-json:") // Malformed JSON
	assert.NoError(t, err)
	err = tmpFile.Close()
	assert.NoError(t, err)

	err = middleware.LoadSchemaCacheFromJSON(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal schema cache JSON")
}

func TestGetSchemaFromCache_NotFound(t *testing.T) {
	// cache key does not exist
	cacheKey := "bad-key"
	middleware.SchemaCache.Delete(cacheKey)

	schema, err := middleware.GetSchemaFromCache(cacheKey)
	assert.Error(t, err)
	assert.Empty(t, schema)
	assert.Contains(t, err.Error(), "schema not found")
}
