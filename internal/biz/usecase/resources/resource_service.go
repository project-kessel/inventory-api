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
	"github.com/sony/gobreaker/v2"
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
	SelfSubjectStrategy selfsubject.SelfSubjectStrategy
}

func New(resourceRepository model.ResourceRepository, schemaRepository model.SchemaRepository,
	relations model.RelationsRepository, namespace string, logger log.Logger,
	listenManager pubsub.ListenManagerImpl, waitForNotifBreaker *gobreaker.CircuitBreaker[any], usecaseConfig *UsecaseConfig, metricsCollector *metricscollector.MetricsCollector, metaAuthorizer metaauthorizer.MetaAuthorizer, selfSubjectStrategy selfsubject.SelfSubjectStrategy) *Usecase {
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

	// Log client_id if available (from OIDC authentication)
	if authzCtx.Subject.ClientID != "" {
		log.Infof("Reporting resource request from client_id: %s, resource_type: %s, reporter_type: %s", authzCtx.Subject.ClientID, cmd.ResourceType, cmd.ReporterType)
	}

	var subscription pubsub.Subscription

	txid, err := getNextTransactionID()
	if err != nil {
		return err
	}

	if cmd.TransactionId == nil || *cmd.TransactionId == "" {
		cmd.TransactionId = &txid
	}

	// Validate command against schemas
	if err := uc.validateReportResourceCommand(ctx, cmd); err != nil {
		return status.Errorf(codes.InvalidArgument, "failed validation for report resource: %v", err)
	}

	readAfterWriteEnabled := computeReadAfterWrite(uc, cmd.WriteVisibility, authzCtx.Subject.SubjectId)
	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		subscription = uc.ListenManager.Subscribe(txid.String())
		defer subscription.Unsubscribe()
	}

	// Check for duplicate transaction ID outside the serializable transaction to avoid
	// predicate locks on the transaction_id indexes. The unique constraint on transaction_id
	// provides the actual correctness guarantee, if a duplicate sneaks past this
	// advisory check due to a concurrent commit.
	if cmd.TransactionId != nil {
		alreadyProcessed, err := uc.resourceRepository.HasTransactionIdBeenProcessed(uc.resourceRepository.GetDB(), cmd.TransactionId.String())
		if err != nil {
			return fmt.Errorf("failed to check transaction ID: %w", err)
		}
		if alreadyProcessed {
			log.Infof("Transaction already processed, skipping update: transaction_id=%s", cmd.TransactionId.String())
			return nil
		}
	}

	var operationType model.EventOperationType
	err = uc.resourceRepository.GetTransactionManager().HandleSerializableTransaction(
		ReportResourceOperationName,
		uc.resourceRepository.GetDB(),
		func(tx *gorm.DB) error {
			res, err := uc.resourceRepository.FindResourceByKeys(tx, reporterResourceKey)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("failed to lookup existing resource: %w", err)
			}

			if err == nil && res != nil {
				log.Info("Resource already exists, updating: ")
				operationType = model.OperationTypeUpdated
				return uc.updateResource(tx, cmd, res, txid.String())
			}

			log.Info("Creating new resource")
			operationType = model.OperationTypeCreated
			return uc.createResource(tx, cmd, txid.String())
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

	txid, err := getNextTransactionID()
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
				return uc.resourceRepository.Save(tx, *res, model.OperationTypeDeleted, txid.String())
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
func (uc *Usecase) Check(ctx context.Context, relation model.Relation, sub model.SubjectReference, resourceRef model.ResourceReference, consistency model.Consistency) (model.CheckResult, error) {
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationCheck, metaauthorizer.NewInventoryResource(resourceRef.Reporter().ReporterType(), resourceRef.ResourceType(), resourceRef.ResourceId())); err != nil {
		return model.CheckResult{}, err
	}
	resolved, err := uc.resolveConsistency(ctx, consistency, resourceRef, false)
	if err != nil {
		return model.CheckResult{}, err
	}
	return uc.checkPermission(ctx, relation, sub, resourceRef, resolved)
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
	return uc.checkPermission(ctx, relation, subjectRef, resourceRef, resolved)
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
	// Passing nil tx is deliberate: this read-only consistency lookup should not run in a transaction.
	res, err := uc.resourceRepository.FindResourceByKeys(nil, reporterResourceKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Resource doesn't exist in inventory, fall back to minimize_latency.
			uc.Log.WithContext(ctx).Warnf("Resource not found in inventory, falling back to minimize_latency for ref: %v", resourceRef)
			return "", nil
		}
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
