package usecase

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

// --- Test harness ---

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
	meta *recordingMetaAuthorizer
}

func newRemoveTestHarness(t *testing.T, opts ...removeHarnessOption) *removeTestHarness {
	t.Helper()
	cfg := &removeHarnessConfig{}
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

	uc := NewRemoveResourceRelationshipsUseCase(
		resourceRepo,
		schemaRepo,
		nil, // listen manager: read-after-write not exercised in these tests
		nil, // circuit breaker: read-after-write not exercised in these tests
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

func withRemoveMeta(allowed bool) removeHarnessOption {
	return func(c *removeHarnessConfig) {
		c.meta = &recordingMetaAuthorizer{allowed: allowed}
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

// resetMeta clears recorded meta-authorizer state (useful after setup calls).
func (h *removeTestHarness) resetMeta() {
	if h.meta != nil {
		h.meta.calls = 0
		h.meta.relations = nil
	}
}

// --- Tests ---

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

	// Only the removed value is gone; the rest are unchanged.
	assert.ElementsMatch(t, []interface{}{"RHEL", "Ansible"}, reporterPreferencesAt(t, h, "root"))

	// The common representation is untouched by a reporter-scoped removal.
	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	assert.Equal(t, "ws-1", reps.CommonData()["workspace_id"])
}

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

	// The field is emptied but the resource itself is not removed.
	assert.Nil(t, reporterPreferencesAt(t, h, "root"))

	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	found, err := h.resourceRepo.FindResourceByKeys(nil, key)
	require.NoError(t, err)
	require.NotNil(t, found)

	// The common representation survives the reporter-scoped removal.
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	assert.Equal(t, "ws-1", reps.CommonData()["workspace_id"])
}

func TestRemoveResourceRelationships_NonExistentValueIsIdempotent(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL", "Insights"},
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"OpenShift"},
	}, nil)
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	assert.ElementsMatch(t, []interface{}{"RHEL", "Insights"}, reporterPreferencesAt(t, h, "root"))
}

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

	// No direct_service_preferences existed; nothing changed.
	assert.Nil(t, reporterPreferencesAt(t, h, "root"))
}

func TestRemoveResourceRelationships_ResourceNotFound(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)
	err := h.usecase.RemoveResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.ErrorIs(t, err, model.ErrResourceNotFound)
}

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

	// The field is removed but the resource itself persists with an empty
	// reporter representation (both streams empty is not representable via
	// FindLatestRepresentations, so existence is checked directly).
	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	found, err := h.resourceRepo.FindResourceByKeys(nil, key)
	require.NoError(t, err)
	require.NotNil(t, found)
}

func TestRemoveResourceRelationships_SingleValuedNonMatchingValueNoOp(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	seed := reportCommand(t, "root", map[string]interface{}{
		"direct_billing_account": "acct-1",
	}, nil)
	require.NoError(t, h.reportResource(t, seed))

	// Removing a value that is not the stored one is an idempotent no-op;
	// changing a single-valued relationship requires removing the stored
	// value first, then adding the new one.
	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_billing_account": "acct-2",
	}, nil)
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	key := createReporterResourceKey(t, "root", "workspace", "features", "features-instance-1")
	reps, err := h.resourceRepo.FindLatestRepresentations(nil, key)
	require.NoError(t, err)
	assert.Equal(t, "acct-1", reps.ReporterData()["direct_billing_account"])
}

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
	// Reporter relationships unchanged by a common-scoped removal.
	assert.ElementsMatch(t, []interface{}{"RHEL"}, reporterPreferencesAt(t, h, "root"))
}

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

	// The duplicate is skipped without error.
	require.NoError(t, h.usecase.RemoveResourceRelationships(h.ctx, cmd))

	// The stored relationships are unchanged.
	assert.ElementsMatch(t, []interface{}{"RHEL", "Insights"}, reporterPreferencesAt(t, h, "root"))
}

func TestRemoveResourceRelationships_BothRepresentationsAbsentRejected(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	cmd := removeCommand(t, "root", nil, nil)
	err := h.usecase.RemoveResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestRemoveResourceRelationships_MissingAuthzContext(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(true))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)

	err := h.usecase.RemoveResourceRelationships(context.Background(), cmd)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestRemoveResourceRelationships_MetaAuthzDenied(t *testing.T) {
	h := newRemoveTestHarness(t, withRemoveMeta(false))

	cmd := removeCommand(t, "root", map[string]interface{}{
		"direct_service_preferences": []interface{}{"RHEL"},
	}, nil)

	err := h.usecase.RemoveResourceRelationships(h.ctx, cmd)
	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

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
