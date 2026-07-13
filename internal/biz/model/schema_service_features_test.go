package model_test

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupFeaturesRepo(t *testing.T) *data.InMemorySchemaRepository {
	t.Helper()
	ctx := context.Background()

	repo, err := data.NewInMemorySchemaRepositoryFromDir(
		ctx,
		"../../../data/schema/resources",
		data.FeaturesAwareSchemaFactory,
	)
	require.NoError(t, err)
	return repo
}

func serviceKey(t *testing.T) model.ReporterResourceKey {
	t.Helper()
	resourceType, err := model.NewResourceType("service")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("features")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("features-instance")
	require.NoError(t, err)
	localResourceId, err := model.NewLocalResourceId("svc-001")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		localResourceId,
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)
	return key
}

func billingAccountKey(t *testing.T) model.ReporterResourceKey {
	t.Helper()
	resourceType, err := model.NewResourceType("billing_account")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("features")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("features-instance")
	require.NoError(t, err)
	localResourceId, err := model.NewLocalResourceId("ba-001")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		localResourceId,
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)
	return key
}

func TestFeaturesService_CreateTuples(t *testing.T) {
	repo := setupFeaturesRepo(t)
	sc := model.NewSchemaService(repo, log.NewHelper(log.DefaultLogger))
	ctx := context.Background()
	key := serviceKey(t)

	ver := model.NewVersion(0)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspaces": []interface{}{"ws-1", "ws-2"},
			"billing_account":    []interface{}{"ba-100"},
			"parent":             "parent-svc",
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := sc.CalculateTuplesForResource(ctx, current, nil, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate())
	assert.False(t, result.HasTuplesToDelete())

	creates := *result.TuplesToCreate()
	expected := []model.RelationsTuple{
		model.NewRelationTupleForSubject(key, "allowed_workspaces", "rbac", "workspace", "ws-1"),
		model.NewRelationTupleForSubject(key, "allowed_workspaces", "rbac", "workspace", "ws-2"),
		model.NewRelationTupleForSubject(key, "billing_account", "features", "billing_account", "ba-100"),
		model.NewRelationTupleForSubject(key, "parent", "features", "service", "parent-svc"),
	}
	assert.ElementsMatch(t, expected, creates)
}

func TestFeaturesService_UpdateTuples(t *testing.T) {
	repo := setupFeaturesRepo(t)
	sc := model.NewSchemaService(repo, log.NewHelper(log.DefaultLogger))
	ctx := context.Background()
	key := serviceKey(t)

	ver1 := model.NewVersion(1)
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspaces": []interface{}{"ws-1", "ws-2"},
			"billing_account":    []interface{}{"ba-100"},
			"parent":             "parent-svc",
		}),
		&ver1, nil, nil,
	)
	require.NoError(t, err)

	ver2 := model.NewVersion(2)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspaces": []interface{}{"ws-2", "ws-3"},
			"billing_account":    []interface{}{"ba-200"},
			"parent":             "parent-svc",
		}),
		&ver2, nil, nil,
	)
	require.NoError(t, err)

	result, err := sc.CalculateTuplesForResource(ctx, current, previous, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate())
	assert.True(t, result.HasTuplesToDelete())

	creates := *result.TuplesToCreate()
	deletes := *result.TuplesToDelete()

	expectedCreates := []model.RelationsTuple{
		model.NewRelationTupleForSubject(key, "allowed_workspaces", "rbac", "workspace", "ws-3"),
		model.NewRelationTupleForSubject(key, "billing_account", "features", "billing_account", "ba-200"),
	}
	assert.ElementsMatch(t, expectedCreates, creates)

	expectedDeletes := []model.RelationsTuple{
		model.NewRelationTupleForSubject(key, "allowed_workspaces", "rbac", "workspace", "ws-1"),
		model.NewRelationTupleForSubject(key, "billing_account", "features", "billing_account", "ba-100"),
	}
	assert.ElementsMatch(t, expectedDeletes, deletes)
}

func TestFeaturesService_DeleteTuples(t *testing.T) {
	repo := setupFeaturesRepo(t)
	sc := model.NewSchemaService(repo, log.NewHelper(log.DefaultLogger))
	ctx := context.Background()
	key := serviceKey(t)

	ver := model.NewVersion(1)
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"allowed_workspaces": []interface{}{"ws-1"},
			"billing_account":    []interface{}{"ba-100"},
			"parent":             "parent-svc",
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := sc.CalculateTuplesForResource(ctx, nil, previous, key)
	require.NoError(t, err)

	assert.False(t, result.HasTuplesToCreate())
	assert.True(t, result.HasTuplesToDelete())

	deletes := *result.TuplesToDelete()
	assert.Len(t, deletes, 3) // 1 allowed_workspaces + 1 billing_account + 1 parent
}

func TestFeaturesService_NoChange(t *testing.T) {
	repo := setupFeaturesRepo(t)
	sc := model.NewSchemaService(repo, log.NewHelper(log.DefaultLogger))
	ctx := context.Background()
	key := serviceKey(t)

	sameData := map[string]interface{}{
		"allowed_workspaces": []interface{}{"ws-1"},
		"billing_account":    []interface{}{"ba-100"},
		"parent":             "parent-svc",
	}

	ver1 := model.NewVersion(1)
	previous, err := model.NewRepresentations(
		model.Representation(sameData), &ver1, nil, nil,
	)
	require.NoError(t, err)

	ver2 := model.NewVersion(2)
	current, err := model.NewRepresentations(
		model.Representation(sameData), &ver2, nil, nil,
	)
	require.NoError(t, err)

	result, err := sc.CalculateTuplesForResource(ctx, current, previous, key)
	require.NoError(t, err)

	assert.False(t, result.HasTuplesToCreate())
	assert.False(t, result.HasTuplesToDelete())
}

func TestFeaturesBillingAccount_MultiWorkspaceTuples(t *testing.T) {
	repo := setupFeaturesRepo(t)
	sc := model.NewSchemaService(repo, log.NewHelper(log.DefaultLogger))
	ctx := context.Background()
	key := billingAccountKey(t)

	ver := model.NewVersion(0)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspaces": []interface{}{"ws-billing-1", "ws-billing-2"},
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := sc.CalculateTuplesForResource(ctx, current, nil, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate())
	assert.False(t, result.HasTuplesToDelete())

	creates := *result.TuplesToCreate()
	expected := []model.RelationsTuple{
		model.NewRelationTupleForSubject(key, "workspace", "rbac", "workspace", "ws-billing-1"),
		model.NewRelationTupleForSubject(key, "workspace", "rbac", "workspace", "ws-billing-2"),
	}
	assert.ElementsMatch(t, expected, creates)
}

func TestFeaturesSchemaFactory_FallsBackForOtherTypes(t *testing.T) {
	repo := setupFeaturesRepo(t)
	sc := model.NewSchemaService(repo, log.NewHelper(log.DefaultLogger))
	ctx := context.Background()

	resourceType, err := model.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("HBI")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)
	localResourceId, err := model.NewLocalResourceId("test-host")
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(
		localResourceId,
		resourceType, reporterType, reporterInstanceId,
	)
	require.NoError(t, err)

	ver := model.NewVersion(0)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id": "ws-host",
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := sc.CalculateTuplesForResource(ctx, current, nil, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate())
	creates := *result.TuplesToCreate()
	expected := []model.RelationsTuple{
		model.NewRelationTupleForSubject(key, "workspace", "rbac", "workspace", "ws-host"),
	}
	assert.ElementsMatch(t, expected, creates)
}
