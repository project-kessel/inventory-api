package resources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
	relations "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
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

	cmd, err := reportResourceCommandFromRequest(r)
	if err != nil {
		log.Errorf("failed to build report resource command: %v", err)
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	err = c.Ctl.ReportResource(ctx, cmd, identity.Principal)
	if err != nil {
		log.Errorf("failed to report resource: %v", err)
		// Map domain errors to gRPC status codes
		if errors.Is(err, resources.ErrInvalidReporterForResource) || errors.Is(err, resources.ErrSchemaValidationFailed) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		// Default to internal error for unknown errors
		return nil, status.Errorf(codes.Internal, "failed to report resource: %v", err)
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

func subjectReferenceFromSubject(subject *pb.SubjectReference) *relations.SubjectReference {
	return &relations.SubjectReference{
		Relation: subject.Relation,
		Subject: &relations.ObjectReference{
			Type: &relations.ObjectType{
				Namespace: subject.Resource.GetReporter().GetType(),
				Name:      subject.Resource.GetResourceType(),
			},
			Id: subject.Resource.GetResourceId(),
		},
	}
}

func subjectReferenceFromSubjectV1beta1(subject *relations.SubjectReference) *pb.SubjectReference {
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

func mapCheckBulkRequestToV1beta1(req *pb.CheckBulkRequest) *relations.CheckBulkRequest {
	items := make([]*relations.CheckBulkRequestItem, len(req.GetItems()))
	for i, item := range req.GetItems() {
		items[i] = &relations.CheckBulkRequestItem{
			Resource: &relations.ObjectReference{
				Type: &relations.ObjectType{
					Namespace: item.GetObject().GetReporter().GetType(),
					Name:      item.GetObject().GetResourceType(),
				},
				Id: item.GetObject().GetResourceId(),
			},
			Subject:  subjectReferenceFromSubject(item.GetSubject()),
			Relation: item.GetRelation(),
		}
	}

	return &relations.CheckBulkRequest{
		Items:       items,
		Consistency: convertConsistencyToV1beta1(req.GetConsistency()),
	}
}

func convertConsistencyToV1beta1(consistency *pb.Consistency) *relations.Consistency {
	if consistency == nil {
		return &relations.Consistency{
			Requirement: &relations.Consistency_MinimizeLatency{MinimizeLatency: true},
		}
	}
	if consistency.GetAtLeastAsFresh() != nil {
		return &relations.Consistency{
			Requirement: &relations.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &relations.ConsistencyToken{
					Token: consistency.GetAtLeastAsFresh().GetToken(),
				},
			},
		}
	}
	return &relations.Consistency{
		Requirement: &relations.Consistency_MinimizeLatency{MinimizeLatency: true},
	}
}

func mapCheckBulkResponseFromV1beta1(resp *relations.CheckBulkResponse) *pb.CheckBulkResponse {
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

		if pair.GetItem().GetAllowed() == relations.CheckBulkResponseItem_ALLOWED_TRUE {
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

func ToLookupResourceRequest(request *pb.StreamedListObjectsRequest) (*relations.LookupResourcesRequest, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}
	var pagination *relations.RequestPagination
	if request.Pagination != nil {
		pagination = &relations.RequestPagination{
			Limit:             request.Pagination.Limit,
			ContinuationToken: request.Pagination.ContinuationToken,
		}
	}
	normalizedNamespace := NormalizeType(request.ObjectType.GetReporterType())
	normalizedResourceType := NormalizeType(request.ObjectType.GetResourceType())

	return &relations.LookupResourcesRequest{
		ResourceType: &relations.ObjectType{
			Namespace: normalizedNamespace,
			Name:      normalizedResourceType,
		},
		Relation: request.Relation,
		Subject: &relations.SubjectReference{
			Relation: request.Subject.Relation,
			Subject: &relations.ObjectReference{
				Type: &relations.ObjectType{
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

func ToLookupResourceResponse(response *relations.LookupResourcesResponse) *pb.StreamedListObjectsResponse {
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

// reportResourceCommandFromRequest converts a proto ReportResourceRequest to a domain ReportResourceCommand.
func reportResourceCommandFromRequest(r *pb.ReportResourceRequest) (model.ReportResourceCommand, error) {
	localResourceId, err := model.NewLocalResourceId(r.GetRepresentations().GetMetadata().GetLocalResourceId())
	if err != nil {
		return model.ReportResourceCommand{}, fmt.Errorf("invalid local resource ID: %w", err)
	}

	resourceType, err := model.NewResourceType(r.GetType())
	if err != nil {
		return model.ReportResourceCommand{}, fmt.Errorf("invalid resource type: %w", err)
	}

	reporterType, err := model.NewReporterType(r.GetReporterType())
	if err != nil {
		return model.ReportResourceCommand{}, fmt.Errorf("invalid reporter type: %w", err)
	}

	reporterInstanceId, err := model.NewReporterInstanceId(r.GetReporterInstanceId())
	if err != nil {
		return model.ReportResourceCommand{}, fmt.Errorf("invalid reporter instance ID: %w", err)
	}

	apiHref, err := model.NewApiHref(r.GetRepresentations().GetMetadata().GetApiHref())
	if err != nil {
		return model.ReportResourceCommand{}, fmt.Errorf("invalid API href: %w", err)
	}

	var consoleHref model.ConsoleHref
	if consoleHrefVal := r.GetRepresentations().GetMetadata().GetConsoleHref(); consoleHrefVal != "" {
		consoleHref, err = model.NewConsoleHref(consoleHrefVal)
		if err != nil {
			return model.ReportResourceCommand{}, fmt.Errorf("invalid console href: %w", err)
		}
	}

	var reporterVersion *model.ReporterVersion
	if reporterVersionValue := r.GetRepresentations().GetMetadata().GetReporterVersion(); reporterVersionValue != "" {
		rv, err := model.NewReporterVersion(reporterVersionValue)
		if err != nil {
			return model.ReportResourceCommand{}, fmt.Errorf("invalid reporter version: %w", err)
		}
		reporterVersion = &rv
	}

	transactionId := model.NewTransactionId(r.GetRepresentations().GetMetadata().GetTransactionId())

	writeVisibility := writeVisibilityFromProto(r.GetWriteVisibility())

	return model.NewReportResourceCommand(
		localResourceId,
		resourceType,
		reporterType,
		reporterInstanceId,
		apiHref,
		consoleHref,
		reporterVersion,
		r.GetRepresentations().GetReporter().AsMap(),
		r.GetRepresentations().GetCommon().AsMap(),
		transactionId,
		writeVisibility,
	)
}

// writeVisibilityFromProto converts the proto WriteVisibility enum to the domain WriteVisibility type.
func writeVisibilityFromProto(v pb.WriteVisibility) model.WriteVisibility {
	switch v {
	case pb.WriteVisibility_IMMEDIATE:
		return model.WriteVisibilityCommitPending
	case pb.WriteVisibility_MINIMIZE_LATENCY:
		return model.WriteVisibilityMinimizeLatency
	default:
		return model.WriteVisibilityUnspecified
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
