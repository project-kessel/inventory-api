package resources

import (
	"context"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Repo struct {
	DB        *gorm.DB
	Authz     authzapi.Authorizer
	Eventer   eventingapi.Manager
	Namespace string
}

func New(db *gorm.DB, authz authzapi.Authorizer, eventer eventingapi.Manager, namespace string) *Repo {
	return &Repo{
		DB:        db,
		Authz:     authz,
		Eventer:   eventer,
		Namespace: namespace,
	}
}

func (r *Repo) Save(ctx context.Context, model *model.Resource) (*model.Resource, error) {
	if err := r.DB.Session(&gorm.Session{FullSaveAssociations: true}).Create(model).Error; err != nil {
		return nil, err
	}

	if r.Eventer != nil {
		err := data.DefaultResourceSendEvent(ctx, model, r.Eventer, *model.CreatedAt, eventingapi.OperationTypeCreated)

		if err != nil {
			return nil, err
		}
	}

	if r.Authz != nil {
		err := data.DefaultSetWorkspace(ctx, r.Namespace, model, r.Authz)
		if err != nil {
			return nil, err
		}
	}

	return model, nil
}

// Update updates a model in the database, updates related tuples in the relations-api, and issues an update event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *Repo) Update(ctx context.Context, model *model.Resource, id string) (*model.Resource, error) {
	// TODO: update the model in inventory

	if r.Eventer != nil {
		err := data.DefaultResourceSendEvent(ctx, model, r.Eventer, *model.UpdatedAt, eventingapi.OperationTypeUpdated)

		if err != nil {
			return nil, err
		}
	}

	if r.Authz != nil {
		err := data.DefaultSetWorkspace(ctx, r.Namespace, model, r.Authz)
		if err != nil {
			return nil, err
		}
	}

	return model, nil
}

// Delete deletes a model from the database, removes related tuples from the relations-api, and issues a delete event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *Repo) Delete(ctx context.Context, id string) error {
	// TODO: Retrieve data from inventory so we have something to publish
	model := &model.Resource{
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

	if r.Eventer != nil {
		err := data.DefaultResourceSendEvent(ctx, model, r.Eventer, time.Now(), eventingapi.OperationTypeDeleted)

		if err != nil {
			return err
		}
	}

	// TODO: delete the workspace tuple

	return nil
}

func (r *Repo) FindByID(context.Context, string) (*model.Resource, error) {
	return nil, nil
}

func (r *Repo) ListAll(context.Context) ([]*model.Resource, error) {
	// var model biz.Resource
	// var count int64
	// if err := r.Db.Model(&model).Count(&count).Error; err != nil {
	// 	return nil, err
	// }

	var results []*model.Resource
	if err := r.DB.Preload(clause.Associations).Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
