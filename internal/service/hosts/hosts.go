package hosts

import (
	"context"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/hosts"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
)

// HostsService handles requests for Rhel hosts
type HostsService struct {
	pb.UnimplementedKesselRhelHostServiceServer

	Ctl *biz.HostUsecase
}

// New creates a new HostsService to handle requests for Rhel hosts
func New(c *biz.HostUsecase) *HostsService {
	return &HostsService{
		Ctl: c,
	}
}

func (c *HostsService) CreateRhelHost(ctx context.Context, r *pb.CreateRhelHostRequest) (*pb.CreateRhelHostResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if h, err := hostFromCreateRequest(r, identity); err == nil {
		h.Metadata.ResourceType = biz.ResourceType
		if resp, err := c.Ctl.Create(ctx, h); err == nil {
			return createResponseFromHost(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *HostsService) UpdateRhelHost(ctx context.Context, r *pb.UpdateRhelHostRequest) (*pb.UpdateRhelHostResponse, error) {
	return nil, nil
}

func (c *HostsService) DeleteRhelHost(ctx context.Context, r *pb.DeleteRhelHostRequest) (*pb.DeleteRhelHostResponse, error) {
	return nil, nil
}

func hostFromCreateRequest(r *pb.CreateRhelHostRequest, identity *authnapi.Identity) (*biz.Host, error) {
	var metadata = &pb.Metadata{}
	if r.RhelHost.Metadata != nil {
		metadata = r.RhelHost.Metadata
	}

	return &biz.Host{
		Metadata: *conv.MetadataFromPb(metadata, r.RhelHost.ReporterData, identity),
	}, nil
}

func createResponseFromHost(*biz.Host) *pb.CreateRhelHostResponse {
	return &pb.CreateRhelHostResponse{}
}
