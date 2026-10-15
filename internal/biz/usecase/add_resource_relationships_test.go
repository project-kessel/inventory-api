package usecase

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"gorm.io/gorm"
)

// --- Test harness ---

type testHarness struct {
	usecase      *AddResourceRelationshipsUseCase
	resourceRepo model.ResourceRepository
	schemaRepo   model.SchemaRepository
	metrics      *metricscollector.MetricsCollector
	meta         *recordingMetaAuthorizer
	ctx          context.Context
}

type harnessOption func(*harnessConfig)

type harnessConfig struct {
	meta *recordingMetaAuthorizer
}

func newTestHarness(t *testing.T, opts ...harnessOption) *testHarness {
	t.Helper()
	cfg := &harnessConfig{}
	for _, o := range opts {
		o(cfg)
	}

	resourceRepo := data.NewFakeResourceRepository()
	schemaRepo := newFakeSchemaRepository(t)
	mc := metricscollector.NewFakeMetricsCollector()
	var metaAuth metaauthorizer.MetaAuthorizer
	if cfg.meta != nil {
		metaAuth = cfg.meta
	}

	uc := NewAddResourceRelationshipsUseCase(
		resourceRepo,
		schemaRepo,
		nil, // listen manager: read-after-write not exercised in these tests
		nil, // circuit breaker: read-after-write not exercised in these tests
		resources.NewUsecaseConfig(),
		mc,
		metaAuth,
		log.DefaultLogger,
	)

	return &testHarness{
		usecase:      uc,
		resourceRepo: resourceRepo,
		schemaRepo:   schemaRepo,
		metrics:      mc,
		meta:         cfg.meta,
		ctx:          testAuthzContext(),
	}
}

func withMeta(allowed bool) harnessOption {
	return func(c *harnessConfig) {
		c.meta = &recordingMetaAuthorizer{allowed: allowed}
	}
}

// reportResource seeds an existing resource through the ReportResource use
// case so that merge behavior is exercised against realistically stored state.
func (h *testHarness) reportResource(t *testing.T, cmd resources.ReportResourceCommand) error {
	t.Helper()
	seedUsecase := resources.New(
		h.resourceRepo,
		h.schemaRepo,
		&data.AllowAllRelationsRepository{},
		"rbac",
		log.DefaultLogger,
		nil,
		nil,
		resources.NewUsecaseConfig(),
		h.metrics,
		h.meta,
		nil,
	)
	return seedUsecase.ReportResource(h.ctx, cmd)
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

func createReporterResourceKey(t *testing.T, localResourceId, resourceType, reporterType, reporterInstance string) model.ReporterResourceKey {
	t.Helper()
	localId, err := model.NewLocalResourceId(localResourceId)
	require.NoError(t, err)
	resType, err := model.NewResourceType(resourceType)
	require.NoError(t, err)
	repType, err := model.NewReporterType(reporterType)
	require.NoError(t, err)
	instanceId, err := model.NewReporterInstanceId(reporterInstance)
	require.NoError(t, err)
	key, err := model.NewReporterResourceKey(localId, resType, repType, instanceId)
	require.NoError(t, err)
	return key
}

// newFakeSchemaRepository registers schemas for the resource types used by the
// add-relationships tests.
//
//   - workspace/features: reporter schema with a multi-valued relationship
//     field (direct_service_preferences) and a single-valued one
//     (direct_billing_account); optional common workspace_id.
//   - host/hbi: common schema requiring workspace_id (used to verify that
//     auto-create is rejected when required non-relationship fields are
//     missing).
func newFakeSchemaRepository(t *testing.T) model.SchemaRepository {
	t.Helper()
	schemaRepository := data.NewInMemorySchemaRepository()

	emptyValidationSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
		},
		"required": []
	}`)

	optionalWorkspaceCommonSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"workspace_id": { "type": "string" }
		},
		"required": []
	}`)

	requiredWorkspaceCommonSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"workspace_id": { "type": "string" }
		},
		"required": ["workspace_id"]
	}`)

	featuresWorkspaceReporterSchema := data.NewJsonSchemaWithWorkspacesFromString(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"direct_service_preferences": {
				"type": "array",
				"items": { "type": "string" }
			},
			"direct_billing_account": { "type": "string" }
		},
		"required": []
	}`)

	workspace, err := model.NewResourceType("workspace")
	require.NoError(t, err)
	host, err := model.NewResourceType("host")
	require.NoError(t, err)
	features, err := model.NewReporterType("features")
	require.NoError(t, err)
	hbi, err := model.NewReporterType("hbi")
	require.NoError(t, err)

	workspaceSchema, err := model.NewResourceSchemaRepresentation(workspace, optionalWorkspaceCommonSchema)
	require.NoError(t, err)
	require.NoError(t, schemaRepository.CreateResourceSchema(context.Background(), workspaceSchema))

	featuresWorkspace, err := model.NewReporterSchemaRepresentation(workspace, features, featuresWorkspaceReporterSchema)
	require.NoError(t, err)
	require.NoError(t, schemaRepository.CreateReporterSchema(context.Background(), featuresWorkspace))

	hostSchema, err := model.NewResourceSchemaRepresentation(host, requiredWorkspaceCommonSchema)
	require.NoError(t, err)
	require.NoError(t, schemaRepository.CreateResourceSchema(context.Background(), hostSchema))

	hbiHost, err := model.NewReporterSchemaRepresentation(host, hbi, emptyValidationSchema)
	require.NoError(t, err)
	require.NoError(t, schemaRepository.CreateReporterSchema(context.Background(), hbiHost))

	return schemaRepository
}

// addCommand builds an AddResourceRelationshipsCommand for the
// workspace/features combination.
func addCommand(t *testing.T, localResourceId string, reporterData, commonData map[string]interface{}) AddResourceRelationshipsCommand {
	t.Helper()
	localId, err := model.NewLocalResourceId(localResourceId)
	require.NoError(t, err)
	resType, err := model.NewResourceType("workspace")
	require.NoError(t, err)
	repType, err := model.NewReporterType("features")
	require.NoError(t, err)
	instanceId, err := model.NewReporterInstanceId("features-instance-1")
	require.NoError(t, err)

	cmd := AddResourceRelationshipsCommand{
		LocalResourceId:    localId,
		ResourceType:       resType,
		ReporterType:       repType,
		ReporterInstanceId: instanceId,
	}
	if reporterData != nil {
		rep, err := model.NewRepresentation(reporterData)
		require.NoError(t, err)
		cmd.ReporterRepresentation = &rep
	}
	if commonData != nil {
		rep, err := model.NewRepresentation(commonData)
		require.NoError(t, err)
		cmd.CommonRepresentation = &rep
	}
	return cmd
}

// reportCommand builds a ReportResourceCommand used to seed state.
func reportCommand(t *testing.T, localResourceId string, reporterData, commonData map[string]interface{}) resources.ReportResourceCommand {
	t.Helper()
	localId, err := model.NewLocalResourceId(localResourceId)
	require.NoError(t, err)
	resType, err := model.NewResourceType("workspace")
	require.NoError(t, err)
	repType, err := model.NewReporterType("features")
	require.NoError(t, err)
	instanceId, err := model.NewReporterInstanceId("features-instance-1")
	require.NoError(t, err)
	apiHref, err := model.NewApiHref("https://api.example.com/resource/123")
	require.NoError(t, err)

	cmd := resources.ReportResourceCommand{
		LocalResourceId:    localId,
		ResourceType:       resType,
		ReporterType:       repType,
		ReporterInstanceId: instanceId,
		ApiHref:            apiHref,
	}
	if reporterData != nil {
		rep, err := model.NewRepresentation(reporterData)
		require.NoError(t, err)
		cmd.ReporterRepresentation = &rep
	}
	if commonData != nil {
		rep, err := model.NewRepresentation(commonData)
		require.NoError(t, err)
		cmd.CommonRepresentation = &rep
	}
	return cmd
}

func reporterPreferences(t *testing.T, h *testHarness, localResourceId string) []interface{} {
	t.Helper()
	key := createReporterResourceKey(t, localResourceId, "workspace", "features", "features-instance-1")
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	require.NotNil(t, reps)
	raw, ok := reps.ReporterData()["direct_service_preferences"]
	if !ok || raw == nil {
		return nil
	}
	values, ok := raw.([]interface{})
	require.True(t, ok, "direct_service_preferences should be an array")
	return values
}

// --- Tests ---

func TestAddResourceRelationships_AutoCreatesResource(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	cmd := addCommand(t, "new-workspace", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)

	require.NoError(t, h.usecase.AddResourceRelationships(h.ctx, cmd))

	key := createReporterResourceKey(t, "new-workspace", "workspace", "features", "features-instance-1")

	// The resource now exists
	res, err := h.resourceRepo.FindResourceByKeys(nil, key)
	require.NoError(t, err)
	require.NotNil(t, res)

	// The initial representation contains the provided relationship
	assert.Equal(t, []interface{}{"RHEL"}, reporterPreferences(t, h, "new-workspace"))
}

func TestAddResourceRelationships_MergesIntoExisting(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL", "Insights"},
	}, map[string]interface{}{
		"workspace_id": "ws-1",
	})
	require.NoError(t, h.reportResource(t, seed))

	cmd := addCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"OpenShift"},
	}, nil)
	require.NoError(t, h.usecase.AddResourceRelationships(h.ctx, cmd))

	// Existing relationships are preserved and the new value appended
	assert.ElementsMatch(t, []interface{}{"RHEL", "Insights", "OpenShift"}, reporterPreferences(t, h, "root"))

	// The common representation is untouched by a reporter-scoped add
	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	assert.Equal(t, "ws-1", reps.CommonData()["workspace_id"])
}

func TestAddResourceRelationships_MergesMultipleValues(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	cmd := addCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"Insights", "Ansible"},
	}, nil)
	require.NoError(t, h.usecase.AddResourceRelationships(h.ctx, cmd))

	assert.ElementsMatch(t, []interface{}{"RHEL", "Insights", "Ansible"}, reporterPreferences(t, h, "root"))
}

func TestAddResourceRelationships_ExistingValueIsIdempotent(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL", "Insights"},
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	cmd := addCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	require.NoError(t, h.usecase.AddResourceRelationships(h.ctx, cmd))

	assert.ElementsMatch(t, []interface{}{"RHEL", "Insights"}, reporterPreferences(t, h, "root"))
}

func TestAddResourceRelationships_SingleValuedFieldSetWhenAbsent(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	cmd := addCommand(t, "root", map[string]interface{}{
		"direct_billing_account": "acct-1",
	}, nil)
	require.NoError(t, h.usecase.AddResourceRelationships(h.ctx, cmd))

	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	assert.Equal(t, "acct-1", reps.ReporterData()["direct_billing_account"])
}

func TestAddResourceRelationships_SingleValuedFieldConflictRejected(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_billing_account": "acct-1",
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	cmd := addCommand(t, "root", map[string]interface{}{
		"direct_billing_account": "acct-2",
	}, nil)
	err := h.usecase.AddResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	// The stored value is unchanged
	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	assert.Equal(t, "acct-1", reps.ReporterData()["direct_billing_account"])
}

func TestAddResourceRelationships_CommonScopedMerge(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	// Seed a reporter-only representation (no common data yet)
	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	cmd := addCommand(t, "root", nil, map[string]interface{}{
		"workspace_id": "ws-1",
	})
	require.NoError(t, h.usecase.AddResourceRelationships(h.ctx, cmd))

	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	assert.Equal(t, "ws-1", reps.CommonData()["workspace_id"])
	// Reporter relationships unchanged by a common-scoped add
	assert.ElementsMatch(t, []interface{}{"RHEL"}, reporterPreferences(t, h, "root"))
}

func TestAddResourceRelationships_TombstonedResourceReturnsNotFound(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	// Tombstone the resource via the DeleteResource use case
	deleteUsecase := resources.New(
		h.resourceRepo,
		h.schemaRepo,
		&data.AllowAllRelationsRepository{},
		"rbac",
		log.DefaultLogger,
		nil,
		nil,
		resources.NewUsecaseConfig(),
		h.metrics,
		h.meta,
		nil,
	)
	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	require.NoError(t, deleteUsecase.Delete(h.ctx, key))

	cmd := addCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"OpenShift"},
	}, nil)
	err := h.usecase.AddResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrResourceNotFound)
}

func TestAddResourceRelationships_AutoCreateRejectedWhenRequiredFieldsMissing(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	// host/hbi requires workspace_id in the common representation; the add
	// only provides reporter fields, so the auto-create must be rejected.
	localId, err := model.NewLocalResourceId("host-1")
	require.NoError(t, err)
	resType, err := model.NewResourceType("host")
	require.NoError(t, err)
	repType, err := model.NewReporterType("hbi")
	require.NoError(t, err)
	instanceId, err := model.NewReporterInstanceId("hbi-instance-1")
	require.NoError(t, err)
	reporterRep, err := model.NewRepresentation(map[string]interface{}{
		"some_relationship": []interface{}{"value-1"},
	})
	require.NoError(t, err)

	cmd := AddResourceRelationshipsCommand{
		LocalResourceId:        localId,
		ResourceType:           resType,
		ReporterType:           repType,
		ReporterInstanceId:     instanceId,
		ReporterRepresentation: &reporterRep,
	}

	err = h.usecase.AddResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	// The resource was not created
	key := createReporterResourceKey(t, "host-1", "host", "hbi", "hbi-instance-1")
	_, err = h.resourceRepo.FindResourceByKeys(nil, key)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestAddResourceRelationships_TransactionIdAlreadyProcessedSkipsWrite(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	txid := model.NewTransactionId("tx-already-processed")
	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	seed.TransactionId = &txid
	require.NoError(t, h.reportResource(t, seed))

	cmd := addCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"OpenShift"},
	}, nil)
	cmd.TransactionId = &txid

	// The duplicate is skipped without error
	require.NoError(t, h.usecase.AddResourceRelationships(h.ctx, cmd))

	// The stored relationships are unchanged
	assert.ElementsMatch(t, []interface{}{"RHEL"}, reporterPreferences(t, h, "root"))
}

func TestAddResourceRelationships_BothRepresentationsAbsentRejected(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	cmd := addCommand(t, "root", nil, nil)
	err := h.usecase.AddResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestAddResourceRelationships_MissingAuthzContext(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	cmd := addCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)

	err := h.usecase.AddResourceRelationships(context.Background(), cmd)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestAddResourceRelationships_MetaAuthzDenied(t *testing.T) {
	h := newTestHarness(t, withMeta(false))

	cmd := addCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)

	err := h.usecase.AddResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestAddResourceRelationships_UsesReportResourceRelation(t *testing.T) {
	h := newTestHarness(t, withMeta(true))

	cmd := addCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	require.NoError(t, h.usecase.AddResourceRelationships(h.ctx, cmd))

	assert.Equal(t, 1, h.meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationReportResource}, h.meta.relations)
}
