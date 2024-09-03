package notificationsintegrations

import (
	"context"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/notificationsintegrations"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
)

// NotificationsIntegrationsService handles requests for Notifications Integrations
type NotificationsIntegrationsService struct {
	pb.UnimplementedKesselNotificationsIntegrationServiceServer

	Ctl *biz.NotificationsIntegrationUsecase
}

// New creates a new NotificationsIntegrationsService to handle requests for Notifications Integrations
func New(c *biz.NotificationsIntegrationUsecase) *NotificationsIntegrationsService {
	return &NotificationsIntegrationsService{
		Ctl: c,
	}
}

func (c *NotificationsIntegrationsService) CreateNotificationsIntegration(ctx context.Context, r *pb.CreateNotificationsIntegrationRequest) (*pb.CreateNotificationsIntegrationResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if h, err := notificationsIntegrationFromCreateRequest(r, identity); err == nil {
		if resp, err := c.Ctl.Create(ctx, h); err == nil {
			return createResponseFromNotificationsIntegration(resp), nil

		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *NotificationsIntegrationsService) UpdateNotificationsIntegration(ctx context.Context, r *pb.UpdateNotificationsIntegrationRequest) (*pb.UpdateNotificationsIntegrationResponse, error) {
	return nil, nil
}

func (c *NotificationsIntegrationsService) DeleteNotificationsIntegration(ctx context.Context, r *pb.DeleteNotificationsIntegrationRequest) (*pb.DeleteNotificationsIntegrationResponse, error) {
	return nil, nil
}

func notificationsIntegrationFromCreateRequest(r *pb.CreateNotificationsIntegrationRequest, identity *authnapi.Identity) (*biz.NotificationsIntegration, error) {
	var metadata = &pb.Metadata{}
	if r.Integration.Metadata != nil {
		metadata = r.Integration.Metadata
	}

	return &biz.NotificationsIntegration{
		Metadata: *conv.MetadataFromPb(metadata, r.Integration.ReporterData, identity),
	}, nil
}

func createResponseFromNotificationsIntegration(h *biz.NotificationsIntegration) *pb.CreateNotificationsIntegrationResponse {
	return &pb.CreateNotificationsIntegrationResponse{
		Integration: &pb.NotificationsIntegration{
			Metadata: conv.MetadataFromModel(&h.Metadata),
		},
	}
}
