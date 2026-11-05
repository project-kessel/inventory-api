package data

import (
	"context"
	"io"
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
