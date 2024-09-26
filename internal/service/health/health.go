package health

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"github.com/project-kessel/inventory-api/internal/authz"
	biz "github.com/project-kessel/inventory-api/internal/biz/health"
	"github.com/spf13/viper"
)

type HealthService struct {
	pb.UnimplementedKesselInventoryHealthServiceServer
	Ctl    *biz.HealthUsecase
	Config *authz.CompletedConfig
}

func New(c *biz.HealthUsecase) *HealthService {
	return &HealthService{
		Ctl: c,
	}
}

func (s *HealthService) GetLivez(ctx context.Context, req *pb.GetLivezRequest) (*pb.GetLivezResponse, error) {
	if viper.GetBool("log.livez") {
		log.Infof("Performing livez check")
	}
	return &pb.GetLivezResponse{
		Status: "OK",
		Code:   200,
	}, nil
}

func (s *HealthService) GetReadyz(ctx context.Context, req *pb.GetReadyzRequest) (*pb.GetReadyzResponse, error) {
	if viper.GetBool("log.readyz") {
		log.Infof("Performing readyz check")
	}
	return s.Ctl.IsBackendAvailable(ctx)
}
