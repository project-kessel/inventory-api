package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// RemoveResourceRelationshipsOperationName identifies the serializable
// transaction used by the RemoveResourceRelationships use case.
const RemoveResourceRelationshipsOperationName = "RemoveResourceRelationships"

// RemoveResourceRelationshipsCommand contains the data needed to remove one or
// more relationships from a resource. This is the domain command used by the
// usecase layer, decoupled from protobuf types.
type RemoveResourceRelationshipsCommand struct {
	// Resource identification
	LocalResourceId    model.LocalResourceId
	ResourceType       model.ResourceType
	ReporterType       model.ReporterType
	ReporterInstanceId model.ReporterInstanceId

	// Idempotency key. When nil (or empty) a new transaction ID is generated.
	TransactionId *model.TransactionId

	// Partial representations naming the relationship values to remove.
	// At least one must be provided.
	ReporterRepresentation *model.Representation
	CommonRepresentation   *model.Representation

	// Write behavior
	WriteVisibility resources.WriteVisibility
}

// RemoveResourceRelationshipsUseCase removes relationship-typed fields from a
// resource's representations without asserting the complete remaining
// relationship state. Only the relationship values named in the request are
// removed; all other relationships are left exactly as they were. If the
// resource does not exist, or the reporter's representation is tombstoned,
// the request fails with ErrResourceNotFound — matching the behaviour of
// DeleteResource. Removing a value that is not present is an idempotent
// no-op.
//
// The use case shares infrastructure with the resources use cases (resource
// repository, schema repository, transaction manager, listen manager) and the
// helper functions in add_resource_relationships.go, but has no shared code
// path with ReportResource.
type RemoveResourceRelationshipsUseCase struct {
	schemaService       *model.SchemaService
	resourceRepository  model.ResourceRepository
	waitForNotifBreaker *gobreaker.CircuitBreaker[any]
	MetaAuthorizer      metaauthorizer.MetaAuthorizer
	Log                 *log.Helper
	ListenManager       pubsub.ListenManagerImpl
	Config              *resources.UsecaseConfig
	MetricsCollector    *metricscollector.MetricsCollector
}

// NewRemoveResourceRelationshipsUseCase creates a use case for removing
// relationship-typed fields from resource representations.
func NewRemoveResourceRelationshipsUseCase(
	resourceRepository model.ResourceRepository,
	schemaRepository model.SchemaRepository,
	listenManager pubsub.ListenManagerImpl,
	waitForNotifBreaker *gobreaker.CircuitBreaker[any],
	usecaseConfig *resources.UsecaseConfig,
	metricsCollector *metricscollector.MetricsCollector,
	metaAuthorizer metaauthorizer.MetaAuthorizer,
	logger log.Logger,
) *RemoveResourceRelationshipsUseCase {
	if metaAuthorizer == nil {
		metaAuthorizer = metaauthorizer.NewSimpleMetaAuthorizer()
	}
	if usecaseConfig == nil {
		usecaseConfig = resources.NewUsecaseConfig()
	}

	return &RemoveResourceRelationshipsUseCase{
		resourceRepository:  resourceRepository,
		schemaService:       model.NewSchemaService(schemaRepository, log.NewHelper(logger)),
		waitForNotifBreaker: waitForNotifBreaker,
		MetaAuthorizer:      metaAuthorizer,
		Log:                 log.NewHelper(logger),
		ListenManager:       listenManager,
		Config:              usecaseConfig,
		MetricsCollector:    metricsCollector,
	}
}

// RemoveResourceRelationships removes the relationship values named in the
// command from the resource identified by its key. The resource must exist
// and not be tombstoned; otherwise the request fails with
// ErrResourceNotFound. The full resulting representation is validated against
// the configured schemas before it is persisted (e.g. removing a required
// field is rejected). Removing values that are not present is an idempotent
// no-op that emits no outbox event.
func (uc *RemoveResourceRelationshipsUseCase) RemoveResourceRelationships(ctx context.Context, cmd RemoveResourceRelationshipsCommand) error {
	// Get authz context - required for authorization checks
	authzCtx, ok := authnapi.FromAuthzContext(ctx)
	if !ok || authzCtx.Subject == nil {
		return status.Error(codes.Unauthenticated, "authentication required")
	}

	reporterResourceKey, err := model.NewReporterResourceKey(
		cmd.LocalResourceId,
		cmd.ResourceType,
		cmd.ReporterType,
		cmd.ReporterInstanceId,
	)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to create reporter resource key: %v", err)
	}

	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationReportResource, metaauthorizer.NewInventoryResourceFromKey(reporterResourceKey)); err != nil {
		return err
	}

	if representationIsEmpty(cmd.ReporterRepresentation) && representationIsEmpty(cmd.CommonRepresentation) {
		return status.Error(codes.InvalidArgument, "at least one of reporter or common relationship fields must be provided")
	}

	var subscription pubsub.Subscription

	txid, err := getNextTransactionID()
	if err != nil {
		return err
	}

	if cmd.TransactionId == nil || *cmd.TransactionId == "" {
		cmd.TransactionId = &txid
	}

	readAfterWriteEnabled := uc.computeReadAfterWrite(cmd.WriteVisibility, authzCtx.Subject.SubjectId)
	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		subscription = uc.ListenManager.Subscribe(txid.String())
		defer subscription.Unsubscribe()
	}

	// Check for duplicate transaction ID outside the serializable transaction to avoid
	// predicate locks on the transaction_id indexes. The unique constraint on transaction_id
	// provides the actual correctness guarantee, if a duplicate sneaks past this
	// advisory check due to a concurrent commit.
	if uc.Config.IdempotencyCheckEnabled && cmd.TransactionId != nil {
		alreadyProcessed, err := uc.resourceRepository.HasTransactionIdBeenProcessed(uc.resourceRepository.GetDB(), *cmd.TransactionId)
		if err != nil {
			return fmt.Errorf("failed to check transaction ID: %w", err)
		}
		if alreadyProcessed {
			uc.Log.Infof("Transaction already processed, skipping update: transaction_id=%s", cmd.TransactionId.String())
			return nil
		}
	}

	var operationType model.EventOperationType
	removeRelationships := func() error {
		return uc.resourceRepository.GetTransactionManager().HandleSerializableTransaction(
			RemoveResourceRelationshipsOperationName,
			uc.resourceRepository.GetDB(),
			func(tx *gorm.DB) error {
				res, err := uc.resourceRepository.FindResourceByKeys(tx, reporterResourceKey)
				if err != nil {
					// A missing resource matches the behaviour of DeleteResource.
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return ErrResourceNotFound
					}
					return fmt.Errorf("failed to lookup existing resource: %w", err)
				}
				if res == nil {
					return ErrResourceNotFound
				}

				operationType = model.OperationTypeUpdated
				return uc.removeRelationshipsFromExistingResource(ctx, tx, cmd, res, reporterResourceKey, txid)
			},
		)
	}

	err = removeRelationships()
	if err != nil && !uc.Config.IdempotencyCheckEnabled && isDuplicateTransactionError(err) {
		uc.Log.Debugf("Idempotency check disabled and duplicate transaction ID detected, retrying with new transaction ID: %s", cmd.TransactionId.String())
		retryTxid, retryErr := getNextTransactionID()
		if retryErr != nil {
			return retryErr
		}
		cmd.TransactionId = &retryTxid
		err = removeRelationships()
	}

	if err != nil {
		// Extract principal for failure logging
		principal := authzCtx.ExtractPrincipal()

		// CRUD operation failed - SEC-MON-REQ-1 compliance (EOI-1 pii_manipulation, EOI-11 warnings_or_errors)
		uc.Log.Warnw("msg", "Resource operation failed",
			"action", "REMOVE_RESOURCE_RELATIONSHIPS",
			"resource_type", cmd.ResourceType.String(),
			"resource_id", cmd.LocalResourceId,
			"reporter_type", cmd.ReporterType.String(),
			"reporter_instance_id", cmd.ReporterInstanceId.String(),
			"principal", principal,
			"outcome", "failure",
			"reason", err.Error(),
		)

		return err
	}

	// Extract principal for success logging
	principal := authzCtx.ExtractPrincipal()

	// CRUD operation - SEC-MON-REQ-1 compliance (EOI-1 pii_manipulation)
	uc.Log.Infow("msg", "Resource operation completed",
		"action", "UPDATE",
		"resource_type", cmd.ResourceType.String(),
		"resource_id", cmd.LocalResourceId,
		"reporter_type", cmd.ReporterType.String(),
		"reporter_instance_id", cmd.ReporterInstanceId.String(),
		"principal", principal,
		"outcome", "success",
	)

	// Increment outbox metrics only after successful transaction commit
	if operationType != nil {
		metricscollector.Incr(uc.MetricsCollector.OutboxEventWrites, string(operationType.OperationType()))
	}

	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		timeoutCtx, cancel := context.WithTimeout(ctx, listenTimeout)
		defer cancel()

		_, err := uc.waitForNotifBreaker.Execute(func() (any, error) {
			err = subscription.BlockForNotification(timeoutCtx)
			if err != nil {
				// Return error for circuit breaker
				return nil, err
			}
			return nil, nil
		})

		if err != nil {
			switch {
			case errors.Is(err, pubsub.ErrWaitContextCancelled):
				uc.Log.WithContext(ctx).Debugf("Reached timeout waiting for notification from consumer")
				return nil
			case errors.Is(err, gobreaker.ErrOpenState):
				uc.Log.WithContext(ctx).Debugf("Circuit breaker is open, skipped waiting for notification from consumer")
				return nil
			case errors.Is(err, gobreaker.ErrTooManyRequests):
				uc.Log.WithContext(ctx).Debugf("Circuit breaker is half-open, skipped waiting for notification from consumer")
				return nil
			default:
				return err
			}
		}
	}

	return nil
}

// removeRelationshipsFromExistingResource subtracts the removal values from
// the stored representations of an existing (non-tombstoned) resource and
// persists the new representation version. When the subtraction changes
// nothing (the named values are not present), no write is performed and no
// outbox event is emitted.
func (uc *RemoveResourceRelationshipsUseCase) removeRelationshipsFromExistingResource(ctx context.Context, tx *gorm.DB, cmd RemoveResourceRelationshipsCommand, res *model.Resource, key model.ReporterResourceKey, txid model.TransactionId) error {
	// Tombstoned representations cannot be modified through the delta
	// endpoints; a new ReportResource call is required to re-create the
	// representation.
	reporterResource, err := findReporterResource(res.ReporterResources(), key)
	if err != nil {
		return err
	}
	if reporterResource.Tombstoned() {
		return ErrResourceNotFound
	}

	// Load the current stored representations from both streams so the
	// subtraction starts from the persisted state.
	current, err := uc.resourceRepository.FindLatestRepresentations(tx, key)
	if err != nil {
		return fmt.Errorf("failed to load current representations: %w", err)
	}

	var storedReporter, storedCommon model.Representation
	if current != nil {
		storedReporter = current.ReporterData()
		storedCommon = current.CommonData()
	}

	removedReporter, reporterChanged, err := removeRelationshipFields(storedReporter, cmd.ReporterRepresentation)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to remove reporter relationships: %v", err)
	}
	removedCommon, commonChanged, err := removeRelationshipFields(storedCommon, cmd.CommonRepresentation)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to remove common relationships: %v", err)
	}

	// Removing values that are not present is an idempotent no-op: no new
	// representation version is persisted and no outbox event is emitted.
	if !reporterChanged && !commonChanged {
		uc.Log.Infof("No relationships to remove, skipping update: resource_id=%s", cmd.LocalResourceId)
		return nil
	}

	// The full resulting representation must satisfy the schema. Streams the
	// request did not touch pass through unchanged; a stream whose values
	// were all removed is preserved as an empty representation so the
	// resource itself continues to exist.
	var removedReporterPtr, removedCommonPtr *model.Representation
	if reporterChanged || len(storedReporter) > 0 {
		removedReporterPtr = &removedReporter
	}
	if commonChanged || len(storedCommon) > 0 {
		removedCommonPtr = &removedCommon
	}
	if removedReporterPtr == nil && removedCommonPtr == nil {
		return status.Error(codes.InvalidArgument, "at least one of reporter or common relationship fields must be provided")
	}
	if err := uc.schemaService.ValidateReportAgainstSchema(ctx, cmd.ResourceType, cmd.ReporterType, removedCommonPtr, removedReporterPtr); err != nil {
		return status.Errorf(codes.InvalidArgument, "failed validation for remove resource relationships: %v", err)
	}

	// The RemoveResourceRelationships contract carries no metadata, so the
	// stored hrefs are preserved on update. The stored reporter version is
	// not carried over: it is per-report metadata and, like ReportResource,
	// each write records the version supplied with that write (none here).
	if err := res.Update(
		key,
		reporterResource.ApiHref(),
		reporterResource.ConsoleHref(),
		nil,
		removedReporterPtr,
		removedCommonPtr,
		*cmd.TransactionId,
	); err != nil {
		return fmt.Errorf("failed to update resource: %w", err)
	}

	return uc.resourceRepository.Save(tx, *res, model.OperationTypeUpdated, txid)
}

// removeRelationshipFields subtracts the removal values from the stored
// representation. Multi-valued (array) fields have the listed values removed
// from the stored set (a field emptied entirely is deleted from the
// representation); single-valued (scalar) fields are removed only when the
// stored value matches the requested value — removing a value that is not
// present is an idempotent no-op. Fields not named in the removal request are
// returned unchanged. The second return value reports whether anything was
// actually removed.
func removeRelationshipFields(stored model.Representation, removals *model.Representation) (model.Representation, bool, error) {
	result := model.Representation{}
	for field, value := range stored {
		result[field] = value
	}
	if removals == nil {
		return result, false, nil
	}

	changed := false
	for field, value := range *removals {
		if value == nil {
			return nil, false, fmt.Errorf("relationship field %q has a null value", field)
		}

		storedValue, exists := result[field]
		if !exists || storedValue == nil {
			// The value is not present: idempotent no-op.
			continue
		}

		switch removalValues := value.(type) {
		case []interface{}:
			storedValues, ok := storedValue.([]interface{})
			if !ok {
				return nil, false, fmt.Errorf("relationship field %q is single-valued and cannot remove an array", field)
			}
			remaining := subtractValues(storedValues, removalValues)
			if len(remaining) == 0 {
				// All values removed: the field itself is removed.
				delete(result, field)
			} else {
				result[field] = remaining
			}
			changed = true
		default:
			// Single-valued: remove only when the stored value matches.
			if valuesEqual(storedValue, value) {
				delete(result, field)
				changed = true
			}
			// A different stored value is untouched: idempotent no-op.
		}
	}

	return result, changed, nil
}

// subtractValues returns the stored values that are not listed for removal.
func subtractValues(stored, removals []interface{}) []interface{} {
	toRemove := make(map[string]struct{}, len(removals))
	for _, value := range removals {
		toRemove[valueKey(value)] = struct{}{}
	}

	remaining := make([]interface{}, 0, len(stored))
	for _, value := range stored {
		if _, remove := toRemove[valueKey(value)]; !remove {
			remaining = append(remaining, value)
		}
	}
	return remaining
}

// enforceMetaAuthzObject calls the MetaAuthorizer to validate access using a MetaObject.
func (uc *RemoveResourceRelationshipsUseCase) enforceMetaAuthzObject(ctx context.Context, relation metaauthorizer.Relation, metaObject metaauthorizer.MetaObject) error {
	return metaauthorizer.EnforceMetaAuthzObject(ctx, uc.MetaAuthorizer, relation, metaObject)
}

// computeReadAfterWrite determines whether the caller requested — and the
// deployment supports — waiting for the write to become visible to the
// consumer before returning.
func (uc *RemoveResourceRelationshipsUseCase) computeReadAfterWrite(writeVisibility resources.WriteVisibility, callerSubject authnapi.SubjectId) bool {
	if writeVisibility == resources.WriteVisibilityUnspecified || writeVisibility == resources.WriteVisibilityMinimizeLatency {
		return false
	}
	return !common.IsNil(uc.ListenManager) && uc.Config.ReadAfterWriteEnabled && isSPInAllowlist(callerSubject, uc.Config.ReadAfterWriteAllowlist)
}
