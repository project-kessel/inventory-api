package resources

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/subject/selfsubject"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- Test harness: reduces boilerplate across all test cases ---

type testHarness struct {
	usecase      *Usecase
	resourceRepo model.ResourceRepository
	meta         *recordingMetaAuthorizer
	ctx          context.Context
}

type harnessOption func(*harnessConfig)

type harnessConfig struct {
	relationsRepo model.RelationsRepository
	meta          *recordingMetaAuthorizer
	usecaseConfig *UsecaseConfig
	logger        log.Logger
	namespace     string
}

func newTestHarness(t *testing.T, opts ...harnessOption) *testHarness {
	t.Helper()
	cfg := &harnessConfig{
		relationsRepo: &data.AllowAllRelationsRepository{},
		usecaseConfig: &UsecaseConfig{},
		logger:        log.DefaultLogger,
		namespace:     "rbac",
	}
	for _, o := range opts {
		o(cfg)
	}

	resourceRepo := data.NewFakeResourceRepository()
	mc := metricscollector.NewFakeMetricsCollector()
	var metaAuth metaauthorizer.MetaAuthorizer
	if cfg.meta != nil {
		metaAuth = cfg.meta
	}

	uc := New(
		resourceRepo,
		newFakeSchemaRepository(t),
		cfg.relationsRepo,
		cfg.namespace,
		cfg.logger,
		nil, nil,
		cfg.usecaseConfig,
		mc,
		metaAuth,
		newTestSelfSubjectStrategy(),
	)

	return &testHarness{
		usecase:      uc,
		resourceRepo: resourceRepo,
		meta:         cfg.meta,
		ctx:          testAuthzContext(),
	}
}

func withMeta(allowed bool) harnessOption {
	return func(c *harnessConfig) {
		c.meta = &recordingMetaAuthorizer{allowed: allowed}
	}
}

func withRelations(repo model.RelationsRepository) harnessOption {
	return func(c *harnessConfig) { c.relationsRepo = repo }
}

func withUsecaseConfig(cfg *UsecaseConfig) harnessOption {
	return func(c *harnessConfig) { c.usecaseConfig = cfg }
}

func withNamespace(ns string) harnessOption {
	return func(c *harnessConfig) { c.namespace = ns }
}

func withLogger(l log.Logger) harnessOption {
	return func(c *harnessConfig) { c.logger = l }
}

// resetMeta clears recorded meta-authorizer state (useful after setup calls).
func (h *testHarness) resetMeta() {
	if h.meta != nil {
		h.meta.calls = 0
		h.meta.relations = nil
	}
}

// --- Test helpers ---

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
	h := newTestHarness(t, withMeta(true))

	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	allowed, _, err := h.usecase.CheckSelf(h.ctx, relation, key, model.NewConsistencyMinimizeLatency())
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, h.meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationCheckSelf}, h.meta.relations)
}

func TestCheckSelf_DeniedByMetaAuthz(t *testing.T) {
	h := newTestHarness(t, withMeta(false))

	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	_, _, err = h.usecase.CheckSelf(h.ctx, relation, key, model.NewConsistencyMinimizeLatency())
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestCheckSelf_MissingAuthzContext(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	_, _, err = h.usecase.CheckSelf(context.Background(), relation, key, model.NewConsistencyMinimizeLatency())
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthzContextMissing)
	assert.Equal(t, 0, h.meta.calls)
}

func TestReportResource_UsesReportRelation(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	cmd := fixture(t).Basic("host", "hbi", "instance-1", "host-1", "workspace-1")
	err := h.usecase.ReportResource(h.ctx, cmd)
	require.NoError(t, err)
	assert.Equal(t, 1, h.meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationReportResource}, h.meta.relations)
}

func TestDelete_UsesDeleteRelation(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	cmd := fixture(t).Basic("host", "hbi", "instance-1", "host-1", "workspace-1")
	err := h.usecase.ReportResource(h.ctx, cmd)
	require.NoError(t, err)

	h.resetMeta()

	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	err = h.usecase.Delete(h.ctx, key)
	require.NoError(t, err)
	assert.Equal(t, 1, h.meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationDeleteResource}, h.meta.relations)
}

func TestCheck_UsesCheckRelation(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	allowed, _, err := h.usecase.Check(h.ctx, relation, subject, key, model.NewConsistencyUnspecified())
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, h.meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationCheck}, h.meta.relations)
}

func TestCheckForUpdate_UsesCheckForUpdateRelation(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	allowed, _, err := h.usecase.CheckForUpdate(h.ctx, relation, subject, key)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 1, h.meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationCheckForUpdate}, h.meta.relations)
}

func TestCheckForUpdateBulk_UsesCheckForUpdateBulkRelation(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("edit")
	require.NoError(t, err)

	cmd := CheckForUpdateBulkCommand{
		Items: []CheckBulkItem{
			{Resource: key, Relation: relation, Subject: subject},
		},
	}
	result, err := h.usecase.CheckForUpdateBulk(h.ctx, cmd)
	require.NoError(t, err)
	require.Len(t, result.Pairs, 1)
	assert.True(t, result.Pairs[0].Result.Allowed)
	assert.Equal(t, 1, h.meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationCheckForUpdateBulk}, h.meta.relations)
}

func TestCheckForUpdateBulk_MetaAuthzDenied(t *testing.T) {
	h := newTestHarness(t, withMeta(false))

	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("update")
	require.NoError(t, err)

	_, err = h.usecase.CheckForUpdateBulk(h.ctx, CheckForUpdateBulkCommand{
		Items: []CheckBulkItem{
			{Resource: key, Relation: relation, Subject: subject},
		},
	})
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
	assert.Equal(t, 1, h.meta.calls)
}

func TestCheckForUpdateBulk_MixedResults(t *testing.T) {
	simpleAuthz := data.NewSimpleRelationsRepository()
	simpleAuthz.Grant("user-1", "update", "hbi", "host", "host-1")

	h := newTestHarness(t, withMeta(true), withRelations(simpleAuthz))

	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	key1 := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	key2 := createReporterResourceKey(t, "host-2", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("update")
	require.NoError(t, err)

	result, err := h.usecase.CheckForUpdateBulk(h.ctx, CheckForUpdateBulkCommand{
		Items: []CheckBulkItem{
			{Resource: key1, Relation: relation, Subject: subject},
			{Resource: key2, Relation: relation, Subject: subject},
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Pairs, 2)
	assert.True(t, result.Pairs[0].Result.Allowed)
	assert.Nil(t, result.Pairs[0].Result.Error)
	assert.False(t, result.Pairs[1].Result.Allowed)
	assert.Nil(t, result.Pairs[1].Result.Error)
	assert.Equal(t, 2, h.meta.calls)
}

func TestCheckSelfBulk_UsesCheckSelfRelationForEachItem(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

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

	resp, err := h.usecase.CheckSelfBulk(h.ctx, cmd)
	require.NoError(t, err)
	require.Len(t, resp.Pairs, 2)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationCheckSelf, metaauthorizer.RelationCheckSelf}, h.meta.relations)
}

func TestCheckSelfBulk_MissingAuthzContext(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	viewRelation, err := model.NewRelation("view")
	require.NoError(t, err)

	cmd := CheckSelfBulkCommand{
		Items: []CheckSelfBulkItem{
			{Resource: key, Relation: viewRelation},
		},
		Consistency: model.NewConsistencyMinimizeLatency(),
	}

	_, err = h.usecase.CheckSelfBulk(context.Background(), cmd)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthzContextMissing)
	assert.Equal(t, 0, h.meta.calls)
}

func TestCheckBulk_RejectsInventoryManagedConsistency(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	viewRelation, err := model.NewRelation("view")
	require.NoError(t, err)
	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")

	_, err = h.usecase.CheckBulk(h.ctx, CheckBulkCommand{
		Items: []CheckBulkItem{
			{
				Resource: key,
				Relation: viewRelation,
				Subject:  subject,
			},
		},
		Consistency: model.NewConsistencyAtLeastAsAcknowledged(),
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "inventory-managed consistency tokens aren't available")
	assert.Equal(t, 0, h.meta.calls)
}

func TestCheckSelfBulk_RejectsInventoryManagedConsistency(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	viewRelation, err := model.NewRelation("view")
	require.NoError(t, err)
	key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")

	_, err = h.usecase.CheckSelfBulk(h.ctx, CheckSelfBulkCommand{
		Items: []CheckSelfBulkItem{
			{
				Resource: key,
				Relation: viewRelation,
			},
		},
		Consistency: model.NewConsistencyAtLeastAsAcknowledged(),
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "inventory-managed consistency tokens aren't available")
	assert.Equal(t, 0, h.meta.calls)
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
			h := newTestHarness(t, withNamespace("test-topic"))

			cmd := fixture(t).Basic(tt.resourceType, tt.reporterType, tt.reporterInstance, tt.localResourceId, tt.workspaceId)
			err := h.usecase.ReportResource(h.ctx, cmd)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			key := createReporterResourceKey(t, tt.localResourceId, tt.resourceType, tt.reporterType, tt.reporterInstance)
			foundResource, err := h.resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err)
			require.NotNil(t, foundResource)

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
			h := newTestHarness(t, withNamespace("test-topic"))

			cmd := fixture(t).Basic(tt.resourceType, tt.reporterType, tt.reporterInstanceId, tt.localResourceId, tt.workspaceId)
			err := h.usecase.ReportResource(h.ctx, cmd)
			require.NoError(t, err)

			key := createReporterResourceKey(t, tt.localResourceId, tt.resourceType, tt.reporterType, tt.reporterInstanceId)
			foundResource, err := h.resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err)
			require.NotNil(t, foundResource)

			assert.Equal(t, 1, metricscollector.GetOutboxEventWriteCount())

			deleteKey := createReporterResourceKeyAllowEmptyInstance(t, tt.localResourceId, tt.resourceType, tt.reporterType, tt.deleteReporterInstanceId)
			deleteFoundResource, err := h.resourceRepo.FindResourceByKeys(nil, deleteKey)
			require.NoError(t, err)
			require.NotNil(t, deleteFoundResource)

			err = h.usecase.Delete(h.ctx, deleteKey)
			require.NoError(t, err)

			assert.Equal(t, 2, metricscollector.GetOutboxEventWriteCount())
		})
	}
}

func TestDelete_ResourceNotFound(t *testing.T) {
	h := newTestHarness(t, withNamespace("test-topic"))

	key := createReporterResourceKey(t, "non-existent-resource", "k8s_cluster", "ocm", "test-instance")
	err := h.usecase.Delete(h.ctx, key)
	require.Error(t, err)
}

func TestReportFindDeleteFind_TombstoneLifecycle(t *testing.T) {
	h := newTestHarness(t, withNamespace("test-topic"))

	cmd := fixture(t).Basic("k8s_cluster", "ocm", "lifecycle-instance", "lifecycle-resource", "lifecycle-workspace")
	err := h.usecase.ReportResource(h.ctx, cmd)
	require.NoError(t, err)

	key := createReporterResourceKey(t, "lifecycle-resource", "k8s_cluster", "ocm", "lifecycle-instance")

	foundResource, err := h.resourceRepo.FindResourceByKeys(nil, key)
	require.NoError(t, err)
	require.NotNil(t, foundResource)

	err = h.usecase.Delete(h.ctx, key)
	require.NoError(t, err)

	foundResource, err = h.resourceRepo.FindResourceByKeys(nil, key)
	require.NoError(t, err)
	require.NotNil(t, foundResource)
	assert.True(t, foundResource.ReporterResources()[0].Serialize().Tombstone, "Resource should be tombstoned")
}

func TestMultipleHostsLifecycle(t *testing.T) {
	h := newTestHarness(t, withNamespace("test-topic"))

	cmd := fixture(t).Basic("host", "hbi", "hbi-instance-1", "host-1", "workspace-1")
	err := h.usecase.ReportResource(h.ctx, cmd)
	require.NoError(t, err, "Should create host1")

	cmd = fixture(t).Basic("host", "hbi", "hbi-instance-1", "host-2", "workspace-1")
	err = h.usecase.ReportResource(h.ctx, cmd)
	require.NoError(t, err, "Should create host2")

	key1 := createReporterResourceKey(t, "host-1", "host", "hbi", "hbi-instance-1")
	key2 := createReporterResourceKey(t, "host-2", "host", "hbi", "hbi-instance-1")

	foundHost1, err := h.resourceRepo.FindResourceByKeys(nil, key1)
	require.NoError(t, err, "Should find host1 after creation")
	require.NotNil(t, foundHost1)

	foundHost2, err := h.resourceRepo.FindResourceByKeys(nil, key2)
	require.NoError(t, err, "Should find host2 after creation")
	require.NotNil(t, foundHost2)

	cmd = fixture(t).Updated("host", "hbi", "hbi-instance-1", "host-1", "workspace-1")
	err = h.usecase.ReportResource(h.ctx, cmd)
	require.NoError(t, err, "Should update host1")

	cmd = fixture(t).Updated("host", "hbi", "hbi-instance-1", "host-2", "workspace-1")
	err = h.usecase.ReportResource(h.ctx, cmd)
	require.NoError(t, err, "Should update host2")

	updatedHost1, err := h.resourceRepo.FindResourceByKeys(nil, key1)
	require.NoError(t, err, "Should find host1 after update")
	require.NotNil(t, updatedHost1)

	updatedHost2, err := h.resourceRepo.FindResourceByKeys(nil, key2)
	require.NoError(t, err, "Should find host2 after update")
	require.NotNil(t, updatedHost2)

	err = h.usecase.Delete(h.ctx, key1)
	require.NoError(t, err, "Should delete host1")

	err = h.usecase.Delete(h.ctx, key2)
	require.NoError(t, err, "Should delete host2")

	foundHost1, err = h.resourceRepo.FindResourceByKeys(nil, key1)
	require.NoError(t, err, "Should find tombstoned host1")
	require.NotNil(t, foundHost1)
	assert.True(t, foundHost1.ReporterResources()[0].Serialize().Tombstone, "Host1 should be tombstoned")

	foundHost2, err = h.resourceRepo.FindResourceByKeys(nil, key2)
	require.NoError(t, err, "Should find tombstoned host2")
	require.NotNil(t, foundHost2)
	assert.True(t, foundHost2.ReporterResources()[0].Serialize().Tombstone, "Host2 should be tombstoned")
}

func TestPartialDataScenarios(t *testing.T) {
	h := newTestHarness(t, withNamespace("test-topic"))

	t.Run("Report resource with rich reporter data and minimal common data", func(t *testing.T) {
		cmd := fixture(t).ReporterRich("k8s_cluster", "ocm", "ocm-instance-1", "reporter-rich-resource", "minimal-workspace")
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Should create resource with rich reporter data")

		key := createReporterResourceKey(t, "reporter-rich-resource", "k8s_cluster", "ocm", "ocm-instance-1")
		foundResource, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource with rich reporter data")
		require.NotNil(t, foundResource)
	})

	t.Run("Report resource with minimal reporter data and rich common data", func(t *testing.T) {
		cmd := fixture(t).CommonRich("k8s_cluster", "ocm", "ocm-instance-1", "common-rich-resource", "rich-workspace")
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Should create resource with rich common data")

		key := createReporterResourceKey(t, "common-rich-resource", "k8s_cluster", "ocm", "ocm-instance-1")
		foundResource, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource with rich common data")
		require.NotNil(t, foundResource)
	})

	t.Run("Report resource with both data, then reporter-focused update, then common-focused update", func(t *testing.T) {
		cmd := fixture(t).Basic("k8s_cluster", "ocm", "ocm-instance-1", "progressive-resource", "initial-workspace")
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Should create resource with both data types")

		key := createReporterResourceKey(t, "progressive-resource", "k8s_cluster", "ocm", "ocm-instance-1")
		foundResource, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after initial creation")
		require.NotNil(t, foundResource)

		cmd = fixture(t).ReporterRich("k8s_cluster", "ocm", "ocm-instance-1", "progressive-resource", "initial-workspace")
		err = h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Should update resource with reporter-focused data")

		foundResource, err = h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after reporter-focused update")
		require.NotNil(t, foundResource)

		cmd = fixture(t).CommonRich("k8s_cluster", "ocm", "ocm-instance-1", "progressive-resource", "updated-workspace")
		err = h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Should update resource with common-focused data")

		finalResource, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after all updates")
		require.NotNil(t, finalResource)
	})
}

func TestResourceLifecycle_ReportUpdateDeleteReport(t *testing.T) {
	t.Run("report new -> update -> delete -> report new", func(t *testing.T) {
		h := newTestHarness(t, withNamespace("test-topic"))

		resourceType := "host"
		reporterType := "hbi"
		reporterInstance := "test-instance"
		localResourceId := "lifecycle-test-host"
		workspaceId := "test-workspace"

		// 1. REPORT NEW: Initial resource creation
		log.Info("Report New ---------------------")
		cmd := fixture(t).Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Initial report should succeed")

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		afterCreate, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after creation")
		require.NotNil(t, afterCreate)
		initialSnapshot := afterCreate.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(0), initialSnapshot.Generation, "Initial generation should be 0")
		assert.Equal(t, uint(0), initialSnapshot.RepresentationVersion, "Initial representationVersion should be 0")
		assert.False(t, initialSnapshot.Tombstone, "Initial tombstone should be false")

		cmd = fixture(t).Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Update should succeed")

		afterUpdate, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after update")
		require.NotNil(t, afterUpdate)
		updateSnapshot := afterUpdate.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(0), updateSnapshot.Generation, "Generation should remain 0 after update (tombstone=false)")
		assert.Equal(t, uint(1), updateSnapshot.RepresentationVersion, "RepresentationVersion should increment to 1 after update")
		assert.False(t, updateSnapshot.Tombstone, "Tombstone should remain false after update")

		err = h.usecase.Delete(h.ctx, key)
		require.NoError(t, err, "Delete should succeed")

		afterDelete, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete)
		deleteSnapshot := afterDelete.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(0), deleteSnapshot.Generation, "Generation should remain 0 after delete")
		assert.Equal(t, uint(2), deleteSnapshot.RepresentationVersion, "RepresentationVersion should increment to 2 after delete")
		assert.True(t, deleteSnapshot.Tombstone, "Resource should be tombstoned after delete")

		cmd = fixture(t).Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Report after delete should succeed")

		afterRevive, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after revival")
		require.NotNil(t, afterRevive)
		reviveSnapshot := afterRevive.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(1), reviveSnapshot.Generation, "Generation should increment to 1 after update on tombstoned resource")
		assert.Equal(t, uint(0), reviveSnapshot.RepresentationVersion, "RepresentationVersion should start fresh at 0 for revival (new generation)")
		assert.False(t, reviveSnapshot.Tombstone, "Resource should no longer be tombstoned after revival update")
	})
}

func TestResourceLifecycle_ReportUpdateDeleteReportDelete(t *testing.T) {
	t.Run("report new -> update -> delete -> report new -> delete", func(t *testing.T) {
		h := newTestHarness(t, withNamespace("test-topic"))

		resourceType := "k8s_cluster"
		reporterType := "ocm"
		reporterInstance := "ocm-instance"
		localResourceId := "lifecycle-test-cluster"
		workspaceId := "test-workspace-2"

		cmd := fixture(t).Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Initial report should succeed")

		cmd = fixture(t).Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Update should succeed")

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)
		err = h.usecase.Delete(h.ctx, key)
		require.NoError(t, err, "First delete should succeed")

		cmd = fixture(t).Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Report after delete should succeed")

		err = h.usecase.Delete(h.ctx, key)
		require.NoError(t, err, "Second delete should succeed")

		finalResource, err := h.resourceRepo.FindResourceByKeys(nil, key)
		if err == gorm.ErrRecordNotFound {
			assert.Nil(t, finalResource, "Resource should not be found if tombstone filter is active")
		} else {
			require.NoError(t, err, "Should find tombstoned resource if filter is removed")
			require.NotNil(t, finalResource)
		}
	})
}

func createReporterResourceKey(t *testing.T, localResourceId, resourceType, reporterType, reporterInstance string) model.ReporterResourceKey {
	t.Helper()
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

// createReporterResourceKeyAllowEmptyInstance is like createReporterResourceKey but
// allows an empty reporterInstance (bypassing validation) for delete-without-instance tests.
func createReporterResourceKeyAllowEmptyInstance(t *testing.T, localResourceId, resourceType, reporterType, reporterInstance string) model.ReporterResourceKey {
	t.Helper()
	localResourceIdType, err := model.NewLocalResourceId(localResourceId)
	require.NoError(t, err)
	resourceTypeType, err := model.NewResourceType(resourceType)
	require.NoError(t, err)
	reporterTypeType, err := model.NewReporterType(reporterType)
	require.NoError(t, err)

	var reporterInstanceIdType model.ReporterInstanceId
	if reporterInstance != "" {
		reporterInstanceIdType, err = model.NewReporterInstanceId(reporterInstance)
		require.NoError(t, err)
	} else {
		reporterInstanceIdType = model.ReporterInstanceId("")
	}

	key, err := model.NewReporterResourceKey(localResourceIdType, resourceTypeType, reporterTypeType, reporterInstanceIdType)
	require.NoError(t, err)
	return key
}

func TestResourceLifecycle_ReportDeleteResubmitDelete(t *testing.T) {
	t.Run("report -> delete -> resubmit same delete", func(t *testing.T) {
		h := newTestHarness(t, withNamespace("test-topic"))

		resourceType := "k8s_cluster"
		reporterType := "ocm"
		reporterInstance := "idempotent-instance"
		localResourceId := "idempotent-test-resource"
		workspaceId := "idempotent-workspace"

		cmd := fixture(t).Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Initial report should succeed")

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		afterReport1, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after initial report")
		require.NotNil(t, afterReport1)
		initialState := afterReport1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(0), initialState.RepresentationVersion, "Initial representationVersion should be 0")
		assert.Equal(t, uint(0), initialState.Generation, "Initial generation should be 0")
		assert.False(t, initialState.Tombstone, "Initial tombstone should be false")

		err = h.usecase.Delete(h.ctx, key)
		require.NoError(t, err, "Delete should succeed")

		afterDelete1, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete1)
		deleteState1 := afterDelete1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(1), deleteState1.RepresentationVersion, "RepresentationVersion should increment after delete")
		assert.Equal(t, uint(0), deleteState1.Generation, "Generation should remain 0 after delete")
		assert.True(t, deleteState1.Tombstone, "Resource should be tombstoned")

		err = h.usecase.Delete(h.ctx, key)
		require.NoError(t, err, "Resubmitted delete should succeed (idempotent)")

		afterDelete2, err := h.resourceRepo.FindResourceByKeys(nil, key)
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
		h := newTestHarness(t, withNamespace("test-topic"))

		resourceType := "host"
		reporterType := "hbi"
		reporterInstance := "idempotent-instance-2"
		localResourceId := "idempotent-test-resource-2"
		workspaceId := "idempotent-workspace-2"

		// 1. REPORT: Initial resource creation
		log.Info("1. Initial Report ---------------------")
		cmd := fixture(t).Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Initial report should succeed")

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		afterReport1, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after initial report")
		require.NotNil(t, afterReport1)
		initialState := afterReport1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(0), initialState.RepresentationVersion, "Initial representationVersion should be 0")
		assert.Equal(t, uint(0), initialState.Generation, "Initial generation should be 0")
		assert.False(t, initialState.Tombstone, "Initial tombstone should be false")

		cmd = fixture(t).Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Resubmitted report should succeed (idempotent)")

		afterReport2, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after duplicate report")
		require.NotNil(t, afterReport2)
		duplicateState := afterReport2.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(1), duplicateState.RepresentationVersion, "RepresentationVersion should increment after duplicate report")
		assert.Equal(t, uint(0), duplicateState.Generation, "Generation should remain 0")
		assert.False(t, duplicateState.Tombstone, "Resource should remain active")

		err = h.usecase.Delete(h.ctx, key)
		require.NoError(t, err, "Delete should succeed")

		afterDelete1, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete1)
		deleteState1 := afterDelete1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(2), deleteState1.RepresentationVersion, "RepresentationVersion should increment after delete")
		assert.Equal(t, uint(0), deleteState1.Generation, "Generation should remain 0 after delete")
		assert.True(t, deleteState1.Tombstone, "Resource should be tombstoned")

		err = h.usecase.Delete(h.ctx, key)
		require.NoError(t, err, "Resubmitted delete should succeed (idempotent)")

		afterDelete2, err := h.resourceRepo.FindResourceByKeys(nil, key)
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
		h := newTestHarness(t, withNamespace("test-topic"))

		resourceType := "k8s_cluster"
		reporterType := "ocm"
		reporterInstance := "complex-idempotent-instance"
		localResourceId := "complex-idempotent-resource"
		workspaceId := "complex-idempotent-workspace"

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		for cycle := 0; cycle < 3; cycle++ {
			t.Logf("=== Cycle %d: Create+Update+Delete ===", cycle)

			cmd := fixture(t).WithCycleData(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, cycle)
			err := h.usecase.ReportResource(h.ctx, cmd)
			require.NoError(t, err, "Report should succeed in cycle %d", cycle)

			afterReport, err := h.resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err, "Should find resource after report in cycle %d", cycle)
			require.NotNil(t, afterReport)
			reportState := afterReport.ReporterResources()[0].Serialize()

			expectedGeneration := uint(cycle)
			assert.Equal(t, expectedGeneration, reportState.Generation, "Generation should be %d in cycle %d", expectedGeneration, cycle)
			assert.Equal(t, uint(0), reportState.RepresentationVersion, "RepresentationVersion should reset to 0 for new generation in cycle %d", cycle)
			assert.False(t, reportState.Tombstone, "Resource should be active after report in cycle %d", cycle)

			cmd = fixture(t).Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
			err = h.usecase.ReportResource(h.ctx, cmd)
			require.NoError(t, err, "Update should succeed in cycle %d", cycle)

			afterUpdate, err := h.resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err, "Should find resource after update in cycle %d", cycle)
			require.NotNil(t, afterUpdate)
			updateState := afterUpdate.ReporterResources()[0].Serialize()
			assert.Equal(t, expectedGeneration, updateState.Generation, "Generation should remain %d after update in cycle %d", expectedGeneration, cycle)
			assert.Equal(t, uint(1), updateState.RepresentationVersion, "RepresentationVersion should increment to 1 after update in cycle %d", cycle)
			assert.False(t, updateState.Tombstone, "Resource should remain active after update in cycle %d", cycle)

			err = h.usecase.Delete(h.ctx, key)
			require.NoError(t, err, "Delete should succeed in cycle %d", cycle)

			afterDelete, err := h.resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err, "Should find tombstoned resource after delete in cycle %d", cycle)
			require.NotNil(t, afterDelete)
			deleteState := afterDelete.ReporterResources()[0].Serialize()
			assert.Equal(t, expectedGeneration, deleteState.Generation, "Generation should remain %d after delete in cycle %d", expectedGeneration, cycle)
			assert.Equal(t, uint(2), deleteState.RepresentationVersion, "RepresentationVersion should increment to 2 after delete in cycle %d", cycle)
			assert.True(t, deleteState.Tombstone, "Resource should be tombstoned after delete in cycle %d", cycle)

			t.Logf("Cycle %d complete: Final state {Generation: %d, RepVersion: %d, Tombstone: %t}",
				cycle, deleteState.Generation, deleteState.RepresentationVersion, deleteState.Tombstone)
		}

		finalResource, err := h.resourceRepo.FindResourceByKeys(nil, key)
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
	h := newTestHarness(t, withNamespace("test-topic"))

	t.Run("Nil TransactionId in command resolves to generated ID and create succeeds", func(t *testing.T) {
		cmd := fixture(t).Basic("host", "hbi", "nil-tx-instance", "local-nil-tx", "ws-nil-tx")
		require.Nil(t, cmd.TransactionId, "fixture Basic leaves TransactionId nil")
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "ReportResource with nil TransactionId should succeed (ID generated)")
		key := createReporterResourceKey(t, "local-nil-tx", "host", "hbi", "nil-tx-instance")
		found, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err)
		require.NotNil(t, found, "Resource should exist after create with nil TransactionId")
	})

	t.Run("Same transaction ID should be idempotent - no changes to representation tables", func(t *testing.T) {
		cmd := fixture(t).WithTransactionId("host", "hbi", "test-instance", "test-resource", "test-workspace", "test-transaction-123")
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "First report should succeed")

		key := createReporterResourceKey(t, "test-resource", "host", "hbi", "test-instance")
		afterFirst, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after first report")
		require.NotNil(t, afterFirst)
		firstState := afterFirst.ReporterResources()[0].Serialize()

		cmd = fixture(t).WithTransactionId("host", "hbi", "test-instance", "test-resource", "test-workspace", "test-transaction-123")
		err = h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Second report with same transaction ID should succeed (idempotent)")

		afterSecond, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after second report")
		require.NotNil(t, afterSecond)
		secondState := afterSecond.ReporterResources()[0].Serialize()

		assert.Equal(t, firstState.RepresentationVersion, secondState.RepresentationVersion, "RepresentationVersion should not change for idempotent request")
		assert.Equal(t, firstState.Generation, secondState.Generation, "Generation should not change for idempotent request")
	})

	t.Run("Different transaction ID should update representations", func(t *testing.T) {
		cmd := fixture(t).WithTransactionId("host", "hbi", "test-instance-2", "test-resource-2", "test-workspace-2", "test-transaction-456")
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "First report should succeed")

		key := createReporterResourceKey(t, "test-resource-2", "host", "hbi", "test-instance-2")
		afterFirst, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after first report")
		require.NotNil(t, afterFirst)
		firstState := afterFirst.ReporterResources()[0].Serialize()

		cmd = fixture(t).UpdatedWithTransactionId("host", "hbi", "test-instance-2", "test-resource-2", "test-workspace-2", "test-transaction-789")
		err = h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Second report with different transaction ID should succeed")

		afterSecond, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after second report")
		require.NotNil(t, afterSecond)
		secondState := afterSecond.ReporterResources()[0].Serialize()

		assert.Equal(t, firstState.RepresentationVersion+1, secondState.RepresentationVersion, "RepresentationVersion should increment for different transaction ID")
		assert.Equal(t, firstState.Generation, secondState.Generation, "Generation should remain the same for update")
	})

	t.Run("Report with transaction ID -> Update with new transaction ID -> Delete should update representations", func(t *testing.T) {
		cmd := fixture(t).WithTransactionId("host", "hbi", "test-instance-3", "test-resource-3", "test-workspace-3", "test-transaction-111")
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "First report should succeed")

		key := createReporterResourceKey(t, "test-resource-3", "host", "hbi", "test-instance-3")
		afterFirst, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after first report")
		require.NotNil(t, afterFirst)
		firstState := afterFirst.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(0), firstState.RepresentationVersion, "Initial representationVersion should be 0")
		assert.Equal(t, uint(0), firstState.Generation, "Initial generation should be 0")
		assert.False(t, firstState.Tombstone, "Initial tombstone should be false")

		cmd = fixture(t).UpdatedWithTransactionId("host", "hbi", "test-instance-3", "test-resource-3", "test-workspace-3", "test-transaction-222")
		err = h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Update with different transaction ID should succeed")

		afterUpdate, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after update")
		require.NotNil(t, afterUpdate)
		updateState := afterUpdate.ReporterResources()[0].Serialize()
		assert.Equal(t, firstState.RepresentationVersion+1, updateState.RepresentationVersion, "RepresentationVersion should increment after update")
		assert.Equal(t, firstState.Generation, updateState.Generation, "Generation should remain the same after update")
		assert.False(t, updateState.Tombstone, "Resource should remain active after update")

		err = h.usecase.Delete(h.ctx, key)
		require.NoError(t, err, "Delete should succeed")

		afterDelete, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete)
		deleteState := afterDelete.ReporterResources()[0].Serialize()
		assert.Equal(t, updateState.RepresentationVersion+1, deleteState.RepresentationVersion, "RepresentationVersion should increment after delete")
		assert.Equal(t, updateState.Generation, deleteState.Generation, "Generation should remain the same after delete")
		assert.True(t, deleteState.Tombstone, "Resource should be tombstoned after delete")
	})

	t.Run("Report with transaction ID -> Report with same transaction ID -> Delete should update representations", func(t *testing.T) {
		cmd := fixture(t).WithTransactionId("host", "hbi", "test-instance-4", "test-resource-4", "test-workspace-4", "test-transaction-333")
		err := h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "First report should succeed")

		key := createReporterResourceKey(t, "test-resource-4", "host", "hbi", "test-instance-4")
		afterFirst, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after first report")
		require.NotNil(t, afterFirst)
		firstState := afterFirst.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(0), firstState.RepresentationVersion, "Initial representationVersion should be 0")
		assert.Equal(t, uint(0), firstState.Generation, "Initial generation should be 0")
		assert.False(t, firstState.Tombstone, "Initial tombstone should be false")

		cmd = fixture(t).WithTransactionId("host", "hbi", "test-instance-4", "test-resource-4", "test-workspace-4", "test-transaction-333")
		err = h.usecase.ReportResource(h.ctx, cmd)
		require.NoError(t, err, "Second report with same transaction ID should succeed (idempotent)")

		afterSecond, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after second report")
		require.NotNil(t, afterSecond)
		secondState := afterSecond.ReporterResources()[0].Serialize()
		assert.Equal(t, firstState.RepresentationVersion, secondState.RepresentationVersion, "RepresentationVersion should not change for idempotent request")
		assert.Equal(t, firstState.Generation, secondState.Generation, "Generation should not change for idempotent request")
		assert.False(t, secondState.Tombstone, "Resource should remain active")

		err = h.usecase.Delete(h.ctx, key)
		require.NoError(t, err, "Delete should succeed")

		afterDelete, err := h.resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete)
		deleteState := afterDelete.ReporterResources()[0].Serialize()
		assert.Equal(t, secondState.RepresentationVersion+1, deleteState.RepresentationVersion, "RepresentationVersion should increment after delete")
		assert.Equal(t, secondState.Generation, deleteState.Generation, "Generation should remain the same after delete")
		assert.True(t, deleteState.Tombstone, "Resource should be tombstoned after delete")
	})
}

func TestReportResource_ValidationSuccess(t *testing.T) {
	h := newTestHarness(t, withNamespace("test-topic"))

	cmd := fixture(t).WithData("host", "hbi", "instance-1", "test-host",
		map[string]interface{}{
			"satellite_id": "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
			"ansible_host": "host-1",
		},
		map[string]interface{}{
			"workspace_id": "ws-123",
		},
	)

	err := h.usecase.ReportResource(h.ctx, cmd)
	require.NoError(t, err)
}

func TestReportResource_ValidationErrors(t *testing.T) {
	h := newTestHarness(t, withNamespace("test-topic"))

	tests := []struct {
		name        string
		cmd         ReportResourceCommand
		expectError string
	}{
		{
			name: "missing type",
			cmd: func() ReportResourceCommand {
				cmd := fixture(t).Basic("host", "hbi", "instance-1", "test-host", "ws-123")
				cmd.ResourceType = ""
				return cmd
			}(),
			expectError: "missing 'type' field",
		},
		{
			name: "missing reporterType",
			cmd: func() ReportResourceCommand {
				cmd := fixture(t).Basic("host", "hbi", "instance-1", "test-host", "ws-123")
				cmd.ReporterType = ""
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
			name: "both representations nil returns error",
			cmd: func() ReportResourceCommand {
				cmd := fixture(t).Basic("host", "hbi", "instance-1", "test-host", "ws-123")
				cmd.ReporterRepresentation = nil
				cmd.CommonRepresentation = nil
				return cmd
			}(),
			expectError: "at least one of reporterRepresentation or commonRepresentation must be provided",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := h.usecase.ReportResource(h.ctx, tc.cmd)
			if tc.expectError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
			} else {
				assert.NoError(t, err)
			}
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
				&data.AllowAllRelationsRepository{},
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

func TestReportResource_BothRepresentationsNil(t *testing.T) {
	h := newTestHarness(t, withNamespace("test-topic"))

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
		ReporterRepresentation: nil,
		CommonRepresentation:   nil,
		WriteVisibility:        WriteVisibilityMinimizeLatency,
	}

	err := h.usecase.ReportResource(h.ctx, cmd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of reporterRepresentation or commonRepresentation must be provided")
}

func TestReportResource_ValidationErrorFormat(t *testing.T) {
	h := newTestHarness(t, withNamespace("test-topic"))

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
			err := h.usecase.ReportResource(h.ctx, tc.cmd)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectErrorMsg)
		})
	}
}

func TestResolveConsistency(t *testing.T) {
	tests := []struct {
		name               string
		featureFlagEnabled bool
		consistency        model.Consistency
		resourceExists     bool
		expectedType       int
		expectedToken      string
		expectedError      bool
	}{
		// Feature flag only affects DEFAULT/UNSPECIFIED behavior - explicit preferences are always honored
		{
			name:               "feature flag disabled - unspecified defaults to minimize_latency",
			featureFlagEnabled: false,
			consistency:        model.NewConsistencyUnspecified(),
			resourceExists:     true,
			expectedToken:      "",
			expectedError:      false,
		},
		{
			name:               "feature flag disabled - minimize_latency returns empty",
			featureFlagEnabled: false,
			consistency:        model.NewConsistencyMinimizeLatency(),
			resourceExists:     false,
			expectedToken:      "",
			expectedError:      false,
		},
		{
			name:               "feature flag disabled - at_least_as_fresh still honored",
			featureFlagEnabled: false,
			consistency:        model.NewConsistencyAtLeastAsFresh(model.ConsistencyToken("client-provided-token")),
			resourceExists:     false,
			expectedToken:      "client-provided-token",
			expectedError:      false,
		},
		{
			name:               "feature flag disabled - at_least_as_acknowledged still honored",
			featureFlagEnabled: false,
			consistency:        model.NewConsistencyAtLeastAsAcknowledged(),
			resourceExists:     true,
			expectedToken:      "", // fake repo returns empty consistency token
			expectedError:      false,
		},
		{
			name:               "feature flag enabled - unspecified defaults to at_least_as_acknowledged (DB lookup)",
			featureFlagEnabled: true,
			consistency:        model.NewConsistencyUnspecified(),
			resourceExists:     true,
			expectedToken:      "", // fake repo returns empty consistency token
			expectedError:      false,
		},
		{
			name:               "feature flag enabled - minimize_latency returns empty token",
			featureFlagEnabled: true,
			consistency:        model.NewConsistencyMinimizeLatency(),
			resourceExists:     false,
			expectedToken:      "",
			expectedError:      false,
		},
		{
			name:               "feature flag enabled - at_least_as_fresh returns provided token",
			featureFlagEnabled: true,
			consistency:        model.NewConsistencyAtLeastAsFresh(model.ConsistencyToken("client-provided-token")),
			resourceExists:     false,
			expectedToken:      "client-provided-token",
			expectedError:      false,
		},
		{
			name:               "feature flag enabled - at_least_as_fresh with empty token",
			featureFlagEnabled: true,
			consistency:        model.NewConsistencyAtLeastAsFresh(model.MinimizeLatencyToken),
			resourceExists:     false,
			expectedToken:      "",
			expectedError:      false,
		},
		{
			name:               "feature flag enabled - at_least_as_acknowledged resource not found falls back to empty",
			featureFlagEnabled: true,
			consistency:        model.NewConsistencyAtLeastAsAcknowledged(),
			resourceExists:     false,
			expectedToken:      "",
			expectedError:      false,
		},
		{
			name:               "feature flag enabled - at_least_as_acknowledged resource exists returns token",
			featureFlagEnabled: true,
			consistency:        model.NewConsistencyAtLeastAsAcknowledged(),
			resourceExists:     true,
			expectedToken:      "", // fake repo returns empty consistency token
			expectedError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHarness(t, withNamespace("test-topic"), withUsecaseConfig(&UsecaseConfig{
				DefaultToAtLeastAsAcknowledged: tt.featureFlagEnabled,
			}))

			reporterResourceKey := createReporterResourceKey(t, "test-resource-123", "host", "hbi", "test-instance")

			if tt.resourceExists {
				cmd := fixture(t).Basic("host", "hbi", "test-instance", "test-resource-123", "test-workspace")
				err := h.usecase.ReportResource(h.ctx, cmd)
				require.NoError(t, err)
			}

			result, err := h.usecase.resolveConsistency(h.ctx, tt.consistency, reporterResourceKey, false)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.expectedToken != "" {
					token := model.ConsistencyAtLeastAsFreshToken(result)
					require.NotNil(t, token)
					assert.Equal(t, tt.expectedToken, token.Serialize())
				}
			}
		})
	}
}

func TestResolveConsistency_OverrideFeatureFlag(t *testing.T) {
	tests := []struct {
		name               string
		featureFlagEnabled bool
		consistency        model.Consistency
		resourceExists     bool
		expectedToken      string
		expectedError      bool
	}{
		{
			name:               "feature flag enabled - unspecified bypasses to minimize_latency when override is true",
			featureFlagEnabled: true,
			consistency:        model.NewConsistencyUnspecified(),
			resourceExists:     true,
			expectedToken:      "",
			expectedError:      false,
		},
		{
			name:               "feature flag disabled - unspecified remains minimize_latency when override is true",
			featureFlagEnabled: false,
			consistency:        model.NewConsistencyUnspecified(),
			resourceExists:     true,
			expectedToken:      "",
			expectedError:      false,
		},
		{
			name:               "feature flag enabled - at_least_as_fresh still returns client token when override is true",
			featureFlagEnabled: true,
			consistency:        model.NewConsistencyAtLeastAsFresh(model.ConsistencyToken("client-provided-token")),
			resourceExists:     false,
			expectedToken:      "client-provided-token",
			expectedError:      false,
		},
		{
			name:               "feature flag enabled - at_least_as_acknowledged still performs DB lookup when override is true",
			featureFlagEnabled: true,
			consistency:        model.NewConsistencyAtLeastAsAcknowledged(),
			resourceExists:     true,
			expectedToken:      "", // fake repo returns empty consistency token
			expectedError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHarness(t, withNamespace("test-topic"), withUsecaseConfig(&UsecaseConfig{
				DefaultToAtLeastAsAcknowledged: tt.featureFlagEnabled,
			}))

			reporterResourceKey := createReporterResourceKey(t, "test-resource-override", "host", "hbi", "test-instance")

			if tt.resourceExists {
				cmd := fixture(t).Basic("host", "hbi", "test-instance", "test-resource-override", "test-workspace")
				err := h.usecase.ReportResource(h.ctx, cmd)
				require.NoError(t, err)
			}

			result, err := h.usecase.resolveConsistency(h.ctx, tt.consistency, reporterResourceKey, true)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.expectedToken != "" {
					token := model.ConsistencyAtLeastAsFreshToken(result)
					require.NotNil(t, token)
					assert.Equal(t, tt.expectedToken, token.Serialize())
				}
			}
		})
	}
}

func TestResolveConsistency_NilConsistencyDefaultsToUnspecified(t *testing.T) {
	h := newTestHarness(t, withNamespace("test-topic"))

	reporterResourceKey := createReporterResourceKey(t, "test-resource-456", "host", "hbi", "test-instance")
	result, err := h.usecase.resolveConsistency(h.ctx, nil, reporterResourceKey, false)

	assert.NoError(t, err)
	assert.Equal(t, model.ConsistencyMinimizeLatency, model.ConsistencyTypeOf(result))
}

func TestResolveConsistency_OverrideFeatureFlag_LogsBypassWhenEnabled(t *testing.T) {
	var logBuf bytes.Buffer
	h := newTestHarness(t,
		withNamespace("test-topic"),
		withLogger(log.NewStdLogger(&logBuf)),
		withUsecaseConfig(&UsecaseConfig{DefaultToAtLeastAsAcknowledged: true}),
	)

	reporterResourceKey := createReporterResourceKey(t, "host-123", "host", "hbi", "instance-1")
	_, err := h.usecase.resolveConsistency(h.ctx, model.NewConsistencyUnspecified(), reporterResourceKey, true)
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(logBuf.String()), "enabled but bypassed for this call")
}

func TestResolveConsistency_OverrideFeatureFlag_LogsEnabledWhenNotBypassed(t *testing.T) {
	var logBuf bytes.Buffer
	h := newTestHarness(t,
		withNamespace("test-topic"),
		withLogger(log.NewStdLogger(&logBuf)),
		withUsecaseConfig(&UsecaseConfig{DefaultToAtLeastAsAcknowledged: true}),
	)

	reporterResourceKey := createReporterResourceKey(t, "host-456", "host", "hbi", "instance-1")
	_, err := h.usecase.resolveConsistency(h.ctx, model.NewConsistencyUnspecified(), reporterResourceKey, false)
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(logBuf.String()), "feature flag default-to-at-least-as-acknowledged is enabled")
}

func TestResolveConsistency_OverrideFeatureFlag_LogsDisabledWhenFeatureOff(t *testing.T) {
	var logBuf bytes.Buffer
	h := newTestHarness(t,
		withNamespace("test-topic"),
		withLogger(log.NewStdLogger(&logBuf)),
		withUsecaseConfig(&UsecaseConfig{DefaultToAtLeastAsAcknowledged: false}),
	)

	reporterResourceKey := createReporterResourceKey(t, "host-789", "host", "hbi", "instance-1")
	_, err := h.usecase.resolveConsistency(h.ctx, model.NewConsistencyUnspecified(), reporterResourceKey, true)
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(logBuf.String()), "feature flag default-to-at-least-as-acknowledged is disabled")
}

func TestCheck_AuthzDecisions(t *testing.T) {
	tests := []struct {
		name           string
		grantSubjectID string
		wantAllowed    bool
	}{
		{"denied - no grants", "", false},
		{"allowed - grant exists", "user-1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			simpleAuthz := data.NewSimpleRelationsRepository()
			if tt.grantSubjectID != "" {
				simpleAuthz.Grant(tt.grantSubjectID, "view", "hbi", "host", "host-1")
			}
			h := newTestHarness(t, withMeta(true), withRelations(simpleAuthz))

			subject, err := buildTestSubjectReference("user-1")
			require.NoError(t, err)
			key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
			relation, err := model.NewRelation("view")
			require.NoError(t, err)

			allowed, token, err := h.usecase.Check(h.ctx, relation, subject, key, model.NewConsistencyUnspecified())
			require.NoError(t, err)
			assert.Equal(t, tt.wantAllowed, allowed)
			assert.NotEmpty(t, token)
			assert.Equal(t, 1, h.meta.calls)
		})
	}
}

func TestCheckForUpdate_AuthzDecisions(t *testing.T) {
	tests := []struct {
		name           string
		grantSubjectID string
		wantAllowed    bool
	}{
		{"denied - no grants", "", false},
		{"allowed - grant exists", "user-1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			simpleAuthz := data.NewSimpleRelationsRepository()
			if tt.grantSubjectID != "" {
				simpleAuthz.Grant(tt.grantSubjectID, "update", "hbi", "host", "host-1")
			}
			h := newTestHarness(t, withMeta(true), withRelations(simpleAuthz))

			subject, err := buildTestSubjectReference("user-1")
			require.NoError(t, err)
			key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
			relation, err := model.NewRelation("update")
			require.NoError(t, err)

			allowed, token, err := h.usecase.CheckForUpdate(h.ctx, relation, subject, key)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAllowed, allowed)
			assert.NotEmpty(t, token)
			assert.Equal(t, 1, h.meta.calls)
		})
	}
}

func TestCheckBulk_RelationsMixedResults(t *testing.T) {
	simpleAuthz := data.NewSimpleRelationsRepository()
	simpleAuthz.Grant("user-1", "view", "hbi", "host", "host-1")

	h := newTestHarness(t, withMeta(true), withRelations(simpleAuthz))

	subject, err := buildTestSubjectReference("user-1")
	require.NoError(t, err)
	key1 := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
	key2 := createReporterResourceKey(t, "host-2", "host", "hbi", "instance-1")
	relation, err := model.NewRelation("view")
	require.NoError(t, err)

	result, err := h.usecase.CheckBulk(h.ctx, CheckBulkCommand{
		Items: []CheckBulkItem{
			{Resource: key1, Relation: relation, Subject: subject},
			{Resource: key2, Relation: relation, Subject: subject},
		},
		Consistency: model.NewConsistencyUnspecified(),
	})
	require.NoError(t, err)
	require.Len(t, result.Pairs, 2)
	assert.True(t, result.Pairs[0].Result.Allowed, "host-1 should be allowed")
	assert.Nil(t, result.Pairs[0].Result.Error)
	assert.False(t, result.Pairs[1].Result.Allowed, "host-2 should be denied")
	assert.Nil(t, result.Pairs[1].Result.Error)
	assert.Equal(t, 2, h.meta.calls)
}

func TestLookupResources_StreamResults(t *testing.T) {
	type grant struct {
		subjectID, resourceID string
	}
	tests := []struct {
		name    string
		grants  []grant
		wantIDs []string
	}{
		{
			"returns granted resources",
			[]grant{{"user-1", "host-1"}, {"user-1", "host-2"}},
			[]string{"host-1", "host-2"},
		},
		{
			"empty - no grants",
			nil,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			simpleAuthz := data.NewSimpleRelationsRepository()
			for _, g := range tt.grants {
				simpleAuthz.Grant(g.subjectID, "view", "hbi", "host", g.resourceID)
			}
			h := newTestHarness(t, withMeta(true), withRelations(simpleAuthz))

			subject, err := buildTestSubjectReference("user-1")
			require.NoError(t, err)
			resourceType, err := model.NewResourceType("host")
			require.NoError(t, err)
			reporterType, err := model.NewReporterType("hbi")
			require.NoError(t, err)
			relation, err := model.NewRelation("view")
			require.NoError(t, err)

			stream, err := h.usecase.LookupResources(h.ctx, LookupResourcesCommand{
				ResourceType: resourceType,
				ReporterType: reporterType,
				Relation:     relation,
				Subject:      subject,
				Consistency:  model.NewConsistencyUnspecified(),
			})
			require.NoError(t, err)

			var resourceIds []string
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				resourceIds = append(resourceIds, string(resp.ResourceId))
			}

			slices.Sort(resourceIds)
			wantSorted := slices.Clone(tt.wantIDs)
			slices.Sort(wantSorted)
			assert.Equal(t, wantSorted, resourceIds)
			assert.Equal(t, 1, h.meta.calls)
		})
	}
}

func TestLookupSubjects_StreamResults(t *testing.T) {
	tests := []struct {
		name            string
		grantSubjectIDs []string
		wantIDs         []string
	}{
		{
			"returns granted subjects",
			[]string{"user-1", "user-2"},
			[]string{"user-1", "user-2"},
		},
		{
			"empty - no grants",
			nil,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			simpleAuthz := data.NewSimpleRelationsRepository()
			for _, subjectID := range tt.grantSubjectIDs {
				simpleAuthz.Grant(subjectID, "view", "hbi", "host", "host-1")
			}
			h := newTestHarness(t, withMeta(true), withRelations(simpleAuthz))

			key := createReporterResourceKey(t, "host-1", "host", "hbi", "instance-1")
			relation, err := model.NewRelation("view")
			require.NoError(t, err)
			subjectType, err := model.NewResourceType("principal")
			require.NoError(t, err)
			subjectReporter, err := model.NewReporterType("rbac")
			require.NoError(t, err)

			stream, err := h.usecase.LookupSubjects(h.ctx, LookupSubjectsCommand{
				Resource:        key,
				Relation:        relation,
				SubjectType:     subjectType,
				SubjectReporter: subjectReporter,
				Consistency:     model.NewConsistencyUnspecified(),
			})
			require.NoError(t, err)

			var subjectIds []string
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				subjectIds = append(subjectIds, string(resp.SubjectId))
			}

			slices.Sort(subjectIds)
			wantSorted := slices.Clone(tt.wantIDs)
			slices.Sort(wantSorted)
			assert.Equal(t, wantSorted, subjectIds)
			assert.Equal(t, 1, h.meta.calls)
		})
	}
}
