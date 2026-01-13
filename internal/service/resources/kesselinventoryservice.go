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
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		log.Errorf("failed to get identity: %v", err)
		return nil, status.Error(codes.Unauthenticated, "failed to get identity")
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
		log.Errorf("failed to get identity: %v", err)
		return nil, status.Error(codes.Unauthenticated, "failed to get identity")
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
		log.Errorf("failed to get identity: %v", err)
		return nil, status.Error(codes.Unauthenticated, "failed to get identity")
	}

	if reporterResourceKey, err := reporterKeyFromResourceReference(req.Object); err == nil {
		if resp, err := s.Ctl.Check(ctx, req.GetRelation(), req.Object.Reporter.GetType(), subjectReferenceFromSubject(req.GetSubject()), reporterResourceKey); err == nil {
			return viewResponseFromAuthzRequestV1beta2(resp), nil
		} else {
			return nil, err
		}
	} else {
		log.Error("Failed to build reporter resource key: ", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to build reporter resource key: %v", err)
	}
}

func (s *InventoryService) CheckForUpdate(ctx context.Context, req *pb.CheckForUpdateRequest) (*pb.CheckForUpdateResponse, error) {
	_, err := middleware.GetIdentity(ctx)
	if err != nil {
		log.Errorf("failed to get identity: %v", err)
		return nil, status.Error(codes.Unauthenticated, "failed to get identity")
	}

	log.Info("CheckForUpdate using v1beta2 db")
	if reporterResourceKey, err := reporterKeyFromResourceReference(req.Object); err == nil {
		if resp, err := s.Ctl.CheckForUpdate(ctx, req.GetRelation(), req.Object.Reporter.GetType(), subjectReferenceFromSubject(req.GetSubject()), reporterResourceKey); err == nil {
			return updateResponseFromAuthzRequestV1beta2(resp), nil
		} else {
			return nil, err
		}
	} else {
		log.Error("Failed to build reporter resource key: ", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to build reporter resource key: %v", err)
	}
}

func (s *InventoryService) CheckBulk(ctx context.Context, req *pb.CheckBulkRequest) (*pb.CheckBulkResponse, error) {
	_, err := middleware.GetIdentity(ctx)
	if err != nil {
		log.Errorf("failed to get identity: %v", err)
		return nil, status.Error(codes.Unauthenticated, "failed to get identity")
	}

	log.Info("CheckBulk using v1beta2 db")
	v1beta1Req := mapCheckBulkRequestToV1beta1(req)
	resp, err := s.Ctl.CheckBulk(ctx, v1beta1Req)
	if err != nil {
		return nil, err
	}
	return mapCheckBulkResponseFromV1beta1(resp), nil
}

func (s *InventoryService) CheckSelf(ctx context.Context, req *pb.CheckSelfRequest) (*pb.CheckSelfResponse, error) {
	// Get identity from context (from x-rh-identity header or other auth methods)
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to get identity: %v", err)
	}

	if reporterResourceKey, err := reporterKeyFromResourceReference(req.Object); err == nil {
		// Derive subject reference from identity (x-rh-identity header)
		subjectRef := subjectReferenceFromIdentity(identity)
		if resp, err := s.Ctl.Check(ctx, req.GetRelation(), req.Object.Reporter.GetType(), subjectRef, reporterResourceKey); err == nil {
			allowed := pb.Allowed_ALLOWED_FALSE
			if resp {
				allowed = pb.Allowed_ALLOWED_TRUE
			}
			response := &pb.CheckSelfResponse{Allowed: allowed}
			// Note: Consistency token not available from Check usecase (returns bool only)
			// If consistency token is needed, usecase.Check would need to be enhanced
			return response, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (s *InventoryService) CheckSelfBulk(ctx context.Context, req *pb.CheckSelfBulkRequest) (*pb.CheckSelfBulkResponse, error) {
	// Get identity from context (from x-rh-identity header or other auth methods)
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to get identity: %v", err)
	}

	log.Info("CheckSelfBulk using v1beta2 db")
	// Map request to v1beta1 format, deriving subject from identity (x-rh-identity header)
	v1beta1Req := mapCheckSelfBulkRequestToV1beta1(req, identity)
	resp, err := s.Ctl.CheckBulk(ctx, v1beta1Req)
	if err != nil {
		return nil, err
	}
	return mapCheckSelfBulkResponseFromV1beta1(resp, req), nil
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

func subjectReferenceFromIdentity(identity *authnapi.Identity) *pbv1beta1.SubjectReference {
	// Determine subject ID based on authentication type
	// For x-rh-identity: Principal is username/email, UserID may also be available
	// For OIDC: Principal is in "domain/subject" format
	var subjectID string

	if identity.AuthType == "x-rh-identity" {
		// For x-rh-identity, prefer UserID if available (more stable identifier)
		// Otherwise fall back to Principal (username/email)
		if identity.UserID != "" {
			subjectID = identity.UserID
		} else if identity.Principal != "" {
			subjectID = identity.Principal
		} else {
			// Fallback: should not happen for authenticated requests
			subjectID = identity.Principal
		}
	} else {
		// For OIDC and other auth types, parse Principal
		// Principal might be in "domain/subject" format (OIDC) or just "subject"
		subjectID = identity.Principal
		if parts := strings.SplitN(identity.Principal, "/", 2); len(parts) == 2 {
			subjectID = parts[1]
		}
	}

	// Determine namespace
	// For x-rh-identity: Type field contains "User", "System", etc. but we use "rbac" as namespace
	// For OIDC: Type is typically empty, default to "rbac"
	namespace := "rbac"
	if identity.AuthType != "x-rh-identity" && identity.Type != "" {
		// For non-x-rh-identity auth types, use Type if set
		namespace = identity.Type
	}

	return &pbv1beta1.SubjectReference{
		Relation: nil, // No relation for direct subject reference
		Subject: &pbv1beta1.ObjectReference{
			Type: &pbv1beta1.ObjectType{
				Namespace: namespace,
				Name:      "principal",
			},
			Id: subjectID,
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

func mapCheckSelfBulkRequestToV1beta1(req *pb.CheckSelfBulkRequest, identity *authnapi.Identity) *pbv1beta1.CheckBulkRequest {
	items := make([]*pbv1beta1.CheckBulkRequestItem, len(req.GetItems()))
	// Derive subject reference from identity (x-rh-identity header or other auth)
	// All items in the bulk request use the same subject (the caller)
	subjectRef := subjectReferenceFromIdentity(identity)

	for i, item := range req.GetItems() {
		items[i] = &pbv1beta1.CheckBulkRequestItem{
			Resource: &pbv1beta1.ObjectReference{
				Type: &pbv1beta1.ObjectType{
					Namespace: item.GetObject().GetReporter().GetType(),
					Name:      item.GetObject().GetResourceType(),
				},
				Id: item.GetObject().GetResourceId(),
			},
			Subject:  subjectRef,
			Relation: item.GetRelation(),
		}
	}

	consistency := convertConsistencyToV1beta1(req.GetConsistencyToken())
	return &pbv1beta1.CheckBulkRequest{
		Items:       items,
		Consistency: consistency,
	}
}

func mapCheckSelfBulkResponseFromV1beta1(resp *pbv1beta1.CheckBulkResponse, req *pb.CheckSelfBulkRequest) *pb.CheckSelfBulkResponse {
	pairs := make([]*pb.CheckSelfBulkResponsePair, len(resp.GetPairs()))
	for i, pair := range resp.GetPairs() {
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
		if pair.GetItem().GetAllowed() == pbv1beta1.CheckBulkResponseItem_ALLOWED_TRUE {
			allowedResponse = pb.Allowed_ALLOWED_TRUE
		}
		itemResponse.Item = &pb.CheckSelfBulkResponseItem{
			Allowed: allowedResponse,
		}

		// Map back to the original request item (no subject in CheckSelfBulkRequestItem)
		requestItem := req.GetItems()[i]
		pairs[i] = &pb.CheckSelfBulkResponsePair{
			Request: &pb.CheckSelfBulkRequestItem{
				Object: &pb.ResourceReference{
					ResourceType: requestItem.GetObject().GetResourceType(),
					ResourceId:   requestItem.GetObject().GetResourceId(),
					Reporter: &pb.ReporterReference{
						Type: requestItem.GetObject().GetReporter().GetType(),
					},
				},
				Relation: requestItem.GetRelation(),
			},
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
	return response
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
