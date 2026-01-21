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
	"github.com/project-kessel/inventory-api/internal/authn/interceptor"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"github.com/project-kessel/inventory-api/internal/server"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/sony/gobreaker"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const (
	DeleteResourceOperationName = "DeleteResource"
	ReportResourceOperationName = "ReportResource"
)

var (
	// ErrResourceNotFound indicates that the requested resource could not be found in the database.
	ErrResourceNotFound = errors.New("resource not found")
	// ErrDatabaseError indicates a generic database error occurred while querying for resources.
	ErrDatabaseError = errors.New("db error while querying for resource")
	// ErrResourceAlreadyExists indicates that a resource with the same identifier already exists.
	ErrResourceAlreadyExists = errors.New("resource already exists")
	// ErrInventoryIdMismatch indicates that the inventory ID in the request doesn't match the existing resource.
	ErrInventoryIdMismatch = errors.New("resource inventory id mismatch")
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
	Eventer             eventingapi.Manager
	Namespace           string
	Log                 *log.Helper
	Server              server.Server
	ListenManager       pubsub.ListenManagerImpl
	Config              *UsecaseConfig
	MetricsCollector    *metricscollector.MetricsCollector
}

func New(resourceRepository data.ResourceRepository,
	authz authzapi.Authorizer, eventer eventingapi.Manager, namespace string, logger log.Logger,
	listenManager pubsub.ListenManagerImpl, waitForNotifBreaker *gobreaker.CircuitBreaker, usecaseConfig *UsecaseConfig, metricsCollector *metricscollector.MetricsCollector) *Usecase {
	return &Usecase{
		resourceRepository:  resourceRepository,
		waitForNotifBreaker: waitForNotifBreaker,
		Authz:               authz,
		Eventer:             eventer,
		Namespace:           namespace,
		Log:                 log.NewHelper(logger),
		ListenManager:       listenManager,
		Config:              usecaseConfig,
		MetricsCollector:    metricsCollector,
	}
}

func (uc *Usecase) ReportResource(ctx context.Context, request *v1beta2.ReportResourceRequest, reporterPrincipal string) error {
	clientID := interceptor.GetClientIDFromContext(ctx)
	log.Info("Reporting resource request: ", request, " client_id: ", clientID)
	var subscription pubsub.Subscription
	txidStr, err := getNextTransactionID()
	if err != nil {
		return err
	}

	readAfterWriteEnabled := computeReadAfterWrite(uc, request.WriteVisibility, reporterPrincipal)
	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		subscription = uc.ListenManager.Subscribe(txidStr)
		defer subscription.Unsubscribe()
	}

	reporterResourceKey, err := getReporterResourceKeyFromRequest(request)
	if err != nil {
		log.Error("failed to create reporter resource key: ", err)
		return status.Errorf(codes.InvalidArgument, "failed to create reporter resource key: %v", err)
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

func (uc *Usecase) Delete(ctx context.Context, reporterResourceKey model.ReporterResourceKey) error {
	txidStr, err := getNextTransactionID()
	if err != nil {
		return err
	}
	clientID := interceptor.GetClientIDFromContext(ctx)
	log.Info("Reporter Resource Key to delete ", reporterResourceKey, " client_id: ", clientID)
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

// Check verifies if a subject has the specified permission on a resource identified by the reporter resource ID.
func (uc *Usecase) Check(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, reporterResourceKey model.ReporterResourceKey, consistency model.ConsistencyConfig) (bool, error) {
	consistencyToken, err := uc.resolveConsistencyToken(ctx, consistency, reporterResourceKey)
	if err != nil {
		return false, err
	}

	allowed, _, err := uc.Authz.Check(ctx, namespace, permission, consistencyToken, reporterResourceKey.ResourceType().Serialize(), reporterResourceKey.LocalResourceId().Serialize(), sub)
	if err != nil {
		return false, err
	}

	if allowed == kessel.CheckResponse_ALLOWED_TRUE {
		return true, nil
	}
	return false, nil
}

// CheckForUpdate forwards the request to Relations CheckForUpdate
func (uc *Usecase) CheckForUpdate(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, reporterResourceKey model.ReporterResourceKey) (bool, error) {
	allowed, _, err := uc.Authz.CheckForUpdate(ctx, namespace, permission, reporterResourceKey.ResourceType().Serialize(), reporterResourceKey.LocalResourceId().Serialize(), sub)
	if err != nil {
		return false, err
	}

	if allowed == kessel.CheckForUpdateResponse_ALLOWED_TRUE {
		return true, nil
	}
	return false, nil
}

// resolveConsistencyToken resolves the consistency token based on the preference.
func (uc *Usecase) resolveConsistencyToken(ctx context.Context, consistency model.ConsistencyConfig, reporterResourceKey model.ReporterResourceKey) (string, error) {
	// Feature flag: when true, force minimize_latency regardless of client request
	// When false, use client-specified consistency (existing logic)
	if viper.GetBool("authz.kessel.force-inventory-default-consistency-minimize-latency") {
		log.Info("Feature flag authz.kessel.force-inventory-default-consistency-minimize-latency is enabled")
		// Feature flag is true - use client-specified consistency (existing logic below)
		switch consistency.Preference {
		case model.ConsistencyMinimizeLatency:
			// No token needed - minimize_latency mode
			log.Info("Using minimize_latency consistency")
			return "", nil

		case model.ConsistencyAtLeastAsFresh:
			// Use the token provided by the caller
			log.Infof("Using at_least_as_fresh consistency with provided token: %s", consistency.Token)
			return consistency.Token, nil

		case model.ConsistencyAtLeastAsAcknowledged:
			// Look up the token from inventory database
			log.Info("Using at_least_as_acknowledged consistency - looking up token from DB")
			res, err := uc.resourceRepository.FindResourceByKeys(nil, reporterResourceKey)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// Resource doesn't exist in inventory, fall back to minimize_latency
					log.Info("Resource not found in inventory, falling back to minimize_latency")
					return "", nil
				}
				return "", err
			}
			token := res.ConsistencyToken().Serialize()
			log.Infof("Found inventory-managed consistency token: %s", token)
			return token, nil

		default:
			// Default to minimize_latency
			log.Info("Unknown consistency preference, defaulting to minimize_latency")
			return "", nil
		}

	} else {
		// Feature flag is false - always look up token from inventory DB
		log.Info("Feature flag disabled - using inventory-managed consistency lookup")
		res, err := uc.resourceRepository.FindResourceByKeys(nil, reporterResourceKey)
		if err != nil {
			log.Info("Did not find resource")
			// If the resource doesn't exist in inventory (ie. no consistency token available)
			// we send a check request with minimize latency
			// err otherwise.
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return "", err
			}
			return "", nil
		}
		return res.ConsistencyToken().Serialize(), nil
	}
}

// CheckBulk forwards the request to Relations CheckBulk
func (uc *Usecase) CheckBulk(ctx context.Context, req *kessel.CheckBulkRequest) (*kessel.CheckBulkResponse, error) {
	resp, err := uc.Authz.CheckBulk(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
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

// LookupResources delegates resource lookup to the authorization service.
func (uc *Usecase) LookupResources(ctx context.Context, request *kessel.LookupResourcesRequest) (grpc.ServerStreamingClient[kessel.LookupResourcesResponse], error) {
	return uc.Authz.LookupResources(ctx, request)
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
