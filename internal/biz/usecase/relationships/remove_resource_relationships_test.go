package relationships

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
)

// newRemoveTestHarness creates a test harness with a fake resource repository,
// in-memory schema repository, and configurable meta-authorizer for exercising
// the RemoveResourceRelationships use case in isolation.
func newRemoveTestHarness(t *testing.T, opts ...removeHarnessOption) *removeTestHarness {
	t.Helper()
	cfg := &removeHarnessConfig{}
	for _, o := range opts {
		o(cfg)
	}

	resourceRepo := data.NewFakeResourceRepository()
	schemaRepo := newFakeSchemaRepository(t)
	if cfg.schemaRepo != nil {
		schemaRepo = cfg.schemaRepo
	}
	mc := metricscollector.NewFakeMetricsCollector()
	var metaAuth metaauthorizer.MetaAuthorizer
	if cfg.meta != nil {
		metaAuth = cfg.meta
	}

	uc := NewRemoveResourceRelationshipsUseCase(
		resourceRepo,
		schemaRepo,
		nil, // listen manager
		nil, // circuit breaker
		resources.NewUsecaseConfig(),
		mc,
		metaAuth,
		log.DefaultLogger,
	)

	return &removeTestHarness{
		usecase:      uc,
		resourceRepo: resourceRepo,
		schemaRepo:   schemaRepo,
		metrics:      mc,
		meta:         cfg.meta,
		ctx:          testAuthzContext(),
	}
}

type removeTestHarness struct {
	usecase      *RemoveResourceRelationshipsUseCase
	resourceRepo model.ResourceRepository
	schemaRepo   model.SchemaRepository
	metrics      *metricscollector.MetricsCollector
	meta         *recordingMetaAuthorizer
	ctx          context.Context
}

type removeHarnessOption func(*removeHarnessConfig)

type removeHarnessConfig struct {
	meta       *recordingMetaAuthorizer
	schemaRepo model.SchemaRepository
}

// withRemoveMeta returns a harness option that sets the meta-authorizer decision.
func withRemoveMeta(allowed bool) removeHarnessOption {
	return func(c *removeHarnessConfig) {
		c.meta = &recordingMetaAuthorizer{allowed: allowed}
	}
}

// withRemoveSchemaRepository returns a harness option that overrides the
// default JSON-schema repository, e.g. to register a schema carrying
// relation metadata for attribute-field rejection tests.
func withRemoveSchemaRepository(repo model.SchemaRepository) removeHarnessOption {
	return func(c *removeHarnessConfig) {
		c.schemaRepo = repo
	}
}

// resetMeta clears recorded meta-authorizer state (useful after setup calls).
func (h *removeTestHarness) resetMeta() {
	if h.meta != nil {
		h.meta.calls = 0
		h.meta.relations = nil
	}
}

// reportResource seeds an existing resource through the ReportResource use
// case so that removal behavior is exercised against realistically stored state.
func (h *removeTestHarness) reportResource(t *testing.T, cmd resources.ReportResourceCommand) error {
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

// removeCommand builds a RemoveResourceRelationshipsCommand for the
// workspace/features combination.
func removeCommand(t *testing.T, localResourceId string, reporterData, commonData map[string]interface{}) RemoveResourceRelationshipsCommand {
	t.Helper()
	localId, err := model.NewLocalResourceId(localResourceId)
	require.NoError(t, err)
	resType, err := model.NewResourceType("workspace")
	require.NoError(t, err)
	repType, err := model.NewReporterType("features")
	require.NoError(t, err)
	instanceId, err := model.NewReporterInstanceId("features-instance-1")
	require.NoError(t, err)

	cmd := RemoveResourceRelationshipsCommand{
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

// reporterPreferencesAt reads current direct_service_preferences values for assertions.
func reporterPreferencesAt(t *testing.T, h *removeTestHarness, localResourceId string) []interface{} {
	t.Helper()
	key := createReporterResourceKey(t, localResourceId, "workspace", "features", "features-instance-1")
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	if err != nil {
		return nil
	}
	raw, ok := reps.ReporterData()["direct_service_preferences"]
	if !ok || raw == nil {
		return nil
	}
	values, ok := raw.([]interface{})
	require.True(t, ok, "direct_service_preferences should be an array")
	return values
}

// --- Tests ---

// TestRemoveResourceRelationships_RemovesFromExisting validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_RemovesFromExisting(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL", "Insights", "Ansible"},
	}, map[string]interface{}{
		"workspace_id": "ws-1",
	})
	require.NoError(t, h.reportResource(t, seed))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"Insights"},
	}, nil)
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	assert.ElementsMatch(t, []interface{}{"RHEL", "Ansible"}, reporterPreferencesAt(t, h, "root"))

	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	assert.Equal(t, "ws-1", reps.CommonData()["workspace_id"])
}

// TestRemoveResourceRelationships_RemovesMultipleValues validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_RemovesMultipleValues(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL", "Insights", "Ansible", "OpenShift"},
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"Ansible", "OpenShift"},
	}, nil)
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	assert.ElementsMatch(t, []interface{}{"RHEL", "Insights"}, reporterPreferencesAt(t, h, "root"))
}

// TestRemoveResourceRelationships_RemovesAllValues validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_RemovesAllValues(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL", "Insights"},
	}, map[string]interface{}{
		"workspace_id": "ws-1",
	})
	require.NoError(t, h.reportResource(t, seed))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL", "Insights"},
	}, nil)
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	assert.Nil(t, reporterPreferencesAt(t, h, "root"))

	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	found, err := h.resourceRepo.FindResourceByKeys(nil, key)
	require.NoError(t, err)
	require.NotNil(t, found)

	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	assert.Equal(t, "ws-1", reps.CommonData()["workspace_id"])
}

// TestRemoveResourceRelationships_NonExistentValueIsIdempotent validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_NonExistentValueIsIdempotent(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL", "Insights"},
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	beforeCount := metricscollector.GetOutboxEventWriteCount()

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"OpenShift"},
	}, nil)
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	assert.ElementsMatch(t, []interface{}{"RHEL", "Insights"}, reporterPreferencesAt(t, h, "root"))
	assert.Equal(t, beforeCount, metricscollector.GetOutboxEventWriteCount(), "no-op removal should not emit an outbox event")
}

// TestRemoveResourceRelationships_NoExistingRelationshipsOfType validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_NoExistingRelationshipsOfType(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_billing_account": "acct-1",
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	assert.Nil(t, reporterPreferencesAt(t, h, "root"))
}

// TestRemoveResourceRelationships_ResourceNotFound validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_ResourceNotFound(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	err := h.usecase.RemoveResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrResourceNotFound)
}

// TestRemoveResourceRelationships_TombstonedResourceReturnsNotFound validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_TombstonedResourceReturnsNotFound(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

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

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	err := h.usecase.RemoveResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrResourceNotFound)
}

// TestRemoveResourceRelationships_SingleValuedFieldRemoved validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_SingleValuedFieldRemoved(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_billing_account": "acct-1",
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_billing_account": "acct-1",
	}, nil)
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	found, err := h.resourceRepo.FindResourceByKeys(nil, key)
	require.NoError(t, err)
	require.NotNil(t, found)
}

// TestRemoveResourceRelationships_SingleValuedNonMatchingValueNoOp validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_SingleValuedNonMatchingValueNoOp(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_billing_account": "acct-1",
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	beforeCount := metricscollector.GetOutboxEventWriteCount()

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_billing_account": "acct-2",
	}, nil)
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	assert.Equal(t, "acct-1", reps.ReporterData()["direct_billing_account"])
	assert.Equal(t, beforeCount, metricscollector.GetOutboxEventWriteCount(), "no-op removal should not emit an outbox event")
}

// TestRemoveResourceRelationships_CommonScopedRemoval validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_CommonScopedRemoval(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, map[string]interface{}{
		"workspace_id": "ws-1",
	})
	require.NoError(t, h.reportResource(t, seed))

	cmd := removeCommand(t, "root", nil, map[string]interface{}{
		"workspace_id": "ws-1",
	})
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	assert.NotContains(t, reps.CommonData(), "workspace_id")
	assert.ElementsMatch(t, []interface{}{"RHEL"}, reporterPreferencesAt(t, h, "root"))
}

// TestRemoveResourceRelationships_TransactionIdAlreadyProcessedSkipsWrite validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_TransactionIdAlreadyProcessedSkipsWrite(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	txid := model.NewTransactionId("tx-already-processed-remove")
	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL", "Insights"},
	}, nil)
	seed.TransactionId = &txid
	require.NoError(t, h.reportResource(t, seed))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	cmd.TransactionId = &txid

	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	assert.ElementsMatch(t, []interface{}{"RHEL", "Insights"}, reporterPreferencesAt(t, h, "root"))
}

// TestRemoveResourceRelationships_BothRepresentationsAbsentRejected validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_BothRepresentationsAbsentRejected(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	cmd := removeCommand(t, "root", nil, nil)
	err := h.usecase.RemoveResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestRemoveResourceRelationships_MissingAuthzContext validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_MissingAuthzContext(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)

	err := h.usecase.RemoveResourceRelationships(context.Background(), cmd)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// TestRemoveResourceRelationships_MetaAuthzDenied validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_MetaAuthzDenied(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(false))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)

	err := h.usecase.RemoveResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

// TestRemoveResourceRelationships_UsesReportResourceRelation validates the corresponding user-story scenario.
func TestRemoveResourceRelationships_UsesReportResourceRelation(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	require.NoError(t, h.reportResource(t, seed))
	h.resetMeta()

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	assert.Equal(t, 1, h.meta.calls)
	assert.Equal(t, []metaauthorizer.Relation{metaauthorizer.RelationReportResource}, h.meta.relations)
}

// TestRemoveResourceRelationships_RejectsAttributeFields validates that
// non-relationship fields in the request are rejected with
// INVALID_ARGUMENT when the schema carries relation metadata, before any
// resource lookup occurs.
func TestRemoveResourceRelationships_RejectsAttributeFields(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true), withRemoveSchemaRepository(newUnifiedSchemaRepository(t)))

	cmd := removeCommand(t, "ws-attr-1",
		map[string]interface{}{
			"direct_service_preferences": []interface{}{"service-a"},
			"display_name":               "not a relationship field",
		}, nil)

	err := h.usecase.RemoveResourceRelationships(h.ctx, cmd)

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), `reporter field "display_name" is not a relationship field`)
}

// TestRemoveResourceRelationships_AcceptsRelationFieldsUnderUnifiedSchema
// validates that removing relationship-typed fields from both representation
// streams succeeds when the schemas carry relation metadata.
func TestRemoveResourceRelationships_AcceptsRelationFieldsUnderUnifiedSchema(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true), withRemoveSchemaRepository(newUnifiedSchemaRepository(t)))

	seed := reportCommand(t, "ws-attr-2", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL", "Insights"},
	}, map[string]interface{}{
		"workspace_id": "ws-1",
	})
	require.NoError(t, h.reportResource(t, seed))

	cmd := removeCommand(t, "ws-attr-2",
		map[string]interface{}{"direct_service_preferences": []interface{}{"Insights"}},
		map[string]interface{}{"workspace_id": "ws-1"})

	err := h.usecase.RemoveResourceRelationships(h.ctx, cmd)

	require.NoError(t, err)
	assert.ElementsMatch(t, []interface{}{"RHEL"}, reporterPreferencesAt(t, h, "ws-attr-2"))
}

// TestRemoveResourceRelationships_RejectsWrongStreamFields validates that
// relation fields supplied in the wrong representation stream are rejected
// with INVALID_ARGUMENT before any resource lookup occurs.
func TestRemoveResourceRelationships_RejectsWrongStreamFields(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true), withRemoveSchemaRepository(newUnifiedSchemaRepository(t)))

	reporterStreamInCommon := removeCommand(t, "ws-stream-1", nil,
		map[string]interface{}{"direct_service_preferences": []interface{}{"service-a"}})
	err := h.usecase.RemoveResourceRelationships(h.ctx, reporterStreamInCommon)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), `common field "direct_service_preferences" is a reporter relationship field, not a common one`)
}
