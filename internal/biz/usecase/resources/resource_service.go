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
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/schema"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"github.com/project-kessel/inventory-api/internal/subject/selfsubject"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
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
	schemaUsecase       *SchemaUsecase
	resourceRepository  data.ResourceRepository
	waitForNotifBreaker *gobreaker.CircuitBreaker
	Authz               authzapi.Authorizer
	MetaAuthorizer      metaauthorizer.MetaAuthorizer
	Namespace           string
	Log                 *log.Helper
	ListenManager       pubsub.ListenManagerImpl
	Config              *UsecaseConfig
	MetricsCollector    *metricscollector.MetricsCollector
	SelfSubjectStrategy selfsubject.SelfSubjectStrategy
}

func New(resourceRepository data.ResourceRepository, schemaRepository schema.Repository,
	authz authzapi.Authorizer, namespace string, logger log.Logger,
	listenManager pubsub.ListenManagerImpl, waitForNotifBreaker *gobreaker.CircuitBreaker, usecaseConfig *UsecaseConfig, metricsCollector *metricscollector.MetricsCollector, metaAuthorizer metaauthorizer.MetaAuthorizer, selfSubjectStrategy selfsubject.SelfSubjectStrategy) *Usecase {
	if metaAuthorizer == nil {
		metaAuthorizer = metaauthorizer.NewSimpleMetaAuthorizer()
	}

	return &Usecase{
		resourceRepository:  resourceRepository,
		schemaUsecase:       NewSchemaUsecase(schemaRepository, log.NewHelper(logger)),
		waitForNotifBreaker: waitForNotifBreaker,
		Authz:               authz,
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
		return fmt.Errorf("failed to create reporter resource key: %w", err)
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

	// Validate command against schemas
	if err := uc.validateReportResourceCommand(ctx, cmd); err != nil {
		return status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	readAfterWriteEnabled := computeReadAfterWrite(uc, cmd.WriteVisibility, authzCtx.Subject.SubjectId)
	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		subscription = uc.ListenManager.Subscribe(txidStr)
		defer subscription.Unsubscribe()
	}

	var operationType biz.EventOperationType
	err = uc.resourceRepository.GetTransactionManager().HandleSerializableTransaction(
		ReportResourceOperationName,
		uc.resourceRepository.GetDB(),
		func(tx *gorm.DB) error {
			// Check for duplicate transaction ID's before we find the resource for quicker returns if it fails
			if cmd.TransactionId != "" {
				alreadyProcessed, err := uc.resourceRepository.HasTransactionIdBeenProcessed(tx, string(cmd.TransactionId))
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
				operationType = biz.OperationTypeUpdated
				return uc.updateResource(tx, cmd, res, txidStr)
			}

			log.Info("Creating new resource")
			operationType = biz.OperationTypeCreated
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
		cmd.TransactionId,
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

	return uc.resourceRepository.Save(tx, resource, biz.OperationTypeCreated, txidStr)
}

func (uc *Usecase) updateResource(tx *gorm.DB, cmd ReportResourceCommand, existingResource *model.Resource, txidStr string) error {
	reporterResourceKey, err := model.NewReporterResourceKey(
		cmd.LocalResourceId,
		cmd.ResourceType,
		cmd.ReporterType,
		cmd.ReporterInstanceId,
	)
	if err != nil {
		return fmt.Errorf("failed to create reporter resource key: %w", err)
	}

	err = existingResource.Update(
		reporterResourceKey,
		cmd.ApiHref,
		cmd.ConsoleHref,
		cmd.ReporterVersion,
		cmd.ReporterRepresentation,
		cmd.CommonRepresentation,
		cmd.TransactionId,
	)
	if err != nil {
		return fmt.Errorf("failed to update resource: %w", err)
	}

	return uc.resourceRepository.Save(tx, *existingResource, biz.OperationTypeUpdated, txidStr)
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
				return uc.resourceRepository.Save(tx, *res, biz.OperationTypeDeleted, txidStr)
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
	metricscollector.Incr(uc.MetricsCollector.OutboxEventWrites, string(biz.OperationTypeDeleted.OperationType()))
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

	// Convert model types to v1beta1 for the Authz interface
	namespace := reporterResourceKey.ReporterType().Serialize()
	v1beta1Subject := subjectToV1Beta1(sub)
	allowed, _, err := uc.Authz.CheckForUpdate(ctx, namespace, relation.Serialize(), reporterResourceKey.ResourceType().Serialize(), reporterResourceKey.LocalResourceId().Serialize(), v1beta1Subject)
	if err != nil {
		return false, err
	}

	if allowed == kessel.CheckForUpdateResponse_ALLOWED_TRUE {
		return true, nil
	}
	return false, nil
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

	// Convert to v1beta1 for the Authz interface
	v1beta1Req := checkBulkCommandToV1beta1(cmd)
	resp, err := uc.Authz.CheckBulk(ctx, v1beta1Req)
	if err != nil {
		return nil, err
	}

	return checkBulkResultFromV1beta1(resp, cmd)
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

	// Convert to CheckBulkCommand with the resolved subject
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

	// Convert to v1beta1 for the Authz interface
	v1beta1Req := checkBulkCommandToV1beta1(bulkCmd)
	resp, err := uc.Authz.CheckBulk(ctx, v1beta1Req)
	if err != nil {
		return nil, err
	}

	return checkBulkResultFromV1beta1(resp, bulkCmd)
}

func (uc *Usecase) checkPermission(ctx context.Context, relation model.Relation, sub model.SubjectReference, reporterResourceKey model.ReporterResourceKey) (bool, error) {
	res, err := uc.resourceRepository.FindResourceByKeys(nil, reporterResourceKey)
	var consistencyToken string
	if err != nil {
		log.Info("Did not find resource")
		// If the resource doesn't exist in inventory (ie. no consistency token available)
		// we send a check request with minimize latency
		// err otherwise.
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, err
		}

		consistencyToken = ""
	} else {
		consistencyToken = res.ConsistencyToken().Serialize()
	}

	// Convert model types to v1beta1 for the Authz interface
	namespace := reporterResourceKey.ReporterType().Serialize()
	v1beta1Subject := subjectToV1Beta1(sub)
	allowed, _, err := uc.Authz.Check(ctx, namespace, relation.Serialize(), consistencyToken, reporterResourceKey.ResourceType().Serialize(), reporterResourceKey.LocalResourceId().Serialize(), v1beta1Subject)
	if err != nil {
		return false, err
	}

	if allowed == kessel.CheckResponse_ALLOWED_TRUE {
		return true, nil
	}
	return false, nil
}

// LookupResources delegates resource lookup to the authorization service.
// Returns a streaming client for receiving lookup results.
// TODO: remove v1beta1 response type
func (uc *Usecase) LookupResources(ctx context.Context, cmd LookupResourcesCommand) (grpc.ServerStreamingClient[kessel.LookupResourcesResponse], error) {
	// Meta-authorize against the resource type (not a specific resource instance)
	metaObject := metaauthorizer.NewResourceTypeRef(cmd.ReporterType, cmd.ResourceType)
	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationLookupResources, metaObject); err != nil {
		return nil, err
	}

	// Convert to v1beta1 for the Authz interface
	v1beta1Req := lookupResourcesCommandToV1beta1(cmd)
	return uc.Authz.LookupResources(ctx, v1beta1Req)
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

	if isReporter, err := uc.schemaUsecase.IsReporterForResource(ctx, resourceType, reporterType); !isReporter {
		if err != nil {
			return err
		}
		return fmt.Errorf("reporter %s does not report resource types: %s", reporterType, resourceType)
	}

	var sanitizedReporterRepresentation map[string]interface{}
	if cmd.ReporterRepresentation != nil {
		sanitizedReporterRepresentation = removeNulls(map[string]interface{}(cmd.ReporterRepresentation))
	}

	// Validate reporter-specific data using the sanitized map
	if err := uc.schemaUsecase.ReporterShallowValidate(ctx, resourceType, reporterType, sanitizedReporterRepresentation); err != nil {
		return err
	}

	// Get common representation (no sanitization needed based on original code)
	var commonRepresentation map[string]interface{}
	if cmd.CommonRepresentation != nil {
		commonRepresentation = map[string]interface{}(cmd.CommonRepresentation)
	}

	// Validate common data
	if err := uc.schemaUsecase.CommonShallowValidate(ctx, resourceType, commonRepresentation); err != nil {
		return err
	}

	return nil
}

// subjectToV1Beta1 converts a model.SubjectReference to a v1beta1 SubjectReference for the Authz interface.
func subjectToV1Beta1(sub model.SubjectReference) *kessel.SubjectReference {
	subKey := sub.Subject()
	ref := &kessel.SubjectReference{
		Subject: &kessel.ObjectReference{
			Type: &kessel.ObjectType{
				Namespace: subKey.ReporterType().Serialize(),
				Name:      subKey.ResourceType().Serialize(),
			},
			Id: subKey.LocalResourceId().Serialize(),
		},
	}
	if sub.HasRelation() {
		relation := sub.Relation().Serialize()
		ref.Relation = &relation
	}
	return ref
}

// checkBulkCommandToV1beta1 converts a CheckBulkCommand to v1beta1 for the Authz interface.
func checkBulkCommandToV1beta1(cmd CheckBulkCommand) *kessel.CheckBulkRequest {
	items := make([]*kessel.CheckBulkRequestItem, len(cmd.Items))
	for i, item := range cmd.Items {
		items[i] = &kessel.CheckBulkRequestItem{
			Resource: &kessel.ObjectReference{
				Type: &kessel.ObjectType{
					Namespace: item.Resource.ReporterType().Serialize(),
					Name:      item.Resource.ResourceType().Serialize(),
				},
				Id: item.Resource.LocalResourceId().Serialize(),
			},
			Relation: item.Relation.Serialize(),
			Subject:  subjectToV1Beta1(item.Subject),
		}
	}

	var consistency *kessel.Consistency
	if !cmd.Consistency.MinimizeLatency() {
		consistency = &kessel.Consistency{
			Requirement: &kessel.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &kessel.ConsistencyToken{
					Token: cmd.Consistency.AtLeastAsFresh().Serialize(),
				},
			},
		}
	} else {
		consistency = &kessel.Consistency{
			Requirement: &kessel.Consistency_MinimizeLatency{
				MinimizeLatency: true,
			},
		}
	}

	return &kessel.CheckBulkRequest{
		Items:       items,
		Consistency: consistency,
	}
}

// checkBulkResultFromV1beta1 converts a v1beta1 CheckBulkResponse to CheckBulkResult.
// Returns error if the response length doesn't match the command items length.
func checkBulkResultFromV1beta1(resp *kessel.CheckBulkResponse, cmd CheckBulkCommand) (*CheckBulkResult, error) {
	respPairs := resp.GetPairs()
	if len(respPairs) != len(cmd.Items) {
		return nil, status.Errorf(codes.Internal, "internal error: mismatched backend check results: expected %d pairs, got %d", len(cmd.Items), len(respPairs))
	}

	pairs := make([]CheckBulkResultPair, len(respPairs))
	for i, pair := range respPairs {
		var resultItem CheckBulkResultItem
		if pair.GetError() != nil {
			resultItem = CheckBulkResultItem{
				Allowed:   false,
				Error:     fmt.Errorf("check failed: %s", pair.GetError().GetMessage()),
				ErrorCode: pair.GetError().GetCode(),
			}
		} else if pair.GetItem() != nil {
			resultItem = CheckBulkResultItem{
				Allowed:   pair.GetItem().GetAllowed() == kessel.CheckBulkResponseItem_ALLOWED_TRUE,
				Error:     nil,
				ErrorCode: 0,
			}
		}

		pairs[i] = CheckBulkResultPair{
			Request: cmd.Items[i],
			Result:  resultItem,
		}
	}

	var token model.ConsistencyToken
	if resp.GetConsistencyToken() != nil {
		token = model.DeserializeConsistencyToken(resp.GetConsistencyToken().GetToken())
	}

	return &CheckBulkResult{
		Pairs:            pairs,
		ConsistencyToken: token,
	}, nil
}

func getNextTransactionID() (string, error) {
	txid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return txid.String(), nil
}

// lookupResourcesCommandToV1beta1 converts a LookupResourcesCommand to v1beta1.
func lookupResourcesCommandToV1beta1(cmd LookupResourcesCommand) *kessel.LookupResourcesRequest {
	var continuationToken *string
	if cmd.Continuation != "" {
		continuationToken = &cmd.Continuation
	}
	var consistency *kessel.Consistency
	if !cmd.Consistency.MinimizeLatency() {
		consistency = &kessel.Consistency{
			Requirement: &kessel.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &kessel.ConsistencyToken{
					Token: cmd.Consistency.AtLeastAsFresh().Serialize(),
				},
			},
		}
	} else {
		consistency = &kessel.Consistency{
			Requirement: &kessel.Consistency_MinimizeLatency{
				MinimizeLatency: true,
			},
		}
	}

	return &kessel.LookupResourcesRequest{
		ResourceType: &kessel.ObjectType{
			Namespace: cmd.ReporterType.Serialize(),
			Name:      cmd.ResourceType.Serialize(),
		},
		Relation: cmd.Relation.Serialize(),
		Subject:  subjectToV1Beta1(cmd.Subject),
		Pagination: &kessel.RequestPagination{
			Limit:             cmd.Limit,
			ContinuationToken: continuationToken,
		},
		Consistency: consistency,
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

func computeReadAfterWrite(uc *Usecase, writeVisibility WriteVisibility, callerSubject authnapi.SubjectId) bool {
	if writeVisibility == WriteVisibilityUnspecified || writeVisibility == WriteVisibilityMinimizeLatency {
		return false
	}
	return !common.IsNil(uc.ListenManager) && uc.Config.ReadAfterWriteEnabled && isSPInAllowlist(callerSubject, uc.Config.ReadAfterWriteAllowlist)
}
