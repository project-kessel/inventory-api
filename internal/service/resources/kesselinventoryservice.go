package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
	kessel "github.com/project-kessel/inventory-api/internal/authz/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	err = c.Ctl.ReportResource(ctx, r, identity.Principal)
	if err != nil {
		return nil, err
	}

	return ResponseFromResource(), nil

}

func (c *InventoryService) DeleteResource(ctx context.Context, r *pb.DeleteResourceRequest) (*pb.DeleteResourceResponse, error) {

	_, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to get identity: %v", err)
	}

	if reporterResourceKey, err := reporterKeyFromResourceReference(r.GetReference()); err == nil {
		if err = c.Ctl.Delete(ctx, reporterResourceKey); err == nil {
			return ResponseFromDeleteResource(), nil
		} else {
			log.Error("Failed to delete resource: ", err)

			if errors.Is(err, resources.ErrResourceNotFound) {
				return nil, status.Errorf(codes.NotFound, "resource not found")
			}
			// Default to internal error for unknown errors
			return nil, status.Errorf(codes.Internal, "failed to delete resource due to an internal error")
		}
	} else {
		log.Error("Failed to build reporter resource key: ", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to build reporter resource key: %v", err)
	}
}

func (s *InventoryService) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	_, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to get identity: %v", err)
	}

	if reporterResourceKey, err := reporterKeyFromResourceReference(req.Object); err == nil {
		if resp, err := s.Ctl.Check(ctx, req.GetRelation(), req.Object.Reporter.GetType(), subjectReferenceFromSubject(req.GetSubject()), reporterResourceKey); err == nil {
			return viewResponseFromAuthzRequestV1beta2(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (s *InventoryService) CheckForUpdate(ctx context.Context, req *pb.CheckForUpdateRequest) (*pb.CheckForUpdateResponse, error) {
	_, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to get identity: %v", err)
	}

	log.Info("CheckForUpdate using v1beta2 db")
	if reporterResourceKey, err := reporterKeyFromResourceReference(req.Object); err == nil {
		if resp, err := s.Ctl.CheckForUpdate(ctx, req.GetRelation(), req.Object.Reporter.GetType(), subjectReferenceFromSubject(req.GetSubject()), reporterResourceKey); err == nil {
			return updateResponseFromAuthzRequestV1beta2(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (s *InventoryService) CheckBulk(ctx context.Context, req *pb.CheckBulkRequest) (*pb.CheckBulkResponse, error) {
	_, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to get identity: %v", err)
	}

	log.Info("CheckBulk using v1beta2 db")
	kesselReq := mapCheckBulkRequestToKessel(req)
	resp, err := s.Ctl.CheckBulk(ctx, kesselReq)
	if err != nil {
		return nil, err
	}
	return mapCheckBulkResponseFromKessel(resp), nil
}

func subjectReferenceFromSubject(subject *pb.SubjectReference) *kessel.SubjectReference {
	return &kessel.SubjectReference{
		Relation: subject.Relation,
		Subject: &kessel.ObjectReference{
			Type: &kessel.ObjectType{
				Namespace: subject.Resource.GetReporter().GetType(),
				Name:      subject.Resource.GetResourceType(),
			},
			Id: subject.Resource.GetResourceId(),
		},
	}
}

func subjectReferenceFromSubjectKessel(subject *kessel.SubjectReference) *pb.SubjectReference {
	return &pb.SubjectReference{
		Relation: subject.Relation,
		Resource: &pb.ResourceReference{
			Reporter: &pb.ReporterReference{
				Type: subject.Subject.Type.Namespace,
			},
			ResourceId:   subject.Subject.Id,
			ResourceType: subject.Subject.Type.Name,
		},
	}
}

func mapCheckBulkRequestToKessel(req *pb.CheckBulkRequest) *kessel.CheckBulkRequest {
	items := make([]*kessel.CheckBulkRequestItem, len(req.GetItems()))
	for i, item := range req.GetItems() {
		items[i] = &kessel.CheckBulkRequestItem{
			Resource: &kessel.ObjectReference{
				Type: &kessel.ObjectType{
					Namespace: item.GetObject().GetReporter().GetType(),
					Name:      item.GetObject().GetResourceType(),
				},
				Id: item.GetObject().GetResourceId(),
			},
			Subject:  subjectReferenceFromSubject(item.GetSubject()),
			Relation: item.GetRelation(),
		}
	}

	return &kessel.CheckBulkRequest{
		Items:       items,
		Consistency: convertConsistencyToKessel(req.GetConsistency()),
	}
}

func convertConsistencyToKessel(consistency *pb.Consistency) *kessel.Consistency {
	if consistency == nil {
		return kessel.NewConsistencyMinimizeLatency()
	}
	if consistency.GetAtLeastAsFresh() != nil {
		return kessel.NewConsistencyAtLeastAsFresh(consistency.GetAtLeastAsFresh().GetToken())
	}
	return kessel.NewConsistencyMinimizeLatency()
}

func mapCheckBulkResponseFromKessel(resp *kessel.CheckBulkResponse) *pb.CheckBulkResponse {
	pairs := make([]*pb.CheckBulkResponsePair, len(resp.Pairs))
	for i, pair := range resp.Pairs {

		errResponse := &pb.CheckBulkResponsePair_Error{}
		itemResponse := &pb.CheckBulkResponsePair_Item{}

		if pair.Error != nil {
			log.Errorf("Error in checkbulk for req: %v error: %v", pair.Request, pair.Error.Error())
			errResponse.Error = status.Convert(pair.Error).Proto()
		}

		allowedResponse := pb.Allowed_ALLOWED_FALSE

		if pair.Item != nil && pair.Item.Allowed == kessel.AllowedTrue {
			allowedResponse = pb.Allowed_ALLOWED_TRUE
		}
		itemResponse.Item = &pb.CheckBulkResponseItem{
			Allowed: allowedResponse,
		}

		pairs[i] = &pb.CheckBulkResponsePair{
			Request: &pb.CheckBulkRequestItem{
				Object: &pb.ResourceReference{
					ResourceType: pair.Request.Resource.Type.Name,
					ResourceId:   pair.Request.Resource.Id,
					Reporter: &pb.ReporterReference{
						Type: pair.Request.Resource.Type.Namespace,
						// InstanceId: Inline with other behavior we dont have this info back from relations
					},
				},
				Relation: pair.Request.Relation,
				Subject:  subjectReferenceFromSubjectKessel(pair.Request.Subject),
			},
		}
		if pair.Error != nil {
			pairs[i].Response = errResponse
		} else {
			pairs[i].Response = itemResponse
		}
	}
	return &pb.CheckBulkResponse{
		Pairs:            pairs,
		ConsistencyToken: &pb.ConsistencyToken{Token: resp.ConsistencyToken.Token},
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
	resourceType, relation, subject, limit, continuationToken, err := ToLookupResourceRequest(req)
	if err != nil {
		return fmt.Errorf("failed to build lookup request: %w", err)
	}

	resourceChan, errChan, err := s.Ctl.LookupResources(ctx, resourceType, relation, subject, limit, continuationToken)
	if err != nil {
		return fmt.Errorf("failed to retrieve resources: %w", err)
	}

	// Read from channels and convert to gRPC stream
	for {
		select {
		case result, ok := <-resourceChan:
			if !ok {
				// Channel closed, check for errors
				if err := <-errChan; err != nil {
					return fmt.Errorf("error during lookup: %w", err)
				}
				return nil
			}
				// Convert channel result to gRPC response
			resp := &pb.StreamedListObjectsResponse{
				Object: &pb.ResourceReference{
					Reporter: &pb.ReporterReference{
						Type: result.Resource.Type.Namespace,
					},
					ResourceId:   result.Resource.Id,
					ResourceType: result.Resource.Type.Name,
				},
				Pagination: &pb.ResponsePagination{
					ContinuationToken: string(result.Continuation),
				},
			}
			if err := stream.Send(resp); err != nil {
				return fmt.Errorf("error sending resource to client: %w", err)
			}
		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("error receiving resource: %w", err)
			}
		}
	}
}

func ToLookupResourceRequest(request *pb.StreamedListObjectsRequest) (*kessel.ObjectType, string, *kessel.SubjectReference, uint32, string, error) {
	if request == nil {
		return nil, "", nil, 0, "", fmt.Errorf("request is nil")
	}

	normalizedNamespace := NormalizeType(request.ObjectType.GetReporterType())
	normalizedResourceType := NormalizeType(request.ObjectType.GetResourceType())

	resourceType := &kessel.ObjectType{
		Namespace: normalizedNamespace,
		Name:      normalizedResourceType,
	}

	relation := request.Relation

	subject := &kessel.SubjectReference{
		Relation: request.Subject.Relation,
		Subject: &kessel.ObjectReference{
			Type: &kessel.ObjectType{
				Name:      request.Subject.Resource.GetResourceType(),
				Namespace: request.Subject.Resource.GetReporter().GetType(),
			},
			Id: request.Subject.Resource.GetResourceId(),
		},
	}

	var limit uint32
	var continuationToken string
	if request.Pagination != nil {
		limit = request.Pagination.Limit
		if request.Pagination.ContinuationToken != nil {
			continuationToken = *request.Pagination.ContinuationToken
		}
	}

	return resourceType, relation, subject, limit, continuationToken, nil
}

func NormalizeType(val string) string {
	normalized := strings.ToLower(val)
	return normalized
}

func reporterKeyFromResourceReference(resource *pb.ResourceReference) (model.ReporterResourceKey, error) {
	localResourceId, err := model.NewLocalResourceId(resource.GetResourceId())
	if err != nil {
		return model.ReporterResourceKey{}, err
	}
	resourceType, err := model.NewResourceType(resource.GetResourceType())
	if err != nil {
		return model.ReporterResourceKey{}, err
	}
	reporterType, err := model.NewReporterType(resource.GetReporter().GetType())
	if err != nil {
		return model.ReporterResourceKey{}, err
	}

	// Handle reporterInstanceId being absent/empty
	var reporterInstanceId model.ReporterInstanceId
	if instanceId := resource.GetReporter().GetInstanceId(); instanceId != "" {
		reporterInstanceId, err = model.NewReporterInstanceId(instanceId)
		if err != nil {
			return model.ReporterResourceKey{}, err
		}
	} else {
		// Use zero value for empty/absent reporterInstanceId
		reporterInstanceId = model.ReporterInstanceId("")
	}

	return model.NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
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

func RequestToResource(r *pb.ReportResourceRequest, identity *authnapi.Identity) (*model_legacy.Resource, error) {
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

func ResponseFromResource() *pb.ReportResourceResponse {
	return &pb.ReportResourceResponse{}
}

func ResponseFromDeleteResource() *pb.DeleteResourceResponse {
	return &pb.DeleteResourceResponse{}
}
