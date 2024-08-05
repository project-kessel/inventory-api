package health

import (
	"context"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
)

type HealthService struct {
	pb.UnimplementedInventoryHealthServiceServer
}

func NewHealthService() *HealthService {
	return &HealthService{}
}

func (s *HealthService) GetLivez(ctx context.Context, req *pb.GetLivezRequest) (*pb.GetLivezResponse, error) {
	return &pb.GetLivezResponse{Status: "OK", Code: 200}, nil
}

func (s *HealthService) GetReadyz(ctx context.Context, req *pb.GetReadyzRequest) (*pb.GetReadyzResponse, error) {
	return &pb.GetReadyzResponse{Status: "OK", Code: 200}, nil
}
