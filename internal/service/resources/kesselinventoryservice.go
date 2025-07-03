package resources

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
	pbv1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
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

	resource, err := RequestToResource(r, identity)
	if err != nil {
		return nil, err
	}
	_, err = c.Ctl.Upsert(ctx, resource, r.GetWriteVisibility())
	log.Info()
	if err != nil {
		return nil, err
	}
	return ResponseFromResource(), nil
}

func (c *InventoryService) DeleteResource(ctx context.Context, r *pb.DeleteResourceRequest) (*pb.DeleteResourceResponse, error) {

	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get identity: %w", err)
	}

	reporterResource, err := RequestToDeleteResource(r, identity)
	if err != nil {
		log.Error("Failed to build reporter resource ID: ", err)
		return nil, fmt.Errorf("failed to build reporter resource ID: %w", err)
	}

	err = c.Ctl.Delete(ctx, reporterResource)
	if err != nil {
		log.Error("Failed to delete resource: ", err)
		return nil, fmt.Errorf("failed to delete resource: %w", err)
	}

	return ResponseFromDeleteResource(), nil
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

func (s *InventoryService) StreamedListObjects(
	req *pb.StreamedListObjectsRequest,
	stream pb.KesselInventoryService_StreamedListObjectsServer,
) error {
	ctx := stream.Context()
	//Example: how to use get the identity from the stream context
	//identity, err := interceptor.FromContextIdentity(ctx)
	//log.Info(identity)
	lookupReq, err := ToLookupResourceRequest(req)
	if err != nil {
		return fmt.Errorf("failed to build lookup request: %w", err)
	}

	clientStream, err := s.Ctl.LookupResources(ctx, lookupReq)
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
		if err := stream.Send(ToLookupResourceResponse(resp)); err != nil {
			return fmt.Errorf("error sending resource to client: %w", err)
		}
	}
}

func ToLookupResourceRequest(request *pb.StreamedListObjectsRequest) (*pbv1beta1.LookupResourcesRequest, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}
	var pagination *pbv1beta1.RequestPagination
	if request.Pagination != nil {
		pagination = &pbv1beta1.RequestPagination{
			Limit:             request.Pagination.Limit,
			ContinuationToken: request.Pagination.ContinuationToken,
		}
	}
	normalizedNamespace := NormalizeType(request.ObjectType.GetReporterType())
	normalizedResourceType := NormalizeType(request.ObjectType.GetResourceType())

	return &pbv1beta1.LookupResourcesRequest{
		ResourceType: &pbv1beta1.ObjectType{
			Namespace: normalizedNamespace,
			Name:      normalizedResourceType,
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
	}, nil
}

func NormalizeType(val string) string {
	normalized := strings.ToLower(val)
	return normalized
}

func ToLookupResourceResponse(response *pbv1beta1.LookupResourcesResponse) *pb.StreamedListObjectsResponse {
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

func RequestToResource(r *pb.ReportResourceRequest, identity *authnapi.Identity) (*model.Resource, error) {
	log.Info("Report Resource Request: ", r)
	var resourceType = r.GetType()
	resourceData, err := conv.ToJsonObject(r)
	if err != nil {
		return nil, err
	}

	var workspaceId, err2 = conv.ExtractWorkspaceId(r.Representations.Common)
	if err2 != nil {
		return nil, err2
	}

	var inventoryId, err3 = conv.ExtractInventoryId(r.GetInventoryId())
	if err3 != nil {
		return nil, err3
	}
	reporterType, err := conv.ExtractReporterType(r.ReporterType)
	if err != nil {
		log.Warn("Missing reporterType")
		return nil, err
	}

	reporterInstanceId, err := conv.ExtractReporterInstanceID(r.ReporterInstanceId)
	if err != nil {
		log.Warn("Missing reporterInstanceId")
		return nil, err
	}

	return conv.ResourceFromPb(resourceType, reporterType, reporterInstanceId, identity.Principal, resourceData, workspaceId, r.Representations, inventoryId), nil
}

func RequestToDeleteResource(r *pb.DeleteResourceRequest, identity *authnapi.Identity) (model.ReporterResourceId, error) {
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

func ResponseFromResource() *pb.ReportResourceResponse {
	return &pb.ReportResourceResponse{}
}

func ResponseFromDeleteResource() *pb.DeleteResourceResponse {
	return &pb.DeleteResourceResponse{}
}
