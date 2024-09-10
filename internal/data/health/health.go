package health

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"gorm.io/gorm"
	"net/http"
	"time"
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
		return newResponse("STORAGE UNHEALTHY: "+storageType, 500), nil
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		log.Errorf("Failed to ping database: %v", err)
		return newResponse("FAILED TO PING DB: "+storageType, 500), nil
	} else {
		log.Infof("Successfully pinged " + storageType + " database")
	}

	// check relations-api livez and readyz endpoints
	log.Infof("Checking readiness of relations-api")
	client := &http.Client{Timeout: 5 * time.Second}
	if err := checkRelationsAPIEndpoint(client, "livez"); err != nil {
		return newResponse("RELATIONS API UNHEALTHY (livez): "+err.Error(), 500), nil
	}
	if err := checkRelationsAPIEndpoint(client, "readyz"); err != nil {
		return newResponse("RELATIONS API UNHEALTHY (readyz): "+err.Error(), 500), nil
	}

	log.Infof("Storage type %s and relations API are healthy", storageType)
	return newResponse("OK: "+storageType+" and relations API", 200), nil
}

func checkRelationsAPIEndpoint(client *http.Client, checkType string) error {
	resp, err := client.Get("http://host.docker.internal:8000/api/authz/" + checkType)
	if err != nil {
		log.Errorf("Failed to perform %s check: %v", checkType, err)
		return err
	}
	defer resp.Body.Close()
	
	log.Infof("relations-api %s check successful", checkType)
	return nil
}

func newResponse(status string, code int) *pb.GetReadyzResponse {
	return &pb.GetReadyzResponse{
		Status: status,
		Code:   uint32(code),
	}
}
