package resources

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
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
	cmd, err := toReportResourceCommand(r)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	err = c.Ctl.ReportResource(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return ResponseFromResource(), nil
}

func (c *InventoryService) DeleteResource(ctx context.Context, r *pb.DeleteResourceRequest) (*pb.DeleteResourceResponse, error) {
	reporterResourceKey, err := reporterKeyFromResourceReference(r.GetReference())
	if err != nil {
		log.Error("Failed to build reporter resource key: ", err)
		return nil, err
	}
	if err = c.Ctl.Delete(ctx, reporterResourceKey); err != nil {
		log.Error("Failed to delete resource: ", err)
		return nil, err
	}
	return ResponseFromDeleteResource(), nil
}

func (s *InventoryService) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	reporterResourceKey, err := reporterKeyFromResourceReference(req.Object)
	if err != nil {
		log.Error("Failed to build reporter resource key: ", err)
		return nil, err
	}
	subjectRef, err := subjectReferenceFromProto(req.GetSubject())
	if err != nil {
		log.Error("Failed to build subject reference: ", err)
		return nil, err
	}
	relation, err := model.NewRelation(req.GetRelation())
	if err != nil {
		log.Error("Failed to build relation: ", err)
		return nil, err
	}
	consistency := ConvertConsistencyToModel(req.GetConsistency())
	resp, err := s.Ctl.Check(ctx, relation, subjectRef, reporterResourceKey, consistency)
	if err != nil {
		return nil, err
	}
	return viewResponseFromAuthzRequestV1beta2(resp), nil
}

func (s *InventoryService) CheckForUpdate(ctx context.Context, req *pb.CheckForUpdateRequest) (*pb.CheckForUpdateResponse, error) {
	log.Info("CheckForUpdate using v1beta2 db")
	reporterResourceKey, err := reporterKeyFromResourceReference(req.Object)
	if err != nil {
		log.Error("Failed to build reporter resource key: ", err)
		return nil, err
	}
	subjectRef, err := subjectReferenceFromProto(req.GetSubject())
	if err != nil {
		log.Error("Failed to build subject reference: ", err)
		return nil, err
	}
	relation, err := model.NewRelation(req.GetRelation())
	if err != nil {
		log.Error("Failed to build relation: ", err)
		return nil, err
	}
	resp, err := s.Ctl.CheckForUpdate(ctx, relation, subjectRef, reporterResourceKey)
	if err != nil {
		return nil, err
	}
	return updateResponseFromAuthzRequestV1beta2(resp), nil
}

func (s *InventoryService) CheckBulk(ctx context.Context, req *pb.CheckBulkRequest) (*pb.CheckBulkResponse, error) {
	log.Info("CheckBulk using v1beta2 db")
	cmd, err := toCheckBulkCommand(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	resp, err := s.Ctl.CheckBulk(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return fromCheckBulkResult(resp, req), nil
}

func (s *InventoryService) CheckSelf(ctx context.Context, req *pb.CheckSelfRequest) (*pb.CheckSelfResponse, error) {
	reporterResourceKey, err := reporterKeyFromResourceReference(req.Object)
	if err != nil {
		return nil, err
	}
	relation, err := model.NewRelation(req.GetRelation())
	if err != nil {
		return nil, err
	}
	consistency := ConvertConsistencyToModel(req.GetConsistency())
	log.Infof("CheckSelf request consistency: %s", consistency.Preference)
	resp, err := s.Ctl.CheckSelf(ctx, relation, reporterResourceKey, consistency)
	if err != nil {
		return nil, err
	}
	allowed := pb.Allowed_ALLOWED_FALSE
	if resp {
		allowed = pb.Allowed_ALLOWED_TRUE
	}
	return &pb.CheckSelfResponse{Allowed: allowed}, nil
}

func (s *InventoryService) CheckSelfBulk(ctx context.Context, req *pb.CheckSelfBulkRequest) (*pb.CheckSelfBulkResponse, error) {
	// Validate input: check items array
	if len(req.GetItems()) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "items array cannot be empty")
	}

	cmd, err := toCheckSelfBulkCommand(req)
	if err != nil {
		return nil, err
	}
	resp, err := s.Ctl.CheckSelfBulk(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return fromCheckSelfBulkResult(resp, req), nil
}

func subjectReferenceFromProto(subject *pb.SubjectReference) (model.SubjectReference, error) {
	localResourceId, err := model.NewLocalResourceId(subject.Resource.GetResourceId())
	if err != nil {
		return model.SubjectReference{}, err
	}
	resourceType, err := model.NewResourceType(subject.Resource.GetResourceType())
	if err != nil {
		return model.SubjectReference{}, err
	}
	// TODO: Reporter is optional and we should also consider instance ID
	reporterType, err := model.NewReporterType(subject.Resource.GetReporter().GetType())
	if err != nil {
		return model.SubjectReference{}, err
	}

	key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, model.ReporterInstanceId(""))
	if err != nil {
		return model.SubjectReference{}, err
	}

	if subject.GetRelation() != "" {
		relation, err := model.NewRelation(subject.GetRelation())
		if err != nil {
			return model.SubjectReference{}, err
		}
		return model.NewSubjectReference(key, &relation), nil
	}

	return model.NewSubjectReferenceWithoutRelation(key), nil
}

// toCheckBulkCommand converts a v1beta2 CheckBulkRequest to a usecase CheckBulkCommand.
func toCheckBulkCommand(req *pb.CheckBulkRequest) (resources.CheckBulkCommand, error) {
	items := make([]resources.CheckBulkItem, len(req.GetItems()))
	for i, item := range req.GetItems() {
		resourceKey, err := reporterKeyFromResourceReference(item.GetObject())
		if err != nil {
			return resources.CheckBulkCommand{}, fmt.Errorf("invalid resource at index %d: %w", i, err)
		}
		subjectRef, err := subjectReferenceFromProto(item.GetSubject())
		if err != nil {
			return resources.CheckBulkCommand{}, fmt.Errorf("invalid subject at index %d: %w", i, err)
		}
		relation, err := model.NewRelation(item.GetRelation())
		if err != nil {
			return resources.CheckBulkCommand{}, fmt.Errorf("invalid relation at index %d: %w", i, err)
		}
		items[i] = resources.CheckBulkItem{
			Resource: resourceKey,
			Relation: relation,
			Subject:  subjectRef,
		}
	}

	consistency := consistencyFromProto(req.GetConsistency())
	return resources.CheckBulkCommand{
		Items:       items,
		Consistency: consistency,
	}, nil
}

// consistencyFromProto converts v1beta2 Consistency to model.Consistency.
func consistencyFromProto(c *pb.Consistency) model.Consistency {
	if c == nil || c.GetAtLeastAsFresh() == nil {
		return model.NewConsistencyMinimizeLatency()
	}
	token := model.DeserializeConsistencyToken(c.GetAtLeastAsFresh().GetToken())
	return model.NewConsistencyAtLeastAsFresh(token)
}

// fromCheckBulkResult converts a usecase CheckBulkResult to v1beta2 CheckBulkResponse.
func fromCheckBulkResult(result *resources.CheckBulkResult, req *pb.CheckBulkRequest) *pb.CheckBulkResponse {
	pairs := make([]*pb.CheckBulkResponsePair, len(result.Pairs))
	for i, pair := range result.Pairs {
		errResponse := &pb.CheckBulkResponsePair_Error{}
		itemResponse := &pb.CheckBulkResponsePair_Item{}

		if pair.Result.Error != nil {
			errorCode := pair.Result.ErrorCode
			if errorCode == 0 {
				errorCode = int32(codes.Internal)
			}
			log.Errorf("Error in checkbulk for item %d, code %d: %v", i, errorCode, pair.Result.Error)
			errResponse.Error = &rpcstatus.Status{
				Code:    errorCode,
				Message: pair.Result.Error.Error(),
			}
		}

		allowedResponse := pb.Allowed_ALLOWED_FALSE
		if pair.Result.Allowed {
			allowedResponse = pb.Allowed_ALLOWED_TRUE
		}
		itemResponse.Item = &pb.CheckBulkResponseItem{
			Allowed: allowedResponse,
		}

		// Use original request item for the response
		var requestItem *pb.CheckBulkRequestItem
		if i < len(req.GetItems()) {
			requestItem = req.GetItems()[i]
		}

		pairs[i] = &pb.CheckBulkResponsePair{
			Request: requestItem,
		}
		if pair.Result.Error != nil {
			pairs[i].Response = errResponse
		} else {
			pairs[i].Response = itemResponse
		}
	}

	resp := &pb.CheckBulkResponse{
		Pairs: pairs,
	}
	if result.ConsistencyToken != "" {
		resp.ConsistencyToken = &pb.ConsistencyToken{Token: result.ConsistencyToken.Serialize()}
	}
	return resp
}

// ConvertConsistencyToModel converts the proto Consistency to internal model type.
func ConvertConsistencyToModel(consistency *pb.Consistency) model.Consistency {
	if consistency == nil {
		// Return unspecified - the usecase layer will handle the default based on feature flag
		return model.NewConsistencyUnspecified()
	}

	if consistency.GetMinimizeLatency() {
		return model.NewConsistencyMinimizeLatency()
	}

	if consistency.GetAtLeastAsAcknowledged() {
		return model.NewConsistencyAtLeastAsAcknowledged()
	}

	if token := consistency.GetAtLeastAsFresh(); token != nil {
		return model.NewConsistencyAtLeastAsFresh(model.ConsistencyToken(token.GetToken()))
	}

	// Return unspecified - the usecase layer will handle the default based on feature flag
	return model.NewConsistencyUnspecified()
}

// toCheckSelfBulkCommand converts a v1beta2 CheckSelfBulkRequest to a usecase CheckSelfBulkCommand.
func toCheckSelfBulkCommand(req *pb.CheckSelfBulkRequest) (resources.CheckSelfBulkCommand, error) {
	items := make([]resources.CheckSelfBulkItem, len(req.GetItems()))
	for i, item := range req.GetItems() {
		resourceKey, err := reporterKeyFromResourceReference(item.GetObject())
		if err != nil {
			return resources.CheckSelfBulkCommand{}, fmt.Errorf("invalid resource at index %d: %w", i, err)
		}
		relation, err := model.NewRelation(item.GetRelation())
		if err != nil {
			return resources.CheckSelfBulkCommand{}, fmt.Errorf("invalid relation at index %d: %w", i, err)
		}
		items[i] = resources.CheckSelfBulkItem{
			Resource: resourceKey,
			Relation: relation,
		}
	}

	consistency := consistencyFromProto(req.GetConsistency())
	return resources.CheckSelfBulkCommand{
		Items:       items,
		Consistency: consistency,
	}, nil
}

// fromCheckSelfBulkResult converts a usecase CheckBulkResult to v1beta2 CheckSelfBulkResponse.
func fromCheckSelfBulkResult(result *resources.CheckBulkResult, req *pb.CheckSelfBulkRequest) *pb.CheckSelfBulkResponse {
	pairs := make([]*pb.CheckSelfBulkResponsePair, len(result.Pairs))
	for i, pair := range result.Pairs {
		errResponse := &pb.CheckSelfBulkResponsePair_Error{}
		itemResponse := &pb.CheckSelfBulkResponsePair_Item{}

		if pair.Result.Error != nil {
			errorCode := pair.Result.ErrorCode
			if errorCode == 0 {
				errorCode = int32(codes.Internal)
			}
			log.Errorf("Error in checkselfbulk for item %d, code %d: %v", i, errorCode, pair.Result.Error)
			errResponse.Error = &rpcstatus.Status{
				Code:    errorCode,
				Message: pair.Result.Error.Error(),
			}
		}

		allowedResponse := pb.Allowed_ALLOWED_FALSE
		if pair.Result.Allowed {
			allowedResponse = pb.Allowed_ALLOWED_TRUE
		}
		itemResponse.Item = &pb.CheckSelfBulkResponseItem{
			Allowed: allowedResponse,
		}

		// Use original request item for the response
		var requestItem *pb.CheckSelfBulkRequestItem
		if i < len(req.GetItems()) {
			requestItem = req.GetItems()[i]
		}

		pairs[i] = &pb.CheckSelfBulkResponsePair{
			Request: requestItem,
		}
		if pair.Result.Error != nil {
			pairs[i].Response = errResponse
		} else {
			pairs[i].Response = itemResponse
		}
	}

	resp := &pb.CheckSelfBulkResponse{
		Pairs: pairs,
	}
	if result.ConsistencyToken != "" {
		resp.ConsistencyToken = &pb.ConsistencyToken{Token: result.ConsistencyToken.Serialize()}
	}
	return resp
}

func (s *InventoryService) StreamedListObjects(
	req *pb.StreamedListObjectsRequest,
	stream pb.KesselInventoryService_StreamedListObjectsServer,
) error {
	ctx := stream.Context()

	if c := req.GetConsistency(); c != nil && c.GetAtLeastAsFresh() != nil {
		log.Debugf("StreamedListObjects consistency: at_least_as_fresh token=%s", c.GetAtLeastAsFresh().GetToken())
	} else {
		log.Debugf("StreamedListObjects consistency: minimize_latency")
	}

	lookupCmd, err := ToLookupResourcesCommand(req)
	if err != nil {
		return err
	}

	clientStream, err := s.Ctl.LookupResources(ctx, lookupCmd)
	if err != nil {
		return err
	}

	for {
		// Receive next message from the server stream
		resp, err := clientStream.Recv()
		if err == io.EOF {
			// Stream ended successfully
			return nil
		}
		if err != nil {
			return err
		}

		// Convert and send the response to the client
		if err := stream.Send(ToLookupResourceResponse(resp)); err != nil {
			return err
		}
	}
}

// ToLookupResourcesCommand converts a v1beta2 StreamedListObjectsRequest to a LookupResourcesCommand.
func ToLookupResourcesCommand(request *pb.StreamedListObjectsRequest) (resources.LookupResourcesCommand, error) {
	if request == nil {
		return resources.LookupResourcesCommand{}, fmt.Errorf("request is nil")
	}
	// TODO: value normalization should be moved to model
	resourceType, err := model.NewResourceType(NormalizeType(request.ObjectType.GetResourceType()))
	if err != nil {
		return resources.LookupResourcesCommand{}, fmt.Errorf("invalid resource type: %w", err)
	}
	reporterType, err := model.NewReporterType(NormalizeType(request.ObjectType.GetReporterType()))
	if err != nil {
		return resources.LookupResourcesCommand{}, fmt.Errorf("invalid reporter type: %w", err)
	}
	relation, err := model.NewRelation(request.Relation)
	if err != nil {
		return resources.LookupResourcesCommand{}, fmt.Errorf("invalid relation: %w", err)
	}
	subjectRef, err := subjectReferenceFromProto(request.Subject)
	if err != nil {
		return resources.LookupResourcesCommand{}, fmt.Errorf("invalid subject: %w", err)
	}
	consistency := ConvertConsistencyToModel(request.GetConsistency())

	var limit uint32 = 1000
	var continuation string
	if request.Pagination != nil {
		limit = request.Pagination.Limit
		if request.Pagination.ContinuationToken != nil {
			continuation = *request.Pagination.ContinuationToken
		}
	}

	return resources.LookupResourcesCommand{
		ResourceType: resourceType,
		ReporterType: reporterType,
		Relation:     relation,
		Subject:      subjectRef,
		Limit:        limit,
		Continuation: continuation,
		Consistency:  consistency,
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

// toReportResourceCommand converts a protobuf ReportResourceRequest to a domain ReportResourceCommand.
// This function handles all the conversion from presentation types to domain types.
func toReportResourceCommand(r *pb.ReportResourceRequest) (resources.ReportResourceCommand, error) {
	localResourceId, err := model.NewLocalResourceId(r.GetRepresentations().GetMetadata().GetLocalResourceId())
	if err != nil {
		return resources.ReportResourceCommand{}, fmt.Errorf("invalid local resource ID: %w", err)
	}

	resourceType, err := model.NewResourceType(r.GetType())
	if err != nil {
		return resources.ReportResourceCommand{}, fmt.Errorf("invalid resource type: %w", err)
	}

	reporterType, err := model.NewReporterType(r.GetReporterType())
	if err != nil {
		return resources.ReportResourceCommand{}, fmt.Errorf("invalid reporter type: %w", err)
	}

	reporterInstanceId, err := model.NewReporterInstanceId(r.GetReporterInstanceId())
	if err != nil {
		return resources.ReportResourceCommand{}, fmt.Errorf("invalid reporter instance ID: %w", err)
	}

	apiHref, err := model.NewApiHref(r.GetRepresentations().GetMetadata().GetApiHref())
	if err != nil {
		return resources.ReportResourceCommand{}, fmt.Errorf("invalid API href: %w", err)
	}

	var consoleHref *model.ConsoleHref
	if consoleHrefVal := r.GetRepresentations().GetMetadata().GetConsoleHref(); consoleHrefVal != "" {
		ch, err := model.NewConsoleHref(consoleHrefVal)
		if err != nil {
			return resources.ReportResourceCommand{}, fmt.Errorf("invalid console href: %w", err)
		}
		consoleHref = &ch
	}

	var reporterVersion *model.ReporterVersion
	if reporterVersionVal := r.GetRepresentations().GetMetadata().GetReporterVersion(); reporterVersionVal != "" {
		rv, err := model.NewReporterVersion(reporterVersionVal)
		if err != nil {
			return resources.ReportResourceCommand{}, fmt.Errorf("invalid reporter version: %w", err)
		}
		reporterVersion = &rv
	}

	var reporterRepresentation *model.Representation
	if r.GetRepresentations().GetReporter() != nil {
		rep, err := model.NewRepresentation(r.GetRepresentations().GetReporter().AsMap())
		if err != nil {
			return resources.ReportResourceCommand{}, fmt.Errorf("invalid reporter representation: %w", err)
		}
		reporterRepresentation = &rep
	}

	var commonRepresentation *model.Representation
	if r.GetRepresentations().GetCommon() != nil {
		rep, err := model.NewRepresentation(r.GetRepresentations().GetCommon().AsMap())
		if err != nil {
			return resources.ReportResourceCommand{}, fmt.Errorf("invalid common representation: %w", err)
		}
		commonRepresentation = &rep
	}

	var transactionId *model.TransactionId
	if txIdVal := r.GetRepresentations().GetMetadata().GetTransactionId(); txIdVal != "" {
		txId := model.NewTransactionId(txIdVal)
		transactionId = &txId
	}

	writeVisibility := writeVisibilityFromProto(r.GetWriteVisibility())

	return resources.ReportResourceCommand{
		LocalResourceId:        localResourceId,
		ResourceType:           resourceType,
		ReporterType:           reporterType,
		ReporterInstanceId:     reporterInstanceId,
		ApiHref:                apiHref,
		ConsoleHref:            consoleHref,
		ReporterVersion:        reporterVersion,
		TransactionId:          transactionId,
		ReporterRepresentation: reporterRepresentation,
		CommonRepresentation:   commonRepresentation,
		WriteVisibility:        writeVisibility,
	}, nil
}

// writeVisibilityFromProto converts a protobuf WriteVisibility to domain WriteVisibility.
func writeVisibilityFromProto(wv pb.WriteVisibility) resources.WriteVisibility {
	switch wv {
	case pb.WriteVisibility_IMMEDIATE:
		return resources.WriteVisibilityConsistent
	case pb.WriteVisibility_MINIMIZE_LATENCY:
		return resources.WriteVisibilityMinimizeLatency
	default:
		return resources.WriteVisibilityUnspecified
	}
}
