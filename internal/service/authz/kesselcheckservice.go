package service

import (
	"context"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relations"
)

type KesselCheckServiceService struct {
	pb.UnimplementedKesselCheckServiceServer
}

func NewKesselCheckServiceService() *KesselCheckServiceService {
	return &KesselCheckServiceService{}
}

func (s *KesselCheckServiceService) CheckForView(ctx context.Context, req *pb.CheckForViewRequest) (*pb.CheckForViewResponse, error) {
	return &pb.CheckForViewResponse{}, nil
}
func (s *KesselCheckServiceService) CheckForUpdate(ctx context.Context, req *pb.CheckForUpdateRequest) (*pb.CheckForUpdateResponse, error) {
	return &pb.CheckForUpdateResponse{}, nil
}
func (s *KesselCheckServiceService) CheckForCreate(ctx context.Context, req *pb.CheckForCreateRequest) (*pb.CheckForCreateResponse, error) {
	return &pb.CheckForCreateResponse{}, nil
}
