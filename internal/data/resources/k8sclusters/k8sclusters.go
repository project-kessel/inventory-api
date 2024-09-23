package k8sclusters

import (
	"context"
	"time"

	"gorm.io/gorm"

	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/resources/k8sclusters"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
)

const namespace = "acm"
const resourceType = "k8scluster"

type k8sclustersRepo struct {
	DB      *gorm.DB
	Authz   authzapi.Authorizer
	Eventer eventingapi.Manager
}

func New(g *gorm.DB, a authzapi.Authorizer, e eventingapi.Manager) *k8sclustersRepo {
	return &k8sclustersRepo{
		DB:      g,
		Authz:   a,
		Eventer: e,
	}
}

func (r *k8sclustersRepo) Save(ctx context.Context, model *biz.K8SCluster) (*biz.K8SCluster, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, nil
	}

	// TODO: Create the cluster in inventory

	if r.Eventer != nil {
		producer, _ := r.Eventer.Lookup(identity, biz.ResourceType, model.ID)
		evt := eventingapi.NewCreatedResourceEvent(biz.ResourceType, model.Metadata.Reporters[0].LocalResourceId, model.Metadata.UpdatedAt, &eventingapi.EventResource[biz.K8SClusterDetail]{
			Metadata:     &model.Metadata,
			ReporterData: model.Metadata.Reporters[0],
			ResourceData: model.ResourceData,
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

// Update updates a cluster in the database, updates related tuples in the relations-api, and issues an update event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *k8sclustersRepo) Update(ctx context.Context, model *biz.K8SCluster, id string) (*biz.K8SCluster, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, nil
	}

	// TODO: update the cluster in inventory

	if r.Eventer != nil {
		producer, _ := r.Eventer.Lookup(identity, biz.ResourceType, model.ID)
		evt := eventingapi.NewUpdatedResourceEvent(biz.ResourceType, model.Metadata.Reporters[0].LocalResourceId, model.Metadata.UpdatedAt, &eventingapi.EventResource[biz.K8SClusterDetail]{
			Metadata:     &model.Metadata,
			ReporterData: model.Metadata.Reporters[0],
			ResourceData: model.ResourceData,
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

// Delete deletes a cluster from the database, removes related tuples from the relations-api, and issues a delete event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *k8sclustersRepo) Delete(ctx context.Context, id string) error {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil
	}

	// TODO: delete the cluster from inventory

	if r.Eventer != nil {
		// TODO: without persistence, we can't lookup the inventory assigned id or info like the external cluster id.
		var dummyId int64 = 0
		producer, _ := r.Eventer.Lookup(identity, biz.ResourceType, dummyId)
		evt := eventingapi.NewDeletedResourceEvent(biz.ResourceType, id, time.Now().UTC(), identity)
		err = producer.Produce(ctx, evt)
		if err != nil {
			return err
		}
	}

	// TODO: delete the workspace tuple

	return nil
}

func (r *k8sclustersRepo) FindByID(context.Context, string) (*biz.K8SCluster, error) {
	return nil, nil
}

func (r *k8sclustersRepo) ListAll(context.Context) ([]*biz.K8SCluster, error) {
	return nil, nil
}
