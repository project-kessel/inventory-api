package hosts

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/hosts"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
)

const (
	resource_type = "rhelhost"
)

// HostsService handles requests for RHEL hosts
type HostsService struct {
	pb.UnimplementedHostsServiceServer

	Ctl *biz.HostUsecase
}

// New creates a new HostsService to handle requests for RHEL hosts
func New(c *biz.HostUsecase) *HostsService {
	return &HostsService{
		Ctl: c,
	}
}

func (c *HostsService) CreateRHELHost(ctx context.Context, r *pb.CreateRHELHostRequest) (*pb.CreateRHELHostResponse, error) {
	if err := r.ValidateAll(); err != nil {
		return nil, err
	}

	//TODO: refactor / abstract resource type strings
	if !strings.EqualFold(r.Host.Metadata.ResourceType, resource_type) {
		return nil, errors.BadRequest("BADREQUEST", fmt.Sprintf("incorrect resource type: expected %s", resource_type))
	}

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

func (c *HostsService) UpdateRHELHost(ctx context.Context, r *pb.UpdateRHELHostRequest) (*pb.UpdateRHELHostResponse, error) {
	if err := r.ValidateAll(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (c *HostsService) DeleteRHELHost(ctx context.Context, r *pb.DeleteRHELHostRequest) (*pb.DeleteRHELHostResponse, error) {
	if err := r.ValidateAll(); err != nil {
		return nil, err
	}
	return nil, nil
}

func hostFromCreateRequest(r *pb.CreateRHELHostRequest, identity *authnapi.Identity) (*biz.Host, error) {
	if identity.Principal != r.Host.ReporterData.ReporterInstanceId {
		return nil, errors.Forbidden("FORBIDDEN", "Reporter identity must match the provided reporter instance identity")
	}

	return &biz.Host{
		Metadata: *conv.MetadataFromPb(r.Host.Metadata, r.Host.ReporterData, identity),
	}, nil
}

func createResponseFromHost(h *biz.Host) *pb.CreateRHELHostResponse {
	return &pb.CreateRHELHostResponse{
		Host: &pb.RHELHost{
			Metadata:  conv.MetadataFromModel(&h.Metadata),
			Reporters: conv.ReportersFromModel(h.Metadata.Reporters),
		},
	}
}
