package relationships

import (
	"context"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	controller "github.com/project-kessel/inventory-api/internal/biz/relationships"
)

// RelationshipsService handles requests for RHEL hosts
type RelationshipsService struct {
	v1beta1.UnimplementedKesselPolicyRelationshipServiceServer

	Controller *controller.RelationshipUsecase
}

// New creates a new RelationshipsService to handle requests for RHEL hosts
func New(c *controller.RelationshipUsecase) *RelationshipsService {
	return &RelationshipsService{
		Controller: c,
	}
}

func (c *RelationshipsService) CreateRelationship(ctx context.Context, r *v1beta1.CreatePolicyRelationshipRequest) (*v1beta1.CreatePolicyRelationshipResponse, error) {
	return nil, nil
}

func (c *RelationshipsService) UpdateResourceRelationshipByUrnHs(ctx context.Context, r *v1beta1.UpdateResourceRelationshipByUrnHsRequest) (*v1beta1.UpdateResourceRelationshipByUrnHsResponse, error) {
	return nil, nil
}

func (c *RelationshipsService) DeleteResourceRelationshipByUrn(ctx context.Context, r *v1beta1.DeleteResourceRelationshipByUrnRequest) (*v1beta1.DeleteResourceRelationshipByUrnResponse, error) {
	return nil, nil
}
