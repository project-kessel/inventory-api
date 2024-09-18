package k8spolicy

import (
	"context"
	biz "github.com/project-kessel/inventory-api/internal/biz/relationships/k8spolicy"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"time"

	"gorm.io/gorm"
)

type K8SPolicyIsPropagatedToK8SClusterRepo struct {
	DB      *gorm.DB
	Eventer eventingapi.Manager
}

type K8SPolicyIsPropagatedToK8SClusterDetail struct {
	Status       string
	K8SPolicyId  int64
	K8SClusterId int64
}

func New(g *gorm.DB, e eventingapi.Manager) *K8SPolicyIsPropagatedToK8SClusterRepo {
	return &K8SPolicyIsPropagatedToK8SClusterRepo{
		DB:      g,
		Eventer: e,
	}
}

func (r *K8SPolicyIsPropagatedToK8SClusterRepo) Save(ctx context.Context, model *biz.K8SPolicyIsPropagatedToK8SCluster) (*biz.K8SPolicyIsPropagatedToK8SCluster, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, nil
	}

	if err := r.DB.Session(&gorm.Session{FullSaveAssociations: true}).Create(model).Error; err != nil {
		return nil, err
	}

	if r.Eventer != nil {
		// TODO: Update the Object that's sent.  This is going to be what we actually emit.
		producer, _ := r.Eventer.Lookup(identity, biz.RelationType, model.ID)
		evt := eventingapi.NewCreatedResourcesRelationshipEvent(biz.RelationType, model.Metadata.Reporters[0].SubjectLocalResourceId, model.Metadata.UpdatedAt, &eventingapi.EventRelationship[K8SPolicyIsPropagatedToK8SClusterDetail]{
			Metadata:     &model.Metadata,
			ReporterData: model.Metadata.Reporters[0],
			RelationshipData: &K8SPolicyIsPropagatedToK8SClusterDetail{
				Status: model.Status,
			},
		})
		err = producer.Produce(ctx, evt)
		if err != nil {
			return nil, err
		}
	}

	return model, nil
}

func (r *K8SPolicyIsPropagatedToK8SClusterRepo) Update(ctx context.Context, model *biz.K8SPolicyIsPropagatedToK8SCluster, id string) (*biz.K8SPolicyIsPropagatedToK8SCluster, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, nil
	}

	// TODO: update the model in inventory

	if r.Eventer != nil {
		producer, _ := r.Eventer.Lookup(identity, biz.RelationType, model.ID)
		evt := eventingapi.NewUpdatedResourcesRelationshipEvent(biz.RelationType, model.Metadata.Reporters[0].SubjectLocalResourceId, model.Metadata.UpdatedAt, &eventingapi.EventRelationship[struct{}]{
			Metadata:     &model.Metadata,
			ReporterData: model.Metadata.Reporters[0],
		})
		err = producer.Produce(ctx, evt)
		if err != nil {
			return nil, err
		}
	}

	return model, nil
}

func (r *K8SPolicyIsPropagatedToK8SClusterRepo) Delete(ctx context.Context, id string) error {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil
	}

	// TODO: delete the model from inventory

	if r.Eventer != nil {
		// TODO: without persistence, we can't lookup the inventory assigned id or other model specific info.
		var dummyId int64 = 0
		producer, _ := r.Eventer.Lookup(identity, biz.RelationType, dummyId)
		// Todo: Load the model to fetch the (subject|object)_local_resource_id to fill in the model
		evt := eventingapi.NewDeletedResourcesRelationshipEvent(biz.RelationType, id, id, time.Now().UTC(), identity)
		err = producer.Produce(ctx, evt)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *K8SPolicyIsPropagatedToK8SClusterRepo) FindByID(context.Context, string) (*biz.K8SPolicyIsPropagatedToK8SCluster, error) {
	return nil, nil
}

func (r *K8SPolicyIsPropagatedToK8SClusterRepo) ListAll(context.Context) ([]*biz.K8SPolicyIsPropagatedToK8SCluster, error) {
	return nil, nil
}
