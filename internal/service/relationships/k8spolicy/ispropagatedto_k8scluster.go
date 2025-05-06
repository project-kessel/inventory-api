package k8spolicy

import (
	"context"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	relationshipsctl "github.com/project-kessel/inventory-api/internal/biz/usecase/relationships"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
)

const RelationType = "k8s-policy_is-propagated-to_k8s-cluster"

// K8SPolicyIsPropagatedToK8SClusterService handles requests for RHEL hosts
type K8SPolicyIsPropagatedToK8SClusterService struct {
	pb.UnimplementedKesselK8SPolicyIsPropagatedToK8SClusterServiceServer

	Controller *relationshipsctl.Usecase
}

// NewKesselK8SPolicyIsPropagatedToK8SClusterServiceV1beta1 creates a new K8SPolicyIsPropagatedToK8SClusterService to handle requests for RHEL hosts
func NewKesselK8SPolicyIsPropagatedToK8SClusterServiceV1beta1(c *relationshipsctl.Usecase) *K8SPolicyIsPropagatedToK8SClusterService {
	return &K8SPolicyIsPropagatedToK8SClusterService{
		Controller: c,
	}
}

func (c *K8SPolicyIsPropagatedToK8SClusterService) CreateK8SPolicyIsPropagatedToK8SCluster(ctx context.Context, r *pb.CreateK8SPolicyIsPropagatedToK8SClusterRequest) (*pb.CreateK8SPolicyIsPropagatedToK8SClusterResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if input, err := fromCreateRequest(r, identity); err == nil {
		if resp, err := c.Controller.Create(ctx, input); err == nil {
			return toCreateResponse(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *K8SPolicyIsPropagatedToK8SClusterService) UpdateK8SPolicyIsPropagatedToK8SCluster(ctx context.Context, r *pb.UpdateK8SPolicyIsPropagatedToK8SClusterRequest) (*pb.UpdateK8SPolicyIsPropagatedToK8SClusterResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if input, err := fromUpdateRequest(r, identity); err == nil {
		// Todo: Update to use the ID - it probably requires a change in the ORM
		if resp, err := c.Controller.Update(ctx, input, model.ReporterRelationshipIdFromRelationship(input)); err == nil {
			return toUpdateResponse(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *K8SPolicyIsPropagatedToK8SClusterService) DeleteK8SPolicyIsPropagatedToK8SCluster(ctx context.Context, r *pb.DeleteK8SPolicyIsPropagatedToK8SClusterRequest) (*pb.DeleteK8SPolicyIsPropagatedToK8SClusterResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if input, err := fromDeleteRequest(r, identity); err == nil {
		if err := c.Controller.Delete(ctx, input); err == nil {
			return toDeleteResponse(), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func fromCreateRequest(r *pb.CreateK8SPolicyIsPropagatedToK8SClusterRequest, identity *authnapi.Identity) (*model.Relationship, error) {
	relationshipData, err := conv.ToJsonObject(r.K8SpolicyIspropagatedtoK8Scluster.RelationshipData)
	if err != nil {
		return nil, err
	}

	return conv.RelationshipFromPb(RelationType, identity.Principal, relationshipData, r.K8SpolicyIspropagatedtoK8Scluster.Metadata, r.K8SpolicyIspropagatedtoK8Scluster.ReporterData)
}

func toCreateResponse(relationship *model.Relationship) *pb.CreateK8SPolicyIsPropagatedToK8SClusterResponse {
	return &pb.CreateK8SPolicyIsPropagatedToK8SClusterResponse{}
}

func fromUpdateRequest(r *pb.UpdateK8SPolicyIsPropagatedToK8SClusterRequest, identity *authnapi.Identity) (*model.Relationship, error) {
	relationshipData, err := conv.ToJsonObject(r.K8SpolicyIspropagatedtoK8Scluster.RelationshipData)
	if err != nil {
		return nil, err
	}

	return conv.RelationshipFromPb(RelationType, identity.Principal, relationshipData, r.K8SpolicyIspropagatedtoK8Scluster.Metadata, r.K8SpolicyIspropagatedtoK8Scluster.ReporterData)
}

func toUpdateResponse(relationship *model.Relationship) *pb.UpdateK8SPolicyIsPropagatedToK8SClusterResponse {
	return &pb.UpdateK8SPolicyIsPropagatedToK8SClusterResponse{}
}

func fromDeleteRequest(r *pb.DeleteK8SPolicyIsPropagatedToK8SClusterRequest, identity *authnapi.Identity) (model.ReporterRelationshipId, error) {
	return conv.ReporterRelationshipIdFromPb(RelationType, identity.Principal, r.ReporterData)
}

func toDeleteResponse() *pb.DeleteK8SPolicyIsPropagatedToK8SClusterResponse {
	return &pb.DeleteK8SPolicyIsPropagatedToK8SClusterResponse{}
}
