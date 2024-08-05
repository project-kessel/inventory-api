package notificationsintegrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/notificationsintegrations"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
)

// NotificationsIntegrationsService handles requests for Notifications Integrations
type NotificationsIntegrationsService struct {
	pb.UnimplementedNotificationsIntegrationsServiceServer

	Ctl *biz.NotificationsIntegrationUsecase
}

// New creates a new NotificationsIntegrationsService to handle requests for Notifications Integrations
func New(c *biz.NotificationsIntegrationUsecase) *NotificationsIntegrationsService {
	return &NotificationsIntegrationsService{
		Ctl: c,
	}
}

func (c *NotificationsIntegrationsService) CreateNotificationsIntegration(ctx context.Context, r *pb.CreateNotificationsIntegrationRequest) (*pb.CreateNotificationsIntegrationResponse, error) {
	if err := r.ValidateAll(); err != nil {
		return nil, err
	}

	//TODO: refactor / abstract resource type strings
	if !strings.EqualFold(r.Integration.Metadata.ResourceType, biz.ResourceType) {
		return nil, errors.BadRequest("BADREQUEST", fmt.Sprintf("incorrect resource type: expected %s", biz.ResourceType))
	}

	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if h, err := notificationsIntegrationFromCreateRequest(r, identity); err == nil {
		if resp, err := c.Ctl.CreateNotificationsIntegration(ctx, h); err == nil {
			return createResponseFromNotificationsIntegration(resp), nil

		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *NotificationsIntegrationsService) UpdateNotificationsIntegration(ctx context.Context, r *pb.UpdateNotificationsIntegrationRequest) (*pb.UpdateNotificationsIntegrationResponse, error) {
	if err := r.ValidateAll(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (c *NotificationsIntegrationsService) DeleteNotificationsIntegration(ctx context.Context, r *pb.DeleteNotificationsIntegrationRequest) (*pb.DeleteNotificationsIntegrationResponse, error) {
	if err := r.ValidateAll(); err != nil {
		return nil, err
	}
	return nil, nil
}

func notificationsIntegrationFromCreateRequest(r *pb.CreateNotificationsIntegrationRequest, identity *authnapi.Identity) (*biz.NotificationsIntegration, error) {
	if identity.Principal != r.Integration.ReporterData.ReporterInstanceId {
		return nil, errors.Forbidden("FORBIDDEN", "Reporter identity must match the provided reporter instance identity")
	}

	return &biz.NotificationsIntegration{
		Metadata: *conv.MetadataFromPb(r.Integration.Metadata, r.Integration.ReporterData, identity),
	}, nil
}

func createResponseFromNotificationsIntegration(h *biz.NotificationsIntegration) *pb.CreateNotificationsIntegrationResponse {
	return &pb.CreateNotificationsIntegrationResponse{
		Integration: &pb.NotificationsIntegration{
			Metadata:  conv.MetadataFromModel(&h.Metadata),
			Reporters: conv.ReportersFromModel(h.Metadata.Reporters),
		},
	}
}
