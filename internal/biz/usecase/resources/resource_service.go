package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/cmd/common"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	"github.com/project-kessel/inventory-api/internal/data"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"github.com/project-kessel/inventory-api/internal/subject/selfsubject"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
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
// It coordinates between repositories, authorization, eventing, and other system components.
type Usecase struct {
	resourceRepository  data.ResourceRepository
	waitForNotifBreaker *gobreaker.CircuitBreaker
	Authz               authzapi.Authorizer
	MetaAuthorizer      metaauthorizer.MetaAuthorizer
	Eventer             eventingapi.Manager
	// TODO: Remove; unused
	Namespace           string
	Log                 *log.Helper
	ListenManager       pubsub.ListenManagerImpl
	Config              *UsecaseConfig
	MetricsCollector    *metricscollector.MetricsCollector
	SelfSubjectStrategy selfsubject.SelfSubjectStrategy
}

func New(resourceRepository data.ResourceRepository,
	authz authzapi.Authorizer, eventer eventingapi.Manager, namespace string, logger log.Logger,
	listenManager pubsub.ListenManagerImpl, waitForNotifBreaker *gobreaker.CircuitBreaker, usecaseConfig *UsecaseConfig, metricsCollector *metricscollector.MetricsCollector, metaAuthorizer metaauthorizer.MetaAuthorizer, selfSubjectStrategy selfsubject.SelfSubjectStrategy) *Usecase {
	if metaAuthorizer == nil {
		metaAuthorizer = metaauthorizer.NewSimpleMetaAuthorizer()
	}

	return &Usecase{
		resourceRepository:  resourceRepository,
		waitForNotifBreaker: waitForNotifBreaker,
		Authz:               authz,
		Eventer:             eventer,
		MetaAuthorizer:      metaAuthorizer,
		Namespace:           namespace,
		Log:                 log.NewHelper(logger),
		ListenManager:       listenManager,
		Config:              usecaseConfig,
		MetricsCollector:    metricsCollector,
		SelfSubjectStrategy: selfSubjectStrategy,
	}
}

func (uc *Usecase) ReportResource(ctx context.Context, request *v1beta2.ReportResourceRequest, reporterPrincipal string) error {
	reporterResourceKey, err := getReporterResourceKeyFromRequest(request)
	if err != nil {
		log.Error("failed to create reporter resource key: ", err)
		return status.Errorf(codes.InvalidArgument, "failed to create reporter resource key: %v", err)
	}

	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationReportResource, metaauthorizer.NewInventoryResourceFromKey(reporterResourceKey)); err != nil {
		return err
	}

	// Log client_id if available (from OIDC authentication)
	if authzCtx, ok := authnapi.FromAuthzContext(ctx); ok && authzCtx.Subject != nil && authzCtx.Subject.ClientID != "" {
		log.Infof("Reporting resource request from client_id: %s, request: %v", authzCtx.Subject.ClientID, request)
	}
	var subscription pubsub.Subscription
	txidStr, err := getNextTransactionID()
	if err != nil {
		return err
	}

	err = validateReportResourceRequest(ctx, request, uc.schemaUsecase)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed validation for report resource: %v", err)
	}

	readAfterWriteEnabled := computeReadAfterWrite(uc, request.WriteVisibility, reporterPrincipal)
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
			transactionId := request.GetRepresentations().GetMetadata().GetTransactionId()
			if transactionId != "" {
				alreadyProcessed, err := uc.resourceRepository.HasTransactionIdBeenProcessed(tx, transactionId)
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
				return uc.updateResource(tx, request, res, txidStr)
			}

			log.Info("Creating new resource")
			operationType = biz.OperationTypeCreated
			return uc.createResource(tx, request, txidStr)
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

func (uc *Usecase) createResource(tx *gorm.DB, request *v1beta2.ReportResourceRequest, txidStr string) error {
	resourceId, err := uc.resourceRepository.NextResourceId()
	if err != nil {
		return err
	}

	reporterResourceId, err := uc.resourceRepository.NextReporterResourceId()
	if err != nil {
		return err
	}

	localResourceId, err := model.NewLocalResourceId(request.GetRepresentations().GetMetadata().GetLocalResourceId())
	if err != nil {
		return fmt.Errorf("invalid local resource ID: %w", err)
	}

	resourceType, err := model.NewResourceType(request.GetType())
	if err != nil {
		return fmt.Errorf("invalid resource type: %w", err)
	}

	reporterType, err := model.NewReporterType(request.GetReporterType())
	if err != nil {
		return fmt.Errorf("invalid reporter type: %w", err)
	}

	reporterInstanceId, err := model.NewReporterInstanceId(request.GetReporterInstanceId())
	if err != nil {
		return fmt.Errorf("invalid reporter instance ID: %w", err)
	}

	apiHref, err := model.NewApiHref(request.GetRepresentations().GetMetadata().GetApiHref())
	if err != nil {
		return fmt.Errorf("invalid API href: %w", err)
	}

	var consoleHref model.ConsoleHref
	if consoleHrefVal := request.GetRepresentations().GetMetadata().GetConsoleHref(); consoleHrefVal != "" {
		consoleHref, err = model.NewConsoleHref(consoleHrefVal)
		if err != nil {
			return fmt.Errorf("invalid console href: %w", err)
		}
	}

	var reporterVersion *model.ReporterVersion
	if reporterVersionValue := request.GetRepresentations().GetMetadata().GetReporterVersion(); reporterVersionValue != "" {
		rv, err := model.NewReporterVersion(reporterVersionValue)
		if err != nil {
			return fmt.Errorf("invalid reporter version: %w", err)
		}
		reporterVersion = &rv
	}

	reporterRepresentation, err := model.NewRepresentation(request.GetRepresentations().GetReporter().AsMap())
	if err != nil {
		return fmt.Errorf("invalid reporter representation: %w", err)
	}

	commonRepresentation, err := model.NewRepresentation(request.GetRepresentations().GetCommon().AsMap())
	if err != nil {
		return fmt.Errorf("invalid common representation: %w", err)
	}

	transactionId := model.NewTransactionId(request.GetRepresentations().GetMetadata().GetTransactionId())

	resource, err := model.NewResource(resourceId, localResourceId, resourceType, reporterType, reporterInstanceId, transactionId, reporterResourceId, apiHref, consoleHref, reporterRepresentation, commonRepresentation, reporterVersion)
	if err != nil {
		return err
	}

	return uc.resourceRepository.Save(tx, resource, biz.OperationTypeCreated, txidStr)
}

func (uc *Usecase) updateResource(tx *gorm.DB, request *v1beta2.ReportResourceRequest, existingResource *model.Resource, txidStr string) error {
	reporterResourceKey, apiHref, consoleHref, reporterVersion, commonData, reporterData, transactionId, err := extractUpdateDataFromRequest(request)
	if err != nil {
		return err
	}

	err = existingResource.Update(
		reporterResourceKey,
		apiHref,
		consoleHref,
		reporterVersion,
		reporterData,
		commonData,
		transactionId,
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
func getReporterResourceKeyFromRequest(request *v1beta2.ReportResourceRequest) (model.ReporterResourceKey, error) {
	localResourceId, err := model.NewLocalResourceId(request.GetRepresentations().GetMetadata().GetLocalResourceId())
	if err != nil {
		return model.ReporterResourceKey{}, fmt.Errorf("invalid local resource ID: %w", err)
	}

	resourceType, err := model.NewResourceType(request.GetType())
	if err != nil {
		return model.ReporterResourceKey{}, fmt.Errorf("invalid resource type: %w", err)
	}

	reporterType, err := model.NewReporterType(request.GetReporterType())
	if err != nil {
		return model.ReporterResourceKey{}, fmt.Errorf("invalid reporter type: %w", err)
	}

	reporterInstanceId, err := model.NewReporterInstanceId(request.GetReporterInstanceId())
	if err != nil {
		return model.ReporterResourceKey{}, fmt.Errorf("invalid reporter instance ID: %w", err)
	}

	return model.NewReporterResourceKey(
		localResourceId,
		resourceType,
		reporterType,
		reporterInstanceId,
	)
}

func extractUpdateDataFromRequest(request *v1beta2.ReportResourceRequest) (
	model.ReporterResourceKey,
	model.ApiHref,
	model.ConsoleHref,
	*model.ReporterVersion,
	model.Representation,
	model.Representation,
	model.TransactionId,
	error,
) {
	reporterResourceKey, err := getReporterResourceKeyFromRequest(request)
	if err != nil {
		return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), "", fmt.Errorf("failed to create reporter resource key: %w", err)
	}

	apiHref, err := model.NewApiHref(request.GetRepresentations().GetMetadata().GetApiHref())
	if err != nil {
		return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), "", fmt.Errorf("invalid API href: %w", err)
	}

	var consoleHref model.ConsoleHref
	if consoleHrefVal := request.GetRepresentations().GetMetadata().GetConsoleHref(); consoleHrefVal != "" {
		consoleHref, err = model.NewConsoleHref(consoleHrefVal)
		if err != nil {
			return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), "", fmt.Errorf("invalid console href: %w", err)
		}
	}

	var reporterVersion *model.ReporterVersion
	if reporterVersionValue := request.GetRepresentations().GetMetadata().GetReporterVersion(); reporterVersionValue != "" {
		rv, err := model.NewReporterVersion(reporterVersionValue)
		if err != nil {
			return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), "", fmt.Errorf("invalid reporter version: %w", err)
		}
		reporterVersion = &rv
	}

	commonRepresentation, err := model.NewRepresentation(request.GetRepresentations().GetCommon().AsMap())
	if err != nil {
		return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), "", fmt.Errorf("invalid common data: %w", err)
	}

	reporterRepresentation, err := model.NewRepresentation(request.GetRepresentations().GetReporter().AsMap())
	if err != nil {
		return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), "", fmt.Errorf("invalid reporter data: %w", err)
	}

	transactionId := model.NewTransactionId(request.GetRepresentations().GetMetadata().GetTransactionId())

	return reporterResourceKey, apiHref, consoleHref, reporterVersion, commonRepresentation, reporterRepresentation, transactionId, nil
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
	}
}

// Check if request comes from SP in allowlist
func isSPInAllowlist(reporterPrincipal string, allowlist []string) bool {
	for _, sp := range allowlist {
		// either specific SP or everyone
		if sp == reporterPrincipal || sp == "*" {
			return true
		}
	}

	return false
}

func computeReadAfterWrite(uc *Usecase, write_visibility v1beta2.WriteVisibility, reporterPrincipal string) bool {
	// read after write functionality is enabled/disabled globally.
	// And executed if request specifies and
	// came from service provider in allowlist
	if write_visibility == v1beta2.WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED || write_visibility == v1beta2.WriteVisibility_MINIMIZE_LATENCY {
		return false
	}
	return !common.IsNil(uc.ListenManager) && uc.Config.ReadAfterWriteEnabled && isSPInAllowlist(reporterPrincipal, uc.Config.ReadAfterWriteAllowlist)
}

func validateReportResourceRequest(ctx context.Context, request *v1beta2.ReportResourceRequest, schemaUseCase *SchemaUsecase) error {
	if request.Type == "" {
		return fmt.Errorf("missing 'type' field")
	}
	if request.ReporterType == "" {
		return fmt.Errorf("missing 'reporterType' field")
	}

	resourceType := request.Type
	reporterType := request.ReporterType

	if isReporter, err := schemaUseCase.IsReporterForResource(ctx, resourceType, reporterType); !isReporter {
		if err != nil {
			return err
		}

		return fmt.Errorf("reporter %s does not report resource types: %s", reporterType, resourceType)
	}

	if request.Representations == nil {
		return fmt.Errorf("missing 'representations'")
	}

	var reporterRepresentation map[string]any
	if request.Representations.Reporter != nil {
		reporterRepresentation = request.Representations.Reporter.AsMap()
	}

	// Remove any fields with null values before validation.
	var sanitizedReporterRepresentation map[string]interface{}
	if reporterRepresentation != nil {
		sanitizedReporterRepresentation = removeNulls(reporterRepresentation)
	} else {
		sanitizedReporterRepresentation = nil
	}

	if sanitizedReporterRepresentation != nil {
		sanitizedStruct, err2 := structpb.NewStruct(sanitizedReporterRepresentation)
		if err2 != nil {
			return fmt.Errorf("failed to rebuild reporter struct: %w", err2)
		}
		request.Representations.Reporter = sanitizedStruct
	}

	// Validate reporter-specific data using the sanitized map
	if err := schemaUseCase.ReporterShallowValidate(ctx, resourceType, reporterType, sanitizedReporterRepresentation); err != nil {
		return err
	}

	var commonRepresentation map[string]any
	if request.Representations.Common != nil {
		commonRepresentation = request.Representations.Common.AsMap()
	}

	// Validate common data
	if err := schemaUseCase.CommonShallowValidate(ctx, resourceType, commonRepresentation); err != nil {
		return err
	}

	return nil
}
