package resources

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/go-kratos/kratos/v2/log"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/server"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"gorm.io/gorm"
)

type ReporterResourceRepository interface {
	Create(context.Context, *model.Resource) (*model.Resource, []*model.Resource, error)
	Update(context.Context, *model.Resource, uuid.UUID) (*model.Resource, []*model.Resource, error)
	Delete(context.Context, uuid.UUID) (*model.Resource, error)
	FindByID(context.Context, uuid.UUID) (*model.Resource, error)
	FindByWorkspaceId(context.Context, string) ([]*model.Resource, error)
	FindByReporterResourceId(context.Context, model.ReporterResourceId) (*model.Resource, error)
	FindByReporterResourceIdv1beta2(context.Context, model.ReporterResourceUniqueIndex) (*model.Resource, error)
	FindByReporterData(context.Context, string, string) (*model.Resource, error)
	FindByInventoryIdAndResourceType(ctx context.Context, inventoryId *uuid.UUID, resourceType string) (*model.Resource, error)
	FindByInventoryIdAndReporter(ctx context.Context, inventoryId *uuid.UUID, reporterInstanceId string, reporterType string) (*model.Resource, error)
	ListAll(context.Context) ([]*model.Resource, error)
}

type InventoryResourceRepository interface {
	FindByID(context.Context, uuid.UUID) (*model.InventoryResource, error)
}

var (
	ErrResourceNotFound      = errors.New("resource not found")
	ErrDatabaseError         = errors.New("db error while querying for resource")
	ErrResourceAlreadyExists = errors.New("resource already exists")
	ErrInventoryIdMismatch   = errors.New("resource inventory id mismatch")
)

type Usecase struct {
	reporterResourceRepository  ReporterResourceRepository
	inventoryResourceRepository InventoryResourceRepository
	Authz                       authzapi.Authorizer
	Eventer                     eventingapi.Manager
	Namespace                   string
	log                         *log.Helper
	Server                      server.Server
	DisablePersistence          bool
}

func New(reporterResourceRepository ReporterResourceRepository, inventoryResourceRepository InventoryResourceRepository,
	authz authzapi.Authorizer, eventer eventingapi.Manager, namespace string, logger log.Logger, disablePersistence bool) *Usecase {
	return &Usecase{
		reporterResourceRepository:  reporterResourceRepository,
		inventoryResourceRepository: inventoryResourceRepository,
		Authz:                       authz,
		Eventer:                     eventer,
		Namespace:                   namespace,
		log:                         log.NewHelper(logger),
		DisablePersistence:          disablePersistence,
	}
}

func (uc *Usecase) Upsert(ctx context.Context, m *model.Resource) (*model.Resource, error) {
	log.Info("upserting resource: ", m)
	ret := m // Default to returning the input model in case persistence is disabled

	if !uc.DisablePersistence {
		// check if the resource already exists
		existingResource, err := uc.reporterResourceRepository.FindByReporterResourceIdv1beta2(ctx, model.ReporterResourceIdv1beta2FromResource(m))

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseError
		}
		log.Info("found existing resource: ", existingResource)
		if existingResource != nil {
			return updateExistingReporterResource(ctx, m, existingResource, uc)
		}

		//TODO: Bug here that needs to be fixed : https://issues.redhat.com/browse/RHCLOUD-39044
		if m.InventoryId != nil {
			err2 := validateSameResourceFromMultipleReportersShareInventoryId(ctx, m, uc)
			if err2 != nil {
				return nil, err2
			}
		}

		log.Info("Creating resource: ", m)
		ret, err2 := createNewReporterResource(ctx, m, uc)
		if err2 != nil {
			return ret, err2
		}
	} else {
		// mock the created at time for eventing
		// TODO: remove this when persistence is always enabled
		now := time.Now()
		m.CreatedAt = &now
	}

	uc.log.WithContext(ctx).Infof("Created Resource: %v(%v)", m.ID, m.ResourceType)
	return ret, nil
}

func createNewReporterResource(ctx context.Context, m *model.Resource, uc *Usecase) (*model.Resource, error) {
	ret, updatedResources, err := uc.reporterResourceRepository.Create(ctx, m)

	if err != nil {
		return nil, err
	}

	//TODO: adding eventing and relations calls for v1beta2 schema demo purposes. Needs to be updated to be done via outbox with the consistency Epic
	if uc.Eventer != nil {
		// Send event for the created resource
		err := biz.DefaultResourceSendEvent(ctx, m, uc.Eventer, *m.CreatedAt, eventingapi.OperationTypeCreated)
		if err != nil {
			return nil, err
		}

		// Send events for any updated resources
		for _, updatedResource := range updatedResources {
			err := biz.DefaultResourceSendEvent(ctx, updatedResource, uc.Eventer, *updatedResource.UpdatedAt, eventingapi.OperationTypeUpdated)
			if err != nil {
				return nil, err
			}
		}
	}

	if uc.Authz != nil {
		// Send workspace for the created resource
		ct, err := biz.DefaultSetWorkspace(ctx, uc.Namespace, ret, uc.Authz, false)
		if err != nil {
			return nil, err
		}

		ret.ConsistencyToken = ct

		if !uc.DisablePersistence {
			_, _, err = uc.reporterResourceRepository.Update(ctx, ret, ret.ID)
			if err != nil {
				return nil, err
			}
		}
		// Send workspace for any updated resources
		for _, updatedResource := range updatedResources {
			ct, err := biz.DefaultSetWorkspace(ctx, uc.Namespace, updatedResource, uc.Authz, false)
			if err != nil {
				return nil, err
			}

			updatedResource.ConsistencyToken = ct
		}
	}
	return ret, nil
}

func validateSameResourceFromMultipleReportersShareInventoryId(ctx context.Context, m *model.Resource, uc *Usecase) error {
	// Multiple reporters should have same inventory id.
	existingInventoryIdResource, err := uc.reporterResourceRepository.FindByInventoryIdAndResourceType(ctx, m.InventoryId, m.ResourceType)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrDatabaseError
	}

	if existingInventoryIdResource != nil {
		existingResourceRepo, err := uc.reporterResourceRepository.FindByInventoryIdAndReporter(ctx, m.InventoryId, m.ReporterInstanceId, m.ReporterType)
		if existingResourceRepo != nil {
			return ErrResourceAlreadyExists
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return nil
}

func updateExistingReporterResource(ctx context.Context, m *model.Resource, existingResource *model.Resource, uc *Usecase) (*model.Resource, error) {

	if m.InventoryId != nil && existingResource.InventoryId.String() != m.InventoryId.String() {
		return nil, ErrInventoryIdMismatch
	}
	log.Info("Updating resource: ", m)
	ret, updatedResources, err := uc.reporterResourceRepository.Update(ctx, m, existingResource.ID)
	if err != nil {
		return nil, err
	}

	//TODO: adding eventing and relations calls for v1beta2 schema demo purposes. Needs to be updated to be done via outbox with the consistency Epic
	if uc.Eventer != nil {
		for _, updatedResource := range updatedResources {
			err := biz.DefaultResourceSendEvent(ctx, updatedResource, uc.Eventer, *updatedResource.UpdatedAt, eventingapi.OperationTypeUpdated)
			if err != nil {
				return nil, err
			}
		}
	}

	if uc.Authz != nil {
		for _, updatedResource := range updatedResources {
			_, err := biz.DefaultSetWorkspace(ctx, uc.Namespace, updatedResource, uc.Authz, true)
			if err != nil {
				return nil, err
			}
		}
	}

	uc.log.WithContext(ctx).Infof("Updated Resource: %v(%v)", m.ID, m.ResourceType)
	return ret, nil
}

func (uc *Usecase) LookupResources(ctx context.Context, request *kessel.LookupResourcesRequest) (grpc.ServerStreamingClient[kessel.LookupResourcesResponse], error) {
	return uc.Authz.LookupResources(ctx, request)
}

func (uc *Usecase) Check(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, id model.ReporterResourceId) (bool, error) {
	res, err := uc.reporterResourceRepository.FindByReporterResourceId(ctx, id)
	if err != nil {
		// If the resource doesn't exist in inventory (ie. no consistency token available)
		// we send a check request with minimize latency
		// err otherwise.
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, err
		}
		res = &model.Resource{ResourceType: id.ResourceType, ReporterResourceId: id.LocalResourceId}
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

func (uc *Usecase) CheckForUpdate(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, id model.ReporterResourceId) (bool, error) {
	res, err := uc.reporterResourceRepository.FindByReporterResourceId(ctx, id)
	recordToken := true
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// resource doesn't exist yet.
			// DONT write consistency token
			// no actual resource exists in DB to update
			recordToken = false
			res = &model.Resource{ResourceType: id.ResourceType, ReporterResourceId: id.LocalResourceId}
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
			_, _, err := uc.reporterResourceRepository.Update(ctx, res, res.ID)
			if err != nil {
				return false, err // we're allowed, but failed to update consistency token
			}
		}

		return true, nil
	}

	return false, nil
}

func (uc *Usecase) ListResourcesInWorkspace(ctx context.Context, permission, namespace string, sub *kessel.SubjectReference, id string) (chan *model.Resource, chan error, error) {
	resource_chan := make(chan *model.Resource)
	error_chan := make(chan error, 1)

	resources, err := uc.reporterResourceRepository.FindByWorkspaceId(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	log.Infof("ListResourcesInWorkspace: resources %+v", resources)

	go func() {
		defer close(resource_chan)
		defer close(error_chan)

		for _, resource := range resources {
			log.Infof("ListResourcesInWorkspace: checkforview on %+v", resource)
			if allowed, _, err := uc.Authz.Check(ctx, namespace, permission, resource, sub); err == nil && allowed == kessel.CheckResponse_ALLOWED_TRUE {
				resource_chan <- resource
			} else if err != nil {
				error_chan <- err
				break
			} else if allowed != kessel.CheckResponse_ALLOWED_TRUE {
				log.Infof("Response was not allowed: %v", allowed)
			}
		}
	}()

	return resource_chan, error_chan, nil
}

// Delete deletes a model from the database, removes related tuples from the relations-api, and issues a delete event.
func (uc *Usecase) Delete(ctx context.Context, id model.ReporterResourceId) error {
	m := &model.Resource{
		// TODO: Create model
	}

	if !uc.DisablePersistence {
		// check if the resource exists
		existingResource, err := uc.reporterResourceRepository.FindByReporterData(ctx, id.ReporterId, id.LocalResourceId)

		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			// Deprecated: fallback case for backwards compatibility
			existingResource, err = uc.reporterResourceRepository.FindByReporterResourceId(ctx, id)
		}

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrResourceNotFound
			}

			return ErrDatabaseError
		}

		m, err = uc.reporterResourceRepository.Delete(ctx, existingResource.ID)
		if err != nil {
			return err
		}

	}

	if uc.Eventer != nil {
		err := biz.DefaultResourceSendEvent(ctx, m, uc.Eventer, time.Now(), eventingapi.OperationTypeDeleted)

		if err != nil {
			return err
		}
	}

	if uc.Authz != nil {
		var resourceType string

		namespace := uc.Namespace
		if id.ReporterType != "" {
			namespace = strings.ToLower(id.ReporterType)
		}
		if m.ResourceType != "" {
			resourceType = m.ResourceType
			err := biz.DefaultUnsetWorkspace(ctx, namespace, id.LocalResourceId, resourceType, uc.Authz)
			if err != nil {
				return err
			}
		}

	}

	uc.log.WithContext(ctx).Infof("Deleted Resource: %v(%v)", m.ID, m.ResourceType)
	return nil

}

//Deprecated. Remove after notifications and ACM migrates to v1beta2

func (uc *Usecase) Create(ctx context.Context, m *model.Resource) (*model.Resource, error) {
	ret := m // Default to returning the input model in case persistence is disabled
	updatedResources := []*model.Resource{}

	if !uc.DisablePersistence {
		// check if the resource already exists
		existingResource, err := uc.reporterResourceRepository.FindByReporterData(ctx, m.ReporterId, m.ReporterResourceId)
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			// Deprecated: fallback case for backwards compatibility
			existingResource, err = uc.reporterResourceRepository.FindByReporterResourceId(ctx, model.ReporterResourceIdFromResource(m))
		}

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDatabaseError
		}

		if existingResource != nil {
			return nil, ErrResourceAlreadyExists
		}

		ret, updatedResources, err = uc.reporterResourceRepository.Create(ctx, m)
		if err != nil {
			return nil, err
		}
	} else {
		// mock the created at time for eventing
		// TODO: remove this when persistence is always enabled
		now := time.Now()
		m.CreatedAt = &now
	}

	if uc.Eventer != nil {
		// Send event for the created resource
		err := biz.DefaultResourceSendEvent(ctx, m, uc.Eventer, *m.CreatedAt, eventingapi.OperationTypeCreated)
		if err != nil {
			return nil, err
		}

		// Send events for any updated resources
		for _, updatedResource := range updatedResources {
			err := biz.DefaultResourceSendEvent(ctx, updatedResource, uc.Eventer, *updatedResource.UpdatedAt, eventingapi.OperationTypeUpdated)
			if err != nil {
				return nil, err
			}
		}
	}

	if uc.Authz != nil {
		// Send workspace for the created resource
		ct, err := biz.DefaultSetWorkspace(ctx, uc.Namespace, ret, uc.Authz, false)
		if err != nil {
			return nil, err
		}

		ret.ConsistencyToken = ct

		if !uc.DisablePersistence {
			_, _, err = uc.reporterResourceRepository.Update(ctx, ret, ret.ID)
			if err != nil {
				return nil, err
			}
		}

		// Send workspace for any updated resources
		for _, updatedResource := range updatedResources {
			ct, err := biz.DefaultSetWorkspace(ctx, uc.Namespace, updatedResource, uc.Authz, false)
			if err != nil {
				return nil, err
			}

			updatedResource.ConsistencyToken = ct
		}
	}

	uc.log.WithContext(ctx).Infof("Created Resource: %v(%v)", m.ID, m.ResourceType)
	return ret, nil
}

//Deprecated. Remove after notifications and ACM migrates to v1beta2

// Update updates a model in the database, updates related tuples in the relations-api, and issues an update event.
func (uc *Usecase) Update(ctx context.Context, m *model.Resource, id model.ReporterResourceId) (*model.Resource, error) {
	ret := m // Default to returning the input model in case persistence is disabled
	updatedResources := []*model.Resource{}

	if !uc.DisablePersistence {
		// check if the resource exists
		existingResource, err := uc.reporterResourceRepository.FindByReporterData(ctx, m.ReporterId, m.ReporterResourceId)
		if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
			// Deprecated: fallback case for backwards compatibility
			existingResource, err = uc.reporterResourceRepository.FindByReporterResourceId(ctx, model.ReporterResourceIdFromResource(m))
		}

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return uc.Create(ctx, m)
			}

			return nil, ErrDatabaseError
		}

		ret, updatedResources, err = uc.reporterResourceRepository.Update(ctx, m, existingResource.ID)
		if err != nil {
			return nil, err
		}
	} else {
		// mock the updated at time for eventing
		// TODO: remove this when persistence is always enabled
		now := time.Now()
		m.UpdatedAt = &now
		updatedResources = append(updatedResources, m)
	}

	if uc.Eventer != nil {
		for _, updatedResource := range updatedResources {
			err := biz.DefaultResourceSendEvent(ctx, updatedResource, uc.Eventer, *updatedResource.UpdatedAt, eventingapi.OperationTypeUpdated)
			if err != nil {
				return nil, err
			}
		}
	}

	if uc.Authz != nil {
		for _, updatedResource := range updatedResources {
			_, err := biz.DefaultSetWorkspace(ctx, uc.Namespace, updatedResource, uc.Authz, true)
			if err != nil {
				return nil, err
			}
		}
	}

	uc.log.WithContext(ctx).Infof("Updated Resource: %v(%v)", m.ID, m.ResourceType)
	return ret, nil

}