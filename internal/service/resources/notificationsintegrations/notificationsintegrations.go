package notificationsintegrations

import (
	"context"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/resources"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
)

const (
	ResourceType = "integration"
)

// NotificationsIntegrationsService handles requests for Notifications Integrations
type NotificationsIntegrationsService struct {
	pb.UnimplementedKesselNotificationsIntegrationServiceServer

	Ctl *resources.Usecase
}

// New creates a new NotificationsIntegrationsService to handle requests for Notifications Integrations
func New(c *resources.Usecase) *NotificationsIntegrationsService {
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
		if resp, err := c.Ctl.Update(ctx, h, model.ReporterResourceIdFromResource(h)); err == nil {
			return updateResponseFromNotificationsIntegration(resp), nil

		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *NotificationsIntegrationsService) DeleteNotificationsIntegration(ctx context.Context, r *pb.DeleteNotificationsIntegrationRequest) (*pb.DeleteNotificationsIntegrationResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if resourceId, err := fromDeleteRequest(r, identity); err == nil {
		if err := c.Ctl.Delete(ctx, resourceId); err == nil {
			return toDeleteResponse(), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func notificationsIntegrationFromCreateRequest(r *pb.CreateNotificationsIntegrationRequest, identity *authnapi.Identity) (*model.Resource, error) {
	return conv.ResourceFromPb(ResourceType, identity.Principal, nil, r.Integration.Metadata, r.Integration.ReporterData), nil
}

func createResponseFromNotificationsIntegration(h *model.Resource) *pb.CreateNotificationsIntegrationResponse {
	return &pb.CreateNotificationsIntegrationResponse{}
}

func notificationsIntegrationFromUpdateRequest(r *pb.UpdateNotificationsIntegrationRequest, identity *authnapi.Identity) (*model.Resource, error) {
	return conv.ResourceFromPb(ResourceType, identity.Principal, nil, r.Integration.Metadata, r.Integration.ReporterData), nil
}

func updateResponseFromNotificationsIntegration(h *model.Resource) *pb.UpdateNotificationsIntegrationResponse {
	return &pb.UpdateNotificationsIntegrationResponse{}
}

func fromDeleteRequest(r *pb.DeleteNotificationsIntegrationRequest, identity *authnapi.Identity) (model.ReporterResourceId, error) {
	return conv.ReporterResourceIdFromPb(ResourceType, identity.Principal, r.ReporterData), nil
}

func toDeleteResponse() *pb.DeleteNotificationsIntegrationResponse {
	return &pb.DeleteNotificationsIntegrationResponse{}
}
