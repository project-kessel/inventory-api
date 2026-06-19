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
		data.NewFeaturesAwareSchemaFactory(),
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
	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("svc-001"),
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
	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("ba-001"),
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
			"workspace_id":         "ws-main",
			"allowed_workspace_ids": []interface{}{"ws-1", "ws-2"},
			"billing_account_ids":  []interface{}{"ba-100"},
			"parent_service_id":    "parent-svc",
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := sc.CalculateTuplesForResource(ctx, current, nil, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate())
	assert.False(t, result.HasTuplesToDelete())

	creates := *result.TuplesToCreate()
	assert.Len(t, creates, 4) // 2 allowed_workspaces + 1 billing_account + 1 parent

	type tupleInfo struct {
		relation        string
		subjectType     string
		subjectReporter string
		subjectId       string
	}
	var tuples []tupleInfo
	for _, tuple := range creates {
		tuples = append(tuples, tupleInfo{
			relation:        tuple.Relation().Serialize(),
			subjectType:     tuple.Subject().Resource().ResourceType().Serialize(),
			subjectReporter: tuple.Subject().Resource().Reporter().ReporterType().Serialize(),
			subjectId:       tuple.Subject().Resource().ResourceId().Serialize(),
		})
	}

	assert.Contains(t, tuples, tupleInfo{"allowed_workspaces", "workspace", "rbac", "ws-1"})
	assert.Contains(t, tuples, tupleInfo{"allowed_workspaces", "workspace", "rbac", "ws-2"})
	assert.Contains(t, tuples, tupleInfo{"billing_account", "billing_account", "features", "ba-100"})
	assert.Contains(t, tuples, tupleInfo{"parent", "service", "features", "parent-svc"})
}

func TestFeaturesService_UpdateTuples(t *testing.T) {
	repo := setupFeaturesRepo(t)
	sc := model.NewSchemaService(repo, log.NewHelper(log.DefaultLogger))
	ctx := context.Background()
	key := serviceKey(t)

	ver1 := model.NewVersion(1)
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id":         "ws-main",
			"allowed_workspace_ids": []interface{}{"ws-1", "ws-2"},
			"billing_account_ids":  []interface{}{"ba-100"},
			"parent_service_id":    "parent-svc",
		}),
		&ver1, nil, nil,
	)
	require.NoError(t, err)

	ver2 := model.NewVersion(2)
	current, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id":         "ws-main",
			"allowed_workspace_ids": []interface{}{"ws-2", "ws-3"},
			"billing_account_ids":  []interface{}{"ba-200"},
			"parent_service_id":    "parent-svc",
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

	// ws-3 added, ba-200 added
	assert.Len(t, creates, 2)
	// ws-1 removed, ba-100 removed
	assert.Len(t, deletes, 2)

	type tupleInfo struct {
		relation  string
		subjectId string
	}
	var createInfos, deleteInfos []tupleInfo
	for _, tuple := range creates {
		createInfos = append(createInfos, tupleInfo{
			relation:  tuple.Relation().Serialize(),
			subjectId: tuple.Subject().Resource().ResourceId().Serialize(),
		})
	}
	for _, tuple := range deletes {
		deleteInfos = append(deleteInfos, tupleInfo{
			relation:  tuple.Relation().Serialize(),
			subjectId: tuple.Subject().Resource().ResourceId().Serialize(),
		})
	}

	assert.Contains(t, createInfos, tupleInfo{"allowed_workspaces", "ws-3"})
	assert.Contains(t, createInfos, tupleInfo{"billing_account", "ba-200"})
	assert.Contains(t, deleteInfos, tupleInfo{"allowed_workspaces", "ws-1"})
	assert.Contains(t, deleteInfos, tupleInfo{"billing_account", "ba-100"})
}

func TestFeaturesService_DeleteTuples(t *testing.T) {
	repo := setupFeaturesRepo(t)
	sc := model.NewSchemaService(repo, log.NewHelper(log.DefaultLogger))
	ctx := context.Background()
	key := serviceKey(t)

	ver := model.NewVersion(1)
	previous, err := model.NewRepresentations(
		model.Representation(map[string]interface{}{
			"workspace_id":         "ws-main",
			"allowed_workspace_ids": []interface{}{"ws-1"},
			"billing_account_ids":  []interface{}{"ba-100"},
			"parent_service_id":    "parent-svc",
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
		"workspace_id":         "ws-main",
		"allowed_workspace_ids": []interface{}{"ws-1"},
		"billing_account_ids":  []interface{}{"ba-100"},
		"parent_service_id":    "parent-svc",
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
			"workspace_ids": []interface{}{"ws-billing-1", "ws-billing-2"},
		}),
		&ver, nil, nil,
	)
	require.NoError(t, err)

	result, err := sc.CalculateTuplesForResource(ctx, current, nil, key)
	require.NoError(t, err)

	assert.True(t, result.HasTuplesToCreate())
	assert.False(t, result.HasTuplesToDelete())

	creates := *result.TuplesToCreate()
	assert.Len(t, creates, 2)

	type tupleInfo struct {
		relation    string
		subjectId   string
		subjectType string
		namespace   string
	}
	var tuples []tupleInfo
	for _, tuple := range creates {
		tuples = append(tuples, tupleInfo{
			relation:    tuple.Relation().Serialize(),
			subjectId:   tuple.Subject().Resource().ResourceId().Serialize(),
			subjectType: tuple.Subject().Resource().ResourceType().Serialize(),
			namespace:   tuple.Subject().Resource().Reporter().ReporterType().Serialize(),
		})
	}
	assert.Contains(t, tuples, tupleInfo{"workspace", "ws-billing-1", "workspace", "rbac"})
	assert.Contains(t, tuples, tupleInfo{"workspace", "ws-billing-2", "workspace", "rbac"})
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
	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("test-host"),
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
	assert.Len(t, creates, 1)
	assert.Equal(t, "workspace", creates[0].Relation().Serialize())
	assert.Equal(t, "ws-host", creates[0].Subject().Resource().ResourceId().Serialize())
}
