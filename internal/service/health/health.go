package health

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	biz "github.com/project-kessel/inventory-api/internal/biz/health"
)

type HealthService struct {
	pb.UnimplementedKesselInventoryHealthServiceServer

	Ctl *biz.HealthUsecase
}

func New(c *biz.HealthUsecase) *HealthService {
	return &HealthService{
		Ctl: c,
	}
}

func (s *HealthService) GetLivez(ctx context.Context, req *pb.GetLivezRequest) (*pb.GetLivezResponse, error) {
	log.Infof("Performing livez check")
	return &pb.GetLivezResponse{
		Status: "OK",
		Code:   200,
	}, nil
}

func (s *HealthService) GetReadyz(ctx context.Context, req *pb.GetReadyzRequest) (*pb.GetReadyzResponse, error) {

	return s.Ctl.IsBackendAvailable(ctx)
}
