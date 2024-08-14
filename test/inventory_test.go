package test

import (
	"context"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	v1 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"os"
	"sync"
	"testing"
	"time"
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

var localInventoryContainer *LocalInventoryContainer

func TestMain(m *testing.M) {
	var err error
	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

	localInventoryContainer, err = CreateInventoryAPIContainer(logger)
	if err != nil {
		fmt.Printf("Error initializing Docker localInventoryContainer: %s", err)
		os.Exit(-1)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func(p string) {
		err := waitForServiceToBeReady(p)
		if err != nil {
			//	localInventoryContainer.Close()
			panic(fmt.Errorf("Error waiting for Kessel Inventory to start: %w", err))
		}
		wg.Done()
	}(localInventoryContainer.gRPCport)

	wg.Wait()

	result := m.Run()
	localInventoryContainer.Close()
	os.Exit(result)
}

func waitForServiceToBeReady(port string) error {
	address := fmt.Sprintf("localhost:%s", port)
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
	t.Parallel()
	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%s", localInventoryContainer.gRPCport),
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
	t.Parallel()
	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%s", localInventoryContainer.gRPCport),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Print(err)
	}

	client := v1.NewKesselInventoryHealthServiceClient(conn)
	_, err = client.GetReadyz(context.Background(), &v1.GetReadyzRequest{})
	assert.NoError(t, err)
}

func TestInventoryAPIGRPC_RhelHost_CreateRhelHost(t *testing.T) {
	t.Parallel()
	kcurl := fmt.Sprintf("http://localhost:%s", localInventoryContainer.kccontainer.GetPort("8084/tcp"))
	token, err := GetJWTToken(kcurl)
	if err != nil {
		log.Errorf("failed to generate token:%s", err)
	}

	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost:%s", localInventoryContainer.gRPCport),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		WithInsecureBearerToken(token.AccessToken),
	)
	if err != nil {
		fmt.Print(err)
	}

	client := v1beta1.NewKesselRhelHostServiceClient(conn)
	request := v1beta1.CreateRhelHostRequest{Host: &v1beta1.RhelHost{
		Metadata: &v1beta1.Metadata{
			ResourceType: "rhel-host",
			Workspace:    "",
		},
		ReporterData: &v1beta1.ReporterData{
			ReporterType:       v1beta1.ReporterData_REPORTER_TYPE_OCM,
			ReporterInstanceId: "service-account-svc-test",
			ConsoleHref:        "www.example.com",
			ApiHref:            "www.example.com",
			LocalResourceId:    "1",
			ReporterVersion:    "0.1",
		},
	}}
	resp, err := client.CreateRhelHost(context.Background(), &request)
	assert.Equal(t, request.Host.Metadata.ResourceType, resp.Host.Metadata.ResourceType)
	assert.NoError(t, err)
}

//func TestInventoryAPIGRPC_RhelHost_UpdateRhelHost(t *testing.T) {
//	t.Parallel()
//	conn, err := grpc.NewClient(
//		fmt.Sprintf("localhost:%s", localInventoryContainer.gRPCport),
//		grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
//	if err != nil {
//		fmt.Print(err)
//	}
//
//	client := v1beta1.NewKesselRhelHostServiceClient(conn)
//	request := v1beta1.UpdateRhelHostRequest{
//		Resource: "",
//		Host:     nil,
//	}
//	_, err = client.UpdateRhelHost(context.Background(), &request)
//	assert.NoError(t, err)
//}
