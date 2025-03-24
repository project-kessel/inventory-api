package resources

import (
	"context"
	"errors"

	"github.com/project-kessel/inventory-api/internal/consumer"

	"github.com/google/uuid"

	"github.com/go-kratos/kratos/v2/log"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/server"
	"gorm.io/gorm"
)

type ReporterResourceRepository interface {
	Create(context.Context, *model.Resource, string) (*model.Resource, error)
	Update(context.Context, *model.Resource, uuid.UUID, string) (*model.Resource, error)
	Delete(context.Context, uuid.UUID, string) (*model.Resource, error)
	FindByID(context.Context, uuid.UUID) (*model.Resource, error)
	FindByReporterResourceId(context.Context, model.ReporterResourceId) (*model.Resource, error)
	FindByReporterData(context.Context, string, string) (*model.Resource, error)
	ListAll(context.Context) ([]*model.Resource, error)
}

type InventoryResourceRepository interface {
	FindByID(context.Context, uuid.UUID) (*model.InventoryResource, error)
}

var (
	ErrResourceNotFound      = errors.New("resource not found")
	ErrDatabaseError         = errors.New("db error while querying for resource")
	ErrResourceAlreadyExists = errors.New("resource already exists")
)

type Usecase struct {
	reporterResourceRepository  ReporterResourceRepository
	inventoryResourceRepository InventoryResourceRepository
	Authz                       authzapi.Authorizer
	Eventer                     eventingapi.Manager
	Consumer                    consumer.InventoryConsumer
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

func (uc *Usecase) Create(ctx context.Context, m *model.Resource) (*model.Resource, error) {
	ret := m // Default to returning the input model in case persistence is disabled

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

		ret, err = uc.reporterResourceRepository.Create(ctx, m, uc.Namespace)
		if err != nil {
			return nil, err
		}
	}

	uc.log.WithContext(ctx).Infof("Created Resource: %v(%v)", m.ID, m.ResourceType)
	return ret, nil
}

// Update updates a model in the database, updates related tuples in the relations-api, and issues an update event.
func (uc *Usecase) Update(ctx context.Context, m *model.Resource, id model.ReporterResourceId) (*model.Resource, error) {
	ret := m // Default to returning the input model in case persistence is disabled

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

		ret, err = uc.reporterResourceRepository.Update(ctx, m, existingResource.ID, uc.Namespace)
		if err != nil {
			return nil, err
		}
	}

	uc.log.WithContext(ctx).Infof("Updated Resource: %v(%v)", m.ID, m.ResourceType)
	return ret, nil

}

// Delete deletes a model from the database, removes related tuples from the relations-api, and issues a delete event.
func (uc *Usecase) Delete(ctx context.Context, id model.ReporterResourceId) error {
	var m *model.Resource

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

		m, err = uc.reporterResourceRepository.Delete(ctx, existingResource.ID, uc.Namespace)
		if err != nil {
			return err
		}
	}

	uc.log.WithContext(ctx).Infof("Deleted Resource: %v(%v)", m.ID, m.ResourceType)
	return nil

}
