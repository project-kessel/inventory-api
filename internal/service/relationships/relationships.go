package relationships

import (
	"context"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	controller "github.com/project-kessel/inventory-api/internal/biz/relationships"
)

// RelationshipsService handles requests for RHEL hosts
type RelationshipsService struct {
	v1beta1.UnimplementedRelationshipsServiceServer

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

func (c *RelationshipsService) UpdateResourceRelationshipByURNHs(ctx context.Context, r *v1beta1.UpdateResourceRelationshipByURNHsRequest) (*v1beta1.UpdateResourceRelationshipByURNResponse, error) {
	return nil, nil
}

func (c *RelationshipsService) DeleteResourceRelationshipByURN(ctx context.Context, r *v1beta1.DeleteResourceRelationshipByURNRequest) (*v1beta1.DeleteResourceRelationshipByURNResponse, error) {
	return nil, nil
}
