package health

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"github.com/project-kessel/inventory-api/internal/authz"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type healthRepo struct {
	DB            *gorm.DB
	Authz         authzapi.Authorizer
	CompletedAuth authz.CompletedConfig
}

func New(g *gorm.DB, a authzapi.Authorizer, completedAuth authz.CompletedConfig) *healthRepo {
	return &healthRepo{
		DB:            g,
		Authz:         a,
		CompletedAuth: completedAuth,
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
	}

	if dbErr != nil {
		log.Errorf("STORAGE UNHEALTHY: %s", storageType)
		return newResponse("STORAGE UNHEALTHY: "+storageType, 500), nil
	}

	if apiErr != nil {
		log.Errorf("RELATIONS-API UNHEALTHY")
		return newResponse("RELATIONS-API UNHEALTHY", 500), nil
	}
	if authz.CheckAuthorizer(r.CompletedAuth) == "Kessel" {
		if viper.GetBool("log.readyz") {
			log.Infof("Storage type %s and relations-api %s", storageType, health.GetStatus())
		}
		return newResponse("STORAGE "+storageType+" and RELATIONS-API", 200), nil
	}

	return newResponse("Storage type "+storageType, 200), nil
}

func (r *healthRepo) IsRelationsAvailable(ctx context.Context) (*pb.GetReadyzResponse, error) {
	health, apiErr := r.Authz.Health(ctx)
	if apiErr != nil {
		log.Errorf("RELATIONS-API UNHEALTHY")
		return newResponse("RELATIONS-API UNHEALTHY", 500), nil
	}
	if authz.CheckAuthorizer(r.CompletedAuth) == "Kessel" {
		if viper.GetBool("log.readyz") {
			log.Infof("relations-api %s", health.GetStatus())
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
