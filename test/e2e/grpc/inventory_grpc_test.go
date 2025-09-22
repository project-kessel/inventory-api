package grpc

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	v1 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type insecuregRPCMetadataCreds map[string]string

func (c insecuregRPCMetadataCreds) RequireTransportSecurity() bool { return false }
func (c insecuregRPCMetadataCreds) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return c, nil
}

// WithInsecureBearerToken returns a grpc.DialOption that adds a standard HTTP
// Bearer token to all requests sent from an insecure client.
// Must be used in conjunction with `insecure.NewCredentials()`.
func WithInsecureBearerToken(token string) grpc.DialOption {
	return grpc.WithPerRPCCredentials(insecuregRPCMetadataCreds{"authorization": "Bearer " + token})
}

var inventoryapi_grpc_url string

func TestMain(m *testing.M) {
	inventoryapi_grpc_url = os.Getenv("INV_GRPC_URL")
	if inventoryapi_grpc_url == "" {
		err := fmt.Errorf("INV_GRPC_URL environment variable not set")
		inventoryapi_grpc_url = "localhost:9081"
		log.Info(err)
	}
	err := waitForServiceToBeReady()
	if err != nil {
		err = fmt.Errorf("inventory health endpoint response failed: %s", err)
		log.Info(err)
	}
	result := m.Run()
	os.Exit(result)
}

func waitForServiceToBeReady() error {
	address := inventoryapi_grpc_url
	limit := 50
	wait := 250 * time.Millisecond
	started := time.Now()

	for i := 0; i < limit; i++ {
		conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			time.Sleep(wait)
			continue
		}
		client := grpc_health_v1.NewHealthClient(conn)
		resp, err := client.Check(context.TODO(), &grpc_health_v1.HealthCheckRequest{})
		if err != nil {
			time.Sleep(wait)
			continue
		}

		switch resp.Status {
		case grpc_health_v1.HealthCheckResponse_NOT_SERVING, grpc_health_v1.HealthCheckResponse_SERVICE_UNKNOWN:
			time.Sleep(wait)
			continue
		case grpc_health_v1.HealthCheckResponse_SERVING:
			return nil
		}
	}

	return fmt.Errorf("the health endpoint didn't respond successfully within %f seconds.", time.Since(started).Seconds())
}

func TestInventoryAPIGRPC_livez(t *testing.T) {
	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Print(err)
	}

	client := v1.NewKesselInventoryHealthServiceClient(conn)
	_, err = client.GetLivez(context.Background(), &v1.GetLivezRequest{})
	assert.NoError(t, err)
}

func TestInventoryAPIGRPC_Readyz(t *testing.T) {
	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Print(err)
	}

	client := v1.NewKesselInventoryHealthServiceClient(conn)
	_, err = client.GetReadyz(context.Background(), &v1.GetReadyzRequest{})
	assert.NoError(t, err)
}
