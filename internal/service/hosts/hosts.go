package hosts

import (
	"context"
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/common"
	biz "github.com/project-kessel/inventory-api/internal/biz/hosts"
	"github.com/project-kessel/inventory-api/internal/middleware"
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

	var tags []*common.Tag
	for _, t := range r.Host.Metadata.Tags {
		tags = append(tags, &common.Tag{Key: t.Key, Value: t.Value})
	}

	return &biz.Host{
		Metadata: common.Metadata{
			ID:           r.Host.Metadata.Id,
			ResourceType: r.Host.Metadata.ResourceType,
			Workspace:    r.Host.Metadata.Workspace,
			Tags:         tags,

			FirstReportedBy: identity.Principal,
			LastReportedBy:  identity.Principal,

			FirstReported: time.Now(),
			LastReported:  time.Now(),
		},
		Reporters: []*common.Reporter{{
			ReporterID:      identity.Principal,
			ReporterType:    r.Host.ReporterData.ReporterType.String(),
			ReporterVersion: r.Host.ReporterData.ReporterVersion,

			LocalResourceId: r.Host.ReporterData.LocalResourceId,

			ConsoleHref: r.Host.ReporterData.ConsoleHref,
			ApiHref:     r.Host.ReporterData.ApiHref,
		}},
	}, nil
}

func createResponseFromHost(h *biz.Host) *v1beta1.CreateRHELHostResponse {
	var tags []*v1beta1.ResourceTag
	for _, t := range h.Metadata.Tags {
		tags = append(tags, &v1beta1.ResourceTag{Key: t.Key, Value: t.Value})
	}

	var reporters []*v1beta1.ReporterData
	for _, r := range h.Reporters {
		reporters = append(reporters, &v1beta1.ReporterData{
			ReporterInstanceId: r.ReporterID,
			ReporterType:       v1beta1.ReporterData_ReporterTypeEnum(v1beta1.ReporterData_ReporterTypeEnum_value[r.ReporterType]),
			ReporterVersion:    r.ReporterVersion,

			LocalResourceId: r.LocalResourceId,

			ConsoleHref: r.ConsoleHref,
			ApiHref:     r.ApiHref,
		})
	}

	return &v1beta1.CreateRHELHostResponse{
		Host: &v1beta1.RHELHost{
			Metadata: &v1beta1.Metadata{
				Id:              h.Metadata.ID,
				ResourceType:    h.Metadata.ResourceType,
				FirstReported:   timestamppb.New(h.Metadata.FirstReported),
				LastReported:    timestamppb.New(h.Metadata.LastReported),
				FirstReportedBy: h.Metadata.FirstReportedBy,
				LastReportedBy:  h.Metadata.LastReportedBy,
				Tags:            tags,
			},
			Reporters: reporters,
		}}
}
