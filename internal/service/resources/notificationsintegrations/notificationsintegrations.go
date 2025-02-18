package notificationsintegrations

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/resources"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/protobuf/types/known/timestamppb"

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
func (c *NotificationsIntegrationsService) ListNotificationsIntegrations(r *pb.ListNotificationsIntegrationsRequest, conn pb.KesselNotificationsIntegrationService_ListNotificationsIntegrationsServer) error {
	ctx := conn.Context()
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return err
	}

	// if resources, err := notificationsIntegrationsFromListRequest(identity, r); err == nil {
	// 	if resp, err := c.Ctl.ListNotificationsIntegrations(ctx, r.GetRelation(), r.GetResourceType().Namespace, r.GetSubject(), *resource); err == nil {
	// 		return listResponseFromNotificationsIntegrations(resp), nil
	// 	}
	// } else {
	// 	return nil, err
	// }

	resources, errs, err := c.Ctl.ListResourcesInWorkspace(ctx, r.GetRelation(), r.ResourceType.GetNamespace(), &v1beta1.SubjectReference{
		Relation: r.GetSubject().Relation,
		Subject: &v1beta1.ObjectReference{
			Type: &v1beta1.ObjectType{
				Namespace: r.GetSubject().GetSubject().GetType().GetNamespace(),
				Name:      r.GetSubject().GetSubject().GetType().GetName(),
			},
			Id: r.GetSubject().GetSubject().GetId(),
		},
	}, r.Parent.GetId())

	if err != nil {
		return fmt.Errorf("failed to retrieve integrations: %w", err)
	}

	for resource := range resources {
		re, err := notificationsIntegrationFromResource(resource)
		if err != nil {
			return fmt.Errorf("failed to send integrations: %w", err)
		}

		err = conn.Send(&pb.ListNotificationsIntegrationsResponse{
			Integrations: re,
		})

		if err != nil {
			return fmt.Errorf("error sending integrations: %w", err)
		}
	}

	err, ok := <-errs
	if ok {
		return fmt.Errorf("error while streaming: %w", err)
	}

	return nil
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

// func notificationsIntegrationsFromListRequest(identity *authnapi.Identity, r *pb.ListNotificationsIntegrationsRequest) (*model.Resource, error) {
// 	return conv.ResourceFromPb()
// }

func listResponseFromNotificationsIntegrations(h *model.Resource) *pb.ListNotificationsIntegrationsResponse {
	return &pb.ListNotificationsIntegrationsResponse{}
}

func notificationsIntegrationFromResource(r *model.Resource) (*pb.NotificationsIntegration, error) {
	var reporterType int
	reporterType, err := strconv.Atoi(r.Reporter.ReporterType)
	if err != nil {
		return nil, err
	}

	return &pb.NotificationsIntegration{
		Metadata: &pb.Metadata{
			Id:           r.ID.String(),
			ResourceType: ResourceType,
			CreatedAt:    time_to_timestamp(r.CreatedAt),
			UpdatedAt:    time_to_timestamp(r.UpdatedAt),
			DeletedAt:    nil,
			OrgId:        r.OrgId,
			WorkspaceId:  r.WorkspaceId,
			// Labels:       labels_to_pb(r.Labels),
		},
		ReporterData: &pb.ReporterData{
			ReporterType:       pb.ReporterData_ReporterType(reporterType),
			ReporterInstanceId: r.Reporter.ReporterId,
			ConsoleHref:        r.ConsoleHref,
			ApiHref:            r.ApiHref,
			LocalResourceId:    r.Reporter.LocalResourceId,
			ReporterVersion:    r.Reporter.ReporterVersion,
		},
	}, nil
}

func time_to_timestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}

	return timestamppb.New(*t)
}
