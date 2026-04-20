package health

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/config/relations"
)

type healthRepo struct {
	DB              *gorm.DB
	Relations       model.RelationsRepository
	RelationsConfig relations.CompletedConfig
}

func New(g *gorm.DB, a model.RelationsRepository, relationsConfig relations.CompletedConfig) *healthRepo {
	return &healthRepo{
		DB:              g,
		Relations:       a,
		RelationsConfig: relationsConfig,
	}
}

func (r *healthRepo) IsBackendAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	storageType := r.DB.Name()
	sqlDB, dbErr := r.DB.DB()
	if dbErr == nil {
		dbErr = sqlDB.PingContext(ctx)
	}
	health, apiErr := r.Relations.Health(ctx)

	if dbErr != nil && apiErr != nil {
		log.Errorf("STORAGE UNHEALTHY: %s and RELATIONS-API UNHEALTHY", storageType)
		return newResponse("STORAGE UNHEALTHY: "+storageType+" and RELATIONS-API UNHEALTHY", 500), nil
	}

	if dbErr != nil {
		log.Errorf("STORAGE UNHEALTHY: %s", storageType)
		return newResponse("STORAGE UNHEALTHY: "+storageType, 500), nil
	}

	if apiErr != nil {
		log.Errorf("RELATIONS-API UNHEALTHY")
		return newResponse("RELATIONS-API UNHEALTHY", 500), nil
	}
	if relations.CheckRelationsImpl(r.RelationsConfig) == "Kessel" {
		if viper.GetBool("log.readyz") {
			log.Infof("Storage type %s and relations-api %s", storageType, health.Status)
		}
		return newResponse("STORAGE "+storageType+" and RELATIONS-API", 200), nil
	}

	return newResponse("Storage type "+storageType, 200), nil
}

func (r *healthRepo) IsRelationsAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	health, apiErr := r.Relations.Health(ctx)
	if apiErr != nil {
		log.Errorf("RELATIONS-API UNHEALTHY")
		return newResponse("RELATIONS-API UNHEALTHY", 500), nil
	}
	if relations.CheckRelationsImpl(r.RelationsConfig) == "Kessel" {
		if viper.GetBool("log.readyz") {
			log.Infof("relations-api %s", health.Status)
		}
	}
	return newResponse("RELATIONS-API", 200), nil
}

func newResponse(status string, code int) *pb.GetReadyzResponse {
	return &pb.GetReadyzResponse{
		Status: status,
		Code:   uint32(code),
	}
}
