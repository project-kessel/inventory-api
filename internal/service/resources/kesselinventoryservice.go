package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/go-kratos/kratos/v2/log"
	pbv1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
)

type InventoryService struct {
	pb.UnimplementedKesselInventoryServiceServer
	Ctl *resources.Usecase
}

func NewKesselInventoryServiceV1beta2(c *resources.Usecase) *InventoryService {
	return &InventoryService{
		Ctl: c,
	}
}

func (c *InventoryService) ReportResource(ctx context.Context, r *pb.ReportResourceRequest) (*pb.ReportResourceResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	resource, err := requestToResource(r, identity)
	if err != nil {
		return nil, err
	}
	_, err = c.Ctl.Upsert(ctx, resource, r.GetWaitForSync())
	log.Info()
	if err != nil {
		return nil, err
	}
	return responseFromResource(), nil
}

func (c *InventoryService) DeleteResource(ctx context.Context, r *pb.DeleteResourceRequest) (*pb.DeleteResourceResponse, error) {

	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get identity: %w", err)
	}

	reporterResource, err := requestToDeleteResource(r, identity)
	if err != nil {
		log.Error("Failed to build reporter resource ID: ", err)
		return nil, fmt.Errorf("failed to build reporter resource ID: %w", err)
	}

	err = c.Ctl.Delete(ctx, reporterResource)
	if err != nil {
		log.Error("Failed to delete resource: ", err)
		return nil, fmt.Errorf("failed to delete resource: %w", err)
	}

	return responseFromDeleteResource(), nil
}

func (s *InventoryService) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if resource, err := authzFromRequestV1beta2(identity, req.Object); err == nil {
		if resp, err := s.Ctl.Check(ctx, req.GetRelation(), req.Object.Reporter.GetType(), &pbv1beta1.SubjectReference{
			Relation: req.GetSubject().Relation,
			Subject: &pbv1beta1.ObjectReference{
				Type: &pbv1beta1.ObjectType{
					Namespace: req.GetSubject().Resource.GetReporter().GetType(),
					Name:      req.GetSubject().Resource.GetResourceType(),
				},
				Id: req.GetSubject().Resource.GetResourceId(),
			},
		}, *resource); err == nil {
			return viewResponseFromAuthzRequestV1beta2(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (s *InventoryService) CheckForUpdate(ctx context.Context, req *pb.CheckForUpdateRequest) (*pb.CheckForUpdateResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if resource, err := authzFromRequestV1beta2(identity, req.Object); err == nil {
		if resp, err := s.Ctl.CheckForUpdate(ctx, req.GetRelation(), req.Object.Reporter.GetType(), &pbv1beta1.SubjectReference{
			Relation: req.GetSubject().Relation,
			Subject: &pbv1beta1.ObjectReference{
				Type: &pbv1beta1.ObjectType{
					Namespace: req.GetSubject().Resource.GetReporter().GetType(),
					Name:      req.GetSubject().Resource.GetResourceType(),
				},
				Id: req.GetSubject().Resource.GetResourceId(),
			},
		}, *resource); err == nil {
			return updateResponseFromAuthzRequestV1beta2(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (s *InventoryService) LookupResources(
	req *pb.StreamedListObjectsRequest,
	stream pb.KesselInventoryService_StreamedListObjectsServer,
) error {
	ctx := stream.Context()
	clientStream, err := s.Ctl.LookupResources(ctx, toLookupResourceRequest(req))
	if err != nil {
		return fmt.Errorf("failed to retrieve resources: %w", err)
	}

	for {
		// Receive next message from the server stream
		resp, err := clientStream.Recv()
		if err == io.EOF {
			// Stream ended successfully
			return nil
		}
		if err != nil {
			return fmt.Errorf("error receiving resource: %w", err)
		}

		// Convert and send the response to the client
		if err := stream.Send(toLookupResourceResponse(resp)); err != nil {
			return fmt.Errorf("error sending resource to client: %w", err)
		}
	}
}

func toLookupResourceRequest(request *pb.StreamedListObjectsRequest) *pbv1beta1.LookupResourcesRequest {
	if request == nil {
		return nil
	}
	var pagination *pbv1beta1.RequestPagination
	if request.Pagination != nil {
		pagination = &pbv1beta1.RequestPagination{
			Limit:             request.Pagination.Limit,
			ContinuationToken: request.Pagination.ContinuationToken,
		}
	}
	return &pbv1beta1.LookupResourcesRequest{
		ResourceType: &pbv1beta1.ObjectType{
			Namespace: request.ObjectType.GetReporterType(),
			Name:      request.ObjectType.GetResourceType(),
		},
		Relation: request.Relation,
		Subject: &pbv1beta1.SubjectReference{
			Relation: request.Subject.Relation,
			Subject: &pbv1beta1.ObjectReference{
				Type: &pbv1beta1.ObjectType{
					Name:      request.Subject.Resource.GetResourceType(),
					Namespace: request.Subject.Resource.GetReporter().GetType(),
				},
				Id: request.Subject.Resource.GetResourceId(),
			},
		},
		Pagination: pagination,
	}
}

func toLookupResourceResponse(response *pbv1beta1.LookupResourcesResponse) *pb.StreamedListObjectsResponse {
	return &pb.StreamedListObjectsResponse{
		Object: &pb.ResourceReference{
			Reporter: &pb.ReporterReference{
				Type: response.Resource.Type.Namespace,
			},
			ResourceId: response.Resource.Id,
		},
		Pagination: &pb.ResponsePagination{
			ContinuationToken: response.Pagination.ContinuationToken,
		},
	}
}

func authzFromRequestV1beta2(identity *authnapi.Identity, resource *pb.ResourceReference) (*model.ReporterResourceId, error) {
	return &model.ReporterResourceId{
		LocalResourceId: resource.ResourceId,
		ResourceType:    resource.ResourceType,
		ReporterId:      identity.Principal,
		ReporterType:    identity.Type,
	}, nil
}

func viewResponseFromAuthzRequestV1beta2(allowed bool) *pb.CheckResponse {
	if allowed {
		return &pb.CheckResponse{Allowed: pb.Allowed_ALLOWED_TRUE}
	} else {
		return &pb.CheckResponse{Allowed: pb.Allowed_ALLOWED_FALSE}
	}
}

func updateResponseFromAuthzRequestV1beta2(allowed bool) *pb.CheckForUpdateResponse {
	if allowed {
		return &pb.CheckForUpdateResponse{Allowed: pb.Allowed_ALLOWED_TRUE}
	} else {
		return &pb.CheckForUpdateResponse{Allowed: pb.Allowed_ALLOWED_FALSE}
	}
}

func requestToResource(r *pb.ReportResourceRequest, identity *authnapi.Identity) (*model.Resource, error) {
	log.Info("Report Resource Request: ", r)
	var resourceType = r.Resource.GetType()
	resourceData, err := conv.ToJsonObject(r.Resource)
	if err != nil {
		return nil, err
	}

	var workspaceId, err2 = conv.ExtractWorkspaceId(r.Resource.Representations.Common)
	if err2 != nil {
		return nil, err2
	}

	var inventoryId, err3 = conv.ExtractInventoryId(r.Resource.GetInventoryId())
	if err3 != nil {
		return nil, err3
	}
	reporterType, err := conv.ExtractReporterType(r.Resource.ReporterType)
	if err != nil {
		log.Warn("Missing reporterType")
		return nil, err
	}

	reporterInstanceId, err := conv.ExtractReporterInstanceID(r.Resource.ReporterInstanceId)
	if err != nil {
		log.Warn("Missing reporterInstanceId")
		return nil, err
	}

	return conv.ResourceFromPb(resourceType, reporterType, reporterInstanceId, identity.Principal, resourceData, workspaceId, r.Resource.Representations, inventoryId), nil
}

func requestToDeleteResource(r *pb.DeleteResourceRequest, identity *authnapi.Identity) (model.ReporterResourceId, error) {
	log.Info("Delete Resource Request: ", r)

	reference := r.GetReference()
	if reference == nil {
		return model.ReporterResourceId{}, fmt.Errorf("reference is required but was nil")
	}

	reporter := reference.GetReporter()
	if reporter == nil {
		return model.ReporterResourceId{}, fmt.Errorf("reporter is required but was nil")
	}

	localResourceId := reference.GetResourceId()
	reporterType := reporter.GetType()
	resourceType := reference.GetResourceType()

	reporterResourceId := model.ReporterResourceId{
		LocalResourceId: localResourceId,
		ReporterType:    reporterType,
		ReporterId:      identity.Principal,
		ResourceType:    resourceType,
	}

	return reporterResourceId, nil
}

func responseFromResource() *pb.ReportResourceResponse {
	return &pb.ReportResourceResponse{}
}

func responseFromDeleteResource() *pb.DeleteResourceResponse {
	return &pb.DeleteResourceResponse{}
}
