package serve

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/config/schema"
	inmemoryConfig "github.com/project-kessel/inventory-api/internal/config/schema/inmemory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSchemaRepositoryUnifiedYAML(t *testing.T) {
	dir := t.TempDir()
	contents := `schema_version: "source-commit"
name: host
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
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "host.yaml"), []byte(contents), 0o644))

	options := schema.NewOptions()
	options.Repository = schema.InMemoryRepository
	options.InMemory.Type = inmemoryConfig.UnifiedYAMLRepository
	options.InMemory.Path = dir
	config, errors := schema.NewConfig(options).Complete()
	require.Empty(t, errors)

	repository, err := newSchemaRepository(
		context.Background(),
		config,
		log.NewHelper(log.NewStdLogger(io.Discard)),
	)

	require.NoError(t, err)
	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	resourceSchema, err := repository.GetResourceSchema(context.Background(), resourceType)
	require.NoError(t, err)
	valid, err := resourceSchema.Schema().Validate(map[string]interface{}{"workspace_id": "workspace-1"})
	assert.True(t, valid)
	assert.NoError(t, err)
}
