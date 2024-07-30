package hosts

import (
	"context"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	controller "github.com/project-kessel/inventory-api/internal/biz/hosts"
)

// HostsService handles requests for RHEL hosts
type HostsService struct {
	v1beta1.UnimplementedHostsServiceServer

	Controller *controller.HostUsecase
}

// New creates a new HostsService to handle requests for RHEL hosts
func New(c *controller.HostUsecase) *HostsService {
	return &HostsService{
		Controller: c,
	}
}

func (c *HostsService) CreateRHELHost(ctx context.Context, r *v1beta1.CreateRHELHostRequest) (*v1beta1.CreateRHELHostResponse, error) {
	return nil, nil
}

func (c *HostsService) UpdateRHELHost(ctx context.Context, r *v1beta1.UpdateRHELHostRequest) (*v1beta1.UpdateRHELHostResponse, error) {
	return nil, nil
}

func (c *HostsService) DeleteRHELHost(ctx context.Context, r *v1beta1.DeleteRHELHostRequest) (*v1beta1.DeleteRHELHostResponse, error) {
	return nil, nil
}
