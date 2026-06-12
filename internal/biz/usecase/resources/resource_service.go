package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

	kratosErrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/cmd/common"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	DeleteResourceOperationName = "DeleteResource"
	ReportResourceOperationName = "ReportResource"
)

// Domain errors re-exported from model package.
var (
	ErrResourceNotFound      = model.ErrResourceNotFound
	ErrDatabaseError         = model.ErrDatabaseError
	ErrResourceAlreadyExists = model.ErrResourceAlreadyExists
	ErrInventoryIdMismatch   = model.ErrInventoryIdMismatch
)

// Application-layer errors for self-subject resolution.
var (
	// ErrSelfSubjectMissing indicates the subject could not be derived for self checks.
	ErrSelfSubjectMissing = errors.New("self subject missing")
)

const listenTimeout = 10 * time.Second

// UsecaseConfig contains configuration flags that control the behavior of usecase operations.
// These flags should be consistent across all handlers.
type UsecaseConfig struct {
	ReadAfterWriteEnabled          bool
	ReadAfterWriteAllowlist        []string
	ConsumerEnabled                bool
	DefaultToAtLeastAsAcknowledged bool
	IdempotencyCheckEnabled        bool
}

func NewUsecaseConfig() *UsecaseConfig {
	return &UsecaseConfig{
		IdempotencyCheckEnabled: true,
	}
}

// Usecase provides business logic operations for resource management in the inventory system.
// It coordinates between repositories, authorization, and other system components.
type Usecase struct {
	schemaService       *model.SchemaService
	resourceRepository  model.ResourceRepository
	waitForNotifBreaker *gobreaker.CircuitBreaker[any]
	Relations           model.RelationsRepository
	MetaAuthorizer      metaauthorizer.MetaAuthorizer
	Namespace           string
	Log                 *log.Helper
	ListenManager       pubsub.ListenManagerImpl
	Config              *UsecaseConfig
	MetricsCollector    *metricscollector.MetricsCollector
	SelfSubjectStrategy SelfSubjectStrategy
}

func New(resourceRepository model.ResourceRepository, schemaRepository model.SchemaRepository,
	relations model.RelationsRepository, namespace string, logger log.Logger,
	listenManager pubsub.ListenManagerImpl, waitForNotifBreaker *gobreaker.CircuitBreaker[any], usecaseConfig *UsecaseConfig, metricsCollector *metricscollector.MetricsCollector, metaAuthorizer metaauthorizer.MetaAuthorizer, selfSubjectStrategy SelfSubjectStrategy) *Usecase {
	if metaAuthorizer == nil {
		metaAuthorizer = metaauthorizer.NewSimpleMetaAuthorizer()
	}

	return &Usecase{
		resourceRepository:  resourceRepository,
		schemaService:       model.NewSchemaService(schemaRepository, log.NewHelper(logger)),
		waitForNotifBreaker: waitForNotifBreaker,
		Relations:           relations,
		MetaAuthorizer:      metaAuthorizer,
		Namespace:           namespace,
		Log:                 log.NewHelper(logger),
		ListenManager:       listenManager,
		Config:              usecaseConfig,
		MetricsCollector:    metricsCollector,
		SelfSubjectStrategy: selfSubjectStrategy,
	}
}

func (uc *Usecase) ReportResource(ctx context.Context, cmd ReportResourceCommand) error {
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
		log.Error("failed to create reporter resource key: ", err)
		return status.Errorf(codes.InvalidArgument, "failed to create reporter resource key: %v", err)
	}

	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationReportResource, metaauthorizer.NewInventoryResourceFromKey(reporterResourceKey)); err != nil {
		return err
	}

	var subscription pubsub.Subscription

	txid, err := getNextTransactionID()
	if err != nil {
		return err
	}

	if cmd.TransactionId == nil || *cmd.TransactionId == "" {
		cmd.TransactionId = &txid
	}

	if err := uc.schemaService.ValidateReportAgainstSchema(ctx, cmd.ResourceType, cmd.ReporterType, cmd.CommonRepresentation, cmd.ReporterRepresentation); err != nil {
		return status.Errorf(codes.InvalidArgument, "failed validation for report resource: %v", err)
	}

	readAfterWriteEnabled := computeReadAfterWrite(uc, cmd.WriteVisibility, authzCtx.Subject.SubjectId)
	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		subscription = uc.ListenManager.Subscribe(txid.String())
		defer subscription.Unsubscribe()
	}

	// Advisory duplicate-transaction-ID check. Runs in a short read-only
	// serializable transaction. The unique constraint on transaction_id provides
	// the actual correctness guarantee; this check is an optimization to avoid
	// the heavier write transaction when possible.
	if uc.Config.IdempotencyCheckEnabled && cmd.TransactionId != nil {
		tx, beginErr := uc.resourceRepository.Begin("")
		if beginErr != nil {
			return fmt.Errorf("failed to begin idempotency check: %w", beginErr)
		}
		alreadyProcessed, checkErr := tx.HasTransactionIdBeenProcessed(*cmd.TransactionId)
		_ = tx.Rollback()
		if checkErr != nil {
			return fmt.Errorf("failed to check transaction ID: %w", checkErr)
		}
		if alreadyProcessed {
			log.Infof("Transaction already processed, skipping update: transaction_id=%s", cmd.TransactionId.String())
			return nil
		}
	}

	var operationType model.EventOperationType
	reportResource := func() error {
		return uc.reportResourceWithRetry(reporterResourceKey, cmd, txid, &operationType)
	}

	err = reportResource()
	if err != nil && !uc.Config.IdempotencyCheckEnabled && isDuplicateTransactionError(err) {
		log.Debugf("Idempotency check disabled and duplicate transaction ID detected, retrying with new transaction ID: %s", cmd.TransactionId.String())
		retryTxid, retryErr := getNextTransactionID()
		if retryErr != nil {
			return retryErr
		}
		cmd.TransactionId = &retryTxid
		err = reportResource()
	}

	if err != nil {
		// Extract principal for failure logging
		principal := authzCtx.ExtractPrincipal()

		// CRUD operation failed - SEC-MON-REQ-1 compliance (EOI-1 pii_manipulation, EOI-11 warnings_or_errors)
		uc.Log.Warnw("msg", "Resource operation failed",
			"action", "REPORT_RESOURCE",
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

	// Determine action based on operation type
	var action string
	switch operationType {
	case model.OperationTypeCreated:
		action = "CREATE"
	case model.OperationTypeUpdated:
		action = "UPDATE"
	default:
		action = "REPORT_RESOURCE"
	}

	// CRUD operation - SEC-MON-REQ-1 compliance (EOI-1 pii_manipulation)
	uc.Log.Infow("msg", "Resource operation completed",
		"action", action,
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

func (uc *Usecase) reportResourceWithRetry(reporterResourceKey model.ReporterResourceKey, cmd ReportResourceCommand, txid model.TransactionId, operationType *model.EventOperationType) error {
	maxRetries := uc.resourceRepository.MaxSerializationRetries()
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		tx, err := uc.resourceRepository.Begin(ReportResourceOperationName)
		if err != nil {
			return err
		}

		res, err := tx.FindResourceByKeys(reporterResourceKey)
		if err != nil && !errors.Is(err, model.ErrResourceNotFound) {
			_ = tx.Rollback()
			if errors.Is(err, model.ErrSerializationFailure) {
				lastErr = err
				continue
			}
			return fmt.Errorf("failed to lookup existing resource: %w", err)
		}

		if err == nil && res != nil {
			log.Info("Resource already exists, updating: ")
			*operationType = model.OperationTypeUpdated
			if err := uc.updateResource(tx, cmd, res, txid); err != nil {
				_ = tx.Rollback()
				if errors.Is(err, model.ErrSerializationFailure) {
					lastErr = err
					continue
				}
				return err
			}
		} else {
			log.Info("Creating new resource")
			*operationType = model.OperationTypeCreated
			if err := uc.createResource(tx, cmd, txid); err != nil {
				_ = tx.Rollback()
				if errors.Is(err, model.ErrSerializationFailure) {
					lastErr = err
					continue
				}
				return err
			}
		}

		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			if errors.Is(err, model.ErrSerializationFailure) {
				lastErr = err
				continue
			}
			return err
		}
		return nil
	}
	uc.resourceRepository.RecordSerializationExhaustion(ReportResourceOperationName)
	log.Errorf("transaction failed after %d attempts: %v", maxRetries, lastErr)
	return fmt.Errorf("transaction failed after %d attempts: %w", maxRetries, lastErr)
}

func (uc *Usecase) createResource(tx model.ResourceTx, cmd ReportResourceCommand, txid model.TransactionId) error {
	resourceId, err := tx.NextResourceId()
	if err != nil {
		return err
	}

	reporterResourceId, err := tx.NextReporterResourceId()
	if err != nil {
		return err
	}

	resource, err := model.NewResource(
		resourceId,
		cmd.LocalResourceId,
		cmd.ResourceType,
		cmd.ReporterType,
		cmd.ReporterInstanceId,
		*cmd.TransactionId,
		reporterResourceId,
		cmd.ApiHref,
		cmd.ConsoleHref,
		cmd.ReporterRepresentation,
		cmd.CommonRepresentation,
		cmd.ReporterVersion,
	)
	if err != nil {
		return err
	}

	return tx.Save(resource, model.OperationTypeCreated, txid)
}

func (uc *Usecase) updateResource(tx model.ResourceTx, cmd ReportResourceCommand, existingResource *model.Resource, txid model.TransactionId) error {
	reporterResourceKey, err := model.NewReporterResourceKey(
		cmd.LocalResourceId,
		cmd.ResourceType,
		cmd.ReporterType,
		cmd.ReporterInstanceId,
	)
	if err != nil {
		return err
	}

	err = existingResource.Update(
		reporterResourceKey,
		cmd.ApiHref,
		cmd.ConsoleHref,
		cmd.ReporterVersion,
		cmd.ReporterRepresentation,
		cmd.CommonRepresentation,
		*cmd.TransactionId,
	)
	if err != nil {
		return fmt.Errorf("failed to update resource: %w", err)
	}

	return tx.Save(*existingResource, model.OperationTypeUpdated, txid)
}

func (uc *Usecase) Delete(ctx context.Context, reporterResourceKey model.ReporterResourceKey) error {
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationDeleteResource, metaauthorizer.NewInventoryResourceFromKey(reporterResourceKey)); err != nil {
		return err
	}

	txid, err := getNextTransactionID()
	if err != nil {
		return err
	}

	// Get authz context for logging (guaranteed to exist after enforceMetaAuthzObject)
	authzCtx, _ := authnapi.FromAuthzContext(ctx)

	maxRetries := uc.resourceRepository.MaxSerializationRetries()
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		tx, beginErr := uc.resourceRepository.Begin(DeleteResourceOperationName)
		if beginErr != nil {
			return beginErr
		}

		res, findErr := tx.FindResourceByKeys(reporterResourceKey)
		if findErr != nil {
			_ = tx.Rollback()
			if errors.Is(findErr, model.ErrSerializationFailure) {
				lastErr = findErr
				continue
			}
			if errors.Is(findErr, model.ErrResourceNotFound) {
				return ErrResourceNotFound
			}
			return ErrDatabaseError
		}

		log.Info("Found Resource, deleting: ", res)
		if deleteErr := res.Delete(reporterResourceKey); deleteErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to delete resource: %w", deleteErr)
		}

		if saveErr := tx.Save(*res, model.OperationTypeDeleted, txid); saveErr != nil {
			_ = tx.Rollback()
			if errors.Is(saveErr, model.ErrSerializationFailure) {
				lastErr = saveErr
				continue
			}
			return saveErr
		}

		if commitErr := tx.Commit(); commitErr != nil {
			_ = tx.Rollback()
			if errors.Is(commitErr, model.ErrSerializationFailure) {
				lastErr = commitErr
				continue
			}
			return commitErr
		}

		// Extract principal for success logging
		principal := authzCtx.ExtractPrincipal()

		// DELETE operation - SEC-MON-REQ-1 compliance (EOI-1 pii_manipulation)
		uc.Log.Infow("msg", "Resource deleted",
			"action", "DELETE",
			"resource_type", reporterResourceKey.ResourceType().String(),
			"resource_id", reporterResourceKey.LocalResourceId(),
			"reporter_type", reporterResourceKey.ReporterType().String(),
			"reporter_instance_id", reporterResourceKey.ReporterInstanceId().String(),
			"principal", principal,
			"outcome", "success",
		)

		// Increment outbox metrics only after successful transaction commit
		metricscollector.Incr(uc.MetricsCollector.OutboxEventWrites, string(model.OperationTypeDeleted.OperationType()))
		return nil
	}

	// Extract principal for failure logging
	principal := authzCtx.ExtractPrincipal()

	// DELETE operation failed - SEC-MON-REQ-1 compliance (EOI-1 pii_manipulation, EOI-11 warnings_or_errors)
	uc.Log.Warnw("msg", "Delete resource failed",
		"action", "DELETE",
		"resource_type", reporterResourceKey.ResourceType().String(),
		"resource_id", reporterResourceKey.LocalResourceId(),
		"reporter_type", reporterResourceKey.ReporterType().String(),
		"reporter_instance_id", reporterResourceKey.ReporterInstanceId().String(),
		"principal", principal,
		"outcome", "failure",
		"reason", fmt.Sprintf("%v", lastErr),
	)

	uc.resourceRepository.RecordSerializationExhaustion(DeleteResourceOperationName)
	log.Errorf("delete transaction failed after %d attempts: %v", maxRetries, lastErr)
	return fmt.Errorf("transaction failed after %d attempts: %w", maxRetries, lastErr)
}

// Check verifies if a subject has the specified relation/permission on a resource.
func (uc *Usecase) Check(ctx context.Context, relation model.Relation, sub model.SubjectReference, resourceRef model.ResourceReference, consistency model.Consistency) (model.CheckResult, error) {
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheck, metaauthorizer.NewInventoryResource(resourceRef.Reporter().ReporterType(), resourceRef.ResourceType(), resourceRef.ResourceId())); err != nil {
		return model.CheckResult{}, err
	}
	resolved, err := uc.resolveConsistency(ctx, consistency, resourceRef, false)
	if err != nil {
		return model.CheckResult{}, err
	}
	result, err := uc.checkPermission(ctx, relation, sub, resourceRef, resolved)

	// Get authz context for logging
	authzCtx, _ := authnapi.FromAuthzContext(ctx)
	principal := authzCtx.ExtractPrincipal()

	if err != nil {
		// Operation failed - SEC-MON-REQ-1 compliance (EOI-11 warnings_or_errors)
		uc.Log.Warnw("msg", "Permission check operation failed",
			"action", "CHECK",
			"resource_type", resourceRef.ResourceType().String(),
			"resource_id", string(resourceRef.ResourceId()),
			"relation", relation.String(),
			"principal", principal,
			"outcome", "failure",
			"reason", err.Error(),
		)
	} else if !result.Allowed() {
		// Log permission denials - SEC-MON-REQ-1 compliance (EOI-8 authorization_failure)
		uc.Log.Warnw("msg", "Permission denied",
			"event", "authorization_failure",
			"action", "CHECK",
			"resource_type", resourceRef.ResourceType().String(),
			"resource_id", string(resourceRef.ResourceId()),
			"relation", relation.String(),
			"principal", principal,
			"outcome", "failure",
		)
	}

	return result, err
}

// CheckSelf verifies access for the authenticated user using the self-subject strategy.
func (uc *Usecase) CheckSelf(ctx context.Context, relation model.Relation, resourceRef model.ResourceReference, consistency model.Consistency) (model.CheckResult, error) {
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheckSelf, metaauthorizer.NewInventoryResource(resourceRef.Reporter().ReporterType(), resourceRef.ResourceType(), resourceRef.ResourceId())); err != nil {
		return model.CheckResult{}, err
	}
	subjectRef, err := uc.selfSubjectFromContext(ctx)
	if err != nil {
		return model.CheckResult{}, err
	}
	resolved, err := uc.resolveConsistency(ctx, consistency, resourceRef, true)
	if err != nil {
		return model.CheckResult{}, err
	}
	result, err := uc.checkPermission(ctx, relation, subjectRef, resourceRef, resolved)

	// Get authz context for logging
	authzCtx, _ := authnapi.FromAuthzContext(ctx)
	principal := authzCtx.ExtractPrincipal()

	if err != nil {
		// Operation failed - SEC-MON-REQ-1 compliance (EOI-11 warnings_or_errors)
		uc.Log.Warnw("msg", "Self permission check operation failed",
			"action", "CHECK_SELF",
			"resource_type", resourceRef.ResourceType().String(),
			"resource_id", string(resourceRef.ResourceId()),
			"relation", relation.String(),
			"principal", principal,
			"outcome", "failure",
			"reason", err.Error(),
		)
	} else if !result.Allowed() {
		// Log permission denials - SEC-MON-REQ-1 compliance (EOI-8 authorization_failure)
		uc.Log.Warnw("msg", "Self permission denied",
			"event", "authorization_failure",
			"action", "CHECK_SELF",
			"resource_type", resourceRef.ResourceType().String(),
			"resource_id", string(resourceRef.ResourceId()),
			"relation", relation.String(),
			"principal", principal,
			"outcome", "failure",
		)
	}

	return result, err
}

// CheckForUpdate verifies if a subject can update the resource.
func (uc *Usecase) CheckForUpdate(ctx context.Context, relation model.Relation, sub model.SubjectReference, resourceRef model.ResourceReference) (model.CheckResult, error) {
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheckForUpdate, metaauthorizer.NewInventoryResource(resourceRef.Reporter().ReporterType(), resourceRef.ResourceType(), resourceRef.ResourceId())); err != nil {
		return model.CheckResult{}, err
	}

	rel := model.NewRelationship(resourceRef, relation, sub)
	return uc.Relations.CheckForUpdate(ctx, rel)
}

// CheckForUpdateBulk performs bulk strongly consistent check-for-update permission checks via relations-api.
func (uc *Usecase) CheckForUpdateBulk(ctx context.Context, cmd CheckForUpdateBulkCommand) (*CheckBulkResult, error) {
	for _, item := range cmd.Items {
		if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheckForUpdateBulk, metaauthorizer.NewInventoryResource(item.Resource.Reporter().ReporterType(), item.Resource.ResourceType(), item.Resource.ResourceId())); err != nil {
			uc.Log.WithContext(ctx).Errorf("meta authz failed for check for update bulk item: %v error: %v", item.Resource, err)
			return nil, err
		}
	}

	rels := checkBulkItemsToRelationships(cmd.Items)
	result, err := uc.Relations.CheckForUpdateBulk(ctx, rels)
	if err != nil {
		return nil, err
	}
	if err := validateBulkResultLength(len(rels), len(result.Pairs())); err != nil {
		return nil, err
	}
	return checkBulkResultFromModel(result), nil
}

// CheckBulk performs bulk permission checks.
func (uc *Usecase) CheckBulk(ctx context.Context, cmd CheckBulkCommand) (*CheckBulkResult, error) {
	if model.ConsistencyTypeOf(cmd.Consistency) == model.ConsistencyAtLeastAsAcknowledged {
		return nil, status.Errorf(codes.InvalidArgument, "inventory-managed consistency tokens aren't available")
	}
	for _, item := range cmd.Items {
		if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheckBulk, metaauthorizer.NewInventoryResource(item.Resource.Reporter().ReporterType(), item.Resource.ResourceType(), item.Resource.ResourceId())); err != nil {
			uc.Log.WithContext(ctx).Errorf("meta authz failed for check bulk item: %v error: %v", item.Resource, err)
			return nil, err
		}
	}

	rels := checkBulkItemsToRelationships(cmd.Items)
	result, err := uc.Relations.CheckBulk(ctx, rels, cmd.Consistency)
	if err != nil {
		return nil, err
	}
	if err := validateBulkResultLength(len(rels), len(result.Pairs())); err != nil {
		return nil, err
	}
	return checkBulkResultFromModel(result), nil
}

// CheckSelfBulk performs bulk permission checks for the authenticated user.
func (uc *Usecase) CheckSelfBulk(ctx context.Context, cmd CheckSelfBulkCommand) (*CheckBulkResult, error) {
	if model.ConsistencyTypeOf(cmd.Consistency) == model.ConsistencyAtLeastAsAcknowledged {
		return nil, status.Errorf(codes.InvalidArgument, "inventory-managed consistency tokens aren't available")
	}
	for _, item := range cmd.Items {
		if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheckSelf, metaauthorizer.NewInventoryResource(item.Resource.Reporter().ReporterType(), item.Resource.ResourceType(), item.Resource.ResourceId())); err != nil {
			uc.Log.WithContext(ctx).Errorf("meta authz failed for check self item: %v error: %v", item.Resource, err)
			return nil, err
		}
	}

	subjectRef, err := uc.selfSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	rels := make([]model.Relationship, len(cmd.Items))
	for i, item := range cmd.Items {
		rels[i] = model.NewRelationship(item.Resource, item.Relation, subjectRef)
	}

	result, err := uc.Relations.CheckBulk(ctx, rels, cmd.Consistency)
	if err != nil {
		return nil, err
	}
	if err := validateBulkResultLength(len(rels), len(result.Pairs())); err != nil {
		return nil, err
	}
	return checkBulkResultFromModel(result), nil
}

// checkPermission runs Relations.Check with the resolved consistency.
func (uc *Usecase) checkPermission(ctx context.Context, relation model.Relation, sub model.SubjectReference, resourceRef model.ResourceReference, consistency model.Consistency) (model.CheckResult, error) {
	rel := model.NewRelationship(resourceRef, relation, sub)
	return uc.Relations.Check(ctx, rel, consistency)
}

// LookupObjects delegates resource lookup to the authorization service.
func (uc *Usecase) LookupObjects(ctx context.Context, cmd LookupObjectsCommand) (model.ResultStream[model.LookupObjectsItem], error) {
	if model.ConsistencyTypeOf(cmd.Consistency) == model.ConsistencyAtLeastAsAcknowledged {
		return nil, status.Errorf(codes.InvalidArgument, "inventory-managed consistency tokens aren't available")
	}
	var reporterType model.ReporterType
	if cmd.ObjectType.HasReporterType() {
		reporterType = *cmd.ObjectType.ReporterType()
	}
	metaObject := metaauthorizer.NewResourceTypeRef(reporterType, cmd.ObjectType.ResourceType())
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationLookupResources, metaObject); err != nil {
		return nil, err
	}

	return uc.Relations.LookupObjects(ctx, cmd.ObjectType, cmd.Relation, cmd.Subject, cmd.Pagination, cmd.Consistency)
}

// LookupSubjects delegates subject lookup to the authorization service.
func (uc *Usecase) LookupSubjects(ctx context.Context, cmd LookupSubjectsCommand) (model.ResultStream[model.LookupSubjectsItem], error) {
	if model.ConsistencyTypeOf(cmd.Consistency) == model.ConsistencyAtLeastAsAcknowledged {
		return nil, status.Errorf(codes.InvalidArgument, "inventory-managed consistency tokens aren't available")
	}
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationLookupSubjects, metaauthorizer.NewInventoryResource(cmd.Resource.Reporter().ReporterType(), cmd.Resource.ResourceType(), cmd.Resource.ResourceId())); err != nil {
		return nil, err
	}

	return uc.Relations.LookupSubjects(ctx, cmd.Resource, cmd.Relation, cmd.SubjectType, cmd.SubjectRelation, cmd.Pagination, cmd.Consistency)
}

// lookupConsistencyTokenFromDB looks up the consistency token from the inventory database.
// Returns the token if found, empty string if resource not found, or error for other failures.
// Converts ResourceReference to ReporterResourceKey at the DB boundary.
func (uc *Usecase) lookupConsistencyTokenFromDB(ctx context.Context, resourceRef model.ResourceReference) (string, error) {
	reporterResourceKey, err := reporterKeyFromResourceRef(resourceRef)
	if err != nil {
		return "", fmt.Errorf("failed to build reporter resource key from reference: %w", err)
	}

	tx, err := uc.resourceRepository.Begin("")
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.FindResourceByKeys(reporterResourceKey)
	if err != nil {
		if errors.Is(err, model.ErrResourceNotFound) {
			uc.Log.WithContext(ctx).Warnf("Resource not found in inventory, falling back to minimize_latency for ref: %v", resourceRef)
			return "", nil
		}
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	token := res.ConsistencyToken().Serialize()
	uc.Log.WithContext(ctx).Debug("Found inventory-managed consistency token")
	return token, nil
}

// resolveConsistency resolves the consistency preference to a concrete model.Consistency.
// For unspecified consistency, uses the DefaultToAtLeastAsAcknowledged feature flag to decide.
func (uc *Usecase) resolveConsistency(ctx context.Context, consistency model.Consistency, resourceRef model.ResourceReference, overrideFeatureFlag bool) (model.Consistency, error) {
	featureFlagEnabled := uc.Config.DefaultToAtLeastAsAcknowledged
	if featureFlagEnabled {
		if overrideFeatureFlag {
			uc.Log.WithContext(ctx).Debug("Feature flag default-to-at-least-as-acknowledged is enabled but bypassed for this call")
		} else {
			uc.Log.WithContext(ctx).Debug("Feature flag default-to-at-least-as-acknowledged is enabled")
		}
	} else {
		uc.Log.WithContext(ctx).Debug("Feature flag default-to-at-least-as-acknowledged is disabled")
	}

	switch model.ConsistencyTypeOf(consistency) {
	case model.ConsistencyMinimizeLatency:
		uc.Log.WithContext(ctx).Debug("Using minimize_latency consistency")
		return model.NewConsistencyMinimizeLatency(), nil

	case model.ConsistencyAtLeastAsFresh:
		uc.Log.WithContext(ctx).Debug("Using at_least_as_fresh consistency")
		return consistency, nil

	case model.ConsistencyAtLeastAsAcknowledged:
		uc.Log.WithContext(ctx).Debug("Using at_least_as_acknowledged consistency - looking up token from DB")
		return uc.resolveFromDB(ctx, resourceRef)

	case model.ConsistencyUnspecified:
		if featureFlagEnabled && !overrideFeatureFlag {
			uc.Log.WithContext(ctx).Debug("Default consistency - looking up token from DB")
			return uc.resolveFromDB(ctx, resourceRef)
		}
		uc.Log.WithContext(ctx).Debug("Default consistency - using minimize_latency")
		return model.NewConsistencyMinimizeLatency(), nil

	default:
		return nil, status.Errorf(codes.Internal, "unexpected consistency preference: %v", model.ConsistencyTypeOf(consistency))
	}
}

// resolveFromDB looks up the consistency token from the inventory database and returns a model.Consistency.
func (uc *Usecase) resolveFromDB(ctx context.Context, resourceRef model.ResourceReference) (model.Consistency, error) {
	tokenStr, err := uc.lookupConsistencyTokenFromDB(ctx, resourceRef)
	if err != nil {
		return nil, err
	}
	if tokenStr == "" {
		return model.NewConsistencyMinimizeLatency(), nil
	}
	return model.NewConsistencyAtLeastAsFresh(model.DeserializeConsistencyToken(tokenStr)), nil
}

func (uc *Usecase) selfSubjectFromContext(ctx context.Context) (model.SubjectReference, error) {
	authzCtx, ok := authnapi.FromAuthzContext(ctx)
	if !ok {
		return model.SubjectReference{}, metaauthorizer.ErrMetaAuthzContextMissing
	}
	if uc == nil || uc.SelfSubjectStrategy == nil {
		return model.SubjectReference{}, ErrSelfSubjectMissing
	}
	subjectRef, err := uc.SelfSubjectStrategy.SubjectFromAuthorizationContext(authzCtx)
	if err != nil {
		return model.SubjectReference{}, ErrSelfSubjectMissing
	}
	return subjectRef, nil
}

// enforceMetaAuthzObject calls the MetaAuthorizer to validate access using a MetaObject.
func (uc *Usecase) enforceMetaAuthzObject(ctx context.Context, relation metaauthorizer.Relation, metaObject metaauthorizer.MetaObject) error {
	return metaauthorizer.EnforceMetaAuthzObject(ctx, uc.MetaAuthorizer, relation, metaObject)
}

func isDuplicateTransactionError(err error) bool {
	var kratosErr *kratosErrors.Error
	if errors.As(err, &kratosErr) {
		return kratosErr.Reason == model.ReasonNonUniqueTransactionID
	}
	return false
}

func getNextTransactionID() (model.TransactionId, error) {
	txid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return model.NewTransactionId(txid.String()), nil
}

// checkBulkItemsToRelationships converts usecase-layer CheckBulkItems to model Relationships.
func checkBulkItemsToRelationships(items []CheckBulkItem) []model.Relationship {
	rels := make([]model.Relationship, len(items))
	for i, item := range items {
		rels[i] = model.NewRelationship(item.Resource, item.Relation, item.Subject)
	}
	return rels
}

func validateBulkResultLength(expected, actual int) error {
	if actual != expected {
		return status.Errorf(codes.Internal,
			"internal error: mismatched check results: expected %d pairs, got %d", expected, actual)
	}
	return nil
}

// checkBulkResultFromModel converts a model.CheckBulkResult to the usecase-layer CheckBulkResult.
func checkBulkResultFromModel(result model.CheckBulkResult) *CheckBulkResult {
	pairs := make([]CheckBulkResultPair, len(result.Pairs()))
	for i, p := range result.Pairs() {
		pairs[i] = CheckBulkResultPair{
			Request: CheckBulkItem{
				Resource: p.Request().Object(),
				Relation: p.Request().Relation(),
				Subject:  p.Request().Subject(),
			},
			Result: CheckBulkResultItem{
				Allowed:   p.Result().Allowed(),
				Error:     p.Result().Err(),
				ErrorCode: p.Result().ErrorCode(),
			},
		}
	}
	return &CheckBulkResult{
		Pairs:            pairs,
		ConsistencyToken: result.ConsistencyToken(),
	}
}

// isSPInAllowlist checks if the caller subject is in the allowlist.
func isSPInAllowlist(callerSubject authnapi.SubjectId, allowlist []string) bool {
	for _, sp := range allowlist {
		if sp == string(callerSubject) || sp == "*" {
			return true
		}
	}

	return false
}

// reporterKeyFromResourceRef converts a ResourceReference to a ReporterResourceKey
// for DB lookups at the repository boundary.
func reporterKeyFromResourceRef(ref model.ResourceReference) (model.ReporterResourceKey, error) {
	var reporterType model.ReporterType
	var instanceId model.ReporterInstanceId
	if ref.HasReporter() {
		reporterType = ref.Reporter().ReporterType()
		if ref.Reporter().HasInstanceId() {
			instanceId = *ref.Reporter().InstanceId()
		}
	}
	return model.NewReporterResourceKey(ref.ResourceId(), ref.ResourceType(), reporterType, instanceId)
}

func computeReadAfterWrite(uc *Usecase, writeVisibility WriteVisibility, callerSubject authnapi.SubjectId) bool {
	if writeVisibility == WriteVisibilityUnspecified || writeVisibility == WriteVisibilityMinimizeLatency {
		return false
	}
	return !common.IsNil(uc.ListenManager) && uc.Config.ReadAfterWriteEnabled && isSPInAllowlist(callerSubject, uc.Config.ReadAfterWriteAllowlist)
}
