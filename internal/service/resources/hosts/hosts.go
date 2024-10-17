package hosts

import (
	"context"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
)

const (
	ResourceType = "rhel-host"
)

// HostsService handles requests for Rhel hosts
type HostsService struct {
	pb.UnimplementedKesselRhelHostServiceServer

	Ctl *resources.Usecase
}

// New creates a new HostsService to handle requests for Rhel hosts
func New(c *resources.Usecase) *HostsService {
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
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if h, err := hostFromUpdateRequest(r, identity); err == nil {
		// Todo: Update to use the right ID
		if resp, err := c.Ctl.Update(ctx, h, model.ReporterResourceId{
			LocalResourceId: r.RhelHost.ReporterData.LocalResourceId,
			ResourceType:    ResourceType,
			ReporterId:      identity.Principal,
		}); err == nil {
			return updateResponseFromHost(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *HostsService) DeleteRhelHost(ctx context.Context, r *pb.DeleteRhelHostRequest) (*pb.DeleteRhelHostResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if localResourceId, err := fromDeleteRequest(r); err == nil {
		if err := c.Ctl.Delete(ctx, model.ReporterResourceId{
			LocalResourceId: localResourceId,
			ResourceType:    ResourceType,
			ReporterId:      identity.Principal,
		}); err == nil {
			return toDeleteResponse(), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func hostFromCreateRequest(r *pb.CreateRhelHostRequest, identity *authnapi.Identity) (*model.Resource, error) {
	return conv.ResourceFromPb(ResourceType, identity.Principal, nil, r.RhelHost.Metadata, r.RhelHost.ReporterData), nil
}

func createResponseFromHost(resource *model.Resource) *pb.CreateRhelHostResponse {
	return &pb.CreateRhelHostResponse{}
}

func hostFromUpdateRequest(r *pb.UpdateRhelHostRequest, identity *authnapi.Identity) (*model.Resource, error) {
	return conv.ResourceFromPb(ResourceType, identity.Principal, nil, r.RhelHost.Metadata, r.RhelHost.ReporterData), nil
}

func updateResponseFromHost(resource *model.Resource) *pb.UpdateRhelHostResponse {
	return &pb.UpdateRhelHostResponse{}
}

func fromDeleteRequest(r *pb.DeleteRhelHostRequest) (string, error) {
	// Todo: Find out what IDs are we going to be using - is it inventory ids? or resources from reporters?
	return r.ReporterData.LocalResourceId, nil
}

func toDeleteResponse() *pb.DeleteRhelHostResponse {
	return &pb.DeleteRhelHostResponse{}
}
