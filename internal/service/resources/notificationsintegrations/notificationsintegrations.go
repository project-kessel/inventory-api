package notificationsintegrations

import (
	"context"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/resources/notificationsintegrations"
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
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if h, err := notificationsIntegrationFromUpdateRequest(r, identity); err == nil {
		// Todo: Update to use the right ID
		if resp, err := c.Ctl.Update(ctx, h, ""); err == nil {
			return updateResponseFromNotificationsIntegration(resp), nil

		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *NotificationsIntegrationsService) DeleteNotificationsIntegration(ctx context.Context, r *pb.DeleteNotificationsIntegrationRequest) (*pb.DeleteNotificationsIntegrationResponse, error) {
	if input, err := fromDeleteRequest(r); err == nil {
		if err := c.Ctl.Delete(ctx, input); err == nil {
			return toDeleteResponse(), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
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
	return &pb.CreateNotificationsIntegrationResponse{}
}

func notificationsIntegrationFromUpdateRequest(r *pb.UpdateNotificationsIntegrationRequest, identity *authnapi.Identity) (*biz.NotificationsIntegration, error) {
	var metadata = &pb.Metadata{}
	if r.Integration.Metadata != nil {
		metadata = r.Integration.Metadata
	}

	return &biz.NotificationsIntegration{
		Metadata: *conv.MetadataFromPb(metadata, r.Integration.ReporterData, identity),
	}, nil
}

func updateResponseFromNotificationsIntegration(h *biz.NotificationsIntegration) *pb.UpdateNotificationsIntegrationResponse {
	return &pb.UpdateNotificationsIntegrationResponse{}
}

func fromDeleteRequest(r *pb.DeleteNotificationsIntegrationRequest) (string, error) {
	// Todo: Find out what IDs are we going to be using - is it inventory ids? or resources from reporters?
	return r.ReporterData.LocalResourceId, nil
}

func toDeleteResponse() *pb.DeleteNotificationsIntegrationResponse {
	return &pb.DeleteNotificationsIntegrationResponse{}
}
