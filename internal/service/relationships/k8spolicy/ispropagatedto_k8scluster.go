package k8spolicy

import (
	"context"
	"fmt"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/relationships/k8spolicy"
	controller "github.com/project-kessel/inventory-api/internal/biz/relationships/k8spolicy"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
)

// K8SPolicyIsPropagatedToK8SClusterService handles requests for RHEL hosts
type K8SPolicyIsPropagatedToK8SClusterService struct {
	relationships.UnimplementedKesselK8SPolicyIsPropagatedToK8SClusterServiceServer

	Controller *controller.K8SPolicyIsPropagatedToK8SClusterUsecase
}

// New creates a new K8SPolicyIsPropagatedToK8SClusterService to handle requests for RHEL hosts
func New(c *controller.K8SPolicyIsPropagatedToK8SClusterUsecase) *K8SPolicyIsPropagatedToK8SClusterService {
	return &K8SPolicyIsPropagatedToK8SClusterService{
		Controller: c,
	}
}

func (c *K8SPolicyIsPropagatedToK8SClusterService) CreateK8SPolicyIsPropagatedToK8SCluster(ctx context.Context, r *relationships.CreateK8SPolicyIsPropagatedToK8SClusterRequest) (*relationships.CreateK8SPolicyIsPropagatedToK8SClusterResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if input, err := fromCreateRequest(r, identity); err == nil {
		input.Metadata.RelationshipType = biz.RelationType
		if resp, err := c.Controller.Create(ctx, input); err == nil {
			return toCreateResponse(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *K8SPolicyIsPropagatedToK8SClusterService) UpdateK8SPolicyIsPropagatedToK8SCluster(ctx context.Context, r *relationships.UpdateK8SPolicyIsPropagatedToK8SClusterRequest) (*relationships.UpdateK8SPolicyIsPropagatedToK8SClusterResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if input, err := fromUpdateRequest(r, identity); err == nil {
		input.Metadata.RelationshipType = biz.RelationType
		// Todo: Update to use the ID - it probably requires a change in the ORM
		if resp, err := c.Controller.Update(ctx, input, ""); err == nil {
			return toUpdateResponse(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *K8SPolicyIsPropagatedToK8SClusterService) DeleteK8SPolicyIsPropagatedToK8SCluster(ctx context.Context, r *relationships.DeleteK8SPolicyIsPropagatedToK8SClusterRequest) (*relationships.DeleteK8SPolicyIsPropagatedToK8SClusterResponse, error) {
	if input, err := fromDeleteRequest(r); err == nil {
		if err := c.Controller.Delete(ctx, input); err == nil {
			return toDeleteResponse(), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func fromCreateRequest(r *relationships.CreateK8SPolicyIsPropagatedToK8SClusterRequest, identity *authnapi.Identity) (*biz.K8SPolicyIsPropagatedToK8SCluster, error) {
	var metadata = &relationships.Metadata{}
	if r.K8SpolicyIspropagatedtoK8Scluster.Metadata != nil {
		metadata = r.K8SpolicyIspropagatedtoK8Scluster.Metadata
	}

	return &biz.K8SPolicyIsPropagatedToK8SCluster{
		Metadata: *conv.RelationshipMetadataFromPb(metadata, r.K8SpolicyIspropagatedtoK8Scluster.ReporterData, identity),
		Status:   r.K8SpolicyIspropagatedtoK8Scluster.RelationshipData.Status.String(),
	}, nil
}

func toCreateResponse(*biz.K8SPolicyIsPropagatedToK8SCluster) *relationships.CreateK8SPolicyIsPropagatedToK8SClusterResponse {
	return &relationships.CreateK8SPolicyIsPropagatedToK8SClusterResponse{}
}

func fromUpdateRequest(r *relationships.UpdateK8SPolicyIsPropagatedToK8SClusterRequest, identity *authnapi.Identity) (*biz.K8SPolicyIsPropagatedToK8SCluster, error) {
	var metadata = &relationships.Metadata{}
	if r.K8SpolicyIspropagatedtoK8Scluster.Metadata != nil {
		metadata = r.K8SpolicyIspropagatedtoK8Scluster.Metadata
	}

	return &biz.K8SPolicyIsPropagatedToK8SCluster{
		Metadata: *conv.RelationshipMetadataFromPb(metadata, r.K8SpolicyIspropagatedtoK8Scluster.ReporterData, identity),
		Status:   r.K8SpolicyIspropagatedtoK8Scluster.RelationshipData.Status.String(),
	}, nil
}

func toUpdateResponse(*biz.K8SPolicyIsPropagatedToK8SCluster) *relationships.UpdateK8SPolicyIsPropagatedToK8SClusterResponse {
	return &relationships.UpdateK8SPolicyIsPropagatedToK8SClusterResponse{}
}

func fromDeleteRequest(r *relationships.DeleteK8SPolicyIsPropagatedToK8SClusterRequest) (string, error) {
	// Todo: Find out what IDs are we going to be using - is it inventory ids? or resources from reporters?
	return fmt.Sprintf("%s_%s", r.ReporterData.ObjectLocalResourceId, r.ReporterData.SubjectLocalResourceId), nil
}

func toDeleteResponse() *relationships.DeleteK8SPolicyIsPropagatedToK8SClusterResponse {
	return &relationships.DeleteK8SPolicyIsPropagatedToK8SClusterResponse{}
}
