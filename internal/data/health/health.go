package health

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"gorm.io/gorm"
)

type healthRepo struct {
	DB    *gorm.DB
	Authz authzapi.Authorizer
}

func New(g *gorm.DB, a authzapi.Authorizer) *healthRepo {
	return &healthRepo{
		DB:    g,
		Authz: a,
	}
}

func (r *healthRepo) IsBackendAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	storageType := r.DB.Dialector.Name()
	log.Infof("Checking readiness for storage type: %s", storageType)

	sqlDB, err := r.DB.DB()
	if err != nil {
		log.Errorf("Failed to retrieve DB: %v", err)
		return newResponse("STORAGE UNHEALTHY: "+storageType, 500), nil
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		log.Errorf("Failed to ping database: %v", err)
		return newResponse("FAILED TO PING DB: "+storageType, 500), nil
	} else {
		log.Infof("Successfully pinged %s database", storageType)
	}
	// if kessel is enabled
	health, err := r.Authz.Health(ctx)
	log.Infof("health status %s error status %s", health, err)
	if err != nil {
		log.Infof("Checking readiness of relations-api")

		//log.Infof("Storage type %s and relations API are healthy", storageType)
		return newResponse("KESSEL UNHEALTHY:"+health.GetStatus(), 500), nil
	}
	return newResponse("OK: "+storageType, 200), nil // relations down (500)
}

func newResponse(status string, code int) *pb.GetReadyzResponse {
	return &pb.GetReadyzResponse{
		Status: status,
		Code:   uint32(code),
	}
}
