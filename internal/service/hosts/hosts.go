package hosts

import (
	"context"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	bizcommon "github.com/project-kessel/inventory-api/internal/biz/common"
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
	if err := r.ValidateAll(); err != nil {
		return nil, err
	}

	// TODO: Use in UPDATE
	////TODO: refactor / abstract resource type strings
	//if !strings.EqualFold(r.Host.Metadata.ResourceType, biz.ResourceType) {
	//	return nil, errors.BadRequest("BADREQUEST", fmt.Sprintf("incorrect resource type: expected %s", biz.ResourceType))
	//}

	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if h, err := hostFromCreateRequest(r, identity); err == nil {
		if resp, err := c.Ctl.CreateHost(ctx, h); err == nil {
			return createResponseFromHost(resp), nil

		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *HostsService) UpdateRhelHost(ctx context.Context, r *pb.UpdateRhelHostRequest) (*pb.UpdateRhelHostResponse, error) {
	if err := r.ValidateAll(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (c *HostsService) DeleteRhelHost(ctx context.Context, r *pb.DeleteRhelHostRequest) (*pb.DeleteRhelHostResponse, error) {
	if err := r.ValidateAll(); err != nil {
		return nil, err
	}
	return nil, nil
}

func hostFromCreateRequest(r *pb.CreateRhelHostRequest, identity *authnapi.Identity) (*biz.Host, error) {
	var metadata = bizcommon.Metadata{}
	if r.Host.Metadata != nil {
		metadata = *conv.MetadataFromPb(r.Host.Metadata, r.Host.ReporterData, identity)
	}

	return &biz.Host{
		Metadata: metadata,
	}, nil
}

func createResponseFromHost(h *biz.Host) *pb.CreateRhelHostResponse {
	return &pb.CreateRhelHostResponse{}
}
