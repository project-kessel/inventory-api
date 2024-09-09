package health

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
)

type HealthService struct {
	pb.UnimplementedKesselInventoryHealthServiceServer
	DB *gorm.DB
}

func NewHealthService(db *gorm.DB) *HealthService {
	return &HealthService{
		DB: db,
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
	//determine storage type based on Gorm Dialector
	storageType := s.DB.Dialector.Name()
	log.Infof("Checking readiness for storage type: %s", storageType)

	sqlDB, err := s.DB.DB()
	if err != nil {
		log.Errorf("Failed to retrieve DB: %v", err)
		return &pb.GetReadyzResponse{
			Status: "STORAGE UNHEALTHY: " + storageType,
			Code:   500,
		}, nil
	}
	err = sqlDB.PingContext(ctx)
	if err != nil {
		log.Errorf("Failed to ping database: %v", err)
		return &pb.GetReadyzResponse{
			Status: "FAILED TO PING DB: " + storageType,
			Code:   500,
		}, nil
	}

	log.Infof("Storage type %s is healthy", storageType)
	return &pb.GetReadyzResponse{
		Status: "OK: " + storageType,
		Code:   200,
	}, nil
}
