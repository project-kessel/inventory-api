package middleware_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadResourceSchema(t *testing.T) {
	tmpDir := t.TempDir()
	resourceType := "host"
	reporterType := "hbi"
	dirPath := filepath.Join(tmpDir, resourceType, "reporters", reporterType)
	require.NoError(t, os.MkdirAll(dirPath, 0755))

	schemaPath := filepath.Join(dirPath, resourceType+".json")
	sample := `{"type":"object"}`
	require.NoError(t, os.WriteFile(schemaPath, []byte(sample), 0644))

	data, exists, err := middleware.LoadResourceSchema(resourceType, reporterType, tmpDir)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, sample, data)
}

func TestLoadCommonRepresentationDataSchema(t *testing.T) {
	tmpDir := t.TempDir()
	resourceType := "host"
	dirPath := filepath.Join(tmpDir, resourceType)
	require.NoError(t, os.MkdirAll(dirPath, 0755))

	schemaPath := filepath.Join(dirPath, "common_representation.json")
	sample := `{"type":"common"}`
	require.NoError(t, os.WriteFile(schemaPath, []byte(sample), 0644))

	data, err := middleware.LoadCommonRepresentationDataSchema(resourceType, tmpDir)
	require.NoError(t, err)
	assert.Equal(t, sample, data)
}
