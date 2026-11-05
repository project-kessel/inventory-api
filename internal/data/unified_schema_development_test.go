package data

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevelopmentUnifiedSchemaFixtures(t *testing.T) {
	dir := filepath.Join("..", "..", "development", "unified-schema", "resources")
	schemas, err := LoadUnifiedSchemasFromDirectory(dir)
	require.NoError(t, err)
	require.Len(t, schemas, 7)

	repository, err := NewInMemorySchemaRepositoryFromUnifiedYAMLDir(context.Background(), dir)
	require.NoError(t, err)

	expectedReporters := map[string]string{
		"billing_account":           "features",
		"host":                      "hbi",
		"k8s_cluster":               "acm",
		"k8s_policy":                "acm",
		"notifications_integration": "notifications",
		"service":                   "features",
		"workspace":                 "features",
	}

	for resourceName, reporterName := range expectedReporters {
		resourceType, err := model.NewResourceType(resourceName)
		require.NoError(t, err)
		_, err = repository.GetResourceSchema(context.Background(), resourceType)
		require.NoError(t, err, resourceName)

		reporterType, err := model.NewReporterType(reporterName)
		require.NoError(t, err)
		_, err = repository.GetReporterSchema(context.Background(), resourceType, reporterType)
		assert.NoError(t, err, resourceName+":"+reporterName)
	}
}

// TestDevelopmentUnifiedSchemaFixtures_BillingAccountServicesTuples verifies
// reporter-relation wiring through SchemaService, not just the tuple adapter.
func TestDevelopmentUnifiedSchemaFixtures_BillingAccountServicesTuples(t *testing.T) {
	dir := filepath.Join("..", "..", "development", "unified-schema", "resources")
	repository, err := NewInMemorySchemaRepositoryFromUnifiedYAMLDir(context.Background(), dir)
	require.NoError(t, err)

	schemaService := model.NewSchemaService(repository, log.NewHelper(log.NewStdLogger(io.Discard)))
	resourceType, err := model.NewResourceType("billing_account")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("features")
	require.NoError(t, err)
	reporterInstanceID, err := model.NewReporterInstanceId("instance-1")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("billing-account-1"),
		resourceType,
		reporterType,
		reporterInstanceID,
	)
	require.NoError(t, err)

	reporterVersion := model.NewVersion(1)
	current, err := model.NewRepresentations(
		nil,
		nil,
		model.Representation{
			"services": []interface{}{"service-1", "service-2"},
		},
		&reporterVersion,
	)
	require.NoError(t, err)

	tuples, err := schemaService.CalculateTuplesForResource(context.Background(), current, nil, key)

	require.NoError(t, err)
	require.Len(t, *tuples.TuplesToCreate(), 2)
	assert.Equal(t, "services", (*tuples.TuplesToCreate())[0].Relation().String())
	assert.Equal(t, "services", (*tuples.TuplesToCreate())[1].Relation().String())
	serviceIDs := []string{
		(*tuples.TuplesToCreate())[0].Subject().Resource().ResourceId().String(),
		(*tuples.TuplesToCreate())[1].Subject().Resource().ResourceId().String(),
	}
	assert.ElementsMatch(t, []string{"service-1", "service-2"}, serviceIDs)
}

// TestDevelopmentUnifiedSchemaFixtures_WorkspaceTuplesMatchJSONPath asserts the
// unified workspace:features schema produces exactly the same tuples as the
// hardcoded JSON-path schema (featuresWorkspaceSchema) for identical inputs,
// including the boolean wildcard relations.
func TestDevelopmentUnifiedSchemaFixtures_WorkspaceTuplesMatchJSONPath(t *testing.T) {
	dir := filepath.Join("..", "..", "development", "unified-schema", "resources")
	repository, err := NewInMemorySchemaRepositoryFromUnifiedYAMLDir(context.Background(), dir)
	require.NoError(t, err)

	resourceType, err := model.NewResourceType("workspace")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("features")
	require.NoError(t, err)
	reporterInstanceID, err := model.NewReporterInstanceId("instance-1")
	require.NoError(t, err)
	localResourceID, err := model.NewLocalResourceId("workspace-1")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(localResourceID, resourceType, reporterType, reporterInstanceID)
	require.NoError(t, err)

	unifiedSchema, err := repository.GetReporterSchema(context.Background(), resourceType, reporterType)
	require.NoError(t, err)

	cache := loadSchemaCacheForParity(t)
	jsonPathSchema := NewFeaturesWorkspaceSchemaFromString(cache["workspace:features"])

	currentVersion := model.NewVersion(2)
	current, err := model.NewRepresentations(nil, nil, model.Representation{
		"direct_billing_account":         "billing-account-2",
		"direct_service_preferences":     []interface{}{"service-2", "service-3"},
		"desire_all_services":            true,
		"ignore_inherited_paid_services": true,
	}, &currentVersion)
	require.NoError(t, err)

	previousVersion := model.NewVersion(1)
	previous, err := model.NewRepresentations(nil, nil, model.Representation{
		"direct_billing_account":            "billing-account-1",
		"direct_service_preferences":        []interface{}{"service-1", "service-2"},
		"desire_all_services":               false,
		"ignore_inherited_desired_services": true,
	}, &previousVersion)
	require.NoError(t, err)

	unifiedTuples, err := unifiedSchema.Schema().CalculateTuples(current, previous, key)
	require.NoError(t, err)
	jsonTuples, err := jsonPathSchema.CalculateTuples(current, previous, key)
	require.NoError(t, err)

	require.True(t, jsonTuples.HasTuplesToCreate(), "JSON path must produce create tuples for this input")
	require.True(t, jsonTuples.HasTuplesToDelete(), "JSON path must produce delete tuples for this input")
	assert.ElementsMatch(t, *jsonTuples.TuplesToCreate(), *unifiedTuples.TuplesToCreate(),
		"unified workspace:features schema must produce the same create tuples as the hardcoded JSON path")
	assert.ElementsMatch(t, *jsonTuples.TuplesToDelete(), *unifiedTuples.TuplesToDelete(),
		"unified workspace:features schema must produce the same delete tuples as the hardcoded JSON path")
}

// TestDevelopmentUnifiedSchemaFixtures_MatchJSONSchemaCache asserts the unified
// YAML fixtures declare the same validation surface (properties, types, and
// required fields) as the JSON schemas in schema_cache.json, for every
// resource and reporter. This keeps the two schema sources from drifting.
func TestDevelopmentUnifiedSchemaFixtures_MatchJSONSchemaCache(t *testing.T) {
	dir := filepath.Join("..", "..", "development", "unified-schema", "resources")
	schemas, err := LoadUnifiedSchemasFromDirectory(dir)
	require.NoError(t, err)
	require.Len(t, schemas, 7)

	cache := loadSchemaCacheForParity(t)

	for _, schema := range schemas {
		compareParitySchema(t, cache, "common:"+schema.Name, schema.Common.Schema)
		for _, reporter := range schema.Reporters {
			require.NotNil(t, reporter.Schema, "%s:%s must declare a schema", schema.Name, reporter.Name)
			compareParitySchema(t, cache, schema.Name+":"+reporter.Name, reporter.Schema)
		}
	}
}

// loadSchemaCacheForParity reads schema_cache.json into a map of schema key to
// raw JSON schema string.
func loadSchemaCacheForParity(t *testing.T) map[string]string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "schema_cache.json"))
	require.NoError(t, err)
	var cache map[string]string
	require.NoError(t, json.Unmarshal(contents, &cache))
	return cache
}

// compareParitySchema compares one unified YAML schema section against its
// JSON counterpart: same property names, same types, same required fields.
func compareParitySchema(t *testing.T, cache map[string]string, key string, yamlSchema map[string]interface{}) {
	t.Helper()
	raw, ok := cache[key]
	require.True(t, ok, "schema_cache.json has no entry for %q", key)

	var jsonSchema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &jsonSchema))

	yamlProperties, ok := yamlSchema["properties"].(map[string]interface{})
	require.True(t, ok, "%s: unified schema must define properties", key)

	yamlTypes := make(map[string]string, len(yamlProperties))
	for name, property := range yamlProperties {
		propertySchema, ok := property.(map[string]interface{})
		require.True(t, ok, "%s: property %q must be a schema object", key, name)
		propertyType, _ := propertySchema["type"].(string)
		yamlTypes[name] = propertyType
	}

	jsonTypes := make(map[string]string, len(jsonSchema.Properties))
	for name, property := range jsonSchema.Properties {
		jsonTypes[name] = property.Type
	}

	assert.Equal(t, jsonTypes, yamlTypes, "%s: property names and types must match schema_cache.json", key)

	var yamlRequired []string
	if required, ok := yamlSchema["required"].([]interface{}); ok {
		for _, name := range required {
			yamlRequired = append(yamlRequired, fmt.Sprintf("%v", name))
		}
	}
	assert.ElementsMatch(t, jsonSchema.Required, yamlRequired, "%s: required fields must match schema_cache.json", key)
}
