package getresource

import (
	"context"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	"github.com/project-kessel/inventory-api/internal/biz/getresources"
	"github.com/project-kessel/inventory-api/internal/service/common"
)

type GetResourceService struct {
	pb.UnimplementedResourceQueryServiceServer

	Ctl *getresources.Usecase
}

func New(c *getresources.Usecase) *GetResourceService {
	return &GetResourceService{
		Ctl: c,
	}
}

func (c *GetResourceService) GetResourceByResourceId(ctx context.Context, r *pb.GetResourceByResourceIdRequest) (*pb.GetResourceByResourceIdResponse, error) {
	resource, err := c.Ctl.FindById(ctx, r.ResourceId)
	if err != nil {
		return nil, err
	}

	pbResource, err := common.ToResourcePb(resource, nil)
	if err != nil {
		return nil, err
	}

	return &pb.GetResourceByResourceIdResponse{Resource: pbResource}, nil
}
