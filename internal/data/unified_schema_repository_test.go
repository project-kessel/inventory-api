package data

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInMemorySchemaRepositoryFromUnifiedYAMLDir_RegistersSchemas(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "host.yaml"), []byte(validUnifiedSchemaYAML("host")), 0o644))

	repository, err := NewInMemorySchemaRepositoryFromUnifiedYAMLDir(context.Background(), dir)

	require.NoError(t, err)
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	resourceSchema, err := repository.GetResourceSchema(context.Background(), resourceType)
	require.NoError(t, err)

	valid, err := resourceSchema.Schema().Validate(map[string]interface{}{"workspace_id": "workspace-1"})
	assert.True(t, valid)
	assert.NoError(t, err)

	reporterType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	reporterSchema, err := repository.GetReporterSchema(context.Background(), resourceType, reporterType)
	require.NoError(t, err)
	valid, err = reporterSchema.Schema().Validate(map[string]interface{}{"host_id": "host-1"})
	assert.True(t, valid)
	assert.NoError(t, err)
}

func TestNewInMemorySchemaRepositoryFromUnifiedYAMLDir_AllowsReporterWithoutSchema(t *testing.T) {
	dir := t.TempDir()
	contents := validUnifiedSchemaYAML("host") + "\n  - name: rbac\n    description: RBAC reporter\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "host.yaml"), []byte(contents), 0o644))

	repository, err := NewInMemorySchemaRepositoryFromUnifiedYAMLDir(context.Background(), dir)

	require.NoError(t, err)
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("rbac")
	require.NoError(t, err)
	reporterSchema, err := repository.GetReporterSchema(context.Background(), resourceType, reporterType)
	require.NoError(t, err)
	assert.Nil(t, reporterSchema.Schema())
}

func TestNewInMemorySchemaRepositoryFromUnifiedYAMLDir_IsAtomicOnLoadFailure(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "host.yaml"), []byte(validUnifiedSchemaYAML("host")), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cluster.yaml"), []byte(replaceOnce(validUnifiedSchemaYAML("cluster"), "cardinality: one", "cardinality: invalid")), 0o644))

	repository, err := NewInMemorySchemaRepositoryFromUnifiedYAMLDir(context.Background(), dir)

	assert.Nil(t, repository)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster.yaml")
}
