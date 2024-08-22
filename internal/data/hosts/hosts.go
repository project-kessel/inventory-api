package hosts

import (
	"context"
	"github.com/project-kessel/inventory-api/internal/biz/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/go-kratos/kratos/v2/log"

	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/hosts"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
)

type hostsRepo struct {
	Db      *gorm.DB
	Authz   authzapi.Authorizer
	Eventer eventingapi.Manager

	Log *log.Helper
}

func New(g *gorm.DB, a authzapi.Authorizer, e eventingapi.Manager, l *log.Helper) *hostsRepo {
	return &hostsRepo{
		Db:      g,
		Authz:   a,
		Eventer: e,

		Log: l,
	}
}

func (r *hostsRepo) Save(ctx context.Context, model *biz.Host) (*biz.Host, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if err := r.Db.Session(&gorm.Session{FullSaveAssociations: true}).Create(model).Error; err != nil {
		return nil, err
	}

	if r.Eventer != nil {
		// TODO: handle eventing errors
		// TODO: Update the Object that's sent.  This is going to be what we actually emit.
		producer, _ := r.Eventer.Lookup(identity, biz.ResourceType, model.ID)
		evt := &eventingapi.Event{
			EventType:    "Create",
			ResourceType: biz.ResourceType,
			Object:       model,
		}
		err = producer.Produce(ctx, evt)
		if err != nil {
			return nil, err
		}
	}
	return model, nil
}

func (r *hostsRepo) Update(context.Context, *biz.Host, string) (*biz.Host, error) {
	return nil, nil
}

func (r *hostsRepo) Delete(context.Context, string) error {
	return nil
}

func (r *hostsRepo) FindByID(ctx context.Context, resourceId common.ResourceId) (*biz.Host, error) {
	host := biz.Host{}

	reporter := common.Reporter{}
	session := gorm.Session{}

	err := r.Db.Session(&session).Take(&reporter, "local_resource_id = ? and reporter_type = ? and reporter_id = ?", resourceId.LocalResourceId, resourceId.ReporterType, resourceId.ReporterId).Error
	if err != nil {
		return nil, err
	}

	r.Db.Session(&session).Joins("Metadata").Take(&host, "metadata.id = ?", reporter.MetadataID)
	if err != nil {
		return nil, err
	}

	return &host, nil
}

func (r *hostsRepo) ListAll(context.Context) ([]*biz.Host, error) {
	// var model biz.Host
	// var count int64
	// if err := r.Db.Model(&model).Count(&count).Error; err != nil {
	// 	return nil, err
	// }

	var results []*biz.Host
	if err := r.Db.Preload(clause.Associations).Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
