package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/authn/interceptor"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/server"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
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
	// ErrInvalidReporterForResource indicates the reporter type is not valid for the given resource type.
	ErrInvalidReporterForResource = errors.New("invalid reporter for resource type")
	// ErrSchemaValidationFailed indicates the resource data failed schema validation.
	ErrSchemaValidationFailed = errors.New("schema validation failed")
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
// It coordinates between the Store, authorization, eventing, and other system components.
type Usecase struct {
	store               model.Store
	clusterBroadcast    model.ClusterBroadcast
	schemaService       *model.SchemaService
	waitForNotifBreaker *gobreaker.CircuitBreaker
	Authz               authzapi.Authorizer
	Eventer             eventingapi.Manager
	Namespace           string
	Log                 *log.Helper
	Server              server.Server
	Config              *UsecaseConfig
	MetricsCollector    *metricscollector.MetricsCollector
}

func New(store model.Store, clusterBroadcast model.ClusterBroadcast,
	schemaRepository model.SchemaRepository, authz authzapi.Authorizer, eventer eventingapi.Manager, namespace string, logger log.Logger,
	waitForNotifBreaker *gobreaker.CircuitBreaker, usecaseConfig *UsecaseConfig, metricsCollector *metricscollector.MetricsCollector) *Usecase {
	return &Usecase{
		store:               store,
		clusterBroadcast:    clusterBroadcast,
		schemaService:       model.NewSchemaService(schemaRepository, log.NewHelper(logger)),
		waitForNotifBreaker: waitForNotifBreaker,
		Authz:               authz,
		Eventer:             eventer,
		Namespace:           namespace,
		Log:                 log.NewHelper(logger),
		Config:              usecaseConfig,
		MetricsCollector:    metricsCollector,
	}
}

// ReportResource creates or updates a resource based on the provided command.
// The reporterPrincipal is used for authorization checks and read-after-write allowlist validation.
func (uc *Usecase) ReportResource(ctx context.Context, cmd model.ReportResourceCommand, reporterPrincipal string) error {
	clientID := interceptor.GetClientIDFromContext(ctx)
	log.Infof("Reporting resource: key=%v, client_id=%s", cmd.Key(), clientID)

	// Validate the reporter is allowed for this resource type
	if err := uc.validateSchema(ctx, cmd); err != nil {
		return err
	}

	txidStr, err := getNextTransactionID()
	if err != nil {
		return err
	}

	readAfterWriteEnabled := uc.computeReadAfterWrite(cmd.WantsCommitPending(), reporterPrincipal)
	reporterResourceKey := cmd.Key()

	// Begin transaction
	tx, err := uc.store.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Safe to call after Commit

	repo := tx.ResourceRepository()

	// Check for duplicate transaction ID's before we find the resource for quicker returns if it fails
	transactionId := cmd.TransactionId().String()
	if transactionId != "" {
		alreadyProcessed, err := repo.ContainsEventForTransactionId(transactionId)
		if err != nil {
			return fmt.Errorf("failed to check transaction ID: %w", err)
		}
		if alreadyProcessed {
			log.Info("Transaction already processed, skipping update")
			return nil
		}
	}

	res, err := repo.FindResourceByKeys(reporterResourceKey)
	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("failed to lookup existing resource: %w", err)
	}

	var operationType biz.EventOperationType
	if err == nil && res != nil {
		log.Info("Resource already exists, updating")
		operationType = biz.OperationTypeUpdated
		if err := uc.updateResource(repo, cmd, res, txidStr); err != nil {
			return err
		}
	} else {
		log.Info("Creating new resource")
		operationType = biz.OperationTypeCreated
		if err := uc.createResource(repo, cmd, txidStr); err != nil {
			return err
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Increment outbox metrics only after successful transaction commit
	if operationType != nil {
		metricscollector.Incr(uc.MetricsCollector.OutboxEventWrites, string(operationType.OperationType()))
	}

	// Wait for replication if read-after-write is enabled
	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		uc.waitForReplication(ctx, txidStr)
	}

	return nil
}

// waitForReplication waits for the replication to complete, using circuit breaker.
func (uc *Usecase) waitForReplication(ctx context.Context, txid string) {
	if uc.clusterBroadcast == nil {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, listenTimeout)
	defer cancel()

	_, err := uc.waitForNotifBreaker.Execute(func() (interface{}, error) {
		return nil, uc.clusterBroadcast.Wait(timeoutCtx, model.SignalKey(txid))
	})

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			uc.Log.WithContext(ctx).Debugf("Reached timeout waiting for replication")
		case errors.Is(err, gobreaker.ErrOpenState):
			uc.Log.WithContext(ctx).Debugf("Circuit breaker is open, skipped waiting for replication")
		case errors.Is(err, gobreaker.ErrTooManyRequests):
			uc.Log.WithContext(ctx).Debugf("Circuit breaker is half-open, skipped waiting for replication")
		default:
			uc.Log.WithContext(ctx).Warnf("Error waiting for replication: %v", err)
		}
	}
}

// isNotFoundError checks if the error indicates a resource was not found.
func isNotFoundError(err error) bool {
	return errors.Is(err, ErrResourceNotFound)
}

// validateSchema validates the resource command against the schema.
// It checks that the reporter is valid for the resource type and validates
// both the common and reporter representations against their schemas.
func (uc *Usecase) validateSchema(ctx context.Context, cmd model.ReportResourceCommand) error {
	resourceType := cmd.Key().ResourceType().String()
	reporterType := cmd.Key().ReporterType().String()

	// Check if the reporter is valid for this resource type
	isValid, err := uc.schemaService.IsReporterForResource(ctx, resourceType, reporterType)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSchemaValidationFailed, err)
	}
	if !isValid {
		return fmt.Errorf("%w: reporter '%s' is not valid for resource type '%s'", ErrInvalidReporterForResource, reporterType, resourceType)
	}

	// Validate common representation against schema
	if err := uc.schemaService.CommonShallowValidate(ctx, resourceType, cmd.CommonRepresentation().Data()); err != nil {
		return fmt.Errorf("%w: common representation: %v", ErrSchemaValidationFailed, err)
	}

	// Validate reporter representation against schema
	if err := uc.schemaService.ReporterShallowValidate(ctx, resourceType, reporterType, cmd.ReporterRepresentation().Data()); err != nil {
		return fmt.Errorf("%w: reporter representation: %v", ErrSchemaValidationFailed, err)
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

	// Begin transaction
	tx, err := uc.store.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Safe to call after Commit

	repo := tx.ResourceRepository()
	res, err := repo.FindResourceByKeys(reporterResourceKey)

	if err != nil {
		if isNotFoundError(err) {
			return ErrResourceNotFound
		}
		return ErrDatabaseError
	}

	if res == nil {
		return ErrResourceNotFound
	}

	log.Info("Found Resource, deleting: ", res)
	if err := res.Delete(reporterResourceKey); err != nil {
		return fmt.Errorf("failed to delete resource: %w", err)
	}

	if err := repo.Save(*res, biz.OperationTypeDeleted, txidStr); err != nil {
		return fmt.Errorf("failed to save deleted resource: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Increment outbox metrics only after successful transaction commit
	metricscollector.Incr(uc.MetricsCollector.OutboxEventWrites, string(biz.OperationTypeDeleted.OperationType()))
	return nil
}

// Check verifies if a subject has the specified permission on a resource identified by the reporter resource ID.
func (uc *Usecase) Check(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, reporterResourceKey model.ReporterResourceKey) (bool, error) {
	// Use a read-only transaction to find the resource
	tx, err := uc.store.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ResourceRepository().FindResourceByKeys(reporterResourceKey)
	var consistencyToken string
	if err != nil {
		log.Info("Did not find resource")
		// If the resource doesn't exist in inventory (ie. no consistency token available)
		// we send a check request with minimize latency
		// err otherwise.
		if !isNotFoundError(err) {
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

func (uc *Usecase) createResource(repo model.ResourceRepository, cmd model.ReportResourceCommand, txidStr string) error {
	resourceId, err := repo.NextResourceId()
	if err != nil {
		return err
	}

	reporterResourceId, err := repo.NextReporterResourceId()
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

	return repo.Save(resource, biz.OperationTypeCreated, txidStr)
}

func (uc *Usecase) updateResource(repo model.ResourceRepository, cmd model.ReportResourceCommand, existingResource *model.Resource, txidStr string) error {
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

	return repo.Save(*existingResource, biz.OperationTypeUpdated, txidStr)
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
	return uc.store != nil && uc.Config.ReadAfterWriteEnabled && isSPInAllowlist(reporterPrincipal, uc.Config.ReadAfterWriteAllowlist)
}
