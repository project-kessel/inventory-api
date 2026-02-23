package resources

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authz/allow"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/subject/selfsubject"
)

func testAuthzContext() context.Context {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("test-user"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	return authnapi.NewAuthzContext(context.Background(), authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject:  claims,
	})
}

type testSelfSubjectStrategy struct{}

func (testSelfSubjectStrategy) SubjectFromAuthorizationContext(authzContext authnapi.AuthzContext) (model.SubjectReference, error) {
	if !authzContext.IsAuthenticated() || authzContext.Subject.SubjectId == "" {
		return model.SubjectReference{}, fmt.Errorf("subject claims not found")
	}
	subjectID := string(authzContext.Subject.SubjectId)
	return buildTestSubjectReference(subjectID)
}

// buildTestSubjectReference creates a model.SubjectReference for testing.
func buildTestSubjectReference(subjectID string) (model.SubjectReference, error) {
	localResourceId, err := model.NewLocalResourceId(subjectID)
	if err != nil {
		return model.SubjectReference{}, err
	}
	resourceType, err := model.NewResourceType("principal")
	if err != nil {
		return model.SubjectReference{}, err
	}
	reporterType, err := model.NewReporterType("rbac")
	if err != nil {
		return model.SubjectReference{}, err
	}
	key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, model.ReporterInstanceId(""))
	if err != nil {
		return model.SubjectReference{}, err
	}
	return model.NewSubjectReferenceWithoutRelation(key), nil
}

func newTestSelfSubjectStrategy() selfsubject.SelfSubjectStrategy {
	return testSelfSubjectStrategy{}
}

type recordingMetaAuthorizer struct {
	allowed   bool
	err       error
	relations []metaauthorizer.Relation
	calls     int
}

func (r *recordingMetaAuthorizer) Check(_ context.Context, _ metaauthorizer.MetaObject, relation metaauthorizer.Relation, _ authnapi.AuthzContext) (bool, error) {
	r.calls++
	r.relations = append(r.relations, relation)
	return r.allowed, r.err
}

func TestCheckSelf_UsesCheckSelfRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	usecase := New(
		data.NewFakeResourceRepository(),
		newFakeSchemaRepository(t),
		&allow.AllowAllAuthz{},
		"rbac",
		log.DefaultLogger,
		nil,
		nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		meta,
		newTestSelfSubjectStrategy(),
	)

	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	allowed, err := usecase.CheckSelf(ctx, relation, key)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationCheckSelf}, meta.relations)
}

func TestCheckSelf_DeniedByMetaAuthz(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: false}
	usecase := New(
		data.NewFakeResourceRepository(),
		newFakeSchemaRepository(t),
		&allow.AllowAllAuthz{},
		"rbac",
		log.DefaultLogger,
		nil,
		nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		meta,
		newTestSelfSubjectStrategy(),
	)

	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	_, err = usecase.CheckSelf(ctx, relation, key)
	assert.ErrorIs(t, err, ErrMetaAuthorizationDenied)
}

func TestCheckSelf_MissingAuthzContext(t *testing.T) {
	meta := &recordingMetaAuthorizer{allowed: true}
	usecase := New(
		data.NewFakeResourceRepository(),
		newFakeSchemaRepository(t),
		&allow.AllowAllAuthz{},
		"rbac",
		log.DefaultLogger,
		nil,
		nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		meta,
		newTestSelfSubjectStrategy(),
	)

	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	_, err = usecase.CheckSelf(context.Background(), relation, key)
	assert.ErrorIs(t, err, ErrMetaAuthzContextMissing)
	assert.Equal(t, 0, meta.calls)
}

func TestReportResource_UsesReportRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	usecase := New(
		data.NewFakeResourceRepository(),
		newFakeSchemaRepository(t),
		&allow.AllowAllAuthz{},
		"rbac",
		log.DefaultLogger,
		nil,
		nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		meta,
		newTestSelfSubjectStrategy(),
	)

	cmd := fixture(t).Basic("host", "hbi", "instance-1", "host-1", "workspace-1")
	err := usecase.ReportResource(ctx, cmd)
	require.NoError(t, err)
	assert.Equal(t, 1, meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationReportResource}, meta.relations)
}

func TestDelete_UsesDeleteRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	usecase := New(
		data.NewFakeResourceRepository(),
		newFakeSchemaRepository(t),
		&allow.AllowAllAuthz{},
		"rbac",
		log.DefaultLogger,
		nil,
		nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		meta,
		newTestSelfSubjectStrategy(),
	)

	cmd := fixture(t).Basic("host", "hbi", "instance-1", "host-1", "workspace-1")
	err := usecase.ReportResource(ctx, cmd)
	require.NoError(t, err)

	meta.calls = 0
	meta.relations = nil

	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	err = usecase.Delete(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, 1, meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationDeleteResource}, meta.relations)
}

func TestCheck_UsesCheckRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	usecase := New(
		data.NewFakeResourceRepository(),
		newFakeSchemaRepository(t),
		&allow.AllowAllAuthz{},
		"rbac",
		log.DefaultLogger,
		nil,
		nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		meta,
		newTestSelfSubjectStrategy(),
	)

	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	allowed, err := usecase.Check(ctx, relation, subject, key)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationCheck}, meta.relations)
}

func TestCheckForUpdate_UsesCheckForUpdateRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	usecase := New(
		data.NewFakeResourceRepository(),
		newFakeSchemaRepository(t),
		&allow.AllowAllAuthz{},
		"rbac",
		log.DefaultLogger,
		nil,
		nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		meta,
		newTestSelfSubjectStrategy(),
	)

	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	allowed, err := usecase.CheckForUpdate(ctx, relation, subject, key)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationCheckForUpdate}, meta.relations)
}

func TestCheckSelfBulk_UsesCheckSelfRelationForEachItem(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	usecase := New(
		data.NewFakeResourceRepository(),
		newFakeSchemaRepository(t),
		&allow.AllowAllAuthz{},
		"rbac",
		log.DefaultLogger,
		nil,
		nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		meta,
		newTestSelfSubjectStrategy(),
	)

	key1 := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	key2 := createReporterResourceKey(t, "host-2", "host", "hbi", "instance-1")
	viewRelation, err := model.NewRelation("view")
	require.NoError(t, err)
	editRelation, err := model.NewRelation("edit")
	require.NoError(t, err)

	cmd := CheckSelfBulkCommand{
		Items: []CheckSelfBulkItem{
			{Resource: key1, Relation: viewRelation},
			{Resource: key2, Relation: editRelation},
		},
		Consistency: model.NewConsistencyMinimizeLatency(),
	}

	resp, err := usecase.CheckSelfBulk(ctx, cmd)
	require.NoError(t, err)
	require.Len(t, resp.Pairs, 2)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationCheckSelf, metaauthorizer.RelationCheckSelf}, meta.relations)
}

func TestCheckSelfBulk_MissingAuthzContext(t *testing.T) {
	meta := &recordingMetaAuthorizer{allowed: true}
	usecase := New(
		data.NewFakeResourceRepository(),
		newFakeSchemaRepository(t),
		&allow.AllowAllAuthz{},
		"rbac",
		log.DefaultLogger,
		nil,
		nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		meta,
		newTestSelfSubjectStrategy(),
	)

	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	viewRelation, err := model.NewRelation("view")
	require.NoError(t, err)

	cmd := CheckSelfBulkCommand{
		Items: []CheckSelfBulkItem{
			{Resource: key, Relation: viewRelation},
		},
		Consistency: model.NewConsistencyMinimizeLatency(),
	}

	_, err = usecase.CheckSelfBulk(context.Background(), cmd)
	assert.ErrorIs(t, err, ErrMetaAuthzContextMissing)
	assert.Equal(t, 0, meta.calls)
}

func newFakeSchemaRepository(t *testing.T) model.SchemaRepository {
	schemaRepository := data.NewInMemorySchemaRepository()

	emptyValidationSchema := model.NewJsonSchemaValidatorFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
		},
		"required": []
	}`)

	withWorkspaceValidationSchema := model.NewJsonSchemaValidatorFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"workspace_id": { "type": "string" }
		},
		"required": ["workspace_id"]
	}`)

	err := schemaRepository.CreateResourceSchema(context.Background(), model.ResourceSchema{
		ResourceType:     "k8s_cluster",
		ValidationSchema: withWorkspaceValidationSchema,
	})
	assert.NoError(t, err)

	err = schemaRepository.CreateReporterSchema(context.Background(), model.ReporterSchema{
		ResourceType:     "k8s_cluster",
		ReporterType:     "ocm",
		ValidationSchema: emptyValidationSchema,
	})
	assert.NoError(t, err)

	err = schemaRepository.CreateResourceSchema(context.Background(), model.ResourceSchema{
		ResourceType:     "host",
		ValidationSchema: withWorkspaceValidationSchema,
	})
	assert.NoError(t, err)

	err = schemaRepository.CreateReporterSchema(context.Background(), model.ReporterSchema{
		ResourceType:     "host",
		ReporterType:     "hbi",
		ValidationSchema: emptyValidationSchema,
	})
	assert.NoError(t, err)

	return schemaRepository
}

func TestReportResource(t *testing.T) {
	tests := []struct {
		name             string
		resourceType     string
		reporterType     string
		reporterInstance string
		localResourceId  string
		workspaceId      string
		expectError      bool
	}{
		{
			name:             "creates new k8s cluster resource",
			resourceType:     "k8s_cluster",
			reporterType:     "ocm",
			reporterInstance: "test-instance",
			localResourceId:  "test-local-resource",
			workspaceId:      "test-workspace",
			expectError:      false,
		},
		{
			name:             "creates new host resource",
			resourceType:     "host",
			reporterType:     "hbi",
			reporterInstance: "hbi-instance",
			localResourceId:  "test-host-123",
			workspaceId:      "test-workspace-2",
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testAuthzContext()
			logger := log.DefaultLogger

			resourceRepo := data.NewFakeResourceRepository()
			schemaRepo := newFakeSchemaRepository(t)
			authorizer := &allow.AllowAllAuthz{}
			usecaseConfig := &UsecaseConfig{
				ReadAfterWriteEnabled: false,
				ConsumerEnabled:       false,
			}
			mc := metricscollector.NewFakeMetricsCollector()
			usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())

			cmd := fixture(t).Basic(tt.resourceType, tt.reporterType, tt.reporterInstance, tt.localResourceId, tt.workspaceId)
			err := usecase.ReportResource(ctx, cmd)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			localResourceId, err := model.NewLocalResourceId(tt.localResourceId)
			require.NoError(t, err)
			resourceType, err := model.NewResourceType(tt.resourceType)
			require.NoError(t, err)
			reporterType, err := model.NewReporterType(tt.reporterType)
			require.NoError(t, err)
			reporterInstanceId, err := model.NewReporterInstanceId(tt.reporterInstance)
			require.NoError(t, err)

			key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
			require.NoError(t, err)

			foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err)
			require.NotNil(t, foundResource)

			// Report should trigger a single outbox event write
			assert.Equal(t, 1, metricscollector.GetOutboxEventWriteCount())
		})
	}
}

func TestReportResourceThenDelete(t *testing.T) {
	tests := []struct {
		name                     string
		resourceType             string
		reporterType             string
		reporterInstanceId       string
		localResourceId          string
		workspaceId              string
		deleteReporterInstanceId string
		expectError              bool
	}{
		{
			name:                     "deletes resource with reporterInstanceId",
			resourceType:             "k8s_cluster",
			reporterType:             "ocm",
			reporterInstanceId:       "delete-test-instance",
			localResourceId:          "delete-test-resource",
			workspaceId:              "delete-test-workspace",
			deleteReporterInstanceId: "delete-test-instance",
			expectError:              false,
		},
		{
			name:                     "deletes resource without reporterInstanceId",
			resourceType:             "host",
			reporterType:             "hbi",
			reporterInstanceId:       "delete-test-instance-2",
			localResourceId:          "delete-test-resource-2",
			workspaceId:              "delete-test-workspace-2",
			deleteReporterInstanceId: "",
			expectError:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testAuthzContext()
			logger := log.DefaultLogger

			resourceRepo := data.NewFakeResourceRepository()
			schemaRepo := newFakeSchemaRepository(t)
			authorizer := &allow.AllowAllAuthz{}
			usecaseConfig := &UsecaseConfig{
				ReadAfterWriteEnabled: false,
				ConsumerEnabled:       false,
			}
			mc := metricscollector.NewFakeMetricsCollector()
			usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())

			cmd := fixture(t).Basic(tt.resourceType, tt.reporterType, tt.reporterInstanceId, tt.localResourceId, tt.workspaceId)
			err := usecase.ReportResource(ctx, cmd)
			require.NoError(t, err)

			localResourceId, err := model.NewLocalResourceId(tt.localResourceId)
			require.NoError(t, err)
			resourceType, err := model.NewResourceType(tt.resourceType)
			require.NoError(t, err)
			reporterType, err := model.NewReporterType(tt.reporterType)
			require.NoError(t, err)
			reporterInstanceId, err := model.NewReporterInstanceId(tt.reporterInstanceId)
			require.NoError(t, err)

			key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
			require.NoError(t, err)

			foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err)
			require.NotNil(t, foundResource)

			// Report should trigger a single outbox event write
			assert.Equal(t, 1, metricscollector.GetOutboxEventWriteCount())

			var deleteReporterInstanceId model.ReporterInstanceId
			if tt.deleteReporterInstanceId != "" {
				deleteReporterInstanceId, err = model.NewReporterInstanceId(tt.deleteReporterInstanceId)
				require.NoError(t, err)
			} else {
				deleteReporterInstanceId = model.ReporterInstanceId("")
			}

			deleteKey, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, deleteReporterInstanceId)
			require.NoError(t, err)

			deleteFoundResource, err := resourceRepo.FindResourceByKeys(nil, deleteKey)
			require.NoError(t, err)
			require.NotNil(t, deleteFoundResource)

			err = usecase.Delete(ctx, key)
			require.NoError(t, err)

			// Delete should trigger another outbox event write
			assert.Equal(t, 2, metricscollector.GetOutboxEventWriteCount())
		})
	}
}

func TestDelete_ResourceNotFound(t *testing.T) {
	logger := log.DefaultLogger

	resourceRepo := data.NewFakeResourceRepository()
	schemaRepo := newFakeSchemaRepository(t)
	authorizer := &allow.AllowAllAuthz{}
	usecaseConfig := &UsecaseConfig{
		ReadAfterWriteEnabled: false,
		ConsumerEnabled:       false,
	}

	mc := metricscollector.NewFakeMetricsCollector()
	usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())

	localResourceId, err := model.NewLocalResourceId("non-existent-resource")
	require.NoError(t, err)
	resourceType, err := model.NewResourceType("k8s_cluster")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("ocm")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)

	key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
	require.NoError(t, err)

	err = usecase.Delete(testAuthzContext(), key)
	require.Error(t, err)
}

func TestReportFindDeleteFind_TombstoneLifecycle(t *testing.T) {
	ctx := testAuthzContext()
	logger := log.DefaultLogger

	resourceRepo := data.NewFakeResourceRepository()
	schemaRepo := newFakeSchemaRepository(t)
	authorizer := &allow.AllowAllAuthz{}
	usecaseConfig := &UsecaseConfig{
		ReadAfterWriteEnabled: false,
		ConsumerEnabled:       false,
	}

	mc := metricscollector.NewFakeMetricsCollector()
	usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())

	cmd := fixture(t).Basic("k8s_cluster", "ocm", "lifecycle-instance", "lifecycle-resource", "lifecycle-workspace")
	err := usecase.ReportResource(ctx, cmd)
	require.NoError(t, err)

	localResourceId, err := model.NewLocalResourceId("lifecycle-resource")
	require.NoError(t, err)
	resourceType, err := model.NewResourceType("k8s_cluster")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("ocm")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("lifecycle-instance")
	require.NoError(t, err)

	key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
	require.NoError(t, err)

	foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
	require.NoError(t, err)
	require.NotNil(t, foundResource)

	err = usecase.Delete(ctx, key)
	require.NoError(t, err)

	foundResource, err = resourceRepo.FindResourceByKeys(nil, key)
	// With tombstone filter removed, we should find the tombstoned resource
	require.NoError(t, err)
	require.NotNil(t, foundResource)
	assert.True(t, foundResource.ReporterResources()[0].Serialize().Tombstone, "Resource should be tombstoned")
}

func TestMultipleHostsLifecycle(t *testing.T) {
	ctx := testAuthzContext()
	logger := log.DefaultLogger

	resourceRepo := data.NewFakeResourceRepository()
	schemaRepo := newFakeSchemaRepository(t)
	authorizer := &allow.AllowAllAuthz{}
	usecaseConfig := &UsecaseConfig{
		ReadAfterWriteEnabled: false,
		ConsumerEnabled:       false,
	}

	mc := metricscollector.NewFakeMetricsCollector()
	usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())

	// Create 2 hosts
	cmd := fixture(t).Basic("host", "hbi", "hbi-instance-1", "host-1", "workspace-1")
	err := usecase.ReportResource(ctx, cmd)
	require.NoError(t, err, "Should create host1")

	cmd = fixture(t).Basic("host", "hbi", "hbi-instance-1", "host-2", "workspace-1")
	err = usecase.ReportResource(ctx, cmd)
	require.NoError(t, err, "Should create host2")

	// Verify both hosts can be found
	key1, err := model.NewReporterResourceKey("host-1", "host", "hbi", "hbi-instance-1")
	require.NoError(t, err)
	key2, err := model.NewReporterResourceKey("host-2", "host", "hbi", "hbi-instance-1")
	require.NoError(t, err)

	foundHost1, err := resourceRepo.FindResourceByKeys(nil, key1)
	require.NoError(t, err, "Should find host1 after creation")
	require.NotNil(t, foundHost1)

	foundHost2, err := resourceRepo.FindResourceByKeys(nil, key2)
	require.NoError(t, err, "Should find host2 after creation")
	require.NotNil(t, foundHost2)

	// Update both hosts by reporting them again with updated data
	cmd = fixture(t).Updated("host", "hbi", "hbi-instance-1", "host-1", "workspace-1")
	err = usecase.ReportResource(ctx, cmd)
	require.NoError(t, err, "Should update host1")

	cmd = fixture(t).Updated("host", "hbi", "hbi-instance-1", "host-2", "workspace-1")
	err = usecase.ReportResource(ctx, cmd)
	require.NoError(t, err, "Should update host2")

	// Verify both updated hosts can still be found
	updatedHost1, err := resourceRepo.FindResourceByKeys(nil, key1)
	require.NoError(t, err, "Should find host1 after update")
	require.NotNil(t, updatedHost1)

	updatedHost2, err := resourceRepo.FindResourceByKeys(nil, key2)
	require.NoError(t, err, "Should find host2 after update")
	require.NotNil(t, updatedHost2)

	// Delete both hosts
	err = usecase.Delete(ctx, key1)
	require.NoError(t, err, "Should delete host1")

	err = usecase.Delete(ctx, key2)
	require.NoError(t, err, "Should delete host2")

	// Verify both hosts can be found (tombstoned) with tombstone filter removed
	foundHost1, err = resourceRepo.FindResourceByKeys(nil, key1)
	require.NoError(t, err, "Should find tombstoned host1")
	require.NotNil(t, foundHost1)
	assert.True(t, foundHost1.ReporterResources()[0].Serialize().Tombstone, "Host1 should be tombstoned")

	foundHost2, err = resourceRepo.FindResourceByKeys(nil, key2)
	require.NoError(t, err, "Should find tombstoned host2")
	require.NotNil(t, foundHost2)
	assert.True(t, foundHost2.ReporterResources()[0].Serialize().Tombstone, "Host2 should be tombstoned")
}

func TestPartialDataScenarios(t *testing.T) {
	ctx := testAuthzContext()
	logger := log.DefaultLogger

	resourceRepo := data.NewFakeResourceRepository()
	schemaRepo := newFakeSchemaRepository(t)
	authorizer := &allow.AllowAllAuthz{}
	usecaseConfig := &UsecaseConfig{
		ReadAfterWriteEnabled: false,
		ConsumerEnabled:       false,
	}

	mc := metricscollector.NewFakeMetricsCollector()
	usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())

	t.Run("Report resource with rich reporter data and minimal common data", func(t *testing.T) {
		cmd := fixture(t).ReporterRich("k8s_cluster", "ocm", "ocm-instance-1", "reporter-rich-resource", "minimal-workspace")
		err := usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Should create resource with rich reporter data")

		key, err := model.NewReporterResourceKey("reporter-rich-resource", "k8s_cluster", "ocm", "ocm-instance-1")
		require.NoError(t, err)

		foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource with rich reporter data")
		require.NotNil(t, foundResource)
	})

	t.Run("Report resource with minimal reporter data and rich common data", func(t *testing.T) {
		cmd := fixture(t).CommonRich("k8s_cluster", "ocm", "ocm-instance-1", "common-rich-resource", "rich-workspace")
		err := usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Should create resource with rich common data")

		key, err := model.NewReporterResourceKey("common-rich-resource", "k8s_cluster", "ocm", "ocm-instance-1")
		require.NoError(t, err)

		foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource with rich common data")
		require.NotNil(t, foundResource)
	})

	t.Run("Report resource with both data, then reporter-focused update, then common-focused update", func(t *testing.T) {
		// 1. Initial report with both reporter and common data
		cmd := fixture(t).Basic("k8s_cluster", "ocm", "ocm-instance-1", "progressive-resource", "initial-workspace")
		err := usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Should create resource with both data types")

		key, err := model.NewReporterResourceKey("progressive-resource", "k8s_cluster", "ocm", "ocm-instance-1")
		require.NoError(t, err)

		foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after initial creation")
		require.NotNil(t, foundResource)

		// 2. Reporter-focused update
		cmd = fixture(t).ReporterRich("k8s_cluster", "ocm", "ocm-instance-1", "progressive-resource", "initial-workspace")
		err = usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Should update resource with reporter-focused data")

		foundResource, err = resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after reporter-focused update")
		require.NotNil(t, foundResource)

		// 3. Common-focused update
		cmd = fixture(t).CommonRich("k8s_cluster", "ocm", "ocm-instance-1", "progressive-resource", "updated-workspace")
		err = usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Should update resource with common-focused data")

		finalResource, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after all updates")
		require.NotNil(t, finalResource)
	})
}

func TestResourceLifecycle_ReportUpdateDeleteReport(t *testing.T) {
	t.Run("report new -> update -> delete -> report new", func(t *testing.T) {
		ctx := testAuthzContext()
		logger := log.DefaultLogger

		resourceRepo := data.NewFakeResourceRepository()
		schemaRepo := newFakeSchemaRepository(t)
		authorizer := &allow.AllowAllAuthz{}
		usecaseConfig := &UsecaseConfig{
			ReadAfterWriteEnabled: false,
			ConsumerEnabled:       false,
		}

		mc := metricscollector.NewFakeMetricsCollector()
		usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())

		resourceType := "host"
		reporterType := "hbi"
		reporterInstance := "test-instance"
		localResourceId := "lifecycle-test-host"
		workspaceId := "test-workspace"

		// 1. REPORT NEW: Initial resource creation
		log.Info("Report New ---------------------")
		cmd := fixture(t).Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Initial report should succeed")

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Verify initial state: generation = 0, representationVersion = 0
		afterCreate, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after creation")
		require.NotNil(t, afterCreate)
		initialSnapshot := afterCreate.ReporterResources()[0].Serialize()
		initialGeneration := initialSnapshot.Generation
		initialRepVersion := initialSnapshot.RepresentationVersion
		initialTombstone := initialSnapshot.Tombstone
		assert.Equal(t, uint(0), initialGeneration, "Initial generation should be 0")
		assert.Equal(t, uint(0), initialRepVersion, "Initial representationVersion should be 0")
		assert.False(t, initialTombstone, "Initial tombstone should be false")

		log.Info("Update 1 ---------------------")
		// 2. UPDATE: Update the resource
		cmd = fixture(t).Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Update should succeed")

		// Verify state after update: representationVersion incremented, generation unchanged
		afterUpdate, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after update")
		require.NotNil(t, afterUpdate)
		updateSnapshot := afterUpdate.ReporterResources()[0].Serialize()
		updateGeneration := updateSnapshot.Generation
		updateRepVersion := updateSnapshot.RepresentationVersion
		updateTombstone := updateSnapshot.Tombstone
		assert.Equal(t, uint(0), updateGeneration, "Generation should remain 0 after update (tombstone=false)")
		assert.Equal(t, uint(1), updateRepVersion, "RepresentationVersion should increment to 1 after update")
		assert.False(t, updateTombstone, "Tombstone should remain false after update")

		// 3. DELETE: Delete the resource
		log.Info("Delete ---------------------")
		err = usecase.Delete(ctx, key)
		require.NoError(t, err, "Delete should succeed")

		// Verify state after delete: representationVersion incremented, generation unchanged, tombstoned
		afterDelete, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete)
		deleteSnapshot := afterDelete.ReporterResources()[0].Serialize()
		deleteGeneration := deleteSnapshot.Generation
		deleteRepVersion := deleteSnapshot.RepresentationVersion
		deleteTombstone := deleteSnapshot.Tombstone
		assert.Equal(t, uint(0), deleteGeneration, "Generation should remain 0 after delete")
		assert.Equal(t, uint(2), deleteRepVersion, "RepresentationVersion should increment to 2 after delete")
		assert.True(t, deleteTombstone, "Resource should be tombstoned after delete")

		// 4. REPORT NEW: Report the same resource again after deletion (this should be an update)
		log.Info("Revive again ---------------------")
		cmd = fixture(t).Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Report after delete should succeed")

		// Verify final state after update on tombstoned resource
		afterRevive, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after revival")
		require.NotNil(t, afterRevive)
		reviveSnapshot := afterRevive.ReporterResources()[0].Serialize()
		reviveGeneration := reviveSnapshot.Generation
		reviveRepVersion := reviveSnapshot.RepresentationVersion
		reviveTombstone := reviveSnapshot.Tombstone
		assert.Equal(t, uint(1), reviveGeneration, "Generation should increment to 1 after update on tombstoned resource")
		assert.Equal(t, uint(0), reviveRepVersion, "RepresentationVersion should start fresh at 0 for revival (new generation)")
		assert.False(t, reviveTombstone, "Resource should no longer be tombstoned after revival update")
	})
}

func TestResourceLifecycle_ReportUpdateDeleteReportDelete(t *testing.T) {
	t.Run("report new -> update -> delete -> report new -> delete", func(t *testing.T) {
		ctx := testAuthzContext()
		logger := log.DefaultLogger

		resourceRepo := data.NewFakeResourceRepository()
		schemaRepo := newFakeSchemaRepository(t)
		authorizer := &allow.AllowAllAuthz{}
		usecaseConfig := &UsecaseConfig{
			ReadAfterWriteEnabled: false,
			ConsumerEnabled:       false,
		}

		mc := metricscollector.NewFakeMetricsCollector()
		usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())

		resourceType := "k8s_cluster"
		reporterType := "ocm"
		reporterInstance := "ocm-instance"
		localResourceId := "lifecycle-test-cluster"
		workspaceId := "test-workspace-2"

		// 1. REPORT NEW: Initial resource creation
		cmd := fixture(t).Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Initial report should succeed")

		// 2. UPDATE: Update the resource
		cmd = fixture(t).Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Update should succeed")

		// 3. DELETE: Delete the resource
		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)
		err = usecase.Delete(ctx, key)
		require.NoError(t, err, "First delete should succeed")

		// 4. REPORT NEW: Report the same resource again after deletion
		cmd = fixture(t).Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Report after delete should succeed")

		// 5. DELETE: Delete the recreated resource
		err = usecase.Delete(ctx, key)
		require.NoError(t, err, "Second delete should succeed")

		// Verify final state - resource should be tombstoned
		finalResource, err := resourceRepo.FindResourceByKeys(nil, key)
		if err == gorm.ErrRecordNotFound {
			// This is expected with current tombstone filtering
			assert.Nil(t, finalResource, "Resource should not be found if tombstone filter is active")
		} else {
			// If tombstone filter is removed, we should find the resource
			require.NoError(t, err, "Should find tombstoned resource if filter is removed")
			require.NotNil(t, finalResource)
		}
	})
}

func createReporterResourceKey(t *testing.T, localResourceId, resourceType, reporterType, reporterInstance string) model.ReporterResourceKey {
	localResourceIdType, err := model.NewLocalResourceId(localResourceId)
	require.NoError(t, err)
	resourceTypeType, err := model.NewResourceType(resourceType)
	require.NoError(t, err)
	reporterTypeType, err := model.NewReporterType(reporterType)
	require.NoError(t, err)
	reporterInstanceIdType, err := model.NewReporterInstanceId(reporterInstance)
	require.NoError(t, err)

	key, err := model.NewReporterResourceKey(localResourceIdType, resourceTypeType, reporterTypeType, reporterInstanceIdType)
	require.NoError(t, err)
	return key
}

func TestResourceLifecycle_ReportDeleteResubmitDelete(t *testing.T) {
	t.Run("report -> delete -> resubmit same delete", func(t *testing.T) {
		ctx := testAuthzContext()
		logger := log.DefaultLogger

		resourceRepo := data.NewFakeResourceRepository()
		schemaRepo := newFakeSchemaRepository(t)
		authorizer := &allow.AllowAllAuthz{}
		usecaseConfig := &UsecaseConfig{
			ReadAfterWriteEnabled: false,
			ConsumerEnabled:       false,
		}

		mc := metricscollector.NewFakeMetricsCollector()
		usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())

		resourceType := "k8s_cluster"
		reporterType := "ocm"
		reporterInstance := "idempotent-instance"
		localResourceId := "idempotent-test-resource"
		workspaceId := "idempotent-workspace"

		// 1. REPORT: Initial resource creation
		log.Info("1. Initial Report ---------------------")
		cmd := fixture(t).Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Initial report should succeed")

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Verify initial state
		afterReport1, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after initial report")
		require.NotNil(t, afterReport1)
		initialState := afterReport1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(0), initialState.RepresentationVersion, "Initial representationVersion should be 0")
		assert.Equal(t, uint(0), initialState.Generation, "Initial generation should be 0")
		assert.False(t, initialState.Tombstone, "Initial tombstone should be false")

		// 2. DELETE: Delete the resource
		log.Info("2. Delete ---------------------")
		err = usecase.Delete(ctx, key)
		require.NoError(t, err, "Delete should succeed")

		// Verify delete state
		afterDelete1, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete1)
		deleteState1 := afterDelete1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(1), deleteState1.RepresentationVersion, "RepresentationVersion should increment after delete")
		assert.Equal(t, uint(0), deleteState1.Generation, "Generation should remain 0 after delete")
		assert.True(t, deleteState1.Tombstone, "Resource should be tombstoned")

		// 3. RESUBMIT SAME DELETE: Should be idempotent
		log.Info("3. Resubmit Delete ---------------------")
		err = usecase.Delete(ctx, key)
		require.NoError(t, err, "Resubmitted delete should succeed (idempotent)")

		// Verify state after duplicate delete (operations are idempotent - no changes for tombstoned resources)
		afterDelete2, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should still find tombstoned resource")
		require.NotNil(t, afterDelete2)
		deleteState2 := afterDelete2.ReporterResources()[0].Serialize()
		assert.Equal(t, deleteState1.RepresentationVersion, deleteState2.RepresentationVersion, "RepresentationVersion should remain unchanged for duplicate delete on tombstoned resource")
		assert.Equal(t, deleteState1.Generation, deleteState2.Generation, "Generation should be unchanged by duplicate delete")
		assert.True(t, deleteState2.Tombstone, "Resource should still be tombstoned")

	})
}

func TestResourceLifecycle_ReportResubmitDeleteResubmit(t *testing.T) {
	t.Run("report -> resubmit same report -> delete -> resubmit same delete", func(t *testing.T) {
		ctx := testAuthzContext()
		logger := log.DefaultLogger

		resourceRepo := data.NewFakeResourceRepository()
		schemaRepo := newFakeSchemaRepository(t)
		authorizer := &allow.AllowAllAuthz{}
		usecaseConfig := &UsecaseConfig{
			ReadAfterWriteEnabled: false,
			ConsumerEnabled:       false,
		}

		mc := metricscollector.NewFakeMetricsCollector()
		usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())
		resourceType := "host"
		reporterType := "hbi"
		reporterInstance := "idempotent-instance-2"
		localResourceId := "idempotent-test-resource-2"
		workspaceId := "idempotent-workspace-2"

		// 1. REPORT: Initial resource creation
		log.Info("1. Initial Report ---------------------")
		cmd := fixture(t).Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Initial report should succeed")

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Verify initial state
		afterReport1, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after initial report")
		require.NotNil(t, afterReport1)
		initialState := afterReport1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(0), initialState.RepresentationVersion, "Initial representationVersion should be 0")
		assert.Equal(t, uint(0), initialState.Generation, "Initial generation should be 0")
		assert.False(t, initialState.Tombstone, "Initial tombstone should be false")

		// 2. RESUBMIT SAME REPORT: Should be idempotent
		log.Info("2. Resubmit Same Report ---------------------")
		cmd = fixture(t).Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Resubmitted report should succeed (idempotent)")

		// Verify state after duplicate report (should increment representation version)
		afterReport2, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after duplicate report")
		require.NotNil(t, afterReport2)
		duplicateState := afterReport2.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(1), duplicateState.RepresentationVersion, "RepresentationVersion should increment after duplicate report")
		assert.Equal(t, uint(0), duplicateState.Generation, "Generation should remain 0")
		assert.False(t, duplicateState.Tombstone, "Resource should remain active")

		// 3. DELETE: Delete the resource
		log.Info("3. Delete ---------------------")
		err = usecase.Delete(ctx, key)
		require.NoError(t, err, "Delete should succeed")

		// Verify delete state
		afterDelete1, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete1)
		deleteState1 := afterDelete1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(2), deleteState1.RepresentationVersion, "RepresentationVersion should increment after delete")
		assert.Equal(t, uint(0), deleteState1.Generation, "Generation should remain 0 after delete")
		assert.True(t, deleteState1.Tombstone, "Resource should be tombstoned")

		// 4. RESUBMIT SAME DELETE: Should be idempotent
		log.Info("4. Resubmit Delete ---------------------")
		err = usecase.Delete(ctx, key)
		require.NoError(t, err, "Resubmitted delete should succeed (idempotent)")

		// Verify final state after duplicate delete (operations are idempotent - no changes for tombstoned resources)
		afterDelete2, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should still find tombstoned resource")
		require.NotNil(t, afterDelete2)
		finalDeleteState := afterDelete2.ReporterResources()[0].Serialize()
		assert.Equal(t, deleteState1.RepresentationVersion, finalDeleteState.RepresentationVersion, "RepresentationVersion should remain unchanged for duplicate delete on tombstoned resource")
		assert.Equal(t, deleteState1.Generation, finalDeleteState.Generation, "Generation should be unchanged by duplicate delete")
		assert.True(t, finalDeleteState.Tombstone, "Resource should still be tombstoned")
	})
}

func TestResourceLifecycle_ComplexIdempotency(t *testing.T) {
	t.Run("3 cycles of create+update+delete for same resource", func(t *testing.T) {
		ctx := testAuthzContext()
		logger := log.DefaultLogger

		resourceRepo := data.NewFakeResourceRepository()
		schemaRepo := newFakeSchemaRepository(t)
		authorizer := &allow.AllowAllAuthz{}
		usecaseConfig := &UsecaseConfig{
			ReadAfterWriteEnabled: false,
			ConsumerEnabled:       false,
		}

		mc := metricscollector.NewFakeMetricsCollector()
		usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())

		resourceType := "k8s_cluster"
		reporterType := "ocm"
		reporterInstance := "complex-idempotent-instance"
		localResourceId := "complex-idempotent-resource"
		workspaceId := "complex-idempotent-workspace"

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Run 3 cycles of create+update+delete
		for cycle := 0; cycle < 3; cycle++ {
			t.Logf("=== Cycle %d: Create+Update+Delete ===", cycle)

			// 1. REPORT (CREATE or UPDATE): Should find existing or create new
			log.Infof("Cycle %d: Report Resource", cycle)
			cmd := fixture(t).WithCycleData(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, cycle)
			err := usecase.ReportResource(ctx, cmd)
			require.NoError(t, err, "Report should succeed in cycle %d", cycle)

			// Verify state after report
			afterReport, err := resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err, "Should find resource after report in cycle %d", cycle)
			require.NotNil(t, afterReport)
			reportState := afterReport.ReporterResources()[0].Serialize()

			expectedGeneration := uint(cycle) // Generation should be 0, 1, 2 for cycles 0, 1, 2
			assert.Equal(t, expectedGeneration, reportState.Generation, "Generation should be %d in cycle %d", expectedGeneration, cycle)
			assert.Equal(t, uint(0), reportState.RepresentationVersion, "RepresentationVersion should reset to 0 for new generation in cycle %d", cycle)
			assert.False(t, reportState.Tombstone, "Resource should be active after report in cycle %d", cycle)

			// 2. UPDATE: Update the resource
			log.Infof("Cycle %d: Update Resource", cycle)
			cmd = fixture(t).Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
			err = usecase.ReportResource(ctx, cmd)
			require.NoError(t, err, "Update should succeed in cycle %d", cycle)

			// Verify state after update
			afterUpdate, err := resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err, "Should find resource after update in cycle %d", cycle)
			require.NotNil(t, afterUpdate)
			updateState := afterUpdate.ReporterResources()[0].Serialize()
			assert.Equal(t, expectedGeneration, updateState.Generation, "Generation should remain %d after update in cycle %d", expectedGeneration, cycle)
			assert.Equal(t, uint(1), updateState.RepresentationVersion, "RepresentationVersion should increment to 1 after update in cycle %d", cycle)
			assert.False(t, updateState.Tombstone, "Resource should remain active after update in cycle %d", cycle)

			// 3. DELETE: Delete the resource
			log.Infof("Cycle %d: Delete Resource", cycle)
			err = usecase.Delete(ctx, key)
			require.NoError(t, err, "Delete should succeed in cycle %d", cycle)

			// Verify state after delete
			afterDelete, err := resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err, "Should find tombstoned resource after delete in cycle %d", cycle)
			require.NotNil(t, afterDelete)
			deleteState := afterDelete.ReporterResources()[0].Serialize()
			assert.Equal(t, expectedGeneration, deleteState.Generation, "Generation should remain %d after delete in cycle %d", expectedGeneration, cycle)
			assert.Equal(t, uint(2), deleteState.RepresentationVersion, "RepresentationVersion should increment to 2 after delete in cycle %d", cycle)
			assert.True(t, deleteState.Tombstone, "Resource should be tombstoned after delete in cycle %d", cycle)

			t.Logf("Cycle %d complete: Final state {Generation: %d, RepVersion: %d, Tombstone: %t}",
				cycle, deleteState.Generation, deleteState.RepresentationVersion, deleteState.Tombstone)
		}

		// Final verification: Resource should be in generation 2 after 3 cycles
		finalResource, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find final resource")
		require.NotNil(t, finalResource)
		finalState := finalResource.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(2), finalState.Generation, "Final generation should be 2 after 3 cycles")
		assert.True(t, finalState.Tombstone, "Final resource should be tombstoned")
	})
}

func TestGetCurrentAndPreviousWorkspaceID(t *testing.T) {
	// Test the GetCurrentAndPreviousWorkspaceID function with test data
	tests := []struct {
		name             string
		current          *model.Representations
		previous         *model.Representations
		currentVersion   uint
		expectedCurrent  string
		expectedPrevious string
	}{
		{
			name:             "extract current and previous workspace IDs",
			current:          createTestRep(t, uint(2), map[string]interface{}{"workspace_id": "workspace-new"}),
			previous:         createTestRep(t, uint(1), map[string]interface{}{"workspace_id": "workspace-old"}),
			currentVersion:   2,
			expectedCurrent:  "workspace-new",
			expectedPrevious: "workspace-old",
		},
		{
			name:             "extract only current workspace ID",
			current:          createTestRep(t, uint(0), map[string]interface{}{"workspace_id": "workspace-initial"}),
			previous:         nil,
			currentVersion:   0,
			expectedCurrent:  "workspace-initial",
			expectedPrevious: "",
		},
		{
			name:             "no workspace IDs found",
			current:          createTestRep(t, uint(1), map[string]interface{}{"other_field": "value"}),
			previous:         nil,
			currentVersion:   1,
			expectedCurrent:  "",
			expectedPrevious: "",
		},
		{
			name:             "empty workspace ID ignored",
			current:          createTestRep(t, uint(1), map[string]interface{}{"workspace_id": ""}),
			previous:         nil,
			currentVersion:   1,
			expectedCurrent:  "",
			expectedPrevious: "",
		},
		{
			name:             "workspace ID with special characters",
			current:          createTestRep(t, uint(1), map[string]interface{}{"workspace_id": "workspace-with-dashes_and_underscores"}),
			previous:         nil,
			currentVersion:   1,
			expectedCurrent:  "workspace-with-dashes_and_underscores",
			expectedPrevious: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, previous := data.GetCurrentAndPreviousWorkspaceID(tt.current, tt.previous)

			assert.Equal(t, tt.expectedCurrent, current)
			assert.Equal(t, tt.expectedPrevious, previous)
		})
	}
}

// Helper function to create a Representations for testing
func createTestRep(t *testing.T, version uint, data map[string]interface{}) *model.Representations {
	rep, err := model.NewRepresentations(
		model.Representation(data),
		&version,
		nil,
		nil,
	)
	require.NoError(t, err)
	return rep
}

func TestTransactionIdIdempotency(t *testing.T) {
	ctx := testAuthzContext()
	logger := log.DefaultLogger

	resourceRepo := data.NewFakeResourceRepository()
	schemaRepo := newFakeSchemaRepository(t)
	authorizer := &allow.AllowAllAuthz{}
	usecaseConfig := &UsecaseConfig{
		ReadAfterWriteEnabled: false,
		ConsumerEnabled:       false,
	}

	mc := metricscollector.NewFakeMetricsCollector()
	usecase := New(resourceRepo, schemaRepo, authorizer, "test-topic", logger, nil, nil, usecaseConfig, mc, nil, newTestSelfSubjectStrategy())

	t.Run("Same transaction ID should be idempotent - no changes to representation tables", func(t *testing.T) {
		resourceType := "host"
		reporterType := "hbi"
		reporterInstance := "test-instance"
		localResourceId := "test-resource"
		workspaceId := "test-workspace"
		transactionId := "test-transaction-123"

		// 1. First report with transaction ID
		cmd := fixture(t).WithTransactionId(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, transactionId)
		err := usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "First report should succeed")

		// Get the resource key for verification
		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Verify initial state
		afterFirst, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after first report")
		require.NotNil(t, afterFirst)
		firstState := afterFirst.ReporterResources()[0].Serialize()
		initialRepVersion := firstState.RepresentationVersion
		initialGeneration := firstState.Generation

		// 2. Second report with SAME transaction ID - should be idempotent
		cmd = fixture(t).WithTransactionId(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, transactionId)
		err = usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Second report with same transaction ID should succeed (idempotent)")

		// Verify no changes were made to representation tables
		afterSecond, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after second report")
		require.NotNil(t, afterSecond)
		secondState := afterSecond.ReporterResources()[0].Serialize()

		assert.Equal(t, initialRepVersion, secondState.RepresentationVersion, "RepresentationVersion should not change for idempotent request")
		assert.Equal(t, initialGeneration, secondState.Generation, "Generation should not change for idempotent request")
	})

	t.Run("Different transaction ID should update representations", func(t *testing.T) {
		resourceType := "host"
		reporterType := "hbi"
		reporterInstance := "test-instance-2"
		localResourceId := "test-resource-2"
		workspaceId := "test-workspace-2"
		transactionId1 := "test-transaction-456"
		transactionId2 := "test-transaction-789"

		// 1. First report with transaction ID
		cmd := fixture(t).WithTransactionId(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, transactionId1)
		err := usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "First report should succeed")

		// Get the resource key for verification
		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Verify initial state
		afterFirst, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after first report")
		require.NotNil(t, afterFirst)
		firstState := afterFirst.ReporterResources()[0].Serialize()
		initialRepVersion := firstState.RepresentationVersion
		initialGeneration := firstState.Generation

		// 2. Second report with DIFFERENT transaction ID - should update representations
		cmd = fixture(t).UpdatedWithTransactionId(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, transactionId2)
		err = usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Second report with different transaction ID should succeed")

		// Verify representations were updated
		afterSecond, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after second report")
		require.NotNil(t, afterSecond)
		secondState := afterSecond.ReporterResources()[0].Serialize()

		assert.Equal(t, initialRepVersion+1, secondState.RepresentationVersion, "RepresentationVersion should increment for different transaction ID")
		assert.Equal(t, initialGeneration, secondState.Generation, "Generation should remain the same for update")
	})

	t.Run("Report with transaction ID -> Update with new transaction ID -> Delete should update representations", func(t *testing.T) {
		resourceType := "host"
		reporterType := "hbi"
		reporterInstance := "test-instance-3"
		localResourceId := "test-resource-3"
		workspaceId := "test-workspace-3"
		transactionId1 := "test-transaction-111"
		transactionId2 := "test-transaction-222"

		// 1. First report with transaction ID
		cmd := fixture(t).WithTransactionId(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, transactionId1)
		err := usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "First report should succeed")

		// Get the resource key for verification
		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Verify initial state
		afterFirst, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after first report")
		require.NotNil(t, afterFirst)
		firstState := afterFirst.ReporterResources()[0].Serialize()
		initialRepVersion := firstState.RepresentationVersion
		initialGeneration := firstState.Generation
		assert.Equal(t, uint(0), initialRepVersion, "Initial representationVersion should be 0")
		assert.Equal(t, uint(0), initialGeneration, "Initial generation should be 0")
		assert.False(t, firstState.Tombstone, "Initial tombstone should be false")

		// 2. Update with DIFFERENT transaction ID - should update representations
		cmd = fixture(t).UpdatedWithTransactionId(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, transactionId2)
		err = usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Update with different transaction ID should succeed")

		// Verify state after update
		afterUpdate, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after update")
		require.NotNil(t, afterUpdate)
		updateState := afterUpdate.ReporterResources()[0].Serialize()
		assert.Equal(t, initialRepVersion+1, updateState.RepresentationVersion, "RepresentationVersion should increment after update")
		assert.Equal(t, initialGeneration, updateState.Generation, "Generation should remain the same after update")
		assert.False(t, updateState.Tombstone, "Resource should remain active after update")

		// 3. Delete resource (no transaction ID) - should update representations
		err = usecase.Delete(ctx, key)
		require.NoError(t, err, "Delete should succeed")

		// Verify state after delete
		afterDelete, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete)
		deleteState := afterDelete.ReporterResources()[0].Serialize()
		assert.Equal(t, updateState.RepresentationVersion+1, deleteState.RepresentationVersion, "RepresentationVersion should increment after delete")
		assert.Equal(t, updateState.Generation, deleteState.Generation, "Generation should remain the same after delete")
		assert.True(t, deleteState.Tombstone, "Resource should be tombstoned after delete")
	})

	t.Run("Report with transaction ID -> Report with same transaction ID -> Delete should update representations", func(t *testing.T) {
		resourceType := "host"
		reporterType := "hbi"
		reporterInstance := "test-instance-4"
		localResourceId := "test-resource-4"
		workspaceId := "test-workspace-4"
		transactionId := "test-transaction-333"

		// 1. First report with transaction ID
		cmd := fixture(t).WithTransactionId(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, transactionId)
		err := usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "First report should succeed")

		// Get the resource key for verification
		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Verify initial state
		afterFirst, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after first report")
		require.NotNil(t, afterFirst)
		firstState := afterFirst.ReporterResources()[0].Serialize()
		initialRepVersion := firstState.RepresentationVersion
		initialGeneration := firstState.Generation
		assert.Equal(t, uint(0), initialRepVersion, "Initial representationVersion should be 0")
		assert.Equal(t, uint(0), initialGeneration, "Initial generation should be 0")
		assert.False(t, firstState.Tombstone, "Initial tombstone should be false")

		// 2. Second report with SAME transaction ID - should be idempotent
		cmd = fixture(t).WithTransactionId(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, transactionId)
		err = usecase.ReportResource(ctx, cmd)
		require.NoError(t, err, "Second report with same transaction ID should succeed (idempotent)")

		// Verify state hasn't changed (idempotent)
		afterSecond, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after second report")
		require.NotNil(t, afterSecond)
		secondState := afterSecond.ReporterResources()[0].Serialize()
		assert.Equal(t, initialRepVersion, secondState.RepresentationVersion, "RepresentationVersion should not change for idempotent request")
		assert.Equal(t, initialGeneration, secondState.Generation, "Generation should not change for idempotent request")
		assert.False(t, secondState.Tombstone, "Resource should remain active")

		// 3. Delete resource (no transaction ID) - should update representations
		err = usecase.Delete(ctx, key)
		require.NoError(t, err, "Delete should succeed")

		// Verify state after delete
		afterDelete, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete)
		deleteState := afterDelete.ReporterResources()[0].Serialize()
		assert.Equal(t, secondState.RepresentationVersion+1, deleteState.RepresentationVersion, "RepresentationVersion should increment after delete")
		assert.Equal(t, secondState.Generation, deleteState.Generation, "Generation should remain the same after delete")
		assert.True(t, deleteState.Tombstone, "Resource should be tombstoned after delete")
	})
}

// TestReportResource_ValidationSuccess tests that a valid request passes validation.
func TestReportResource_ValidationSuccess(t *testing.T) {
	ctx := testAuthzContext()
	schemaRepo := newFakeSchemaRepository(t)
	usecase := New(
		data.NewFakeResourceRepository(),
		schemaRepo,
		&allow.AllowAllAuthz{},
		"test-topic",
		log.DefaultLogger,
		nil, nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestSelfSubjectStrategy(),
	)

	cmd := fixture(t).WithData("host", "hbi", "instance-1", "test-host",
		map[string]interface{}{
			"satellite_id": "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
			"ansible_host": "host-1",
		},
		map[string]interface{}{
			"workspace_id": "ws-123",
		},
	)

	err := usecase.ReportResource(ctx, cmd)
	require.NoError(t, err)
}

// TestReportResource_ValidationErrors tests field validation errors.
func TestReportResource_ValidationErrors(t *testing.T) {
	ctx := testAuthzContext()
	schemaRepo := newFakeSchemaRepository(t)
	usecase := New(
		data.NewFakeResourceRepository(),
		schemaRepo,
		&allow.AllowAllAuthz{},
		"test-topic",
		log.DefaultLogger,
		nil, nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestSelfSubjectStrategy(),
	)

	tests := []struct {
		name        string
		cmd         ReportResourceCommand
		expectError string
	}{
		{
			name: "missing type",
			cmd: func() ReportResourceCommand {
				cmd := fixture(t).Basic("host", "hbi", "instance-1", "test-host", "ws-123")
				cmd.ResourceType = "" // empty type
				return cmd
			}(),
			expectError: "missing 'type' field",
		},
		{
			name: "missing reporterType",
			cmd: func() ReportResourceCommand {
				cmd := fixture(t).Basic("host", "hbi", "instance-1", "test-host", "ws-123")
				cmd.ReporterType = "" // empty reporter type
				return cmd
			}(),
			expectError: "missing 'reporterType' field",
		},
		{
			name:        "reporter type not allowed for resource type",
			cmd:         fixture(t).Basic("host", "unknown_reporter", "instance-1", "test-host", "ws-123"),
			expectError: "reporter unknown_reporter does not report resource types: host",
		},
		{
			name: "missing reporter representation",
			cmd: func() ReportResourceCommand {
				cmd := fixture(t).Basic("host", "hbi", "instance-1", "test-host", "ws-123")
				cmd.ReporterRepresentation = nil
				return cmd
			}(),
			expectError: "invalid reporter representation: representation required",
		},
		{
			name: "missing common representation",
			cmd: func() ReportResourceCommand {
				cmd := fixture(t).Basic("host", "hbi", "instance-1", "test-host", "ws-123")
				cmd.CommonRepresentation = nil
				return cmd
			}(),
			expectError: "invalid common representation: representation required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := usecase.ReportResource(ctx, tc.cmd)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectError)
		})
	}
}

// TestReportResource_SchemaValidation tests schema-based validation scenarios.
func TestReportResource_SchemaValidation(t *testing.T) {
	tests := []struct {
		name           string
		resourceType   string
		reporterType   string
		reporterData   map[string]interface{}
		commonData     map[string]interface{}
		reporterSchema string
		commonSchema   string
		expectError    bool
		expectedError  string
	}{
		{
			name:         "Reporter schema with NO required fields - empty reporter data should pass",
			resourceType: "test_resource",
			reporterType: "test_reporter",
			reporterData: map[string]interface{}{"optional": "value"},
			commonData:   map[string]interface{}{"workspace_id": "ws-456"},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" }
				},
				"required": []
			}`,
			commonSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" }
				},
				"required": ["workspace_id"]
			}`,
			expectError: false,
		},
		{
			name:         "Common schema with NO required fields - empty common data should pass",
			resourceType: "test_resource",
			reporterType: "test_reporter",
			reporterData: map[string]interface{}{"policy_id": "pol-123"},
			commonData:   map[string]interface{}{"optional": "value"},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" }
				},
				"required": []
			}`,
			commonSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" }
				},
				"required": []
			}`,
			expectError: false,
		},
		{
			name:         "Both schemas with NO required fields - both empty should pass",
			resourceType: "test_resource",
			reporterType: "test_reporter",
			reporterData: map[string]interface{}{"optional": "value"},
			commonData:   map[string]interface{}{"optional": "value"},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" }
				}
			}`,
			commonSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" }
				}
			}`,
			expectError: false,
		},
		{
			name:         "Reporter schema with required fields - missing required field should fail",
			resourceType: "test_resource",
			reporterType: "test_reporter",
			reporterData: map[string]interface{}{"other_field": "value"},
			commonData:   map[string]interface{}{"workspace_id": "ws-456"},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" }
				},
				"required": ["policy_id"]
			}`,
			commonSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" }
				},
				"required": ["workspace_id"]
			}`,
			expectError:   true,
			expectedError: "policy_id",
		},
		{
			name:         "Common schema with required fields - missing required field should fail",
			resourceType: "test_resource",
			reporterType: "test_reporter",
			reporterData: map[string]interface{}{"policy_id": "pol-123"},
			commonData:   map[string]interface{}{"other_field": "value"},
			reporterSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"policy_id": { "type": "string" }
				},
				"required": []
			}`,
			commonSchema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"workspace_id": { "type": "string" }
				},
				"required": ["workspace_id"]
			}`,
			expectError:   true,
			expectedError: "workspace_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testAuthzContext()
			schemaRepository := data.NewInMemorySchemaRepository()

			err := schemaRepository.CreateResourceSchema(context.Background(), model.ResourceSchema{
				ResourceType:     tc.resourceType,
				ValidationSchema: model.NewJsonSchemaValidatorFromString(tc.commonSchema),
			})
			require.NoError(t, err)

			err = schemaRepository.CreateReporterSchema(context.Background(), model.ReporterSchema{
				ResourceType:     tc.resourceType,
				ReporterType:     tc.reporterType,
				ValidationSchema: model.NewJsonSchemaValidatorFromString(tc.reporterSchema),
			})
			require.NoError(t, err)

			usecase := New(
				data.NewFakeResourceRepository(),
				schemaRepository,
				&allow.AllowAllAuthz{},
				"test-topic",
				log.DefaultLogger,
				nil, nil,
				&UsecaseConfig{},
				metricscollector.NewFakeMetricsCollector(),
				nil,
				newTestSelfSubjectStrategy(),
			)

			cmd := fixture(t).WithData(tc.resourceType, tc.reporterType, "instance-1", "resource-123",
				tc.reporterData,
				tc.commonData,
			)

			err = usecase.ReportResource(ctx, cmd)
			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedError != "" {
					assert.Contains(t, err.Error(), tc.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- ReportResource: Error Message Format Tests ---
// These tests verify the exact error message format returned by the usecase layer.
// They serve as a contract and must be updated if error formats change.

// TestReportResource_RepresentationRequiredError tests that nil representations
// return RepresentationRequiredError with the correct message format.
func TestReportResource_RepresentationRequiredError(t *testing.T) {
	ctx := testAuthzContext()
	schemaRepo := newFakeSchemaRepository(t)
	usecase := New(
		data.NewFakeResourceRepository(),
		schemaRepo,
		&allow.AllowAllAuthz{},
		"test-topic",
		log.DefaultLogger,
		nil, nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestSelfSubjectStrategy(),
	)

	commonRep, _ := model.NewRepresentation(map[string]interface{}{"workspace_id": "ws-123"})
	reporterRep, _ := model.NewRepresentation(map[string]interface{}{"satellite_id": "sat-123"})

	tests := []struct {
		name                   string
		reporterRepresentation *model.Representation
		commonRepresentation   *model.Representation
		expectErrorMsg         string
	}{
		{
			name:                   "nil reporter representation",
			reporterRepresentation: nil,
			commonRepresentation:   &commonRep,
			expectErrorMsg:         "invalid reporter representation: representation required",
		},
		{
			name:                   "nil common representation",
			reporterRepresentation: &reporterRep,
			commonRepresentation:   nil,
			expectErrorMsg:         "invalid common representation: representation required",
		},
		{
			name:                   "both nil",
			reporterRepresentation: nil,
			commonRepresentation:   nil,
			expectErrorMsg:         "invalid reporter representation: representation required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			localResId, _ := model.NewLocalResourceId("test-host")
			resType, _ := model.NewResourceType("host")
			repType, _ := model.NewReporterType("hbi")
			repInstanceId, _ := model.NewReporterInstanceId("instance-1")
			apiHref, _ := model.NewApiHref("https://api.example.com/resource/123")

			cmd := ReportResourceCommand{
				LocalResourceId:        localResId,
				ResourceType:           resType,
				ReporterType:           repType,
				ReporterInstanceId:     repInstanceId,
				ApiHref:                apiHref,
				ReporterRepresentation: tc.reporterRepresentation,
				CommonRepresentation:   tc.commonRepresentation,
				WriteVisibility:        WriteVisibilityMinimizeLatency,
			}

			err := usecase.ReportResource(ctx, cmd)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectErrorMsg)

			// Verify it's a RepresentationRequiredError type
			var repReqErr *RepresentationRequiredError
			assert.True(t, errors.As(err, &repReqErr), "expected RepresentationRequiredError type")
		})
	}
}

// TestReportResource_ValidationErrorFormat tests that validation failures
// return errors with the expected message format.
func TestReportResource_ValidationErrorFormat(t *testing.T) {
	ctx := testAuthzContext()
	schemaRepo := newFakeSchemaRepository(t)
	usecase := New(
		data.NewFakeResourceRepository(),
		schemaRepo,
		&allow.AllowAllAuthz{},
		"test-topic",
		log.DefaultLogger,
		nil, nil,
		&UsecaseConfig{},
		metricscollector.NewFakeMetricsCollector(),
		nil,
		newTestSelfSubjectStrategy(),
	)

	tests := []struct {
		name           string
		cmd            ReportResourceCommand
		expectErrorMsg string
	}{
		{
			name: "missing type field",
			cmd: func() ReportResourceCommand {
				cmd := fixture(t).Basic("host", "hbi", "instance-1", "test-host", "ws-123")
				cmd.ResourceType = ""
				return cmd
			}(),
			expectErrorMsg: "failed validation for report resource: missing 'type' field",
		},
		{
			name: "missing reporterType field",
			cmd: func() ReportResourceCommand {
				cmd := fixture(t).Basic("host", "hbi", "instance-1", "test-host", "ws-123")
				cmd.ReporterType = ""
				return cmd
			}(),
			expectErrorMsg: "failed validation for report resource: missing 'reporterType' field",
		},
		{
			name:           "unknown reporter for resource type",
			cmd:            fixture(t).Basic("host", "unknown_reporter", "instance-1", "test-host", "ws-123"),
			expectErrorMsg: "failed validation for report resource: reporter unknown_reporter does not report resource types: host",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := usecase.ReportResource(ctx, tc.cmd)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectErrorMsg)
		})
	}
}
