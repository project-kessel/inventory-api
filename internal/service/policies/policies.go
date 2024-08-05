package policies

import (
	"context"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	controller "github.com/project-kessel/inventory-api/internal/biz/policies"
)

// PoliciesService handles requests for RHEL hosts
type PoliciesService struct {
	v1beta1.UnimplementedKesselPolicyServiceServer

	Controller *controller.PolicyUsecase
}

// New creates a new PoliciesService to handle requests for RHEL hosts
func New(c *controller.PolicyUsecase) *PoliciesService {
	return &PoliciesService{
		Controller: c,
	}
}

func (c *PoliciesService) CreatePolicy(ctx context.Context, r *v1beta1.CreatePolicyRequest) (*v1beta1.CreatePolicyResponse, error) {
	return nil, nil
}

func (c *PoliciesService) UpdatePolicy(ctx context.Context, r *v1beta1.UpdatePolicyRequest) (*v1beta1.UpdatePolicyResponse, error) {
	return nil, nil
}

func (c *PoliciesService) DeletePolicy(ctx context.Context, r *v1beta1.DeletePolicyRequest) (*v1beta1.DeletePolicyResponse, error) {
	return nil, nil
}
