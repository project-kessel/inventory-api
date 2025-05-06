package health

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/viper"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"github.com/project-kessel/inventory-api/internal/authz"
	biz "github.com/project-kessel/inventory-api/internal/biz/health"
)

type HealthService struct {
	pb.UnimplementedKesselInventoryHealthServiceServer
	Ctl    *biz.HealthUsecase
	Config *authz.CompletedConfig
}

var (
	livezLogged  = false
	readyzLogged = false
)

func New(c *biz.HealthUsecase) *HealthService {
	return &HealthService{
		Ctl: c,
	}
}

func (s *HealthService) GetLivez(ctx context.Context, req *pb.GetLivezRequest) (*pb.GetLivezResponse, error) {
	if viper.GetBool("log.livez") {
		log.Infof("Performing livez check")
	} else {
		if !livezLogged {
			log.Infof("Livez logs disabled")
			livezLogged = true
		}
	}
	return &pb.GetLivezResponse{
		Status: "OK",
		Code:   200,
	}, nil
}

func (s *HealthService) GetReadyz(ctx context.Context, req *pb.GetReadyzRequest) (*pb.GetReadyzResponse, error) {
	if viper.GetBool("log.readyz") {
		log.Infof("Performing readyz check")
	} else {
		if !readyzLogged {
			log.Infof("Readyz logs disabled")
			readyzLogged = true
		}
	}
	return s.Ctl.IsBackendAvailable(ctx)
}
