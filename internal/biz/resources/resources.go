package resources

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"time"
)

type ResourceRepository interface {
	Save(ctx context.Context, resource *model.Resource) (*model.Resource, error)
	Update(context.Context, *model.Resource, uint64) (*model.Resource, error)
	Delete(context.Context, uint64) error
	FindByID(context.Context, uint64) (*model.Resource, error)
	ListAll(context.Context) ([]*model.Resource, error)
}

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
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (uc *Usecase) Update(ctx context.Context, m *model.Resource, id string) (*model.Resource, error) {
	if ret, err := uc.repository.Update(ctx, m, 0); err != nil {
		return nil, err
	} else {
		if uc.Eventer != nil {
			err := biz.DefaultResourceSendEvent(ctx, m, uc.Eventer, *m.UpdatedAt, eventingapi.OperationTypeUpdated)

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

		uc.log.WithContext(ctx).Infof("Updated Resource: %v(%v)", m.ID, m.ResourceType)
		return ret, nil
	}
}

// Delete deletes a model from the database, removes related tuples from the relations-api, and issues a delete event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (uc *Usecase) Delete(ctx context.Context, id string) error {
	if err := uc.repository.Delete(ctx, 0); err != nil {
		return err
	} else {
		// TODO: Retrieve data from inventory so we have something to publish
		m := &model.Resource{
			ID:           0,
			ResourceData: nil,
			ResourceType: "",
			WorkspaceId:  "",
			Reporter:     model.ResourceReporter{},
			ConsoleHref:  "",
			ApiHref:      "",
			Labels:       nil,
		}

		// TODO: delete the model from inventory

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
