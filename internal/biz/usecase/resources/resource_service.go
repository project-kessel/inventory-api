package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/authn/interceptor"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/schema"
	"github.com/project-kessel/inventory-api/internal/data"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"github.com/project-kessel/inventory-api/internal/server"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
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
	schemaUsecase       *SchemaUsecase
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
	schemaRepository schema.Repository, authz authzapi.Authorizer, eventer eventingapi.Manager, namespace string, logger log.Logger,
	listenManager pubsub.ListenManagerImpl, waitForNotifBreaker *gobreaker.CircuitBreaker, usecaseConfig *UsecaseConfig, metricsCollector *metricscollector.MetricsCollector) *Usecase {
	return &Usecase{
		resourceRepository:  resourceRepository,
		schemaUsecase:       NewSchemaUsecase(schemaRepository, log.NewHelper(logger)),
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

// ReportResource creates or updates a resource based on the provided command.
// The reporterPrincipal is used for authorization checks and read-after-write allowlist validation.
func (uc *Usecase) ReportResource(ctx context.Context, cmd model.ReportResourceCommand, reporterPrincipal string) error {
	clientID := interceptor.GetClientIDFromContext(ctx)
	log.Infof("Reporting resource: key=%v, client_id=%s", cmd.Key(), clientID)

	var subscription pubsub.Subscription
	txidStr, err := getNextTransactionID()
	if err != nil {
		return err
	}

	readAfterWriteEnabled := uc.computeReadAfterWrite(cmd.WantsCommitPending(), reporterPrincipal)
	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		subscription = uc.ListenManager.Subscribe(txidStr)
		defer subscription.Unsubscribe()
	}

	reporterResourceKey := cmd.Key()

	var operationType biz.EventOperationType
	err = uc.resourceRepository.GetTransactionManager().HandleSerializableTransaction(
		ReportResourceOperationName,
		uc.resourceRepository.GetDB(),
		func(tx *gorm.DB) error {
			// Check for duplicate transaction ID's before we find the resource for quicker returns if it fails
			transactionId := cmd.TransactionId().String()
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
				log.Info("Resource already exists, updating")
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
func (uc *Usecase) Check(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, reporterResourceKey model.ReporterResourceKey) (bool, error) {
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

// CheckBulk forwards the request to Relations CheckBulk
func (uc *Usecase) CheckBulk(ctx context.Context, req *kessel.CheckBulkRequest) (*kessel.CheckBulkResponse, error) {
	resp, err := uc.Authz.CheckBulk(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (uc *Usecase) createResource(tx *gorm.DB, cmd model.ReportResourceCommand, txidStr string) error {
	resourceId, err := uc.resourceRepository.NextResourceId()
	if err != nil {
		return err
	}

	reporterResourceId, err := uc.resourceRepository.NextReporterResourceId()
	if err != nil {
		return err
	}

	key := cmd.Key()
	resource, err := model.NewResource(
		resourceId,
		key.LocalResourceId(),
		key.ResourceType(),
		key.ReporterType(),
		key.ReporterInstanceId(),
		cmd.TransactionId(),
		reporterResourceId,
		cmd.ApiHref(),
		cmd.ConsoleHref(),
		cmd.ReporterRepresentation(),
		cmd.CommonRepresentation(),
		cmd.ReporterVersion(),
	)
	if err != nil {
		return err
	}

	return uc.resourceRepository.Save(tx, resource, biz.OperationTypeCreated, txidStr)
}

func (uc *Usecase) updateResource(tx *gorm.DB, cmd model.ReportResourceCommand, existingResource *model.Resource, txidStr string) error {
	err := existingResource.Update(
		cmd.Key(),
		cmd.ApiHref(),
		cmd.ConsoleHref(),
		cmd.ReporterVersion(),
		cmd.ReporterRepresentation(),
		cmd.CommonRepresentation(),
		cmd.TransactionId(),
	)
	if err != nil {
		return fmt.Errorf("failed to update resource: %w", err)
	}

	return uc.resourceRepository.Save(tx, *existingResource, biz.OperationTypeUpdated, txidStr)
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

// isSPInAllowlist checks if the reporter principal is in the allowlist for read-after-write.
func isSPInAllowlist(reporterPrincipal string, allowlist []string) bool {
	for _, sp := range allowlist {
		// either specific SP or everyone
		if sp == reporterPrincipal || sp == "*" {
			return true
		}
	}
	return false
}

// computeReadAfterWrite determines if read-after-write should be enabled for this request.
func (uc *Usecase) computeReadAfterWrite(wantsCommitPending bool, reporterPrincipal string) bool {
	// Read-after-write functionality is enabled/disabled globally.
	// It's executed if the request specifies commit-pending visibility
	// and came from a service provider in the allowlist.
	if !wantsCommitPending {
		return false
	}
	return !common.IsNil(uc.ListenManager) && uc.Config.ReadAfterWriteEnabled && isSPInAllowlist(reporterPrincipal, uc.Config.ReadAfterWriteAllowlist)
}
