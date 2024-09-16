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
	sqlDB, dbErr := r.DB.DB()
	if dbErr == nil {
		dbErr = sqlDB.PingContext(ctx)
	}
	health, apiErr := r.Authz.Health(ctx)

	if dbErr != nil && apiErr != nil {
		log.Errorf("STORAGE UNHEALTHY: %s and RELATIONS-API UNHEALTHY", storageType)
		return newResponse("STORAGE UNHEALTHY: "+storageType+" and RELATIONS-API UNHEALTHY", 500), nil
	} else if dbErr != nil {
		log.Errorf("STORAGE UNHEALTHY: %s", storageType)
		return newResponse("STORAGE UNHEALTHY: "+storageType, 500), nil
	} else if apiErr != nil {
		log.Errorf("RELATIONS-API UNHEALTHY")
		return newResponse("RELATIONS-API UNHEALTHY", 500), nil
	}
	log.Infof("Storage type %s and relations-api %s", storageType, health.GetStatus())
	return newResponse("Storage type "+storageType+" and relations-api "+health.GetStatus(), 200), nil
}

func newResponse(status string, code int) *pb.GetReadyzResponse {
	return &pb.GetReadyzResponse{
		Status: status,
		Code:   uint32(code),
	}
}
