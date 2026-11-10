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

// RemoveResourceRelationships removes one or more specific relationships from
// a resource without asserting the complete remaining relationship state.
// Only the relationship values named in the request are removed; all other
// relationships are left untouched. Returns NOT_FOUND if the resource does
// not exist or is tombstoned.
func (s *InventoryService) RemoveResourceRelationships(ctx context.Context, r *pb.RemoveResourceRelationshipsRequest) (*pb.RemoveResourceRelationshipsResponse, error) {
	cmd, err := toRemoveResourceRelationshipsCommand(r)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	if err := s.RemoveRelationshipsCtl.RemoveResourceRelationships(ctx, cmd); err != nil {
		return nil, err
	}

	return &pb.RemoveResourceRelationshipsResponse{}, nil
}

// toRemoveResourceRelationshipsCommand converts a protobuf RemoveResourceRelationshipsRequest
// to the domain RemoveResourceRelationshipsCommand used by the usecase layer.
func toRemoveResourceRelationshipsCommand(r *pb.RemoveResourceRelationshipsRequest) (relationsusecase.RemoveResourceRelationshipsCommand, error) {
	localResourceId, err := model.NewLocalResourceId(r.GetLocalResourceId())
	if err != nil {
		return relationsusecase.RemoveResourceRelationshipsCommand{}, fmt.Errorf("invalid local resource ID: %w", err)
	}

	resourceType, err := model.NewResourceType(r.GetType())
	if err != nil {
		return relationsusecase.RemoveResourceRelationshipsCommand{}, fmt.Errorf("invalid resource type: %w", err)
	}

	reporterType, err := model.NewReporterType(r.GetReporterType())
	if err != nil {
		return relationsusecase.RemoveResourceRelationshipsCommand{}, fmt.Errorf("invalid reporter type: %w", err)
	}

	reporterInstanceId, err := model.NewReporterInstanceId(r.GetReporterInstanceId())
	if err != nil {
		return relationsusecase.RemoveResourceRelationshipsCommand{}, fmt.Errorf("invalid reporter instance ID: %w", err)
	}

	reporterRepresentation, err := representationFromProtoStruct(r.GetReporter(), "reporter")
	if err != nil {
		return relationsusecase.RemoveResourceRelationshipsCommand{}, err
	}

	commonRepresentation, err := representationFromProtoStruct(r.GetCommon(), "common")
	if err != nil {
		return relationsusecase.RemoveResourceRelationshipsCommand{}, err
	}

	var transactionId *model.TransactionId
	if txIdVal := r.GetTransactionId(); txIdVal != "" {
		txId := model.NewTransactionId(txIdVal)
		transactionId = &txId
	}

	return relationsusecase.RemoveResourceRelationshipsCommand{
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
