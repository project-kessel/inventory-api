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
	resourceRef, err := resourceReferenceFromProto(req.Object)
	if err != nil {
		log.Error("Failed to build resource reference: ", err)
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
	consistency := consistencyFromProto(req.GetConsistency())
	result, err := s.Ctl.Check(ctx, relation, subjectRef, resourceRef, consistency)
	if err != nil {
		return nil, err
	}
	return viewResponseFromAuthzRequestV1beta2(result.Allowed(), result.ConsistencyToken()), nil
}

func (s *InventoryService) CheckForUpdate(ctx context.Context, req *pb.CheckForUpdateRequest) (*pb.CheckForUpdateResponse, error) {
	log.Info("CheckForUpdate using v1beta2 db")
	resourceRef, err := resourceReferenceFromProto(req.Object)
	if err != nil {
		log.Error("Failed to build resource reference: ", err)
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
	result, err := s.Ctl.CheckForUpdate(ctx, relation, subjectRef, resourceRef)
	if err != nil {
		return nil, err
	}
	return updateResponseFromAuthzRequestV1beta2(result.Allowed(), result.ConsistencyToken()), nil
}

func (s *InventoryService) CheckForUpdateBulk(ctx context.Context, req *pb.CheckForUpdateBulkRequest) (*pb.CheckForUpdateBulkResponse, error) {
	log.Info("CheckForUpdateBulk using v1beta2 db")

	cmd, err := toCheckForUpdateBulkCommand(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	resp, err := s.Ctl.CheckForUpdateBulk(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return fromCheckForUpdateBulkResult(resp, req), nil
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
	resourceRef, err := resourceReferenceFromProto(req.Object)
	if err != nil {
		return nil, err
	}
	relation, err := model.NewRelation(req.GetRelation())
	if err != nil {
		return nil, err
	}
	consistency := consistencyFromProto(req.GetConsistency())
	log.Debugf("CheckSelf request consistency: %s", model.ConsistencyTypeOf(consistency))
	result, err := s.Ctl.CheckSelf(ctx, relation, resourceRef, consistency)
	if err != nil {
		return nil, err
	}
	allowed := pb.Allowed_ALLOWED_FALSE
	if result.Allowed() {
		allowed = pb.Allowed_ALLOWED_TRUE
	}
	response := &pb.CheckSelfResponse{Allowed: allowed}
	if result.ConsistencyToken() != model.MinimizeLatencyToken {
		response.ConsistencyToken = &pb.ConsistencyToken{Token: result.ConsistencyToken().Serialize()}
	}
	return response, nil
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

// resourceReferenceFromProto converts a v1beta2 ResourceReference to a model.ResourceReference.
// Used by Relations-flow endpoints (Check, CheckForUpdate, CheckBulk, LookupSubjects, etc.).
func resourceReferenceFromProto(resource *pb.ResourceReference) (model.ResourceReference, error) {
	localResourceId, err := model.NewLocalResourceId(resource.GetResourceId())
	if err != nil {
		return model.ResourceReference{}, err
	}
	resourceType, err := model.NewResourceType(resource.GetResourceType())
	if err != nil {
		return model.ResourceReference{}, err
	}
	reporterType, err := model.NewReporterType(resource.GetReporter().GetType())
	if err != nil {
		return model.ResourceReference{}, err
	}

	var instanceId *model.ReporterInstanceId
	if id := resource.GetReporter().GetInstanceId(); id != "" {
		rid, err := model.NewReporterInstanceId(id)
		if err != nil {
			return model.ResourceReference{}, err
		}
		instanceId = &rid
	}

	reporter := model.NewReporterReference(reporterType, instanceId)
	return model.NewResourceReference(resourceType, localResourceId, &reporter), nil
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
	reporterType, err := model.NewReporterType(subject.Resource.GetReporter().GetType())
	if err != nil {
		return model.SubjectReference{}, err
	}

	reporter := model.NewReporterReference(reporterType, nil)
	resource := model.NewResourceReference(resourceType, localResourceId, &reporter)

	if subject.GetRelation() != "" {
		relation, err := model.NewRelation(subject.GetRelation())
		if err != nil {
			return model.SubjectReference{}, err
		}
		return model.NewSubjectReference(resource, &relation), nil
	}

	return model.NewSubjectReferenceWithoutRelation(resource), nil
}

// protoToCheckBulkItem converts a single *pb.CheckBulkRequestItem to a resources.CheckBulkItem.
// Both CheckBulkRequest and CheckForUpdateBulkRequest share the same item type, so this helper
// is reused by toCheckBulkCommand and toCheckForUpdateBulkCommand.
func protoToCheckBulkItem(item *pb.CheckBulkRequestItem, idx int) (resources.CheckBulkItem, error) {
	resourceRef, err := resourceReferenceFromProto(item.GetObject())
	if err != nil {
		return resources.CheckBulkItem{}, fmt.Errorf("invalid resource at index %d: %w", idx, err)
	}
	subjectRef, err := subjectReferenceFromProto(item.GetSubject())
	if err != nil {
		return resources.CheckBulkItem{}, fmt.Errorf("invalid subject at index %d: %w", idx, err)
	}
	relation, err := model.NewRelation(item.GetRelation())
	if err != nil {
		return resources.CheckBulkItem{}, fmt.Errorf("invalid relation at index %d: %w", idx, err)
	}
	return resources.CheckBulkItem{
		Resource: resourceRef,
		Relation: relation,
		Subject:  subjectRef,
	}, nil
}

// toCheckForUpdateBulkCommand converts a v1beta2 CheckForUpdateBulkRequest to a usecase CheckForUpdateBulkCommand.
func toCheckForUpdateBulkCommand(req *pb.CheckForUpdateBulkRequest) (resources.CheckForUpdateBulkCommand, error) {
	items := make([]resources.CheckBulkItem, len(req.GetItems()))
	for i, item := range req.GetItems() {
		bulkItem, err := protoToCheckBulkItem(item, i)
		if err != nil {
			return resources.CheckForUpdateBulkCommand{}, err
		}
		items[i] = bulkItem
	}
	return resources.CheckForUpdateBulkCommand{Items: items}, nil
}

// toCheckBulkCommand converts a v1beta2 CheckBulkRequest to a usecase CheckBulkCommand.
func toCheckBulkCommand(req *pb.CheckBulkRequest) (resources.CheckBulkCommand, error) {
	items := make([]resources.CheckBulkItem, len(req.GetItems()))
	for i, item := range req.GetItems() {
		bulkItem, err := protoToCheckBulkItem(item, i)
		if err != nil {
			return resources.CheckBulkCommand{}, err
		}
		items[i] = bulkItem
	}

	consistency := consistencyFromProto(req.GetConsistency())
	return resources.CheckBulkCommand{
		Items:       items,
		Consistency: consistency,
	}, nil
}

// ConsistencyFromProto converts v1beta2 Consistency to model.Consistency.
// Used by Check, CheckSelf, CheckBulk, CheckSelfBulk, and LookupResources.
func consistencyFromProto(c *pb.Consistency) model.Consistency {
	if c.GetMinimizeLatency() {
		return model.NewConsistencyMinimizeLatency()
	}
	if c.GetAtLeastAsAcknowledged() {
		return model.NewConsistencyAtLeastAsAcknowledged()
	}
	if c.GetAtLeastAsFresh() != nil {
		token := model.DeserializeConsistencyToken(c.GetAtLeastAsFresh().GetToken())
		return model.NewConsistencyAtLeastAsFresh(token)
	}
	return model.NewConsistencyUnspecified()
}

func paginationFromProto(p *pb.RequestPagination) *model.Pagination {
	if p == nil {
		return nil
	}
	return &model.Pagination{
		Limit:        p.Limit,
		Continuation: p.ContinuationToken,
	}
}

// checkBulkResultItemToProtoFields derives the proto Allowed enum and, if the result carries an
// error, a populated *rpcstatus.Status ready for embedding in a response pair. opName is used
// only for the error log message. Returns (allowed, nil) when the item succeeded.
func checkBulkResultItemToProtoFields(item resources.CheckBulkResultItem, idx int, opName string) (pb.Allowed, *rpcstatus.Status) {
	allowed := pb.Allowed_ALLOWED_FALSE
	if item.Allowed {
		allowed = pb.Allowed_ALLOWED_TRUE
	}
	if item.Error == nil {
		return allowed, nil
	}
	errorCode := item.ErrorCode
	if errorCode == 0 {
		errorCode = int32(codes.Internal)
	}
	log.Errorf("Error in %s for item %d, code %d: %v", opName, idx, errorCode, item.Error)
	return allowed, &rpcstatus.Status{
		Code:    errorCode,
		Message: item.Error.Error(),
	}
}

// fromCheckBulkResult converts a usecase CheckBulkResult to v1beta2 CheckBulkResponse.
func fromCheckBulkResult(result *resources.CheckBulkResult, req *pb.CheckBulkRequest) *pb.CheckBulkResponse {
	pairs := make([]*pb.CheckBulkResponsePair, len(result.Pairs))
	for i, pair := range result.Pairs {
		allowed, errStatus := checkBulkResultItemToProtoFields(pair.Result, i, "checkbulk")

		var requestItem *pb.CheckBulkRequestItem
		if i < len(req.GetItems()) {
			requestItem = req.GetItems()[i]
		}

		p := &pb.CheckBulkResponsePair{Request: requestItem}
		if errStatus != nil {
			p.Response = &pb.CheckBulkResponsePair_Error{Error: errStatus}
		} else {
			p.Response = &pb.CheckBulkResponsePair_Item{Item: &pb.CheckBulkResponseItem{Allowed: allowed}}
		}
		pairs[i] = p
	}

	resp := &pb.CheckBulkResponse{Pairs: pairs}
	if result.ConsistencyToken != "" {
		resp.ConsistencyToken = &pb.ConsistencyToken{Token: result.ConsistencyToken.Serialize()}
	}
	return resp
}

// fromCheckForUpdateBulkResult converts a usecase CheckBulkResult to v1beta2 CheckForUpdateBulkResponse.
func fromCheckForUpdateBulkResult(result *resources.CheckBulkResult, req *pb.CheckForUpdateBulkRequest) *pb.CheckForUpdateBulkResponse {
	pairs := make([]*pb.CheckForUpdateBulkResponsePair, len(result.Pairs))
	for i, pair := range result.Pairs {
		allowed, errStatus := checkBulkResultItemToProtoFields(pair.Result, i, "checkforupdatebulk")

		var requestItem *pb.CheckBulkRequestItem
		if i < len(req.GetItems()) {
			requestItem = req.GetItems()[i]
		}

		p := &pb.CheckForUpdateBulkResponsePair{Request: requestItem}
		if errStatus != nil {
			p.Response = &pb.CheckForUpdateBulkResponsePair_Error{Error: errStatus}
		} else {
			p.Response = &pb.CheckForUpdateBulkResponsePair_Item{Item: &pb.CheckForUpdateBulkResponseItem{Allowed: allowed}}
		}
		pairs[i] = p
	}

	resp := &pb.CheckForUpdateBulkResponse{Pairs: pairs}
	if result.ConsistencyToken != "" {
		resp.ConsistencyToken = &pb.ConsistencyToken{Token: result.ConsistencyToken.Serialize()}
	}
	return resp
}

// toCheckSelfBulkCommand converts a v1beta2 CheckSelfBulkRequest to a usecase CheckSelfBulkCommand.
func toCheckSelfBulkCommand(req *pb.CheckSelfBulkRequest) (resources.CheckSelfBulkCommand, error) {
	items := make([]resources.CheckSelfBulkItem, len(req.GetItems()))
	for i, item := range req.GetItems() {
		resourceRef, err := resourceReferenceFromProto(item.GetObject())
		if err != nil {
			return resources.CheckSelfBulkCommand{}, fmt.Errorf("invalid resource at index %d: %w", i, err)
		}
		relation, err := model.NewRelation(item.GetRelation())
		if err != nil {
			return resources.CheckSelfBulkCommand{}, fmt.Errorf("invalid relation at index %d: %w", i, err)
		}
		items[i] = resources.CheckSelfBulkItem{
			Resource: resourceRef,
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

	consistency := consistencyFromProto(req.GetConsistency())
	log.Debugf("StreamedListObjects consistency: %s", model.ConsistencyTypeOf(consistency))

	lookupCmd, err := ToLookupObjectsCommand(req)
	if err != nil {
		return err
	}

	clientStream, err := s.Ctl.LookupObjects(ctx, lookupCmd)
	if err != nil {
		return err
	}

	for {
		item, err := clientStream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if err := stream.Send(ToLookupObjectsResponse(item)); err != nil {
			return err
		}
	}
}

func (s *InventoryService) StreamedListSubjects(
	req *pb.StreamedListSubjectsRequest,
	stream pb.KesselInventoryService_StreamedListSubjectsServer,
) error {
	ctx := stream.Context()

	consistency := consistencyFromProto(req.GetConsistency())
	log.Debugf("StreamedListSubjects consistency: %s", model.ConsistencyTypeOf(consistency))

	lookupCmd, err := ToLookupSubjectsCommand(req)
	if err != nil {
		return err
	}

	clientStream, err := s.Ctl.LookupSubjects(ctx, lookupCmd)
	if err != nil {
		return err
	}

	for {
		item, err := clientStream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if err := stream.Send(ToLookupSubjectsResponse(item)); err != nil {
			return err
		}
	}
}

// ToLookupObjectsCommand converts a v1beta2 StreamedListObjectsRequest to a LookupObjectsCommand.
func ToLookupObjectsCommand(request *pb.StreamedListObjectsRequest) (resources.LookupObjectsCommand, error) {
	if request == nil {
		return resources.LookupObjectsCommand{}, fmt.Errorf("request is nil")
	}
	resourceType, err := model.NewResourceType(NormalizeType(request.ObjectType.GetResourceType()))
	if err != nil {
		return resources.LookupObjectsCommand{}, fmt.Errorf("invalid resource type: %w", err)
	}
	reporterType, err := model.NewReporterType(NormalizeType(request.ObjectType.GetReporterType()))
	if err != nil {
		return resources.LookupObjectsCommand{}, fmt.Errorf("invalid reporter type: %w", err)
	}
	relation, err := model.NewRelation(request.Relation)
	if err != nil {
		return resources.LookupObjectsCommand{}, fmt.Errorf("invalid relation: %w", err)
	}
	subjectRef, err := subjectReferenceFromProto(request.Subject)
	if err != nil {
		return resources.LookupObjectsCommand{}, fmt.Errorf("invalid subject: %w", err)
	}

	objectType := model.NewRepresentationTypeRequired(resourceType, reporterType)
	return resources.LookupObjectsCommand{
		ObjectType:  objectType,
		Relation:    relation,
		Subject:     subjectRef,
		Pagination:  paginationFromProto(request.Pagination),
		Consistency: consistencyFromProto(request.GetConsistency()),
	}, nil
}

func NormalizeType(val string) string {
	normalized := strings.ToLower(val)
	return normalized
}

func ToLookupObjectsResponse(item model.LookupObjectsItem) *pb.StreamedListObjectsResponse {
	obj := item.Object()
	resp := &pb.StreamedListObjectsResponse{
		Object: &pb.ResourceReference{
			ResourceId:   obj.ResourceId().String(),
			ResourceType: obj.ResourceType().String(),
		},
		Pagination: &pb.ResponsePagination{
			ContinuationToken: item.ContinuationToken(),
		},
	}
	if obj.HasReporter() {
		resp.Object.Reporter = &pb.ReporterReference{
			Type: obj.Reporter().ReporterType().String(),
		}
	}
	return resp
}

// ToLookupSubjectsCommand converts a v1beta2 LookupSubjectsRequest to a LookupSubjectsCommand.
func ToLookupSubjectsCommand(request *pb.StreamedListSubjectsRequest) (resources.LookupSubjectsCommand, error) {
	if request == nil {
		return resources.LookupSubjectsCommand{}, fmt.Errorf("request is nil")
	}
	resourceRef, err := resourceReferenceFromProto(request.Resource)
	if err != nil {
		return resources.LookupSubjectsCommand{}, fmt.Errorf("invalid resource: %w", err)
	}
	relation, err := model.NewRelation(request.Relation)
	if err != nil {
		return resources.LookupSubjectsCommand{}, fmt.Errorf("invalid relation: %w", err)
	}
	subjectResType, err := model.NewResourceType(NormalizeType(request.SubjectType.GetResourceType()))
	if err != nil {
		return resources.LookupSubjectsCommand{}, fmt.Errorf("invalid subject type: %w", err)
	}
	subjectReporter, err := model.NewReporterType(NormalizeType(request.SubjectType.GetReporterType()))
	if err != nil {
		return resources.LookupSubjectsCommand{}, fmt.Errorf("invalid subject reporter: %w", err)
	}

	var subjectRelation *model.Relation
	if request.SubjectRelation != nil && *request.SubjectRelation != "" {
		rel, err := model.NewRelation(*request.SubjectRelation)
		if err != nil {
			return resources.LookupSubjectsCommand{}, fmt.Errorf("invalid subject relation: %w", err)
		}
		subjectRelation = &rel
	}

	return resources.LookupSubjectsCommand{
		Resource:        resourceRef,
		Relation:        relation,
		SubjectType:     model.NewRepresentationTypeRequired(subjectResType, subjectReporter),
		SubjectRelation: subjectRelation,
		Pagination:      paginationFromProto(request.Pagination),
		Consistency:     consistencyFromProto(request.GetConsistency()),
	}, nil
}

func ToLookupSubjectsResponse(item model.LookupSubjectsItem) *pb.StreamedListSubjectsResponse {
	subResource := item.Subject().Resource()
	subRef := &pb.SubjectReference{
		Resource: &pb.ResourceReference{
			ResourceId:   subResource.ResourceId().String(),
			ResourceType: subResource.ResourceType().String(),
		},
	}
	if subResource.HasReporter() {
		subRef.Resource.Reporter = &pb.ReporterReference{
			Type: subResource.Reporter().ReporterType().String(),
		}
	}
	if item.Subject().HasRelation() {
		rel := item.Subject().Relation().String()
		subRef.Relation = &rel
	}
	return &pb.StreamedListSubjectsResponse{
		Subject: subRef,
		Pagination: &pb.ResponsePagination{
			ContinuationToken: item.ContinuationToken(),
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

func viewResponseFromAuthzRequestV1beta2(allowed bool, consistencyToken model.ConsistencyToken) *pb.CheckResponse {
	response := &pb.CheckResponse{}
	if allowed {
		response.Allowed = pb.Allowed_ALLOWED_TRUE
	} else {
		response.Allowed = pb.Allowed_ALLOWED_FALSE
	}
	if consistencyToken != model.MinimizeLatencyToken {
		response.ConsistencyToken = &pb.ConsistencyToken{Token: consistencyToken.Serialize()}
	}
	return response
}

func updateResponseFromAuthzRequestV1beta2(allowed bool, consistencyToken model.ConsistencyToken) *pb.CheckForUpdateResponse {
	response := &pb.CheckForUpdateResponse{}
	if allowed {
		response.Allowed = pb.Allowed_ALLOWED_TRUE
	} else {
		response.Allowed = pb.Allowed_ALLOWED_FALSE
	}
	if consistencyToken != "" {
		response.ConsistencyToken = &pb.ConsistencyToken{Token: consistencyToken.Serialize()}
	}
	return response
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
		reporterMap := r.GetRepresentations().GetReporter().AsMap()
		if len(reporterMap) == 0 {
			return resources.ReportResourceCommand{}, fmt.Errorf("representation data cannot be empty")
		}
		rep, err := model.NewRepresentation(reporterMap)
		if err != nil {
			return resources.ReportResourceCommand{}, fmt.Errorf("invalid reporter representation: %w", err)
		}
		reporterRepresentation = &rep
	}

	var commonRepresentation *model.Representation
	if r.GetRepresentations().GetCommon() != nil {
		commonMap := r.GetRepresentations().GetCommon().AsMap()
		if len(commonMap) == 0 {
			return resources.ReportResourceCommand{}, fmt.Errorf("representation data cannot be empty")
		}
		rep, err := model.NewRepresentation(commonMap)
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
