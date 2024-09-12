package notificationsintegrations

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/notificationsintegrations"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
)

const namespace = "notifications"
const resourceType = "integration"

type notificationsintegrationsRepo struct {
	DB      *gorm.DB
	Authz   authzapi.Authorizer
	Eventer eventingapi.Manager
}

func New(g *gorm.DB, a authzapi.Authorizer, e eventingapi.Manager) *notificationsintegrationsRepo {
	return &notificationsintegrationsRepo{
		DB:      g,
		Authz:   a,
		Eventer: e,
	}
}

func (r *notificationsintegrationsRepo) Save(ctx context.Context, model *biz.NotificationsIntegration) (*biz.NotificationsIntegration, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, nil
	}

	if err := r.DB.Session(&gorm.Session{FullSaveAssociations: true}).Create(model).Error; err != nil {
		return nil, err
	}

	if r.Eventer != nil {
		// TODO: Update the Object that's sent.  This is going to be what we actually emit.
		producer, _ := r.Eventer.Lookup(identity, biz.ResourceType, model.ID)
		evt := eventingapi.NewAddEvent(biz.ResourceType, model.Metadata.UpdatedAt, &eventingapi.EventResource[struct{}]{
			Metadata:     &model.Metadata,
			ReporterData: model.Metadata.Reporters[0],
		})
		err = producer.Produce(ctx, evt)
		if err != nil {
			return nil, err
		}
	}

	if r.Authz != nil {
		_, err := r.Authz.SetWorkspace(ctx, model.Metadata.Reporters[0].LocalResourceId, model.Metadata.Workspace, namespace, resourceType)
		if err != nil {
			return nil, err
		}
	}

	return model, nil
}

// Update updates a model in the database, updates related tuples in the relations-api, and issues an update event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *notificationsintegrationsRepo) Update(ctx context.Context, model *biz.NotificationsIntegration, id string) (*biz.NotificationsIntegration, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, nil
	}

	// TODO: update the model in inventory

	if r.Eventer != nil {
		producer, _ := r.Eventer.Lookup(identity, biz.ResourceType, model.ID)
		evt := eventingapi.NewUpdateEvent(biz.ResourceType, model.Metadata.UpdatedAt, &eventingapi.EventResource[struct{}]{
			Metadata:     &model.Metadata,
			ReporterData: model.Metadata.Reporters[0],
		})
		err = producer.Produce(ctx, evt)
		if err != nil {
			return nil, err
		}
	}

	if r.Authz != nil {
		_, err := r.Authz.SetWorkspace(ctx, model.Metadata.Reporters[0].LocalResourceId, model.Metadata.Workspace, namespace, resourceType)
		if err != nil {
			return nil, err
		}
	}

	return model, nil
}

// Delete deletes a model from the database, removes related tuples from the relations-api, and issues a delete event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *notificationsintegrationsRepo) Delete(ctx context.Context, id string) error {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil
	}

	// TODO: delete the model from inventory

	if r.Eventer != nil {
		// TODO: without persistence, we can't lookup the inventory assigned id or other model specific info.
		var dummyId int64 = 0
		producer, _ := r.Eventer.Lookup(identity, biz.ResourceType, dummyId)
		evt := eventingapi.NewDeleteEvent(biz.ResourceType, id, time.Now().UTC(), identity)
		err = producer.Produce(ctx, evt)
		if err != nil {
			return err
		}
	}

	// TODO: delete the workspace tuple

	return nil
}

func (r *notificationsintegrationsRepo) FindByID(context.Context, string) (*biz.NotificationsIntegration, error) {
	return nil, nil
}

func (r *notificationsintegrationsRepo) ListAll(context.Context) ([]*biz.NotificationsIntegration, error) {
	// var model biz.NotificationsIntegration
	// var count int64
	// if err := r.Db.Model(&model).Count(&count).Error; err != nil {
	// 	return nil, err
	// }

	var results []*biz.NotificationsIntegration
	if err := r.DB.Preload(clause.Associations).Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
