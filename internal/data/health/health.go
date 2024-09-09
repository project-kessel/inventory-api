package health

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"gorm.io/gorm"
)

type healthRepo struct {
	DB *gorm.DB
}

func New(g *gorm.DB) *healthRepo {
	return &healthRepo{
		DB: g,
	}
}

func (r *healthRepo) IsBackendAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	storageType := r.DB.Dialector.Name()
	log.Infof("Checking readiness for storage type: %s", storageType)

	sqlDB, err := r.DB.DB()
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
