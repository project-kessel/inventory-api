package resources

import (
	"context"
	"errors"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"gorm.io/gorm"
)

type ResourceRepository interface {
	Save(context.Context, *model.Resource) (*model.Resource, error)
	Update(context.Context, *model.Resource, uint64) (*model.Resource, error)
	Delete(context.Context, uint64) (*model.Resource, error)
	FindByID(context.Context, uint64) (*model.Resource, error)
	FindByReporterResourceId(context.Context, model.ReporterResourceId) (*model.Resource, error)
	ListAll(context.Context) ([]*model.Resource, error)
}

var (
	ErrResourceNotFound      = errors.New("resource not found")
	ErrDatabaseError         = errors.New("db error while querying for resource")
	ErrResourceAlreadyExists = errors.New("resource already exists")
)

type Usecase struct {
	repository ResourceRepository
	Authz      authzapi.Authorizer
	Eventer    eventingapi.Manager
	Namespace  string
	log        *log.Helper
}

func New(repository ResourceRepository, authz authzapi.Authorizer, eventer eventingapi.Manager, namespace string, logger log.Logger) *Usecase {
	return &Usecase{
		repository: repository,
		Authz:      authz,
		Eventer:    eventer,
		Namespace:  namespace,
		log:        log.NewHelper(logger),
	}
}

func (uc *Usecase) Create(ctx context.Context, m *model.Resource) (*model.Resource, error) {
	// check if the resource already exists
	resource, err := uc.repository.FindByReporterResourceId(ctx, model.ReporterResourceIdFromResource(m))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDatabaseError
	}
	if resource != nil {
		return nil, ErrResourceAlreadyExists
	}

	if ret, err := uc.repository.Save(ctx, m); err != nil {
		return nil, err
	} else {
		if uc.Eventer != nil {
			err := biz.DefaultResourceSendEvent(ctx, m, uc.Eventer, *m.CreatedAt, eventingapi.OperationTypeCreated)

			if err != nil {
				return nil, err
			}
		}

		if uc.Authz != nil {
			err := biz.DefaultSetWorkspace(ctx, uc.Namespace, m, uc.Authz)
			if err != nil {
				return nil, err
			}
		}

		uc.log.WithContext(ctx).Infof("Created Resource: %v(%v)", m.ID, m.ResourceType)
		return ret, nil
	}
}

// Update updates a model in the database, updates related tuples in the relations-api, and issues an update event.
func (uc *Usecase) Update(ctx context.Context, m *model.Resource, id model.ReporterResourceId) (*model.Resource, error) {
	// check if the resource exists
	existingResource, err := uc.repository.FindByReporterResourceId(ctx, model.ReporterResourceIdFromResource(m))

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uc.Create(ctx, m)
		} else {
			return nil, ErrDatabaseError
		}
	}

	if ret, err := uc.repository.Update(ctx, m, existingResource.ID); err != nil {
		return nil, err
	} else {
		if uc.Eventer != nil {
			err := biz.DefaultResourceSendEvent(ctx, m, uc.Eventer, *m.UpdatedAt, eventingapi.OperationTypeUpdated)

			if err != nil {
				return nil, err
			}
		}

		if uc.Authz != nil {
			// Todo: Update workspace if there is any change
			err := biz.DefaultSetWorkspace(ctx, uc.Namespace, m, uc.Authz)
			if err != nil {
				return nil, err
			}
		}

		uc.log.WithContext(ctx).Infof("Updated Resource: %v(%v)", m.ID, m.ResourceType)
		return ret, nil
	}
}

// Delete deletes a model from the database, removes related tuples from the relations-api, and issues a delete event.
func (uc *Usecase) Delete(ctx context.Context, id model.ReporterResourceId) error {
	// check if the resource exists
	existingResource, err := uc.repository.FindByReporterResourceId(ctx, id)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrResourceNotFound
		} else {
			return ErrDatabaseError
		}
	}

	if m, err := uc.repository.Delete(ctx, existingResource.ID); err != nil {
		return err
	} else {
		if uc.Eventer != nil {
			err := biz.DefaultResourceSendEvent(ctx, m, uc.Eventer, time.Now(), eventingapi.OperationTypeDeleted)

			if err != nil {
				return err
			}
		}

		// TODO: delete the workspace tuple

		uc.log.WithContext(ctx).Infof("Deleted Resource: %v(%v)", m.ID, m.ResourceType)
		return nil
	}
}
