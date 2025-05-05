package notificationsintegrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"

	"google.golang.org/grpc"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/project-kessel/inventory-api/internal/biz/model"

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

// NewKesselNotificationsIntegrationsServiceV1beta1 creates a new NotificationsIntegrationsService to handle requests for Notifications Integrations
func NewKesselNotificationsIntegrationsServiceV1beta1(c *resources.Usecase) *NotificationsIntegrationsService {
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

// UpdateNotificationsIntegrations deprecated
func (c *NotificationsIntegrationsService) UpdateNotificationsIntegrations(stream grpc.ClientStreamingServer[pb.UpdateNotificationsIntegrationsRequest, pb.UpdateNotificationsIntegrationsResponse]) error {
	// authn streaming middleware does authenticate, but it does not currently pass the identity to the stream context
	// we hardcode the identity here
	identity := &authnapi.Identity{
		Principal: "UpdateNotificationsIntegrations-bulk-upload",
	}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, identity)

	upserts := 0
	for {
		req, streamErr := stream.Recv()
		if streamErr != nil {
			if req == nil && errors.Is(streamErr, io.EOF) {
				return stream.SendAndClose(&pb.UpdateNotificationsIntegrationsResponse{UpsertsCompleted: int32(upserts)})
			}
			return streamErr
		}

		if _, err := c.UpdateNotificationsIntegration(ctx, &pb.UpdateNotificationsIntegrationRequest{
			Integration: req.GetIntegration(),
		}); err != nil {
			return err
		}
		upserts++
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
	// ignore identity for a sec, no streaming middleware setup.
	// Message: Expected *api.Identity
	// _, err := middleware.GetIdentity(ctx)
	// if err != nil {
	// return err
	// }

	log.Info(fmt.Sprintf("ListNotificationsIntegrations: %+v", r))

	resources, errs, err := c.Ctl.ListResourcesInWorkspace(ctx, r.GetRelation(), r.ResourceType.GetNamespace(), &v1beta1.SubjectReference{
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

	log.Infof("ListNotificationsIntegrations: got some resources %v", resources)

	for resource := range resources {
		re, err := notificationsIntegrationFromResource(resource)
		if err != nil {
			return fmt.Errorf("failed to send integrations: %w", err)
		}

		log.Info(fmt.Sprintf("Resource %+v converted to notificationIntegration %+v", resource, re))

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
	return conv.ResourceFromPbv1beta1(ResourceType, identity.Principal, nil, r.Integration.Metadata, r.Integration.ReporterData), nil
}

func createResponseFromNotificationsIntegration(h *model.Resource) *pb.CreateNotificationsIntegrationResponse {
	return &pb.CreateNotificationsIntegrationResponse{}
}

func notificationsIntegrationFromUpdateRequest(r *pb.UpdateNotificationsIntegrationRequest, identity *authnapi.Identity) (*model.Resource, error) {
	return conv.ResourceFromPbv1beta1(ResourceType, identity.Principal, nil, r.Integration.Metadata, r.Integration.ReporterData), nil
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

func notificationsIntegrationFromResource(r *model.Resource) (*pb.NotificationsIntegration, error) {
	return &pb.NotificationsIntegration{
		Metadata: &pb.Metadata{
			Id:           r.ID.String(),
			ResourceType: ResourceType,
			CreatedAt:    time_to_timestamp(r.CreatedAt),
			UpdatedAt:    time_to_timestamp(r.UpdatedAt),
			DeletedAt:    nil,
			OrgId:        r.OrgId,
			WorkspaceId:  r.WorkspaceId,
		},
		ReporterData: &pb.ReporterData{
			ReporterInstanceId: r.ReporterId,
			ConsoleHref:        r.ConsoleHref,
			ApiHref:            r.ApiHref,
			LocalResourceId:    r.ReporterResourceId,
			// ReporterVersion:    r.Reporter.ReporterVersion,
		},
	}, nil
}

func time_to_timestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}

	return timestamppb.New(*t)
}
