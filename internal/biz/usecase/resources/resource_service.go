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
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/data"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"github.com/project-kessel/inventory-api/internal/server"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"sync"
)

// ReporterResourceRepository defines the interface for managing resources in the inventory system.
// It provides CRUD operations and various query methods for resources.
type ReporterResourceRepository interface {
	Create(context.Context, *model_legacy.Resource, string, string) (*model_legacy.Resource, error)
	Update(context.Context, *model_legacy.Resource, uuid.UUID, string, string) (*model_legacy.Resource, error)
	Delete(context.Context, uuid.UUID, string) (*model_legacy.Resource, error)
	FindByID(context.Context, uuid.UUID) (*model_legacy.Resource, error)
	FindByWorkspaceId(context.Context, string) ([]*model_legacy.Resource, error)
	FindByReporterResourceId(context.Context, model_legacy.ReporterResourceId) (*model_legacy.Resource, error)
	FindByReporterResourceIdv1beta2(context.Context, model_legacy.ReporterResourceUniqueIndex) (*model_legacy.Resource, error)
	FindByReporterData(context.Context, string, string) (*model_legacy.Resource, error)
	FindByInventoryIdAndResourceType(ctx context.Context, inventoryId *uuid.UUID, resourceType string) (*model_legacy.Resource, error)
	FindByInventoryIdAndReporter(ctx context.Context, inventoryId *uuid.UUID, reporterInstanceId string, reporterType string) (*model_legacy.Resource, error)
	ListAll(context.Context) ([]*model_legacy.Resource, error)
}

// InventoryResourceRepository defines the interface for accessing inventory resource data.
type InventoryResourceRepository interface {
	FindByID(context.Context, uuid.UUID) (*model_legacy.InventoryResource, error)
}

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
	resourceRepository               data.ResourceRepository
	LegacyReporterResourceRepository ReporterResourceRepository
	inventoryResourceRepository      InventoryResourceRepository
	waitForNotifBreaker              *gobreaker.CircuitBreaker
	Authz                            authzapi.Authorizer
	Eventer                          eventingapi.Manager
	Namespace                        string
	Log                              *log.Helper
	Server                           server.Server
	ListenManager                    pubsub.ListenManagerImpl
	Config                           *UsecaseConfig
}

// NewUsecase creates a new Usecase with minimal required dependencies for consumer usage
func NewUsecase(db *gorm.DB, logger *log.Helper) *Usecase {
	transactionManager := data.NewGormTransactionManager(3) // Default retry count
	resourceRepo := data.NewResourceRepository(db, transactionManager)

	return &Usecase{
		resourceRepository: resourceRepo,
		Log:                logger,
	}
}

func New(resourceRepository data.ResourceRepository, reporterResourceRepository ReporterResourceRepository, inventoryResourceRepository InventoryResourceRepository,
	authz authzapi.Authorizer, eventer eventingapi.Manager, namespace string, logger log.Logger,
	listenManager pubsub.ListenManagerImpl, waitForNotifBreaker *gobreaker.CircuitBreaker, usecaseConfig *UsecaseConfig) *Usecase {
	return &Usecase{
		resourceRepository:               resourceRepository,
		LegacyReporterResourceRepository: reporterResourceRepository,
		inventoryResourceRepository:      inventoryResourceRepository,
		waitForNotifBreaker:              waitForNotifBreaker,
		Authz:                            authz,
		Eventer:                          eventer,
		Namespace:                        namespace,
		Log:                              log.NewHelper(logger),
		ListenManager:                    listenManager,
		Config:                           usecaseConfig,
	}
}

func (uc *Usecase) ReportResource(ctx context.Context, request *v1beta2.ReportResourceRequest, reporterPrincipal string) error {
	log.Info("Reporting resource request: ", request)
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
		return fmt.Errorf("failed to create reporter resource key: %w", err)
	}

	err = uc.resourceRepository.GetTransactionManager().HandleSerializableTransaction(
		uc.resourceRepository.GetDB(),
		func(tx *gorm.DB) error {
			res, err := uc.resourceRepository.FindResourceByKeys(tx, reporterResourceKey)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("failed to lookup existing resource: %w", err)
			}

			if err == nil && res != nil {
				log.Info("Resource already exists, updating: ")
				return uc.updateResource(tx, request, res, txidStr)
			}

			log.Info("Creating new resource")
			return uc.createResource(tx, request, txidStr)
		},
	)

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
	if err != nil {
		return err
	}
	return nil
}

func (uc *Usecase) Delete(reporterResourceKey model.ReporterResourceKey) error {
	txidStr, err := getNextTransactionID()
	if err != nil {
		return err
	}

	log.Info("Reporter Resource Key to delete ", reporterResourceKey)
	err = uc.resourceRepository.GetTransactionManager().HandleSerializableTransaction(
		uc.resourceRepository.GetDB(),
		func(tx *gorm.DB) error {
			res, err := uc.resourceRepository.FindResourceByKeys(tx, reporterResourceKey)

			if err == nil && res != nil {
				log.Info("Found Resource, deleting: ", res)
				err := res.Delete(reporterResourceKey)
				if err != nil {
					return fmt.Errorf("failed to delete resource: %w", err)
				}
				return uc.resourceRepository.Save(tx, *res, model_legacy.OperationTypeDeleted, txidStr)
			} else {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrResourceNotFound
				}
				return ErrDatabaseError
			}
		},
	)
	return err
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

	reporterRepresentation, err := model.NewRepresentation(request.GetRepresentations().GetReporter().AsMap())
	if err != nil {
		return fmt.Errorf("invalid reporter representation: %w", err)
	}

	commonRepresentation, err := model.NewRepresentation(request.GetRepresentations().GetCommon().AsMap())
	if err != nil {
		return fmt.Errorf("invalid common representation: %w", err)
	}

	resource, err := model.NewResource(resourceId, localResourceId, resourceType, reporterType, reporterInstanceId, reporterResourceId, apiHref, consoleHref, reporterRepresentation, commonRepresentation, nil)
	if err != nil {
		return err
	}

	return uc.resourceRepository.Save(tx, resource, model_legacy.OperationTypeCreated, txidStr)
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
	reporterResourceKey, apiHref, consoleHref, reporterVersion, commonData, reporterData, err := extractUpdateDataFromRequest(request)
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
	)
	if err != nil {
		return fmt.Errorf("failed to update resource: %w", err)
	}

	return uc.resourceRepository.Save(tx, *existingResource, model_legacy.OperationTypeUpdated, txidStr)
}

func extractUpdateDataFromRequest(request *v1beta2.ReportResourceRequest) (
	model.ReporterResourceKey,
	model.ApiHref,
	model.ConsoleHref,
	*model.ReporterVersion,
	model.Representation,
	model.Representation,
	error,
) {
	reporterResourceKey, err := getReporterResourceKeyFromRequest(request)
	if err != nil {
		return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), fmt.Errorf("failed to create reporter resource key: %w", err)
	}

	apiHref, err := model.NewApiHref(request.GetRepresentations().GetMetadata().GetApiHref())
	if err != nil {
		return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), fmt.Errorf("invalid API href: %w", err)
	}

	var consoleHref model.ConsoleHref
	if consoleHrefVal := request.GetRepresentations().GetMetadata().GetConsoleHref(); consoleHrefVal != "" {
		consoleHref, err = model.NewConsoleHref(consoleHrefVal)
		if err != nil {
			return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), fmt.Errorf("invalid console href: %w", err)
		}
	}

	var reporterVersion *model.ReporterVersion
	if reporterVersionValue := request.GetRepresentations().GetMetadata().GetReporterVersion(); reporterVersionValue != "" {
		rv, err := model.NewReporterVersion(reporterVersionValue)
		if err != nil {
			return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), fmt.Errorf("invalid reporter version: %w", err)
		}
		reporterVersion = &rv
	}

	commonRepresentation, err := model.NewRepresentation(request.GetRepresentations().GetCommon().AsMap())
	if err != nil {
		return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), fmt.Errorf("invalid common data: %w", err)
	}

	reporterRepresentation, err := model.NewRepresentation(request.GetRepresentations().GetReporter().AsMap())
	if err != nil {
		return model.ReporterResourceKey{}, "", "", nil, model.Representation(nil), model.Representation(nil), fmt.Errorf("invalid reporter data: %w", err)
	}

	return reporterResourceKey, apiHref, consoleHref, reporterVersion, commonRepresentation, reporterRepresentation, nil
}

func getNextTransactionID() (string, error) {
	txid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return txid.String(), nil
}

// Upsert creates a new resource or updates an existing one based on the reporter resource ID.
// It supports read-after-write consistency when enabled and handles notification waiting.
func (uc *Usecase) Upsert(ctx context.Context, m *model_legacy.Resource, write_visibility v1beta2.WriteVisibility) (*model_legacy.Resource, error) {
	log.Info("upserting resource: ", m)
	var ret *model_legacy.Resource
	var subscription pubsub.Subscription
	var txidStr string

	// check if the resource already exists
	existingResource, err := uc.LegacyReporterResourceRepository.FindByReporterResourceIdv1beta2(ctx, model_legacy.ReporterResourceIdv1beta2FromResource(m))

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDatabaseError
	}

	readAfterWriteEnabled := computeReadAfterWriteLegacy(uc, write_visibility, m)
	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		// Generate txid for data layer
		// TODO: Replace this when inventory api has proper api-level transaction ids
		txid, err := uuid.NewV7()
		if err != nil {
			return nil, err
		}
		txidStr = txid.String()
		subscription = uc.ListenManager.Subscribe(txidStr)
		defer subscription.Unsubscribe()
	}

	log.Info("found existing resource: ", existingResource)
	if existingResource != nil {
		return updateExistingReporterResource(ctx, m, existingResource, uc, txidStr)
	}

	//TODO: Bug here that needs to be fixed : https://issues.redhat.com/browse/RHCLOUD-39044
	if m.InventoryId != nil {
		err2 := validateSameResourceFromMultipleReportersShareInventoryId(ctx, m, uc)
		if err2 != nil {
			return nil, err2
		}
	}

	log.Info("Creating resource: ", m)
	ret, err = createNewReporterResource(ctx, m, uc, txidStr)
	if err != nil {
		return ret, err
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
				return ret, nil
			case errors.Is(err, gobreaker.ErrOpenState):
				uc.Log.WithContext(ctx).Debugf("Circuit breaker is open, skipped waiting for notification from consumer")
				return ret, nil
			case errors.Is(err, gobreaker.ErrTooManyRequests):
				uc.Log.WithContext(ctx).Debugf("Circuit breaker is half-open, skipped waiting for notification from consumer")
				return ret, nil
			default:
				return nil, err
			}
		}
	}

	uc.Log.WithContext(ctx).Infof("Upserted Resource: %v(%v)", ret.ID, ret.ResourceType)
	return ret, nil
}

func createNewReporterResource(ctx context.Context, m *model_legacy.Resource, uc *Usecase, txid string) (*model_legacy.Resource, error) {
	ret, err := uc.LegacyReporterResourceRepository.Create(ctx, m, uc.Namespace, txid)

	if err != nil {
		return nil, err
	}

	return ret, nil
}

func validateSameResourceFromMultipleReportersShareInventoryId(ctx context.Context, m *model_legacy.Resource, uc *Usecase) error {
	// Multiple reporters should have same inventory id.
	existingInventoryIdResource, err := uc.LegacyReporterResourceRepository.FindByInventoryIdAndResourceType(ctx, m.InventoryId, m.ResourceType)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrDatabaseError
	}

	if existingInventoryIdResource != nil {
		existingResourceRepo, err := uc.LegacyReporterResourceRepository.FindByInventoryIdAndReporter(ctx, m.InventoryId, m.ReporterInstanceId, m.ReporterType)
		if existingResourceRepo != nil {
			return ErrResourceAlreadyExists
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return nil
}

func updateExistingReporterResource(ctx context.Context, m *model_legacy.Resource, existingResource *model_legacy.Resource, uc *Usecase, txid string) (*model_legacy.Resource, error) {

	if m.InventoryId != nil && existingResource.InventoryId.String() != m.InventoryId.String() {
		return nil, ErrInventoryIdMismatch
	}
	log.Info("Updating resource: ", m)
	ret, err := uc.LegacyReporterResourceRepository.Update(ctx, m, existingResource.ID, uc.Namespace, txid)
	if err != nil {
		return nil, err
	}

	uc.Log.WithContext(ctx).Infof("Updated Resource: %v(%v)", m.ID, m.ResourceType)
	return ret, nil
}

// LookupResources delegates resource lookup to the authorization service.
func (uc *Usecase) LookupResources(ctx context.Context, request *kessel.LookupResourcesRequest) (grpc.ServerStreamingClient[kessel.LookupResourcesResponse], error) {
	return uc.Authz.LookupResources(ctx, request)
}

// CheckLegacy verifies if a subject has the specified permission on a resource identified by the reporter resource ID.
func (uc *Usecase) CheckLegacy(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, id model_legacy.ReporterResourceId) (bool, error) {
	res, err := uc.LegacyReporterResourceRepository.FindByReporterResourceId(ctx, id)
	if err != nil {
		// If the resource doesn't exist in inventory (ie. no consistency token available)
		// we send a check request with minimize latency
		// err otherwise.
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, err
		}
		res = &model_legacy.Resource{ResourceType: id.ResourceType, ReporterResourceId: id.LocalResourceId}
	}

	allowed, _, err := uc.Authz.Check(ctx, namespace, permission, res.ConsistencyToken, res.ResourceType, res.ReporterResourceId, sub)
	if err != nil {
		return false, err
	}

	if allowed == kessel.CheckResponse_ALLOWED_TRUE {
		return true, nil
	}
	return false, nil
}

// CheckForUpdateLegacy verifies if a subject has the specified permission to update a resource,
// and records the consistency token if the check passes and the resource exists.
func (uc *Usecase) CheckForUpdateLegacy(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, id model_legacy.ReporterResourceId) (bool, error) {
	res, err := uc.LegacyReporterResourceRepository.FindByReporterResourceId(ctx, id)
	recordToken := true
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// resource doesn't exist yet.
			// DONT write consistency token
			// no actual resource exists in DB to update
			recordToken = false
			res = &model_legacy.Resource{ResourceType: id.ResourceType, ReporterResourceId: id.LocalResourceId}
		} else {
			return false, err
		}
	}

	allowed, consistency, err := uc.Authz.CheckForUpdate(ctx, namespace, permission, res.ResourceType, res.ReporterResourceId, sub)
	if err != nil {
		return false, err
	}

	if allowed == kessel.CheckForUpdateResponse_ALLOWED_TRUE {
		if id.ResourceType == "workspace" && namespace == "rbac" { //TODO: delete this when workspaces are resources
			return true, nil
		}

		// Only update consistency token if resource exists in DB.
		if recordToken && consistency != nil {
			res.ConsistencyToken = consistency.Token
			_, err := uc.LegacyReporterResourceRepository.Update(ctx, res, res.ID, uc.Namespace, "")
			if err != nil {
				return false, err // we're allowed, but failed to update consistency token
			}
		}

		return true, nil
	}

	return false, nil
}

// ListResourcesInWorkspace retrieves all resources in a workspace and filters them based on authorization.
// It uses worker goroutines to perform authorization checks concurrently and returns channels for results and errors.
func (uc *Usecase) ListResourcesInWorkspace(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, id string) (chan *model_legacy.Resource, chan error, error) {
	resources, err := uc.LegacyReporterResourceRepository.FindByWorkspaceId(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	log.Infof("ListResourcesInWorkspace: resources %+v", resources)

	const NUM_WORKERS = 100

	resourceChan := make(chan *model_legacy.Resource, len(resources))
	allowedChan := make(chan *model_legacy.Resource, len(resources))
	errorChan := make(chan error, 1)
	var wg sync.WaitGroup

	// start workers
	for i := 0; i < NUM_WORKERS; i++ {
		go uc.checkWorker(ctx, permission, namespace, sub, resourceChan, allowedChan, errorChan, &wg)
	}

	wg.Add(len(resources))
	go func() {
		defer close(allowedChan)
		defer close(errorChan)

		// feed workers with resources to check
		for _, res := range resources {
			resourceChan <- res
		}
		close(resourceChan)

		// wait for workers to finish processing resources before close output channels
		wg.Wait()
	}()

	return allowedChan, errorChan, nil
}

func (uc *Usecase) checkWorker(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, resourceChan <-chan *model_legacy.Resource, allowedChan chan<- *model_legacy.Resource, errorChan chan<- error, wg *sync.WaitGroup) {
	for resource := range resourceChan {
		log.Debugf("ListResourcesInWorkspace: checkforview on %+v", resource)

		if allowed, _, err := uc.Authz.Check(ctx, namespace, permission, resource.ConsistencyToken, resource.ResourceType, resource.ReporterResourceId, sub); err == nil && allowed == kessel.CheckResponse_ALLOWED_TRUE {
			allowedChan <- resource
		} else if err != nil {
			errorChan <- err
		} else if allowed != kessel.CheckResponse_ALLOWED_TRUE {
			log.Debugf("ListResourcesInWorkspace: response was not allowed: %v", allowed)
		}
		wg.Done()
	}
}

// Create creates a new resource in the database and waits for consumer notification if configured.
//
// Deprecated: Remove after notifications and ACM migrates to v1beta2.
func (uc *Usecase) Create(ctx context.Context, m *model_legacy.Resource) (*model_legacy.Resource, error) {
	var ret *model_legacy.Resource
	var subscription pubsub.Subscription
	var txidStr string

	// check if the resource already exists
	existingResource, err := uc.LegacyReporterResourceRepository.FindByReporterData(ctx, m.ReporterId, m.ReporterResourceId)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		// Deprecated: fallback case for backwards compatibility
		existingResource, err = uc.LegacyReporterResourceRepository.FindByReporterResourceId(ctx, model_legacy.ReporterResourceIdFromResource(m))
	}

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDatabaseError
	}

	if existingResource != nil {
		return nil, ErrResourceAlreadyExists
	}

	if !common.IsNil(uc.ListenManager) && uc.Config.ConsumerEnabled {
		// Generate txid for data layer
		// TODO: Replace this when inventory api has proper api-level transaction ids
		txid, err := uuid.NewV7()
		if err != nil {
			return nil, err
		}
		txidStr = txid.String()
		subscription = uc.ListenManager.Subscribe(txidStr)
		defer subscription.Unsubscribe()
	}

	ret, err = uc.LegacyReporterResourceRepository.Create(ctx, m, uc.Namespace, txidStr)
	if err != nil {
		return nil, err
	}

	if !common.IsNil(uc.ListenManager) && uc.Config.ConsumerEnabled {
		timeoutCtx, cancel := context.WithTimeout(ctx, listenTimeout)
		defer cancel()

		err = subscription.BlockForNotification(timeoutCtx)
		if err != nil {
			if errors.Is(err, pubsub.ErrWaitContextCancelled) {
				return ret, nil
			}
			return nil, err
		}
	}

	uc.Log.WithContext(ctx).Infof("Created Resource: %v(%v)", ret.ID, ret.ResourceType)
	return ret, nil
}

// Update updates an existing resource in the database, or creates it if it doesn't exist.
// It waits for consumer notification if configured.
//
// Deprecated: Remove after notifications and ACM migrates to v1beta2.
func (uc *Usecase) Update(ctx context.Context, m *model_legacy.Resource, id model_legacy.ReporterResourceId) (*model_legacy.Resource, error) {
	var ret *model_legacy.Resource
	var subscription pubsub.Subscription
	var txidStr string

	// check if the resource exists
	existingResource, err := uc.LegacyReporterResourceRepository.FindByReporterData(ctx, m.ReporterId, m.ReporterResourceId)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		// Deprecated: fallback case for backwards compatibility
		existingResource, err = uc.LegacyReporterResourceRepository.FindByReporterResourceId(ctx, model_legacy.ReporterResourceIdFromResource(m))
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uc.Create(ctx, m)
		}

		return nil, ErrDatabaseError
	}

	if !common.IsNil(uc.ListenManager) && uc.Config.ConsumerEnabled {
		// Generate txid for data layer
		// TODO: Replace this when inventory api has proper api-level transaction ids
		txid, err := uuid.NewV7()
		if err != nil {
			return nil, err
		}
		txidStr = txid.String()
		subscription = uc.ListenManager.Subscribe(txidStr)
		defer subscription.Unsubscribe()
	}

	ret, err = uc.LegacyReporterResourceRepository.Update(ctx, m, existingResource.ID, uc.Namespace, txidStr)
	if err != nil {
		return nil, err
	}

	if !common.IsNil(uc.ListenManager) && uc.Config.ConsumerEnabled {
		timeoutCtx, cancel := context.WithTimeout(ctx, listenTimeout)
		defer cancel()

		err = subscription.BlockForNotification(timeoutCtx)
		if err != nil {
			if errors.Is(err, pubsub.ErrWaitContextCancelled) {
				return ret, nil
			}
			return nil, err
		}
	}

	uc.Log.WithContext(ctx).Infof("Updated Resource: %v(%v)", m.ID, m.ResourceType)
	return ret, nil

}

// Delete removes a resource from the database identified by the reporter resource ID.
func (uc *Usecase) DeleteLegacy(ctx context.Context, id model_legacy.ReporterResourceId) error {
	m := &model_legacy.Resource{}

	// check if the resource exists
	existingResource, err := uc.LegacyReporterResourceRepository.FindByReporterData(ctx, id.ReporterId, id.LocalResourceId)

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		// Deprecated: fallback case for backwards compatibility
		existingResource, err = uc.LegacyReporterResourceRepository.FindByReporterResourceId(ctx, id)
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrResourceNotFound
		}

		return ErrDatabaseError
	}

	m, err = uc.LegacyReporterResourceRepository.Delete(ctx, existingResource.ID, uc.Namespace)
	if err != nil {
		return err
	}

	uc.Log.WithContext(ctx).Infof("Deleted Resource: %v(%v)", m.ID, m.ResourceType)
	return nil

}

// Check if request comes from SP in allowlist
func isSPInAllowlistLegacy(m *model_legacy.Resource, allowlist []string) bool {
	for _, sp := range allowlist {
		// either specific SP or everyone
		if sp == m.ReporterId || sp == "*" {
			return true
		}
	}

	return false
}

func computeReadAfterWriteLegacy(uc *Usecase, write_visibility v1beta2.WriteVisibility, m *model_legacy.Resource) bool {
	// read after write functionality is enabled/disabled globally.
	// And executed if request specifies and
	// came from service provider in allowlist
	if write_visibility == v1beta2.WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED || write_visibility == v1beta2.WriteVisibility_MINIMIZE_LATENCY {
		return false
	}
	return !common.IsNil(uc.ListenManager) && uc.Config.ReadAfterWriteEnabled && isSPInAllowlistLegacy(m, uc.Config.ReadAfterWriteAllowlist)
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

func (uc *Usecase) CalculateTuplesv2(tupleEvent model.TupleEvent) (model.TuplesToReplicate, error) {
	currentVersion := tupleEvent.Version().Uint()
	key := tupleEvent.ReporterResourceKey()

	uc.Log.Infof("CalculateTuplesv2 called - version: %d, key: %+v", currentVersion, key)

	var currentWorkspaceID, previousWorkspaceID string

	// Handle create scenario (version 0)
	if currentVersion == 0 {
		uc.Log.Infof("CREATE scenario - version 0, processing initial tuple")

		// For version 0, we only need the current version data
		representations, err := uc.resourceRepository.FindCommonRepresentationsByVersion(
			nil, key, currentVersion, 0, // previousVersion is 0 for version 0
		)
		if err != nil {
			return model.TuplesToReplicate{}, fmt.Errorf("failed to find common representations: %w", err)
		}

		// Extract workspace_id from current version only
		for _, repr := range representations {
			if workspaceID, exists := repr.Data["workspace_id"].(string); exists && workspaceID != "" {
				if repr.Version == currentVersion {
					currentWorkspaceID = workspaceID
				}
			}
		}

		uc.Log.Infof("Found workspace ID for version 0 - current: '%s'", currentWorkspaceID)
	} else {
		// For updates, we need both current and previous versions
		previousVersion := currentVersion - 1

		// Get common representations for both versions
		representations, err := uc.resourceRepository.FindCommonRepresentationsByVersion(
			nil, key, currentVersion, previousVersion,
		)
		if err != nil {
			return model.TuplesToReplicate{}, fmt.Errorf("failed to find common representations: %w", err)
		}

		// Extract workspace_ids from the representations
		for _, repr := range representations {
			if workspaceID, exists := repr.Data["workspace_id"].(string); exists && workspaceID != "" {
				if repr.Version == currentVersion {
					currentWorkspaceID = workspaceID
				} else if repr.Version == previousVersion {
					previousWorkspaceID = workspaceID
				}
			}
		}

		uc.Log.Infof("Found workspace IDs - current: '%s', previous: '%s'", currentWorkspaceID, previousWorkspaceID)
	}

	// Build tuples based on workspace ID changes
	var tuplesToCreate, tuplesToDelete *[]model.RelationsTuple

	// Always create tuple for current workspace if it exists
	if currentWorkspaceID != "" {
		resourceStr := fmt.Sprintf("%s:%s", key.ResourceType().Serialize(), key.LocalResourceId().Serialize())
		subjectStr := fmt.Sprintf("rbac:workspace:%s", currentWorkspaceID)

		createTuple := model.NewRelationsTuple(resourceStr, "workspace", subjectStr)
		tuplesToCreate = &[]model.RelationsTuple{createTuple}

		uc.Log.Infof("Created tuple to create: %s#workspace@%s", resourceStr, subjectStr)
	}

	// Delete previous tuple if workspace ID changed and previous exists
	if previousWorkspaceID != "" && previousWorkspaceID != currentWorkspaceID {
		resourceStr := fmt.Sprintf("%s:%s", key.ResourceType().Serialize(), key.LocalResourceId().Serialize())
		subjectStr := fmt.Sprintf("rbac:workspace:%s", previousWorkspaceID)

		deleteTuple := model.NewRelationsTuple(resourceStr, "workspace", subjectStr)
		tuplesToDelete = &[]model.RelationsTuple{deleteTuple}

		uc.Log.Infof("Created tuple to delete: %s#workspace@%s", resourceStr, subjectStr)
	}

	// Return empty result if no tuples to process
	if tuplesToCreate == nil && tuplesToDelete == nil {
		uc.Log.Infof("No workspace ID changes detected, no tuples to process")
		return model.TuplesToReplicate{}, nil
	}

	return model.NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
}

func (uc *Usecase) CalculateTuples(tuple *kessel.Relationship, currentCommonVersion model.Version, key model.ReporterResourceKey) (*kessel.RelationTupleFilter, *kessel.RelationTupleFilter) {
	uc.Log.Infof("CalculateTuples called - tuple: %+v, version: %d, key: %+v", tuple, currentCommonVersion.Uint(), key)

	// Use the version passed from GetCurrentVersion instead of recalculating
	actualCurrentVersion := currentCommonVersion.Uint()
	uc.Log.Infof("Using passed version: %d", actualCurrentVersion)

	// Extract configuration from the tuple (cache nested lookups)
	relation := tuple.GetRelation()
	res := tuple.GetResource()
	sub := tuple.GetSubject().GetSubject()
	subType := sub.GetType()
	resType := res.GetType()
	subjectType := subType.GetName()
	subjectNamespace := subType.GetNamespace()
	resourceNamespace := resType.GetNamespace()

	// Extract workspace_id from the incoming tuple (the new one from the message)
	workspaceId := sub.GetId()
	uc.Log.Infof("Using workspace_id from tuple: '%s', version: %d", workspaceId, currentCommonVersion.Uint())

	// Create tuple based on current data
	if workspaceId != "" {
		uc.Log.Infof("Workspace ID found, proceeding with tuple creation")
		// Extract resource information from the tuple (not the event)
		resourceId := res.GetId()
		resourceType := resType.GetName()
		uc.Log.Infof("Creating tuple for resource - ID: %s, Type: %s", resourceId, resourceType)

		// Create the current relationship tuple filter (avoid extra allocations)
		rn, rt, rid := resourceNamespace, resourceType, resourceId
		rel := relation
		sn, st, sid := subjectNamespace, subjectType, workspaceId
		currentTupleFilter := &kessel.RelationTupleFilter{
			ResourceNamespace: &rn,
			ResourceType:      &rt,
			ResourceId:        &rid,
			Relation:          &rel,
			SubjectFilter: &kessel.SubjectFilter{
				SubjectNamespace: &sn,
				SubjectType:      &st,
				SubjectId:        &sid,
			},
		}
		uc.Log.Infof("Created current tuple filter - Resource: %s/%s/%s, Relation: %s, Subject: %s/%s/%s",
			resourceNamespace, resourceType, resourceId, relation, subjectNamespace, subjectType, workspaceId)

		// Determine if this is create, update, or delete based on actual current version
		if actualCurrentVersion == 0 {
			// Create scenario - only create tuple
			uc.Log.Infof("CREATE scenario - version 0, returning current tuple only")
			return currentTupleFilter, nil
		} else if actualCurrentVersion > 0 {
			// Update scenario - create new tuple and delete old one
			previousVersion := actualCurrentVersion - 1
			uc.Log.Infof("UPDATE scenario - version %d, looking for previous version %d to delete", actualCurrentVersion, previousVersion)

			// Query the database directly for the previous workspace ID
			var previousWorkspaceId string

			// Query only the previous workspace_id from common_representations
			var row struct {
				WorkspaceID *string `gorm:"column:workspace_id"`
			}
			err := uc.resourceRepository.GetDB().
				Table("common_representations cr").
				Joins("JOIN reporter_resources rr ON cr.resource_id::uuid = rr.resource_id").
				Where("rr.local_resource_id = ? AND rr.resource_type = ? AND rr.reporter_type = ? AND rr.reporter_instance_id = ? AND cr.version = ?",
					key.LocalResourceId().Serialize(), key.ResourceType().Serialize(), key.ReporterType().Serialize(), key.ReporterInstanceId().Serialize(), previousVersion).
				Select("cr.data->>'workspace_id' AS workspace_id").
				Take(&row).Error

			if err == nil && row.WorkspaceID != nil {
				previousWorkspaceId = *row.WorkspaceID
				uc.Log.Infof("Found previous workspace ID from database: '%s'", previousWorkspaceId)
			}

			if previousWorkspaceId != "" {
				// If previous is same as current, no delete needed
				if previousWorkspaceId == workspaceId {
					uc.Log.Infof("Previous workspace equals current; no delete tuple needed")
					return currentTupleFilter, nil
				}
				// Create the previous version's tuple for deletion
				prevSN, prevST, prevSID := subjectNamespace, subjectType, previousWorkspaceId
				previousTupleFilter := &kessel.RelationTupleFilter{
					ResourceNamespace: &rn,
					ResourceType:      &rt,
					ResourceId:        &rid,
					Relation:          &rel,
					SubjectFilter: &kessel.SubjectFilter{
						SubjectNamespace: &prevSN,
						SubjectType:      &prevST,
						SubjectId:        &prevSID,
					},
				}
				uc.Log.Infof("Found previous version - returning both create and delete tuples")
				uc.Log.Infof("Delete tuple - Resource: %s/%s/%s, Subject: %s/%s/%s",
					resourceNamespace, resourceType, resourceId,
					subjectNamespace, subjectType, previousWorkspaceId)
				return currentTupleFilter, previousTupleFilter
			}
			// If no previous version found, just return current tuple
			uc.Log.Warnf("No previous version %d found with workspace ID, returning current tuple only", previousVersion)
			return currentTupleFilter, nil
		}
	} else {
		uc.Log.Warnf("No workspace ID found in current event, skipping tuple creation")
	}

	uc.Log.Warnf("CalculateTuples returning nil, nil - workspaceId: '%s', version: %d", workspaceId, actualCurrentVersion)
	return nil, nil
}

// ResourceKeyAndCurrentVersionFromTuple combines fetching the reporter resource key and
// the latest version in a single DB round trip using a scalar subselect.
func (uc *Usecase) ResourceKeyAndCurrentVersionFromTuple(tuple *kessel.Relationship) (model.ReporterResourceKey, model.Version, error) {
	uc.Log.Infof("ResourceKeyAndCurrentVersionFromTuple called with tuple: %+v", tuple)

	// Extract resource information from tuple
	resourceIdStr := tuple.GetResource().GetId()
	resourceTypeStr := tuple.GetResource().GetType().GetName()

	// Convert string to UUID for ResourceId
	resourceId, err := uuid.Parse(resourceIdStr)
	if err != nil {
		return model.ReporterResourceKey{}, 0, fmt.Errorf("failed to parse resource ID '%s' as UUID: %w", resourceIdStr, err)
	}

	// Single query to get reporter resource identity and its latest version
	var row struct {
		LocalResourceID    string `gorm:"column:local_resource_id"`
		ResourceType       string `gorm:"column:resource_type"`
		ReporterType       string `gorm:"column:reporter_type"`
		ReporterInstanceID string `gorm:"column:reporter_instance_id"`
		Version            uint   `gorm:"column:version"`
	}

	err = uc.resourceRepository.GetDB().
		Table("reporter_resources rr").
		Select(`rr.local_resource_id, rr.resource_type, rr.reporter_type, rr.reporter_instance_id,
            (SELECT rr2.version FROM reporter_representations rr2
             WHERE rr2.reporter_resource_id = rr.id
             ORDER BY rr2.version DESC LIMIT 1) AS version`).
		Where("rr.local_resource_id = ? AND rr.resource_type = ?", resourceId, resourceTypeStr).
		Take(&row).Error
	if err != nil {
		uc.Log.Errorf("failed to fetch reporter resource and version: %v", err)
		return model.ReporterResourceKey{}, 0, fmt.Errorf("failed to fetch reporter resource and version: %w", err)
	}

	key, err := model.NewReporterResourceKey(
		model.LocalResourceId(row.LocalResourceID),
		model.ResourceType(row.ResourceType),
		model.ReporterType(row.ReporterType),
		model.ReporterInstanceId(row.ReporterInstanceID),
	)
	if err != nil {
		return model.ReporterResourceKey{}, 0, fmt.Errorf("failed to create ReporterResourceKey: %w", err)
	}

	return key, model.Version(row.Version), nil
}
