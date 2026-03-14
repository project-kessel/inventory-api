package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/cmd/common"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"github.com/project-kessel/inventory-api/internal/subject/selfsubject"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
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

// Application-layer errors for meta-authorization and self-subject resolution.
var (
	// ErrMetaAuthorizerUnavailable indicates the meta authorizer is not configured.
	ErrMetaAuthorizerUnavailable = errors.New("meta authorizer unavailable")
	// ErrMetaAuthorizationDenied indicates the meta authorization check failed.
	ErrMetaAuthorizationDenied = errors.New("meta authorization denied")
	// ErrMetaAuthzContextMissing indicates missing authz context in request.
	ErrMetaAuthzContextMissing = errors.New("meta authorization context missing")
	// ErrSelfSubjectMissing indicates the subject could not be derived for self checks.
	ErrSelfSubjectMissing = errors.New("self subject missing")
)

const listenTimeout = 10 * time.Second

// UsecaseConfig contains configuration flags that control the behavior of usecase operations.
// These flags should be consistent across all handlers.
type UsecaseConfig struct {
	ReadAfterWriteEnabled   bool
	ReadAfterWriteAllowlist []string
	ConsumerEnabled         bool
}

// Usecase provides business logic operations for resource management in the inventory system.
// It coordinates between repositories, authorization, and other system components.
type Usecase struct {
	schemaService       *model.SchemaService
	resourceRepository  model.ResourceRepository
	waitForNotifBreaker *gobreaker.CircuitBreaker
	RelationsRepo       model.RelationsRepository
	MetaAuthorizer      metaauthorizer.MetaAuthorizer
	Namespace           string
	Log                 *log.Helper
	ListenManager       pubsub.ListenManagerImpl
	Config              *UsecaseConfig
	MetricsCollector    *metricscollector.MetricsCollector
	SelfSubjectStrategy selfsubject.SelfSubjectStrategy
}

func New(resourceRepository model.ResourceRepository, schemaRepository model.SchemaRepository,
	relationsRepo model.RelationsRepository, namespace string, logger log.Logger,
	listenManager pubsub.ListenManagerImpl, waitForNotifBreaker *gobreaker.CircuitBreaker, usecaseConfig *UsecaseConfig, metricsCollector *metricscollector.MetricsCollector, metaAuthorizer metaauthorizer.MetaAuthorizer, selfSubjectStrategy selfsubject.SelfSubjectStrategy) *Usecase {
	if metaAuthorizer == nil {
		metaAuthorizer = metaauthorizer.NewSimpleMetaAuthorizer()
	}

	return &Usecase{
		resourceRepository:  resourceRepository,
		schemaService:       model.NewSchemaService(schemaRepository, log.NewHelper(logger)),
		waitForNotifBreaker: waitForNotifBreaker,
		RelationsRepo:       relationsRepo,
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

	// Log client_id if available (from OIDC authentication)
	if authzCtx.Subject.ClientID != "" {
		log.Infof("Reporting resource request from client_id: %s, resource_type: %s, reporter_type: %s", authzCtx.Subject.ClientID, cmd.ResourceType, cmd.ReporterType)
	}

	var subscription pubsub.Subscription

	txidStr, err := getNextTransactionID()
	if err != nil {
		return err
	}

	if cmd.TransactionId == nil || *cmd.TransactionId == "" {
		generated := model.NewTransactionId(txidStr)
		cmd.TransactionId = &generated
	}

	// Validate command against schemas
	if err := uc.validateReportResourceCommand(ctx, cmd); err != nil {
		return status.Errorf(codes.InvalidArgument, "failed validation for report resource: %v", err)
	}

	readAfterWriteEnabled := computeReadAfterWrite(uc, cmd.WriteVisibility, authzCtx.Subject.SubjectId)
	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		subscription = uc.ListenManager.Subscribe(txidStr)
		defer subscription.Unsubscribe()
	}

	var operationType model.EventOperationType
	err = uc.resourceRepository.GetTransactionManager().HandleSerializableTransaction(
		ReportResourceOperationName,
		uc.resourceRepository.GetDB(),
		func(tx *gorm.DB) error {
			// Check for duplicate transaction ID's before we find the resource for quicker returns if it fails
			if cmd.TransactionId != nil {
				// TODO: repository should accept the transactionID type natively
				alreadyProcessed, err := uc.resourceRepository.HasTransactionIdBeenProcessed(tx, string(*cmd.TransactionId))
				if err != nil {
					return fmt.Errorf("failed to check transaction ID: %w", err)
				}
				if alreadyProcessed {
					log.Info("Transaction already processed, skipping update")
					return nil
				}
			}

			res, err := uc.resourceRepository.FindResourceByKeys(tx, reporterResourceKey)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("failed to lookup existing resource: %w", err)
			}

			if err == nil && res != nil {
				log.Info("Resource already exists, updating: ")
				operationType = model.OperationTypeUpdated
				return uc.updateResource(tx, cmd, res, txidStr)
			}

			log.Info("Creating new resource")
			operationType = model.OperationTypeCreated
			return uc.createResource(tx, cmd, txidStr)
		},
	)

	if err != nil {
		return err
	}

	// Increment outbox metrics only after successful transaction commit
	if operationType != nil {
		metricscollector.Incr(uc.MetricsCollector.OutboxEventWrites, string(operationType.OperationType()))
	}

	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		timeoutCtx, cancel := context.WithTimeout(ctx, listenTimeout)
		defer cancel()

		_, err := uc.waitForNotifBreaker.Execute(func() (interface{}, error) {
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

func (uc *Usecase) createResource(tx *gorm.DB, cmd ReportResourceCommand, txidStr string) error {
	resourceId, err := uc.resourceRepository.NextResourceId()
	if err != nil {
		return err
	}

	reporterResourceId, err := uc.resourceRepository.NextReporterResourceId()
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

	return uc.resourceRepository.Save(tx, resource, model.OperationTypeCreated, txidStr)
}

func (uc *Usecase) updateResource(tx *gorm.DB, cmd ReportResourceCommand, existingResource *model.Resource, txidStr string) error {
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

	return uc.resourceRepository.Save(tx, *existingResource, model.OperationTypeUpdated, txidStr)
}

func (uc *Usecase) Delete(ctx context.Context, reporterResourceKey model.ReporterResourceKey) error {
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationDeleteResource, metaauthorizer.NewInventoryResourceFromKey(reporterResourceKey)); err != nil {
		return err
	}

	txidStr, err := getNextTransactionID()
	if err != nil {
		return err
	}
	// Log client_id if available (from OIDC authentication)
	if authzCtx, ok := authnapi.FromAuthzContext(ctx); ok && authzCtx.Subject != nil && authzCtx.Subject.ClientID != "" {
		log.Infof("Deleting resource %v from client_id: %s", reporterResourceKey, authzCtx.Subject.ClientID)
	}

	err = uc.resourceRepository.GetTransactionManager().HandleSerializableTransaction(
		DeleteResourceOperationName,
		uc.resourceRepository.GetDB(),
		func(tx *gorm.DB) error {
			res, err := uc.resourceRepository.FindResourceByKeys(tx, reporterResourceKey)

			if err == nil && res != nil {
				log.Info("Found Resource, deleting: ", res)
				err := res.Delete(reporterResourceKey)
				if err != nil {
					return fmt.Errorf("failed to delete resource: %w", err)
				}
				return uc.resourceRepository.Save(tx, *res, model.OperationTypeDeleted, txidStr)
			} else {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrResourceNotFound
				}
				return ErrDatabaseError
			}
		},
	)

	if err != nil {
		return err
	}

	// Increment outbox metrics only after successful transaction commit
	metricscollector.Incr(uc.MetricsCollector.OutboxEventWrites, string(model.OperationTypeDeleted.OperationType()))
	return nil
}

// Check verifies if a subject has the specified relation/permission on a resource.
func (uc *Usecase) Check(ctx context.Context, relation model.Relation, sub model.SubjectReference, reporterResourceKey model.ReporterResourceKey) (bool, error) {
	// TODO: should also check caller is allowed to check subject also
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheck, metaauthorizer.NewInventoryResourceFromKey(reporterResourceKey)); err != nil {
		return false, err
	}

	return uc.checkPermission(ctx, relation, sub, reporterResourceKey)
}

// CheckSelf verifies access for the authenticated user using the self-subject strategy.
// Uses relation="check_self" for meta-authorization.
func (uc *Usecase) CheckSelf(ctx context.Context, relation model.Relation, reporterResourceKey model.ReporterResourceKey) (bool, error) {
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheckSelf, metaauthorizer.NewInventoryResourceFromKey(reporterResourceKey)); err != nil {
		return false, err
	}
	subjectRef, err := uc.selfSubjectFromContext(ctx)
	if err != nil {
		return false, err
	}
	return uc.checkPermission(ctx, relation, subjectRef, reporterResourceKey)
}

// CheckForUpdate verifies if a subject can update the resource.
func (uc *Usecase) CheckForUpdate(ctx context.Context, relation model.Relation, sub model.SubjectReference, reporterResourceKey model.ReporterResourceKey) (bool, error) {
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheckForUpdate, metaauthorizer.NewInventoryResourceFromKey(reporterResourceKey)); err != nil {
		return false, err
	}

	allowed, _, err := uc.RelationsRepo.CheckForUpdate(ctx, reporterResourceKey, relation, sub)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

// CheckBulk performs bulk permission checks.
func (uc *Usecase) CheckBulk(ctx context.Context, cmd CheckBulkCommand) (*CheckBulkResult, error) {
	// Meta-authorization for each item
	for _, item := range cmd.Items {
		if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheckBulk, metaauthorizer.NewInventoryResourceFromKey(item.Resource)); err != nil {
			uc.Log.WithContext(ctx).Errorf("meta authz failed for check bulk item: %v error: %v", item.Resource, err)
			return nil, err
		}
	}

	checkItems := make([]model.CheckItem, len(cmd.Items))
	for i, item := range cmd.Items {
		checkItems[i] = model.CheckItem{
			Resource: item.Resource,
			Relation: item.Relation,
			Subject:  item.Subject,
		}
	}

	results, token, err := uc.RelationsRepo.CheckBulk(ctx, checkItems, cmd.Consistency)
	if err != nil {
		return nil, err
	}

	if len(results) != len(cmd.Items) {
		return nil, status.Errorf(codes.Internal, "internal error: mismatched backend check results: expected %d pairs, got %d", len(cmd.Items), len(results))
	}

	pairs := make([]CheckBulkResultPair, len(results))
	for i, result := range results {
		pairs[i] = CheckBulkResultPair{
			Request: cmd.Items[i],
			Result: CheckBulkResultItem{
				Allowed: result.Allowed,
				Error:   result.Error,
			},
		}
	}

	return &CheckBulkResult{
		Pairs:            pairs,
		ConsistencyToken: token,
	}, nil
}

// CheckSelfBulk performs bulk permission checks for the authenticated user.
// Uses relation="check_self" for meta-authorization.
func (uc *Usecase) CheckSelfBulk(ctx context.Context, cmd CheckSelfBulkCommand) (*CheckBulkResult, error) {
	// Meta-authorization for each item
	for _, item := range cmd.Items {
		if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheckSelf, metaauthorizer.NewInventoryResourceFromKey(item.Resource)); err != nil {
			uc.Log.WithContext(ctx).Errorf("meta authz failed for check self item: %v error: %v", item.Resource, err)
			return nil, err
		}
	}

	subjectRef, err := uc.selfSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	bulkCmd := CheckBulkCommand{
		Items:       make([]CheckBulkItem, len(cmd.Items)),
		Consistency: cmd.Consistency,
	}
	for i, item := range cmd.Items {
		bulkCmd.Items[i] = CheckBulkItem{
			Resource: item.Resource,
			Relation: item.Relation,
			Subject:  subjectRef,
		}
	}

	return uc.CheckBulk(ctx, bulkCmd)
}

func (uc *Usecase) checkPermission(ctx context.Context, relation model.Relation, sub model.SubjectReference, reporterResourceKey model.ReporterResourceKey) (bool, error) {
	res, err := uc.resourceRepository.FindResourceByKeys(nil, reporterResourceKey)
	var consistency model.Consistency
	if err != nil {
		log.Info("Did not find resource")
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, err
		}
		consistency = model.NewConsistencyMinimizeLatency()
	} else {
		token := res.ConsistencyToken()
		if token == "" {
			consistency = model.NewConsistencyMinimizeLatency()
		} else {
			consistency = model.NewConsistencyAtLeastAsFresh(token)
		}
	}

	allowed, _, err := uc.RelationsRepo.Check(ctx, reporterResourceKey, relation, sub, consistency)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

// LookupResources delegates resource lookup to the authorization service.
// Returns an iterator for receiving lookup results.
func (uc *Usecase) LookupResources(ctx context.Context, cmd LookupResourcesCommand) (model.LookupResourcesIterator, error) {
	metaObject := metaauthorizer.NewResourceTypeRef(cmd.ReporterType, cmd.ResourceType)
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationLookupResources, metaObject); err != nil {
		return nil, err
	}

	return uc.RelationsRepo.LookupResources(ctx, model.LookupResourcesQuery{
		ResourceType: cmd.ResourceType,
		ReporterType: cmd.ReporterType,
		Relation:     cmd.Relation,
		Subject:      cmd.Subject,
		Limit:        cmd.Limit,
		Continuation: cmd.Continuation,
		Consistency:  cmd.Consistency,
	})
}

func (uc *Usecase) selfSubjectFromContext(ctx context.Context) (model.SubjectReference, error) {
	authzCtx, ok := authnapi.FromAuthzContext(ctx)
	if !ok {
		return model.SubjectReference{}, ErrMetaAuthzContextMissing
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
	authzCtx, ok := authnapi.FromAuthzContext(ctx)
	if !ok {
		return ErrMetaAuthzContextMissing
	}
	if uc.MetaAuthorizer == nil {
		return ErrMetaAuthorizerUnavailable
	}

	allowed, err := uc.MetaAuthorizer.Check(ctx, metaObject, relation, authzCtx)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrMetaAuthorizationDenied
	}
	return nil
}

// validateReportResourceCommand validates a ReportResourceCommand against schemas.
// It checks that the reporter is allowed for the resource type,
// and validates both reporter and common representations.
func (uc *Usecase) validateReportResourceCommand(ctx context.Context, cmd ReportResourceCommand) error {
	resourceType := cmd.ResourceType.String()
	reporterType := cmd.ReporterType.String()

	if resourceType == "" {
		return fmt.Errorf("missing 'type' field")
	}
	if reporterType == "" {
		return fmt.Errorf("missing 'reporterType' field")
	}

	if isReporter, err := uc.schemaService.IsReporterForResource(ctx, resourceType, reporterType); !isReporter {
		if err != nil {
			return err
		}
		return fmt.Errorf("reporter %s does not report resource types: %s", reporterType, resourceType)
	}

	if cmd.ReporterRepresentation != nil {
		sanitizedReporterRepresentation := removeNulls(map[string]interface{}(*cmd.ReporterRepresentation))
		if err := uc.schemaService.ReporterShallowValidate(ctx, resourceType, reporterType, sanitizedReporterRepresentation); err != nil {
			return err
		}
	}

	if cmd.CommonRepresentation != nil {
		commonRepresentation := map[string]interface{}(*cmd.CommonRepresentation)
		if err := uc.schemaService.CommonShallowValidate(ctx, resourceType, commonRepresentation); err != nil {
			return err
		}
	}

	return nil
}

func getNextTransactionID() (string, error) {
	txid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return txid.String(), nil
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

func computeReadAfterWrite(uc *Usecase, writeVisibility WriteVisibility, callerSubject authnapi.SubjectId) bool {
	if writeVisibility == WriteVisibilityUnspecified || writeVisibility == WriteVisibilityMinimizeLatency {
		return false
	}
	return !common.IsNil(uc.ListenManager) && uc.Config.ReadAfterWriteEnabled && isSPInAllowlist(callerSubject, uc.Config.ReadAfterWriteAllowlist)
}
