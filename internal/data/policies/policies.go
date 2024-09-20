package policies

import (
	"context"
	"time"

	"gorm.io/gorm"

	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/policies"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
)

type policiesRepo struct {
	DB      *gorm.DB
	Authz   authzapi.Authorizer
	Eventer eventingapi.Manager
}

func New(g *gorm.DB, a authzapi.Authorizer, e eventingapi.Manager) *policiesRepo {
	return &policiesRepo{
		DB:      g,
		Authz:   a,
		Eventer: e,
	}
}

func (r *policiesRepo) Save(ctx context.Context, model *biz.Policy) (*biz.Policy, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, nil
	}

	if err := r.DB.Session(&gorm.Session{FullSaveAssociations: true}).Create(model).Error; err != nil {
		return nil, err
	}

	if r.Eventer != nil {
		producer, _ := r.Eventer.Lookup(identity, biz.ResourceType, model.ID)
		// TODO: Update the Object that's sent.  This is going to be what we actually emit.
		evt := eventingapi.NewAddEvent(biz.ResourceType, model.Metadata.UpdatedAt, &eventingapi.EventResource[biz.PolicyDetail]{
			Metadata:     &model.Metadata,
			ReporterData: model.Metadata.Reporters[0],
			ResourceData: model.ResourceData,
		})
		err = producer.Produce(ctx, evt)
		if err != nil {
			return nil, err
		}
	}
	return model, nil
}

// Update updates a model in the database, updates related tuples in the relations-api, and issues an update event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *policiesRepo) Update(ctx context.Context, model *biz.Policy, id string) (*biz.Policy, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, nil
	}

	// TODO: update the model in inventory

	if r.Eventer != nil {
		producer, _ := r.Eventer.Lookup(identity, biz.ResourceType, model.ID)
		evt := eventingapi.NewUpdateEvent(biz.ResourceType, model.Metadata.UpdatedAt, &eventingapi.EventResource[biz.PolicyDetail]{
			Metadata:     &model.Metadata,
			ReporterData: model.Metadata.Reporters[0],
			ResourceData: model.ResourceData,
		})
		err = producer.Produce(ctx, evt)
		if err != nil {
			return nil, err
		}
	}
	return model, nil
}

// Delete deletes a model from the database, removes related tuples from the relations-api, and issues a delete event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *policiesRepo) Delete(ctx context.Context, id string) error {
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
	return nil
}

func (r *policiesRepo) FindByID(context.Context, string) (*biz.Policy, error) {
	return nil, nil
}

func (r *policiesRepo) ListAll(context.Context) ([]*biz.Policy, error) {
	return nil, nil
}
