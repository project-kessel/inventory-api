package relationships

import (
	"context"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
	controller "github.com/project-kessel/inventory-api/internal/biz/relationships"
)

// RelationshipsService handles requests for RHEL hosts
type RelationshipsService struct {
	relationships.UnimplementedKesselPolicyRelationshipServiceServer

	Controller *controller.RelationshipUsecase
}

// New creates a new RelationshipsService to handle requests for RHEL hosts
func New(c *controller.RelationshipUsecase) *RelationshipsService {
	return &RelationshipsService{
		Controller: c,
	}
}

func (c *RelationshipsService) CreateRelationship(ctx context.Context, r *relationships.CreatePolicyRelationshipRequest) (*relationships.CreatePolicyRelationshipResponse, error) {
	return nil, nil
}

func (c *RelationshipsService) UpdateResourceRelationshipByUrnHs(ctx context.Context, r *relationships.UpdateResourceRelationshipByUrnHsRequest) (*relationships.UpdateResourceRelationshipByUrnHsResponse, error) {
	return nil, nil
}

func (c *RelationshipsService) DeleteResourceRelationshipByUrn(ctx context.Context, r *relationships.DeleteResourceRelationshipByUrnRequest) (*relationships.DeleteResourceRelationshipByUrnResponse, error) {
	return nil, nil
}
