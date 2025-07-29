package resources

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"

	"sync"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"github.com/sony/gobreaker"

	"github.com/google/uuid"

	"github.com/go-kratos/kratos/v2/log"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"gorm.io/gorm"

	common "github.com/project-kessel/inventory-api/cmd/common"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/server"
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
	DisablePersistence      bool
	ReadAfterWriteEnabled   bool
	ReadAfterWriteAllowlist []string
	ConsumerEnabled         bool
}

// Usecase provides business logic operations for resource management in the inventory system.
// It coordinates between repositories, authorization, eventing, and other system components.
type Usecase struct {
	ReporterResourceRepository  ReporterResourceRepository
	inventoryResourceRepository InventoryResourceRepository
	waitForNotifBreaker         *gobreaker.CircuitBreaker
	Authz                       authzapi.Authorizer
	Eventer                     eventingapi.Manager
	Namespace                   string
	Log                         *log.Helper
	Server                      server.Server
	ListenManager               pubsub.ListenManagerImpl
	Config                      *UsecaseConfig
}

// New creates a new Usecase instance with the provided dependencies and configuration.
func New(reporterResourceRepository ReporterResourceRepository, inventoryResourceRepository InventoryResourceRepository,
	authz authzapi.Authorizer, eventer eventingapi.Manager, namespace string, logger log.Logger,
	listenManager pubsub.ListenManagerImpl, waitForNotifBreaker *gobreaker.CircuitBreaker, usecaseConfig *UsecaseConfig) *Usecase {
	return &Usecase{
		ReporterResourceRepository:  reporterResourceRepository,
		inventoryResourceRepository: inventoryResourceRepository,
		waitForNotifBreaker:         waitForNotifBreaker,
		Authz:                       authz,
		Eventer:                     eventer,
		Namespace:                   namespace,
		Log:                         log.NewHelper(logger),
		ListenManager:               listenManager,
		Config:                      usecaseConfig,
	}
}

// Upsert creates a new resource or updates an existing one based on the reporter resource ID.
// It supports read-after-write consistency when enabled and handles notification waiting.
func (uc *Usecase) Upsert(ctx context.Context, m *model_legacy.Resource, write_visibility v1beta2.WriteVisibility) (*model_legacy.Resource, error) {
	log.Info("upserting resource: ", m)
	ret := m // Default to returning the input model_legacy in case persistence is disabled
	var subscription pubsub.Subscription
	var txidStr string

	if !uc.Config.DisablePersistence {
		// check if the resource already exists
		existingResource, err := uc.ReporterResourceRepository.FindByReporterResourceIdv1beta2(ctx, model_legacy.ReporterResourceIdv1beta2FromResource(m))

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseError
		}

		readAfterWriteEnabled := computeReadAfterWrite(uc, write_visibility, m)
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
	}

	uc.Log.WithContext(ctx).Infof("Upserted Resource: %v(%v)", ret.ID, ret.ResourceType)
	return ret, nil
}

func createNewReporterResource(ctx context.Context, m *model_legacy.Resource, uc *Usecase, txid string) (*model_legacy.Resource, error) {
	ret, err := uc.ReporterResourceRepository.Create(ctx, m, uc.Namespace, txid)

	if err != nil {
		return nil, err
	}

	return ret, nil
}

func validateSameResourceFromMultipleReportersShareInventoryId(ctx context.Context, m *model_legacy.Resource, uc *Usecase) error {
	// Multiple reporters should have same inventory id.
	existingInventoryIdResource, err := uc.ReporterResourceRepository.FindByInventoryIdAndResourceType(ctx, m.InventoryId, m.ResourceType)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrDatabaseError
	}

	if existingInventoryIdResource != nil {
		existingResourceRepo, err := uc.ReporterResourceRepository.FindByInventoryIdAndReporter(ctx, m.InventoryId, m.ReporterInstanceId, m.ReporterType)
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
	ret, err := uc.ReporterResourceRepository.Update(ctx, m, existingResource.ID, uc.Namespace, txid)
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

// Check verifies if a subject has the specified permission on a resource identified by the reporter resource ID.
func (uc *Usecase) Check(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, id model_legacy.ReporterResourceId) (bool, error) {
	res, err := uc.ReporterResourceRepository.FindByReporterResourceId(ctx, id)
	if err != nil {
		// If the resource doesn't exist in inventory (ie. no consistency token available)
		// we send a check request with minimize latency
		// err otherwise.
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, err
		}
		res = &model_legacy.Resource{ResourceType: id.ResourceType, ReporterResourceId: id.LocalResourceId}
	}

	allowed, _, err := uc.Authz.Check(ctx, namespace, permission, res, sub)
	if err != nil {
		return false, err
	}

	if allowed == kessel.CheckResponse_ALLOWED_TRUE {
		return true, nil
	}
	return false, nil
}

// CheckForUpdate verifies if a subject has the specified permission to update a resource,
// and records the consistency token if the check passes and the resource exists.
func (uc *Usecase) CheckForUpdate(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, id model_legacy.ReporterResourceId) (bool, error) {
	res, err := uc.ReporterResourceRepository.FindByReporterResourceId(ctx, id)
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

	allowed, consistency, err := uc.Authz.CheckForUpdate(ctx, namespace, permission, res, sub)
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
			_, err := uc.ReporterResourceRepository.Update(ctx, res, res.ID, uc.Namespace, "")
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
	resources, err := uc.ReporterResourceRepository.FindByWorkspaceId(ctx, id)
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

		if allowed, _, err := uc.Authz.Check(ctx, namespace, permission, resource, sub); err == nil && allowed == kessel.CheckResponse_ALLOWED_TRUE {
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
	ret := m // Default to returning the input model_legacy in case persistence is disabled
	var subscription pubsub.Subscription
	var txidStr string

	if !uc.Config.DisablePersistence {
		// check if the resource already exists
		existingResource, err := uc.ReporterResourceRepository.FindByReporterData(ctx, m.ReporterId, m.ReporterResourceId)
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			// Deprecated: fallback case for backwards compatibility
			existingResource, err = uc.ReporterResourceRepository.FindByReporterResourceId(ctx, model_legacy.ReporterResourceIdFromResource(m))
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

		ret, err = uc.ReporterResourceRepository.Create(ctx, m, uc.Namespace, txidStr)
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
	}
	uc.Log.WithContext(ctx).Infof("Created Resource: %v(%v)", ret.ID, ret.ResourceType)
	return ret, nil
}

// Update updates an existing resource in the database, or creates it if it doesn't exist.
// It waits for consumer notification if configured.
//
// Deprecated: Remove after notifications and ACM migrates to v1beta2.
func (uc *Usecase) Update(ctx context.Context, m *model_legacy.Resource, id model_legacy.ReporterResourceId) (*model_legacy.Resource, error) {
	ret := m // Default to returning the input model_legacy in case persistence is disabled
	var subscription pubsub.Subscription
	var txidStr string

	if !uc.Config.DisablePersistence {
		// check if the resource exists
		existingResource, err := uc.ReporterResourceRepository.FindByReporterData(ctx, m.ReporterId, m.ReporterResourceId)
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			// Deprecated: fallback case for backwards compatibility
			existingResource, err = uc.ReporterResourceRepository.FindByReporterResourceId(ctx, model_legacy.ReporterResourceIdFromResource(m))
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

		ret, err = uc.ReporterResourceRepository.Update(ctx, m, existingResource.ID, uc.Namespace, txidStr)
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
	}

	uc.Log.WithContext(ctx).Infof("Updated Resource: %v(%v)", m.ID, m.ResourceType)
	return ret, nil

}

// Delete removes a resource from the database identified by the reporter resource ID.
func (uc *Usecase) Delete(ctx context.Context, id model_legacy.ReporterResourceId) error {
	m := &model_legacy.Resource{}

	if !uc.Config.DisablePersistence {
		// check if the resource exists
		existingResource, err := uc.ReporterResourceRepository.FindByReporterData(ctx, id.ReporterId, id.LocalResourceId)

		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			// Deprecated: fallback case for backwards compatibility
			existingResource, err = uc.ReporterResourceRepository.FindByReporterResourceId(ctx, id)
		}

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrResourceNotFound
			}

			return ErrDatabaseError
		}

		m, err = uc.ReporterResourceRepository.Delete(ctx, existingResource.ID, uc.Namespace)
		if err != nil {
			return err
		}
	}

	uc.Log.WithContext(ctx).Infof("Deleted Resource: %v(%v)", m.ID, m.ResourceType)
	return nil

}

// Check if request comes from SP in allowlist
func isSPInAllowlist(m *model_legacy.Resource, allowlist []string) bool {
	for _, sp := range allowlist {
		// either specific SP or everyone
		if sp == m.ReporterId || sp == "*" {
			return true
		}
	}

	return false
}

func computeReadAfterWrite(uc *Usecase, write_visibility v1beta2.WriteVisibility, m *model_legacy.Resource) bool {
	// read after write functionality is enabled/disabled globally.
	// And executed if request specifies and
	// came from service provider in allowlist
	if write_visibility == v1beta2.WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED || write_visibility == v1beta2.WriteVisibility_MINIMIZE_LATENCY {
		return false
	}
	return !common.IsNil(uc.ListenManager) && uc.Config.ReadAfterWriteEnabled && isSPInAllowlist(m, uc.Config.ReadAfterWriteAllowlist)
}
