package notificationsintegrations

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/go-kratos/kratos/v2/log"

	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	models "github.com/project-kessel/inventory-api/internal/biz/notificationsintegrations"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
)

type notificationsIntegrationsRepo struct {
	Db      *gorm.DB
	Authz   authzapi.Authorizer
	Eventer eventingapi.Manager

	Log *log.Helper
}

func New(g *gorm.DB, a authzapi.Authorizer, e eventingapi.Manager, l *log.Helper) *notificationsIntegrationsRepo {
	return &notificationsIntegrationsRepo{
		Db:      g,
		Authz:   a,
		Eventer: e,

		Log: l,
	}
}

func (r *notificationsIntegrationsRepo) Save(ctx context.Context, model *models.NotificationsIntegration) (*models.NotificationsIntegration, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, nil
	}

	if err := r.Db.Session(&gorm.Session{FullSaveAssociations: true}).Create(model).Error; err != nil {
		return nil, err
	}

	if r.Eventer != nil {
		// TODO: handle eventing errors
		// TODO: Update the Object that's sent.  This is going to be what we actually emit.
		producer, _ := r.Eventer.Lookup(identity, "notificationsintegration", model.ID)
		evt := &eventingapi.Event{
			EventType:    "Create",
			ResourceType: "notificationsintegration",
			Object:       model,
		}
		producer.Produce(ctx, evt)
	}
	return model, nil
}

func (r *notificationsIntegrationsRepo) Update(context.Context, *models.NotificationsIntegration, string) (*models.NotificationsIntegration, error) {
	return nil, nil
}

func (r *notificationsIntegrationsRepo) Delete(context.Context, string) error {
	return nil
}

func (r *notificationsIntegrationsRepo) FindByID(context.Context, string) (*models.NotificationsIntegration, error) {
	return nil, nil
}

func (r *notificationsIntegrationsRepo) ListAll(context.Context) ([]*models.NotificationsIntegration, error) {
	// var model models.NotificationsIntegration
	// var count int64
	// if err := r.Db.Model(&model).Count(&count).Error; err != nil {
	// 	return nil, err
	// }

	var results []*models.NotificationsIntegration
	if err := r.Db.Preload(clause.Associations).Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
