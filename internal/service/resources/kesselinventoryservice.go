package resources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	pbv1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
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
	claims, err := middleware.GetClaims(ctx)
	if err != nil {
		log.Errorf("failed to get claims: %v", err)
		return nil, status.Error(codes.Unauthenticated, "failed to get claims")
	}
	err = c.Ctl.ReportResource(ctx, r, string(claims.SubjectId))
	if err != nil {
		return nil, mapServiceError(err)
	}

	return ResponseFromResource(), nil

}

func (c *InventoryService) DeleteResource(ctx context.Context, r *pb.DeleteResourceRequest) (*pb.DeleteResourceResponse, error) {

	_, err := middleware.GetClaims(ctx)
	if err != nil {
		log.Errorf("failed to get claims: %v", err)
		return nil, status.Error(codes.Unauthenticated, "failed to get claims")
	}
	if reporterResourceKey, err := reporterKeyFromResourceReference(r.GetReference()); err == nil {
		if err = c.Ctl.Delete(ctx, reporterResourceKey); err == nil {
			return ResponseFromDeleteResource(), nil
		} else {
			log.Error("Failed to delete resource: ", err)

			if mapped := mapServiceError(err); mapped != err {
				return nil, mapped
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
	_, err := middleware.GetClaims(ctx)
	if err != nil {
		log.Errorf("failed to get claims: %v", err)
		return nil, status.Error(codes.Unauthenticated, "failed to get claims")
	}
	if reporterResourceKey, err := reporterKeyFromResourceReference(req.Object); err == nil {
		if resp, err := s.Ctl.Check(ctx, req.GetRelation(), req.Object.Reporter.GetType(), subjectReferenceFromSubject(req.GetSubject()), reporterResourceKey); err == nil {
			return viewResponseFromAuthzRequestV1beta2(resp), nil
		} else {
			return nil, mapServiceError(err)
		}
	} else {
		log.Error("Failed to build reporter resource key: ", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to build reporter resource key: %v", err)
	}
}

func (s *InventoryService) CheckForUpdate(ctx context.Context, req *pb.CheckForUpdateRequest) (*pb.CheckForUpdateResponse, error) {
	_, err := middleware.GetClaims(ctx)
	if err != nil {
		log.Errorf("failed to get claims: %v", err)
		return nil, status.Error(codes.Unauthenticated, "failed to get claims")
	}
	log.Info("CheckForUpdate using v1beta2 db")
	if reporterResourceKey, err := reporterKeyFromResourceReference(req.Object); err == nil {
		if resp, err := s.Ctl.CheckForUpdate(ctx, req.GetRelation(), req.Object.Reporter.GetType(), subjectReferenceFromSubject(req.GetSubject()), reporterResourceKey); err == nil {
			return updateResponseFromAuthzRequestV1beta2(resp), nil
		} else {
			return nil, mapServiceError(err)
		}
	} else {
		log.Error("Failed to build reporter resource key: ", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to build reporter resource key: %v", err)
	}
}

func (s *InventoryService) CheckBulk(ctx context.Context, req *pb.CheckBulkRequest) (*pb.CheckBulkResponse, error) {
	_, err := middleware.GetClaims(ctx)
	if err != nil {
		log.Errorf("failed to get claims: %v", err)
		return nil, status.Error(codes.Unauthenticated, "failed to get claims")
	}
	log.Info("CheckBulk using v1beta2 db")
	v1beta1Req := mapCheckBulkRequestToV1beta1(req)
	resp, err := s.Ctl.CheckBulk(ctx, v1beta1Req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return mapCheckBulkResponseFromV1beta1(resp), nil
}

func (s *InventoryService) CheckSelf(ctx context.Context, req *pb.CheckSelfRequest) (*pb.CheckSelfResponse, error) {
	if reporterResourceKey, err := reporterKeyFromResourceReference(req.Object); err == nil {
		if resp, err := s.Ctl.CheckSelf(ctx, req.GetRelation(), req.Object.Reporter.GetType(), reporterResourceKey); err == nil {
			allowed := pb.Allowed_ALLOWED_FALSE
			if resp {
				allowed = pb.Allowed_ALLOWED_TRUE
			}
			response := &pb.CheckSelfResponse{Allowed: allowed}
			// Note: Consistency token not available from Check usecase (returns bool only)
			// If consistency token is needed, usecase.Check would need to be enhanced
			return response, nil
		} else {
			return nil, mapServiceError(err)
		}
	} else {
		return nil, err
	}
}

func (s *InventoryService) CheckSelfBulk(ctx context.Context, req *pb.CheckSelfBulkRequest) (*pb.CheckSelfBulkResponse, error) {
	// Validate input: check items array
	if len(req.GetItems()) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "items array cannot be empty")
	}

	v1beta1Req := mapCheckSelfBulkRequestToV1beta1(req)
	resp, err := s.Ctl.CheckSelfBulk(ctx, v1beta1Req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	mappedResp, err := mapCheckSelfBulkResponseFromV1beta1(resp, req)
	if err != nil {
		return nil, err
	}
	return mappedResp, nil
}

func subjectReferenceFromSubject(subject *pb.SubjectReference) *pbv1beta1.SubjectReference {
	return &pbv1beta1.SubjectReference{
		Relation: subject.Relation,
		Subject: &pbv1beta1.ObjectReference{
			Type: &pbv1beta1.ObjectType{
				Namespace: subject.Resource.GetReporter().GetType(),
				Name:      subject.Resource.GetResourceType(),
			},
			Id: subject.Resource.GetResourceId(),
		},
	}
}

func subjectReferenceFromSubjectV1beta1(subject *pbv1beta1.SubjectReference) *pb.SubjectReference {
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

func mapCheckBulkRequestToV1beta1(req *pb.CheckBulkRequest) *pbv1beta1.CheckBulkRequest {
	items := make([]*pbv1beta1.CheckBulkRequestItem, len(req.GetItems()))
	for i, item := range req.GetItems() {
		items[i] = &pbv1beta1.CheckBulkRequestItem{
			Resource: &pbv1beta1.ObjectReference{
				Type: &pbv1beta1.ObjectType{
					Namespace: item.GetObject().GetReporter().GetType(),
					Name:      item.GetObject().GetResourceType(),
				},
				Id: item.GetObject().GetResourceId(),
			},
			Subject:  subjectReferenceFromSubject(item.GetSubject()),
			Relation: item.GetRelation(),
		}
	}

	return &pbv1beta1.CheckBulkRequest{
		Items:       items,
		Consistency: convertConsistencyToV1beta1(req.GetConsistency()),
	}
}

func convertConsistencyToV1beta1(consistency *pb.Consistency) *pbv1beta1.Consistency {
	if consistency == nil {
		return &pbv1beta1.Consistency{
			Requirement: &pbv1beta1.Consistency_MinimizeLatency{MinimizeLatency: true},
		}
	}
	if consistency.GetAtLeastAsFresh() != nil {
		return &pbv1beta1.Consistency{
			Requirement: &pbv1beta1.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &pbv1beta1.ConsistencyToken{
					Token: consistency.GetAtLeastAsFresh().GetToken(),
				},
			},
		}
	}
	return &pbv1beta1.Consistency{
		Requirement: &pbv1beta1.Consistency_MinimizeLatency{MinimizeLatency: true},
	}
}

func mapCheckBulkResponseFromV1beta1(resp *pbv1beta1.CheckBulkResponse) *pb.CheckBulkResponse {
	pairs := make([]*pb.CheckBulkResponsePair, len(resp.GetPairs()))
	for i, pair := range resp.GetPairs() {

		errResponse := &pb.CheckBulkResponsePair_Error{}
		itemResponse := &pb.CheckBulkResponsePair_Item{}

		if pair.GetError() != nil {
			log.Errorf("Error in checkbulk for req: %v error-code: %v error-message: %v", pair.GetRequest(), pair.GetError().GetCode(), pair.GetError().GetMessage())
			errResponse.Error = &rpcstatus.Status{
				Code:    pair.GetError().GetCode(),
				Message: pair.GetError().GetMessage(),
			}
		}

		allowedResponse := pb.Allowed_ALLOWED_FALSE

		if pair.GetItem().GetAllowed() == pbv1beta1.CheckBulkResponseItem_ALLOWED_TRUE {
			allowedResponse = pb.Allowed_ALLOWED_TRUE
		}
		itemResponse.Item = &pb.CheckBulkResponseItem{
			Allowed: allowedResponse,
		}

		pairs[i] = &pb.CheckBulkResponsePair{
			Request: &pb.CheckBulkRequestItem{
				Object: &pb.ResourceReference{
					ResourceType: pair.GetRequest().GetResource().GetType().GetName(),
					ResourceId:   pair.GetRequest().GetResource().GetId(),
					Reporter: &pb.ReporterReference{
						Type: pair.GetRequest().GetResource().GetType().GetNamespace(),
						// InstanceId: Inline with other behavior we dont have this info back from relations
					},
				},
				Relation: pair.GetRequest().GetRelation(),
				Subject:  subjectReferenceFromSubjectV1beta1(pair.GetRequest().GetSubject()),
			},
		}
		if pair.GetError() != nil {
			pairs[i].Response = errResponse
		} else {
			pairs[i].Response = itemResponse
		}
	}
	return &pb.CheckBulkResponse{
		Pairs:            pairs,
		ConsistencyToken: &pb.ConsistencyToken{Token: resp.GetConsistencyToken().GetToken()},
	}
}

func mapCheckSelfBulkRequestToV1beta1(req *pb.CheckSelfBulkRequest) *pbv1beta1.CheckBulkRequest {
	items := make([]*pbv1beta1.CheckBulkRequestItem, len(req.GetItems()))

	for i, item := range req.GetItems() {
		// Note: Input validation is done in CheckSelfBulk handler, but defensive nil checks here
		// for safety in case this function is called from elsewhere
		obj := item.GetObject()
		reporter := obj.GetReporter()

		items[i] = &pbv1beta1.CheckBulkRequestItem{
			Resource: &pbv1beta1.ObjectReference{
				Type: &pbv1beta1.ObjectType{
					Namespace: reporter.GetType(),
					Name:      obj.GetResourceType(),
				},
				Id: obj.GetResourceId(),
			},
			Relation: item.GetRelation(),
		}
	}

	consistency := convertConsistencyToV1beta1(req.GetConsistency())
	return &pbv1beta1.CheckBulkRequest{
		Items:       items,
		Consistency: consistency,
	}
}

func mapCheckSelfBulkResponseFromV1beta1(resp *pbv1beta1.CheckBulkResponse, req *pb.CheckSelfBulkRequest) (*pb.CheckSelfBulkResponse, error) {
	respPairs := resp.GetPairs()
	reqItems := req.GetItems()

	// Treat any mismatch as a hard error to avoid silently dropping results.
	if len(respPairs) != len(reqItems) {
		log.Errorf("Mismatch in CheckSelfBulk response: expected %d pairs, got %d", len(reqItems), len(respPairs))
		return nil, status.Error(codes.Internal, "internal error: mismatched backend check results")
	}

	pairs := make([]*pb.CheckSelfBulkResponsePair, len(respPairs))

	for i := 0; i < len(respPairs); i++ {
		pair := respPairs[i]
		errResponse := &pb.CheckSelfBulkResponsePair_Error{}
		itemResponse := &pb.CheckSelfBulkResponsePair_Item{}

		if pair.GetError() != nil {
			log.Errorf("Error in checkselfbulk for req: %v error-code: %v error-message: %v", pair.GetRequest(), pair.GetError().GetCode(), pair.GetError().GetMessage())
			errResponse.Error = &rpcstatus.Status{
				Code:    pair.GetError().GetCode(),
				Message: pair.GetError().GetMessage(),
			}
		}

		allowedResponse := pb.Allowed_ALLOWED_FALSE
		if pair.GetItem() != nil && pair.GetItem().GetAllowed() == pbv1beta1.CheckBulkResponseItem_ALLOWED_TRUE {
			allowedResponse = pb.Allowed_ALLOWED_TRUE
		}
		itemResponse.Item = &pb.CheckSelfBulkResponseItem{
			Allowed: allowedResponse,
		}

		// Map back to the original request item (no subject in CheckSelfBulkRequestItem)
		// Note: i is guaranteed to be < len(reqItems) because we iterate only up to minLen
		requestItem := reqItems[i]

		// Defensive nil checks (input validation should prevent this, but be safe)
		var requestItemProto *pb.CheckSelfBulkRequestItem
		if requestItem != nil && requestItem.GetObject() != nil {
			obj := requestItem.GetObject()
			reporter := obj.GetReporter()
			requestItemProto = &pb.CheckSelfBulkRequestItem{
				Object: &pb.ResourceReference{
					ResourceType: obj.GetResourceType(),
					ResourceId:   obj.GetResourceId(),
					Reporter: &pb.ReporterReference{
						Type: reporter.GetType(),
					},
				},
				Relation: requestItem.GetRelation(),
			}
		} else {
			// Fallback: create empty request item if data is missing
			log.Errorf("CheckSelfBulk response mapping: requestItem or object is nil at index %d", i)
			requestItemProto = &pb.CheckSelfBulkRequestItem{}
		}

		pairs[i] = &pb.CheckSelfBulkResponsePair{
			Request: requestItemProto,
		}
		if pair.GetError() != nil {
			pairs[i].Response = errResponse
		} else {
			pairs[i].Response = itemResponse
		}
	}

	response := &pb.CheckSelfBulkResponse{
		Pairs: pairs,
	}
	// Set consistency token if available
	if resp.GetConsistencyToken() != nil {
		response.ConsistencyToken = &pb.ConsistencyToken{Token: resp.GetConsistencyToken().GetToken()}
	}
	return response, nil
}

func mapServiceError(err error) error {
	if err == nil {
		return nil
	}
	if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
		return err
	}
	switch {
	case errors.Is(err, resources.ErrMetaAuthzContextMissing):
		return status.Error(codes.Unauthenticated, "authz context missing")
	case errors.Is(err, resources.ErrSelfSubjectMissing):
		return status.Error(codes.Unauthenticated, "self subject missing")
	case errors.Is(err, resources.ErrMetaAuthorizerUnavailable):
		return status.Error(codes.Internal, "meta authorizer unavailable")
	case errors.Is(err, resources.ErrMetaAuthorizationDenied):
		return status.Error(codes.PermissionDenied, "meta authorization denied")
	case errors.Is(err, resources.ErrResourceNotFound):
		return status.Error(codes.NotFound, "resource not found")
	case errors.Is(err, resources.ErrResourceAlreadyExists):
		return status.Error(codes.AlreadyExists, "resource already exists")
	case errors.Is(err, resources.ErrInventoryIdMismatch):
		return status.Error(codes.FailedPrecondition, "resource inventory id mismatch")
	case errors.Is(err, resources.ErrDatabaseError):
		return status.Error(codes.Internal, "internal error")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return err
	}
}

func (s *InventoryService) StreamedListObjects(
	req *pb.StreamedListObjectsRequest,
	stream pb.KesselInventoryService_StreamedListObjectsServer,
) error {
	ctx := stream.Context()
	// Example: how to use get the claims from the stream context
	// claims, err := interceptor.FromContextClaims(ctx)
	// log.Info(claims)
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
			ResourceId:   response.Resource.Id,
			ResourceType: response.Resource.Type.Name,
		},
		Pagination: &pb.ResponsePagination{
			ContinuationToken: response.Pagination.ContinuationToken,
		},
	}
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

func ResponseFromResource() *pb.ReportResourceResponse {
	return &pb.ReportResourceResponse{}
}

func ResponseFromDeleteResource() *pb.DeleteResourceResponse {
	return &pb.DeleteResourceResponse{}
}
