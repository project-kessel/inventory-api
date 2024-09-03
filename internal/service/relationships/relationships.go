package relationships

import (
	"context"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
	controller "github.com/project-kessel/inventory-api/internal/biz/relationships"
)

// RelationshipsService handles requests for RHEL hosts
type RelationshipsService struct {
	relationships.UnimplementedKesselK8SPolicyIsPropagatedToK8SClusterServiceServer

	Controller *controller.RelationshipUsecase
}

// New creates a new RelationshipsService to handle requests for RHEL hosts
func New(c *controller.RelationshipUsecase) *RelationshipsService {
	return &RelationshipsService{
		Controller: c,
	}
}

func (c *RelationshipsService) CreateRelationship(ctx context.Context, r *relationships.CreateK8SPolicyIsPropagatedToK8SClusterRequest) (*relationships.CreateK8SPolicyIsPropagatedToK8SClusterResponse, error) {
	return nil, nil
}

func (c *RelationshipsService) UpdateResourceRelationshipByUrnHs(ctx context.Context, r *relationships.UpdateK8SPolicyIsPropagatedToK8SClusterRequest) (*relationships.UpdateK8SPolicyIsPropagatedToK8SClusterResponse, error) {
	return nil, nil
}

func (c *RelationshipsService) DeleteResourceRelationshipByUrn(ctx context.Context, r *relationships.DeleteK8SPolicyIsPropagatedToK8SClusterRequest) (*relationships.DeleteK8SPolicyIsPropagatedToK8SClusterResponse, error) {
	return nil, nil
}
