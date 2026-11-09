package resources

import (
	"context"
	"fmt"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	relationsusecase "github.com/project-kessel/inventory-api/internal/biz/usecase/relationships"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AddResourceRelationships adds one or more relationships to a resource without
// replacing the complete relationship state. Unlike ReportResource, only the
// relationship fields named in the request are affected; all other fields are
// left untouched. If the resource does not exist it is created with the
// provided relationship fields as its initial representation.
func (s *InventoryService) AddResourceRelationships(ctx context.Context, r *pb.AddResourceRelationshipsRequest) (*pb.AddResourceRelationshipsResponse, error) {
	cmd, err := toAddResourceRelationshipsCommand(r)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	if err := s.AddRelationshipsCtl.AddResourceRelationships(ctx, cmd); err != nil {
		return nil, err
	}

	return &pb.AddResourceRelationshipsResponse{}, nil
}

// toAddResourceRelationshipsCommand converts a protobuf AddResourceRelationshipsRequest
// to the domain AddResourceRelationshipsCommand used by the usecase layer.
func toAddResourceRelationshipsCommand(r *pb.AddResourceRelationshipsRequest) (relationsusecase.AddResourceRelationshipsCommand, error) {
	localResourceId, err := model.NewLocalResourceId(r.GetLocalResourceId())
	if err != nil {
		return relationsusecase.AddResourceRelationshipsCommand{}, fmt.Errorf("invalid local resource ID: %w", err)
	}

	resourceType, err := model.NewResourceType(r.GetType())
	if err != nil {
		return relationsusecase.AddResourceRelationshipsCommand{}, fmt.Errorf("invalid resource type: %w", err)
	}

	reporterType, err := model.NewReporterType(r.GetReporterType())
	if err != nil {
		return relationsusecase.AddResourceRelationshipsCommand{}, fmt.Errorf("invalid reporter type: %w", err)
	}

	reporterInstanceId, err := model.NewReporterInstanceId(r.GetReporterInstanceId())
	if err != nil {
		return relationsusecase.AddResourceRelationshipsCommand{}, fmt.Errorf("invalid reporter instance ID: %w", err)
	}

	reporterRepresentation, err := representationFromProtoStruct(r.GetReporter(), "reporter")
	if err != nil {
		return relationsusecase.AddResourceRelationshipsCommand{}, err
	}

	commonRepresentation, err := representationFromProtoStruct(r.GetCommon(), "common")
	if err != nil {
		return relationsusecase.AddResourceRelationshipsCommand{}, err
	}

	var transactionId *model.TransactionId
	if txIdVal := r.GetTransactionId(); txIdVal != "" {
		txId := model.NewTransactionId(txIdVal)
		transactionId = &txId
	}

	return relationsusecase.AddResourceRelationshipsCommand{
		LocalResourceId:        localResourceId,
		ResourceType:           resourceType,
		ReporterType:           reporterType,
		ReporterInstanceId:     reporterInstanceId,
		TransactionId:          transactionId,
		ReporterRepresentation: reporterRepresentation,
		CommonRepresentation:   commonRepresentation,
		WriteVisibility:        writeVisibilityFromProto(r.GetWriteVisibility()),
	}, nil
}
