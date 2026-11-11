package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"strings"

	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
)

// bearerAuth implements grpc.PerRPCCredentials to inject Authorization
type bearerAuth struct {
	token string
}

func (b *bearerAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", b.token),
	}, nil
}

func (b *bearerAuth) RequireTransportSecurity() bool {
	return false // Set to true if using TLS
}

// V1Beta2
func TestInventoryAPIHTTP_v1beta2_ResourceLifecycle_Host(t *testing.T) {
	enableShortMode(t)

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	assert.NoError(t, err, "Failed to connect gRPC client")

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"satellite_id":            "550e8400-e29b-41d4-a716-446655440000",
		"subscription_manager_id": "550e8400-e29b-41d4-a716-446655440000",
		"insights_id":             "550e8400-e29b-41d4-a716-446655440000",
		"ansible_host":            "abc",
	})
	require.NoError(t, err, "Failed to create structpb for host reporter")

	req := &pbv1beta2.ReportResourceRequest{
		WriteVisibility:    pbv1beta2.WriteVisibility_MINIMIZE_LATENCY,
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "testuser-example-com",
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: "host-abc-123",
				ApiHref:         "https://example.com/api",
				ConsoleHref:     proto.String("https://example.com/console"),
				ReporterVersion: proto.String("0.1"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("workspace-v2"),
				},
			},
			Reporter: reporterStruct,
		},
	}

	_, err = client.ReportResource(ctx, req)
	require.NoError(t, err, "Failed to Report Resource")

	delReq := &pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-abc-123",
			Reporter: &pbv1beta2.ReporterReference{
				Type: "hbi",
			},
		},
	}

	_, err = client.DeleteResource(ctx, delReq)
	assert.NoError(t, err, "Failed to Delete Resource")
}

func TestInventoryAPIHTTP_v1beta2_ResourceLifecycle_Notifications(t *testing.T) {
	enableShortMode(t)

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	assert.NoError(t, err, "Failed to connect gRPC client")

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	// will likely change the notifications json schema, this is here to satisfy validation
	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"reporter_type":        "NOTIFICATIONS",
		"reporter_instance_id": "testuser-example-com",
		"local_resource_id":    "notification-abc-123",
	})
	require.NoError(t, err, "Failed to create structpb for reporter")

	req := pbv1beta2.ReportResourceRequest{

		Type:               "notifications_integration",
		ReporterType:       "NOTIFICATIONS",
		ReporterInstanceId: "testuser-example-com",
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: "notification-abc-123",
				ApiHref:         "https://example.com/api",
				ConsoleHref:     proto.String("https://example.com/console"),
				ReporterVersion: proto.String("0.1"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("workspace-v2"),
				},
			},
			Reporter: reporterStruct, // Notifications may not require a reporter block
		},
	}

	_, err = client.ReportResource(ctx, &req)
	require.NoError(t, err, "Failed to Report Resource")

	delReq := pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "notifications_integration",
			ResourceId:   "notification-abc-123",
			Reporter: &pbv1beta2.ReporterReference{
				Type:       "NOTIFICATIONS",
				InstanceId: proto.String("testuser-example-com"),
			},
		},
	}

	_, err = client.DeleteResource(ctx, &delReq)
	assert.NoError(t, err, "Failed to Delete Resource")

}

func TestInventoryAPIHTTP_v1beta2_ResourceLifecycle_K8S_Cluster(t *testing.T) {
	enableShortMode(t)

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	assert.NoError(t, err, "Failed to connect gRPC client")

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"external_cluster_id": "abcd-efgh-1234",
		"cluster_status":      "READY",
		"cluster_reason":      "All systems operational",
		"kube_version":        "1.31",
		"kube_vendor":         "OPENSHIFT",
		"vendor_version":      "4.16",
		"cloud_platform":      "AWS_UPI",
	})
	require.NoError(t, err, "Failed to create structpb for cluster reporter")

	req := pbv1beta2.ReportResourceRequest{

		Type:               "k8s_cluster",
		ReporterType:       "ACM",
		ReporterInstanceId: "testuser-example-com",
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: "k8s_cluster-abc-123-unique",
				ApiHref:         "https://example.com/api",
				ConsoleHref:     proto.String("https://example.com/console"),
				ReporterVersion: proto.String("0.1"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("workspace-v2"),
				},
			},
			Reporter: reporterStruct,
		},
	}

	_, err = client.ReportResource(ctx, &req)
	require.NoError(t, err, "Failed to Report Resource")

	delReq := pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "k8s_cluster",
			ResourceId:   "k8s_cluster-abc-123-unique",
			Reporter: &pbv1beta2.ReporterReference{
				Type:       "ACM",
				InstanceId: proto.String("testuser-example-com"),
			},
		},
	}

	_, err = client.DeleteResource(ctx, &delReq)
	assert.NoError(t, err, "Failed to Delete Resource")

}

func TestInventoryAPIHTTP_v1beta2_ResourceLifecycle_K8S_Policy(t *testing.T) {
	enableShortMode(t)

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	assert.NoError(t, err, "Failed to connect gRPC client")

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"disabled": true,
		"severity": "MEDIUM",
	})
	require.NoError(t, err, "Failed to create structpb for reporter")

	req := pbv1beta2.ReportResourceRequest{

		Type:               "k8s_policy",
		ReporterType:       "ACM",
		ReporterInstanceId: "testuser-example-com",
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: "k8s_policy-abc-123",
				ApiHref:         "https://example.com/api",
				ConsoleHref:     proto.String("https://example.com/console"),
				ReporterVersion: proto.String("0.1"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("workspace-123"),
				},
			},
			Reporter: reporterStruct,
		},
	}

	_, err = client.ReportResource(ctx, &req)
	require.NoError(t, err, "Failed to Report Resource")

	delReq := pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "k8s_policy",
			ResourceId:   "k8s_policy-abc-123",
			Reporter: &pbv1beta2.ReporterReference{
				Type: "ACM",
			},
		},
	}

	_, err = client.DeleteResource(ctx, &delReq)
	assert.NoError(t, err, "Failed to Delete Resource")

}

//func TestInventoryAPIHTTP_v1beta2_AuthzLifecycle(t *testing.T) {
////
//	c := common.NewConfig(
//		common.WithHTTPUrl(inventoryapi_http_url),
//		common.WithTLSInsecure(insecure),
//		common.WithHTTPTLSConfig(tlsConfig),
//	)
//
//	client, err := v1beta2.NewHttpClient(context.Background(), c)
//	assert.NoError(t, err, "Failed to create v1beta2 HTTP client")
//
//	ctx := context.Background()
//
//	subject := &pbv1beta2.SubjectReference{
//		Resource: &pbv1beta2.ResourceReference{
//			ResourceId:   "bob",
//			ResourceType: "principal",
//			Reporter: &pbv1beta2.ReporterReference{
//				Type: "rbac",
//			},
//		},
//	}
//
//	parent := &pbv1beta2.ResourceReference{
//		ResourceId: "bob_club",
//		Reporter: &pbv1beta2.ReporterReference{
//			Type: "rbac",
//		},
//		ResourceType: "group",
//	}
//
//	// /authz/check
//	checkReq := &pbv1beta2.CheckRequest{
//		Subject:  subject,
//		Relation: "member",
//		Object:   parent,
//	}
//
//	checkResp, err := client.KesselCheckService.Check(ctx, checkReq)
//	assert.NoError(t, err, "check endpoint failed")
//	assert.NotNil(t, checkResp, "check response should not be nil")
//	assert.Equal(t, pbv1beta2.Allowed_ALLOWED_FALSE, checkResp.GetAllowed())
//
//	// /authz/checkforupdate
//	checkUpdateReq := &pbv1beta2.CheckForUpdateRequest{
//		Subject:  subject,
//		Relation: "member",
//		Object:   parent,
//	}
//
//	checkUpdateResp, err := client.KesselCheckService.CheckForUpdate(ctx, checkUpdateReq)
//	assert.NoError(t, err, "checkforupdate endpoint failed")
//	assert.NotNil(t, checkUpdateResp, "checkforupdate response should not be nil")
//	assert.Equal(t, pbv1beta2.Allowed_ALLOWED_FALSE, checkUpdateResp.GetAllowed())
//}

func TestInventoryAPIHTTP_v1beta2_Host_ConsistentWrite(t *testing.T) {
	enableShortMode(t)

	resourceId := "wait-for-sync-host-abc-123"

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"disabled": true,
		"severity": "MEDIUM",
	})
	require.NoError(t, err, "Failed to create structpb for reporter")

	req := pbv1beta2.ReportResourceRequest{
		WriteVisibility:    pbv1beta2.WriteVisibility_IMMEDIATE,
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "testuser-example-com",
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: resourceId,
				ApiHref:         "https://example.com/api",
				ConsoleHref:     proto.String("https://example.com/console"),
				ReporterVersion: proto.String("0.1"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("workspace-v2"),
				},
			},
			Reporter: reporterStruct,
		},
	}

	_, err = client.ReportResource(ctx, &req)
	require.NoError(t, err, "Failed to Report Resource")

	// The consistency token is set asynchronously by the consumer after it
	// processes the outbox event through Kafka and writes back to the DB.
	// Poll until the token appears.
	var host datamodel.Resource
	tokenFound := false
	for i := 0; i < 30; i++ {
		err = db.Table("resource r").
			Select("r.*").
			Joins("JOIN reporter_resources rr ON r.id = rr.resource_id").
			Where("rr.local_resource_id = ?", resourceId).
			First(&host).Error
		if err != nil {
			t.Logf("DB query attempt %d: %v", i+1, err)
		} else if host.ConsistencyToken != "" {
			t.Logf("Consistency token populated on attempt %d", i+1)
			tokenFound = true
			break
		} else {
			t.Logf("Consistency token still empty (attempt %d)", i+1)
		}
		time.Sleep(1 * time.Second)
	}
	require.NoError(t, err, "Error fetching host from DB")
	require.True(t, tokenFound, "Consistency token was not populated within 30s")

	delReq := pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type: "hbi",
			},
		},
	}
	_, err = client.DeleteResource(ctx, &delReq)
	assert.NoError(t, err, "Failed to Delete Resource")
}

func TestInventoryAPIHTTP_v1beta2_workspace_movement_tests(t *testing.T) {
	enableShortMode(t)
	ctx := context.Background()

	// Test configuration
	oldWorkspace := "c100"
	newWorkspace := "c101"
	resourceId := "218e5a67-a098-4958-8063-cb5421a2d6cd"
	reporterType := "hbi"
	reporterInstanceId := "test-instance"

	t.Logf("=== Testing Workspace Change Functionality ===")
	t.Logf("Old Workspace: %s", oldWorkspace)
	t.Logf("New Workspace: %s", newWorkspace)

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	assert.NoError(t, err, "Failed to connect gRPC client")

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	// Step 1: Add resource with old workspace
	t.Logf("1. Adding resource with workspace_id: %s", oldWorkspace)
	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"ansible_host": "test-host.example.com",
	})
	require.NoError(t, err, "Failed to create structpb for host reporter")

	req := &pbv1beta2.ReportResourceRequest{
		WriteVisibility:    pbv1beta2.WriteVisibility_MINIMIZE_LATENCY,
		Type:               "host",
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstanceId,
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: resourceId,
				ApiHref:         "https://api.example.com/hosts/test-host-123",
				ConsoleHref:     proto.String("https://console.example.com/hosts/test-host-123"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue(oldWorkspace),
				},
			},
			Reporter: reporterStruct,
		},
	}

	_, err = client.ReportResource(ctx, req)
	assert.NoError(t, err, "Failed to Report Resource with old workspace")

	// Step 2: Update resource to new workspace
	t.Logf("2. Updating resource to workspace_id: %s", newWorkspace)
	reqUpdated := &pbv1beta2.ReportResourceRequest{
		WriteVisibility:    pbv1beta2.WriteVisibility_MINIMIZE_LATENCY,
		Type:               "host",
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstanceId,
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: resourceId,
				ApiHref:         "https://api.example.com/hosts/test-host-123",
				ConsoleHref:     proto.String("https://console.example.com/hosts/test-host-123"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue(newWorkspace),
				},
			},
			Reporter: reporterStruct,
		},
	}

	_, err = client.ReportResource(ctx, reqUpdated)
	assert.NoError(t, err, "Failed to Update Resource with new workspace")

	// Step 3: Check authorization for OLD workspace (should be ALLOWED_FALSE)
	t.Logf("3. Checking authorization for OLD workspace %s (should be allowed: false)", oldWorkspace)
	t.Log("Waiting for outbox events to be processed (polling up to 30s)...")

	checkOldWorkspace := &pbv1beta2.CheckRequest{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   oldWorkspace,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	}

	// Poll for up to 30 seconds
	oldWorkspaceAllowed := false
	connErrors := 0
	for i := 0; i < 30; i++ {
		checkResp, err := client.Check(ctx, checkOldWorkspace)
		if err == nil && checkResp.GetAllowed() == pbv1beta2.Allowed_ALLOWED_FALSE {
			t.Logf("✓ Old workspace check returned ALLOWED_FALSE (attempt %d)", i+1)
			oldWorkspaceAllowed = true
			break
		}
		if err != nil {
			t.Logf("Check request failed (attempt %d): %v", i+1, err)
			if isConnectionError(err) {
				connErrors++
				if connErrors >= maxConsecutiveConnErrors {
					t.Fatalf("Service unreachable after %d consecutive connection errors, aborting", connErrors)
				}
			}
		} else {
			connErrors = 0
			t.Logf("Old workspace check returned %v (attempt %d), expected ALLOWED_FALSE", checkResp.GetAllowed(), i+1)
		}
		if i < 29 {
			t.Log("Waiting 1s before retry...")
			time.Sleep(1 * time.Second)
		}
	}
	assert.True(t, oldWorkspaceAllowed, "Old workspace authorization check did not return ALLOWED_FALSE within timeout")

	// Step 4: Check authorization for NEW workspace (should be ALLOWED_TRUE)
	t.Logf("4. Checking authorization for NEW workspace %s (should be allowed: true)", newWorkspace)
	t.Log("Waiting for outbox events to be processed (polling up to 30s)...")

	checkNewWorkspace := &pbv1beta2.CheckRequest{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   newWorkspace,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	}

	// Poll for up to 30 seconds
	newWorkspaceAllowed := false
	connErrors = 0
	for i := 0; i < 30; i++ {
		checkResp, err := client.Check(ctx, checkNewWorkspace)
		if err == nil && checkResp.GetAllowed() == pbv1beta2.Allowed_ALLOWED_TRUE {
			t.Logf("✓ New workspace check returned ALLOWED_TRUE (attempt %d)", i+1)
			newWorkspaceAllowed = true
			break
		}
		if err != nil {
			t.Logf("Check request failed (attempt %d): %v", i+1, err)
			if isConnectionError(err) {
				connErrors++
				if connErrors >= maxConsecutiveConnErrors {
					t.Fatalf("Service unreachable after %d consecutive connection errors, aborting", connErrors)
				}
			}
		} else {
			connErrors = 0
			t.Logf("New workspace check returned %v (attempt %d), expected ALLOWED_TRUE", checkResp.GetAllowed(), i+1)
		}
		if i < 29 {
			t.Log("Waiting 1s before retry...")
			time.Sleep(1 * time.Second)
		}
	}
	assert.True(t, newWorkspaceAllowed, "New workspace authorization check did not return ALLOWED_TRUE within timeout")

	t.Log("=== Test Complete ===")

	// Cleanup: Delete the resource
	delReq := &pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
	}
	_, err = client.DeleteResource(ctx, delReq)
	assert.NoError(t, err, "Failed to Delete Resource during cleanup")
}

func TestInventoryAPIHTTP_v1beta2_CheckBulk_WithErrorPair(t *testing.T) {
	enableShortMode(t)

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	assert.NoError(t, err, "Failed to connect gRPC client")

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	// Seed a resource that will yield one TRUE and one FALSE
	resourceId := "checkbulk-host-errorpair-0001"
	reporterType := "hbi"
	reporterInstanceId := "testuser-example-com"
	trueWorkspace := "workspace-true-errorpair"
	falseWorkspace := "workspace-false-errorpair"

	resourceCreated := false
	defer func() {
		if !resourceCreated {
			return
		}
		delReq := &pbv1beta2.DeleteResourceRequest{
			Reference: &pbv1beta2.ResourceReference{
				ResourceType: "host",
				ResourceId:   resourceId,
				Reporter: &pbv1beta2.ReporterReference{
					Type:       reporterType,
					InstanceId: proto.String(reporterInstanceId),
				},
			},
		}
		_, cleanupErr := client.DeleteResource(ctx, delReq)
		assert.NoError(t, cleanupErr, "Failed to Delete Resource during cleanup")
	}()

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"ansible_host": "checkbulk-errorpair-host.example.com",
	})
	require.NoError(t, err, "Failed to create structpb for reporter")

	req := &pbv1beta2.ReportResourceRequest{
		WriteVisibility:    pbv1beta2.WriteVisibility_MINIMIZE_LATENCY,
		Type:               "host",
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstanceId,
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: resourceId,
				ApiHref:         "https://api.example.com/hosts/checkbulk-host-errorpair-0001",
				ConsoleHref:     proto.String("https://console.example.com/hosts/checkbulk-host-errorpair-0001"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue(trueWorkspace),
				},
			},
			Reporter: reporterStruct,
		},
	}
	_, err = client.ReportResource(ctx, req)
	require.NoError(t, err, "Failed to Report Resource")
	resourceCreated = true

	// Build CheckBulk with:
	// - one expected TRUE (trueWorkspace)
	// - one expected FALSE (falseWorkspace)
	// - one invalid subject type to produce an error pair
	itemTrue := &pbv1beta2.CheckBulkRequestItem{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   trueWorkspace,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	}
	itemFalse := &pbv1beta2.CheckBulkRequestItem{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   falseWorkspace,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	}
	itemError := &pbv1beta2.CheckBulkRequestItem{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			// invalid subject resource type to force a per-item error
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "not_a_valid_type",
				ResourceId:   "charlie",
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	}

	checkReq := &pbv1beta2.CheckBulkRequest{
		Items: []*pbv1beta2.CheckBulkRequestItem{itemTrue, itemFalse, itemError},
	}

	// Poll up to 30 seconds to allow eventual consistency and error mapping
	observed := false
	connErrors := 0
	for i := 0; i < 30; i++ {
		resp, err := client.CheckBulk(ctx, checkReq)
		if err == nil && resp != nil && len(resp.GetPairs()) == 3 {
			connErrors = 0
			results := map[string]pbv1beta2.Allowed{}
			errorSubjects := []string{}
			for _, p := range resp.GetPairs() {
				reqItem := p.GetRequest()
				if p.GetItem() != nil {
					// Key by subject ResourceId
					results[reqItem.GetSubject().GetResource().GetResourceId()] = p.GetItem().GetAllowed()
				} else if p.GetError() != nil {
					errorSubjects = append(errorSubjects, reqItem.GetSubject().GetResource().GetResourceId())
				}
			}
			gotTrue := results[trueWorkspace] == pbv1beta2.Allowed_ALLOWED_TRUE
			gotFalse := results[falseWorkspace] == pbv1beta2.Allowed_ALLOWED_FALSE
			gotError := false
			for _, s := range errorSubjects {
				if s == "charlie" {
					gotError = true
					break
				}
			}
			if gotTrue && gotFalse && gotError {
				observed = true
				break
			}
			t.Logf("CheckBulk attempt %d: true=%v false=%v errorPresent=%v", i+1, gotTrue, gotFalse, gotError)
		} else if err != nil {
			t.Logf("CheckBulk request failed (attempt %d): %v", i+1, err)
			if isConnectionError(err) {
				connErrors++
				if connErrors >= maxConsecutiveConnErrors {
					t.Fatalf("Service unreachable after %d consecutive connection errors, aborting", connErrors)
				}
			}
		}
		if i < 29 {
			time.Sleep(1 * time.Second)
		}
	}
	assert.True(t, observed, "CheckBulk with error pair expectations not met within timeout")

}

func TestInventoryAPIHTTP_v1beta2_create_check_delete_check_resource(t *testing.T) {
	enableShortMode(t)
	ctx := context.Background()

	// Test configuration
	resourceId := "00000000-0000-0000-0000-000000000001"
	reporterType := "hbi"
	reporterInstanceId := "testuser-example-com"
	workspace := "workspace-123"

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	assert.NoError(t, err, "Failed to connect gRPC client")

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	// ------- Create -------
	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"ansible_host": "test-host.example.com",
	})
	require.NoError(t, err, "Failed to create structpb for reporter")

	req := &pbv1beta2.ReportResourceRequest{
		WriteVisibility:    pbv1beta2.WriteVisibility_MINIMIZE_LATENCY,
		Type:               "host",
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstanceId,
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: resourceId,
				ApiHref:         "https://api.example.com/hosts/test-host-123",
				ConsoleHref:     proto.String("https://console.example.com/hosts/test-host-123"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue(workspace),
				},
			},
			Reporter: reporterStruct,
		},
	}
	_, err = client.ReportResource(ctx, req)
	require.NoError(t, err, "Failed to Report Resource")

	// ------- Check (expect TRUE) -------
	checkAfterCreate := &pbv1beta2.CheckRequest{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   workspace,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	}

	allowedTrueObserved := false
	connErrors := 0
	for i := 0; i < 30; i++ {
		checkResp, err := client.Check(ctx, checkAfterCreate)
		if err == nil && checkResp.GetAllowed() == pbv1beta2.Allowed_ALLOWED_TRUE {
			t.Logf("✓ Create-check returned ALLOWED_TRUE (attempt %d)", i+1)
			allowedTrueObserved = true
			break
		}
		if err != nil {
			t.Logf("Check request failed (attempt %d): %v", i+1, err)
			if isConnectionError(err) {
				connErrors++
				if connErrors >= maxConsecutiveConnErrors {
					t.Fatalf("Service unreachable after %d consecutive connection errors, aborting", connErrors)
				}
			}
		} else {
			connErrors = 0
			t.Logf("Create-check returned %v (attempt %d), expected ALLOWED_TRUE", checkResp.GetAllowed(), i+1)
		}
		if i < 29 {
			t.Log("Waiting 1s before retry...")
			time.Sleep(1 * time.Second)
		}
	}
	assert.True(t, allowedTrueObserved, "Authorization check after create did not return ALLOWED_TRUE within timeout")

	// ------- Delete -------
	delReq := &pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
	}
	_, err = client.DeleteResource(ctx, delReq)
	assert.NoError(t, err, "Failed to Delete Resource")

	// ------- Check (expect FALSE) -------
	checkAfterDelete := &pbv1beta2.CheckRequest{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   workspace,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	}

	allowedFalseObserved := false
	connErrors = 0
	for i := 0; i < 30; i++ {
		checkResp, err := client.Check(ctx, checkAfterDelete)
		if err == nil && checkResp.GetAllowed() == pbv1beta2.Allowed_ALLOWED_FALSE {
			t.Logf("✓ Delete-check returned ALLOWED_FALSE (attempt %d)", i+1)
			allowedFalseObserved = true
			break
		}
		if err != nil {
			t.Logf("Check request failed (attempt %d): %v", i+1, err)
			if isConnectionError(err) {
				connErrors++
				if connErrors >= maxConsecutiveConnErrors {
					t.Fatalf("Service unreachable after %d consecutive connection errors, aborting", connErrors)
				}
			}
		} else {
			connErrors = 0
			t.Logf("Delete-check returned %v (attempt %d), expected ALLOWED_FALSE", checkResp.GetAllowed(), i+1)
		}
		if i < 29 {
			t.Log("Waiting 1s before retry...")
			time.Sleep(1 * time.Second)
		}
	}
	assert.True(t, allowedFalseObserved, "Authorization check after delete did not return ALLOWED_FALSE within timeout")
}

func enableShortMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}
}

const maxConsecutiveConnErrors = 5

// isConnectionError returns true if the error indicates the service is unreachable.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "transport is closing")
}

func TestInventoryAPIHTTP_v1beta2_CheckBulk_SingleTrueAndFalse(t *testing.T) {
	enableShortMode(t)

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	assert.NoError(t, err, "Failed to connect gRPC client")

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	resourceId := "checkbulk-host-0001"
	reporterType := "hbi"
	reporterInstanceId := "testuser-example-com"
	trueWorkspace := "workspace-true"
	falseWorkspace := "workspace-false"

	resourceCreated := false
	defer func() {
		if !resourceCreated {
			return
		}
		delReq := &pbv1beta2.DeleteResourceRequest{
			Reference: &pbv1beta2.ResourceReference{
				ResourceType: "host",
				ResourceId:   resourceId,
				Reporter: &pbv1beta2.ReporterReference{
					Type:       reporterType,
					InstanceId: proto.String(reporterInstanceId),
				},
			},
		}
		_, cleanupErr := client.DeleteResource(ctx, delReq)
		assert.NoError(t, cleanupErr, "Failed to Delete Resource during cleanup")
	}()

	// Create a resource associated with trueWorkspace
	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"ansible_host": "checkbulk-host.example.com",
	})
	require.NoError(t, err, "Failed to create structpb for reporter")

	req := &pbv1beta2.ReportResourceRequest{
		WriteVisibility:    pbv1beta2.WriteVisibility_MINIMIZE_LATENCY,
		Type:               "host",
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstanceId,
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: resourceId,
				ApiHref:         "https://api.example.com/hosts/checkbulk-host-0001",
				ConsoleHref:     proto.String("https://console.example.com/hosts/checkbulk-host-0001"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue(trueWorkspace),
				},
			},
			Reporter: reporterStruct,
		},
	}
	_, err = client.ReportResource(ctx, req)
	require.NoError(t, err, "Failed to Report Resource")
	resourceCreated = true

	// Build CheckBulk with one expected TRUE and one expected FALSE
	checkBulkReq := &pbv1beta2.CheckBulkRequest{
		Items: []*pbv1beta2.CheckBulkRequestItem{
			{
				Object: &pbv1beta2.ResourceReference{
					ResourceType: "host",
					ResourceId:   resourceId,
					Reporter: &pbv1beta2.ReporterReference{
						Type:       reporterType,
						InstanceId: proto.String(reporterInstanceId),
					},
				},
				Relation: "workspace",
				Subject: &pbv1beta2.SubjectReference{
					Resource: &pbv1beta2.ResourceReference{
						ResourceType: "workspace",
						ResourceId:   trueWorkspace,
						Reporter: &pbv1beta2.ReporterReference{
							Type: "rbac",
						},
					},
				},
			},
			{
				Object: &pbv1beta2.ResourceReference{
					ResourceType: "host",
					ResourceId:   resourceId,
					Reporter: &pbv1beta2.ReporterReference{
						Type:       reporterType,
						InstanceId: proto.String(reporterInstanceId),
					},
				},
				Relation: "workspace",
				Subject: &pbv1beta2.SubjectReference{
					Resource: &pbv1beta2.ResourceReference{
						ResourceType: "workspace",
						ResourceId:   falseWorkspace,
						Reporter: &pbv1beta2.ReporterReference{
							Type: "rbac",
						},
					},
				},
			},
		},
	}

	// Poll up to 30 seconds to account for eventual consistency
	observed := false
	connErrors := 0
	for i := 0; i < 30; i++ {
		resp, err := client.CheckBulk(ctx, checkBulkReq)
		if err == nil && resp != nil && len(resp.GetPairs()) == 2 {
			connErrors = 0
			pairs := resp.GetPairs()
			gotTrue := pairs[0].GetItem() != nil && pairs[0].GetItem().GetAllowed() == pbv1beta2.Allowed_ALLOWED_TRUE
			gotFalse := pairs[1].GetItem() != nil && pairs[1].GetItem().GetAllowed() == pbv1beta2.Allowed_ALLOWED_FALSE
			if gotTrue && gotFalse {
				observed = true
				break
			}
			t.Logf("CheckBulk attempt %d: got allowed = [%v, %v], expected [TRUE, FALSE]", i+1, pairs[0].GetItem().GetAllowed(), pairs[1].GetItem().GetAllowed())
		} else if err != nil {
			t.Logf("CheckBulk request failed (attempt %d): %v", i+1, err)
			if isConnectionError(err) {
				connErrors++
				if connErrors >= maxConsecutiveConnErrors {
					t.Fatalf("Service unreachable after %d consecutive connection errors, aborting", connErrors)
				}
			}
		}
		if i < 29 {
			time.Sleep(1 * time.Second)
		}
	}
	assert.True(t, observed, "CheckBulk didn't return [TRUE, FALSE] within timeout")

}

func TestInventoryAPIHTTP_v1beta2_CheckForUpdate_AllowedTrue(t *testing.T) {
	enableShortMode(t)
	ctx := context.Background()

	resourceId := "checkforupdate-host-0001"
	reporterType := "hbi"
	reporterInstanceId := "testuser-example-com"
	workspace := "workspace-checkforupdate-true"

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	resourceCreated := false
	defer func() {
		if !resourceCreated {
			return
		}
		_, cleanupErr := client.DeleteResource(ctx, &pbv1beta2.DeleteResourceRequest{
			Reference: &pbv1beta2.ResourceReference{
				ResourceType: "host",
				ResourceId:   resourceId,
				Reporter: &pbv1beta2.ReporterReference{
					Type:       reporterType,
					InstanceId: proto.String(reporterInstanceId),
				},
			},
		})
		assert.NoError(t, cleanupErr, "Failed to Delete Resource during cleanup")
	}()

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"ansible_host": "checkforupdate-host.example.com",
	})
	require.NoError(t, err, "Failed to create structpb for reporter")

	_, err = client.ReportResource(ctx, &pbv1beta2.ReportResourceRequest{
		WriteVisibility:    pbv1beta2.WriteVisibility_IMMEDIATE,
		Type:               "host",
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstanceId,
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: resourceId,
				ApiHref:         "https://api.example.com/hosts/checkforupdate-host-0001",
				ConsoleHref:     proto.String("https://console.example.com/hosts/checkforupdate-host-0001"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue(workspace),
				},
			},
			Reporter: reporterStruct,
		},
	})
	require.NoError(t, err, "Failed to Report Resource")
	resourceCreated = true

	resp, err := client.CheckForUpdate(ctx, &pbv1beta2.CheckForUpdateRequest{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   workspace,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	})
	assert.NoError(t, err, "CheckForUpdate request failed")
	assert.NotNil(t, resp, "CheckForUpdate response should not be nil")
	assert.Equal(t, pbv1beta2.Allowed_ALLOWED_TRUE, resp.GetAllowed(), "Expected ALLOWED_TRUE after resource creation")
	assert.NotNil(t, resp.GetConsistencyToken(), "Expected consistency token in CheckForUpdate response")
	assert.NotEmpty(t, resp.GetConsistencyToken().GetToken(), "Expected non-empty consistency token")
}

func TestInventoryAPIHTTP_v1beta2_CheckForUpdate_AllowedFalse(t *testing.T) {
	enableShortMode(t)
	ctx := context.Background()

	resourceId := "checkforupdate-host-0002"
	reporterType := "hbi"
	reporterInstanceId := "testuser-example-com"
	workspace := "workspace-checkforupdate-false-a"
	otherWorkspace := "workspace-checkforupdate-false-b"

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	resourceCreated := false
	defer func() {
		if !resourceCreated {
			return
		}
		_, cleanupErr := client.DeleteResource(ctx, &pbv1beta2.DeleteResourceRequest{
			Reference: &pbv1beta2.ResourceReference{
				ResourceType: "host",
				ResourceId:   resourceId,
				Reporter: &pbv1beta2.ReporterReference{
					Type:       reporterType,
					InstanceId: proto.String(reporterInstanceId),
				},
			},
		})
		assert.NoError(t, cleanupErr, "Failed to Delete Resource during cleanup")
	}()

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"ansible_host": "checkforupdate-host2.example.com",
	})
	require.NoError(t, err, "Failed to create structpb for reporter")

	_, err = client.ReportResource(ctx, &pbv1beta2.ReportResourceRequest{
		WriteVisibility:    pbv1beta2.WriteVisibility_IMMEDIATE,
		Type:               "host",
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstanceId,
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: resourceId,
				ApiHref:         "https://api.example.com/hosts/checkforupdate-host-0002",
				ConsoleHref:     proto.String("https://console.example.com/hosts/checkforupdate-host-0002"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue(workspace),
				},
			},
			Reporter: reporterStruct,
		},
	})
	require.NoError(t, err, "Failed to Report Resource")
	resourceCreated = true

	resp, err := client.CheckForUpdate(ctx, &pbv1beta2.CheckForUpdateRequest{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   otherWorkspace,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	})
	assert.NoError(t, err, "CheckForUpdate request failed")
	assert.NotNil(t, resp, "CheckForUpdate response should not be nil")
	assert.Equal(t, pbv1beta2.Allowed_ALLOWED_FALSE, resp.GetAllowed(), "Expected ALLOWED_FALSE for non-matching workspace")
}

func TestInventoryAPIHTTP_v1beta2_TransactionIdIdempotency(t *testing.T) {
	enableShortMode(t)

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	resourceId := "idempotency-host-" + suffix
	reporterType := "hbi"
	reporterInstanceId := "testuser-example-com"
	txId := "e2e-txn-idempotency-test-" + suffix

	resourceCreated := false
	defer func() {
		if !resourceCreated {
			return
		}
		_, cleanupErr := client.DeleteResource(ctx, &pbv1beta2.DeleteResourceRequest{
			Reference: &pbv1beta2.ResourceReference{
				ResourceType: "host",
				ResourceId:   resourceId,
				Reporter: &pbv1beta2.ReporterReference{
					Type:       reporterType,
					InstanceId: proto.String(reporterInstanceId),
				},
			},
		})
		assert.NoError(t, cleanupErr, "Failed to Delete Resource during cleanup")
	}()

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"ansible_host": "idempotency-host.example.com",
	})
	require.NoError(t, err, "Failed to create structpb for reporter")

	makeReq := func(apiHref string) *pbv1beta2.ReportResourceRequest {
		return &pbv1beta2.ReportResourceRequest{
			WriteVisibility:    pbv1beta2.WriteVisibility_MINIMIZE_LATENCY,
			Type:               "host",
			ReporterType:       reporterType,
			ReporterInstanceId: reporterInstanceId,
			Representations: &pbv1beta2.ResourceRepresentations{
				Metadata: &pbv1beta2.RepresentationMetadata{
					LocalResourceId: resourceId,
					ApiHref:         apiHref,
					IdempotencyKey: &pbv1beta2.RepresentationMetadata_TransactionId{
						TransactionId: txId,
					},
				},
				Common: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"workspace_id": structpb.NewStringValue("ws-idempotency"),
					},
				},
				Reporter: reporterStruct,
			},
		}
	}

	// First report — should succeed and store the transaction ID
	_, err = client.ReportResource(ctx, makeReq("https://api.example.com/v1"))
	require.NoError(t, err, "First report should succeed")
	resourceCreated = true

	// Verify transaction ID is stored in the DB
	var txCount int64
	err = db.Table("reporter_representations").Where("transaction_id = ?", txId).Count(&txCount).Error
	require.NoError(t, err, "Failed to query reporter_representations")
	assert.Greater(t, txCount, int64(0), "transaction_id should be stored in reporter_representations")

	// Verify the api_href from the first report
	var apiHref string
	err = db.Table("reporter_resources").
		Where("local_resource_id = ?", resourceId).
		Select("api_href").
		Scan(&apiHref).Error
	require.NoError(t, err, "Failed to query api_href")
	assert.Equal(t, "https://api.example.com/v1", apiHref)

	// Second report with same transaction_id but different api_href — should be skipped
	_, err = client.ReportResource(ctx, makeReq("https://api.example.com/v2-should-be-ignored"))
	require.NoError(t, err, "Second report should succeed (silently skipped)")

	// Verify api_href was NOT updated
	err = db.Table("reporter_resources").
		Where("local_resource_id = ?", resourceId).
		Select("api_href").
		Scan(&apiHref).Error
	require.NoError(t, err, "Failed to query api_href after second report")
	assert.Equal(t, "https://api.example.com/v1", apiHref,
		"second report with same transaction_id should be a no-op; api_href should not change")
}

func TestInventoryAPIHTTP_v1beta2_CheckBulk_OrderAndEcho(t *testing.T) {
	enableShortMode(t)

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	assert.NoError(t, err, "Failed to connect gRPC client")

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	resourceId := "checkbulk-host-0002"
	reporterType := "hbi"
	reporterInstanceId := "testuser-example-com"
	workspace := "workspace-echo"

	resourceCreated := false
	defer func() {
		if !resourceCreated {
			return
		}
		delReq := &pbv1beta2.DeleteResourceRequest{
			Reference: &pbv1beta2.ResourceReference{
				ResourceType: "host",
				ResourceId:   resourceId,
				Reporter: &pbv1beta2.ReporterReference{
					Type:       reporterType,
					InstanceId: proto.String(reporterInstanceId),
				},
			},
		}
		_, cleanupErr := client.DeleteResource(ctx, delReq)
		assert.NoError(t, cleanupErr, "Failed to Delete Resource during cleanup")
	}()

	// Create resource
	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"ansible_host": "checkbulk2-host.example.com",
	})
	require.NoError(t, err, "Failed to create structpb for reporter")

	req := &pbv1beta2.ReportResourceRequest{
		WriteVisibility:    pbv1beta2.WriteVisibility_MINIMIZE_LATENCY,
		Type:               "host",
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstanceId,
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: resourceId,
				ApiHref:         "https://api.example.com/hosts/checkbulk-host-0002",
				ConsoleHref:     proto.String("https://console.example.com/hosts/checkbulk-host-0002"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue(workspace),
				},
			},
			Reporter: reporterStruct,
		},
	}
	_, err = client.ReportResource(ctx, req)
	require.NoError(t, err, "Failed to Report Resource")
	resourceCreated = true

	item1 := &pbv1beta2.CheckBulkRequestItem{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   workspace,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	}
	item2 := &pbv1beta2.CheckBulkRequestItem{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(reporterInstanceId),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   "not-" + workspace,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	}

	// Poll up to 30 seconds; verify order is preserved and request is echoed
	observed := false
	connErrors := 0
	for i := 0; i < 30; i++ {
		resp, err := client.CheckBulk(ctx, &pbv1beta2.CheckBulkRequest{
			Items: []*pbv1beta2.CheckBulkRequestItem{item1, item2},
		})
		if err == nil && resp != nil && len(resp.GetPairs()) == 2 {
			connErrors = 0
			pairs := resp.GetPairs()
			// Order preserved
			firstReq := pairs[0].GetRequest()
			secondReq := pairs[1].GetRequest()
			orderOK := true

			if firstReq != nil {
				if firstReq.Object.ResourceId != item1.Object.ResourceId ||
					firstReq.Subject.Resource.ResourceId != item1.Subject.Resource.ResourceId ||
					firstReq.Relation != item1.Relation {
					orderOK = false
				}
			}
			if secondReq != nil {
				if secondReq.Object.ResourceId != item2.Object.ResourceId ||
					secondReq.Subject.Resource.ResourceId != item2.Subject.Resource.ResourceId ||
					secondReq.Relation != item2.Relation {
					orderOK = false
				}
			}
			t.Logf("CheckBulk attempt %d: orderOK=%v firstReq=%v item1=%v secondReq=%v item2=%v", i+1, orderOK, firstReq, item1, secondReq, item2)
			// Allowed values as expected in the response
			allowedOK := pairs[0].GetItem() != nil && pairs[0].GetItem().GetAllowed() == pbv1beta2.Allowed_ALLOWED_TRUE &&
				pairs[1].GetItem() != nil && pairs[1].GetItem().GetAllowed() == pbv1beta2.Allowed_ALLOWED_FALSE
			if orderOK && allowedOK {
				observed = true
				break
			}
			t.Logf("CheckBulk attempt %d: orderOK=%v allowed0=%v allowed1=%v", i+1, orderOK, pairs[0].GetItem().GetAllowed(), pairs[1].GetItem().GetAllowed())
		} else if err != nil {
			t.Logf("CheckBulk request failed (attempt %d): %v", i+1, err)
			if isConnectionError(err) {
				connErrors++
				if connErrors >= maxConsecutiveConnErrors {
					t.Fatalf("Service unreachable after %d consecutive connection errors, aborting", connErrors)
				}
			}
		}
		if i < 29 {
			time.Sleep(1 * time.Second)
		}
	}
	assert.True(t, observed, "CheckBulk order/echo expectations not met within timeout")
}

// deltaTestTimeout bounds every RPC issued by the relationship delta tests,
// including the polling retry loops inside waitForCheckAllowed, so a single
// stalled call cannot outlast the tests' retry loops. It is sized to cover
// the worst case of one ~30s retry loop per checked expectation.
const deltaTestTimeout = 10 * time.Minute

// deltaE2EClient connects to the Inventory API with the e2e bearer token and
// returns the client and connection.
func deltaE2EClient(t *testing.T) (pbv1beta2.KesselInventoryServiceClient, *grpc.ClientConn) {
	t.Helper()
	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	t.Cleanup(func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	})
	return pbv1beta2.NewKesselInventoryServiceClient(conn), conn
}

// waitForCheckAllowed polls a Check request until the allowed value matches
// the expectation, tolerating connection errors while the consumer processes
// the write. Fails the test when the expectation is not met in time.
func waitForCheckAllowed(t *testing.T, ctx context.Context, client pbv1beta2.KesselInventoryServiceClient, req *pbv1beta2.CheckRequest, want pbv1beta2.Allowed, desc string) {
	t.Helper()
	connErrors := 0
	for i := 0; i < 30; i++ {
		checkResp, err := client.Check(ctx, req)
		if err == nil && checkResp.GetAllowed() == want {
			t.Logf("✓ %s: %v (attempt %d)", desc, want, i+1)
			return
		}
		if err != nil {
			t.Logf("Check request failed (attempt %d): %v", i+1, err)
			if isConnectionError(err) {
				connErrors++
				if connErrors >= maxConsecutiveConnErrors {
					t.Fatalf("Service unreachable after %d consecutive connection errors, aborting", connErrors)
				}
			}
		} else {
			connErrors = 0
			t.Logf("%s returned %v (attempt %d), expected %v", desc, checkResp.GetAllowed(), i+1, want)
		}
		if i < 29 {
			time.Sleep(1 * time.Second)
		}
	}
	t.Fatalf("%s: expected %v within timeout", desc, want)
}

// deltaFeaturesCheck builds a Check request against the raw
// features_workspace_direct_service_preferences relation, exposing whether
// the tuple for the given service exists.
func deltaFeaturesCheck(workspaceId, serviceId string) *pbv1beta2.CheckRequest {
	return &pbv1beta2.CheckRequest{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "workspace",
			ResourceId:   workspaceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type: "features",
			},
		},
		Relation: "features_workspace_direct_service_preferences",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "service",
				ResourceId:   serviceId,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "features",
				},
			},
		},
	}
}

// TestInventoryAPIHTTP_v1beta2_RelationshipDeltas_Features exercises the
// AddResourceRelationships and RemoveResourceRelationships endpoints against
// reporter-side relationship fields (workspace/features service
// preferences), verifying tuple creates and deletes through permission
// checks. Covers the delta endpoint user stories: auto-create, multi-value
// add, merge without affecting existing values, idempotent add, targeted
// removal, remove-all with the resource persisting, no-op removal, NOT_FOUND
// for missing and tombstoned resources.
func TestInventoryAPIHTTP_v1beta2_RelationshipDeltas_Features(t *testing.T) {
	enableShortMode(t)
	ctx, cancel := context.WithTimeout(context.Background(), deltaTestTimeout)
	t.Cleanup(cancel)

	client, _ := deltaE2EClient(t)

	workspaceId := fmt.Sprintf("delta-e2e-features-%d", time.Now().UnixNano())
	// The features workspace reporter schema validates preference values as
	// UUIDs; the workspace ID is the unique-per-run part, so fixed service
	// UUIDs are safe.
	svcA := "11111111-1111-1111-1111-111111111111"
	svcB := "22222222-2222-2222-2222-222222222222"
	svcC := "33333333-3333-3333-3333-333333333333"

	addPrefs := func(services ...string) *pbv1beta2.AddResourceRelationshipsRequest {
		values := make([]interface{}, len(services))
		for i, s := range services {
			values[i] = s
		}
		prefs, err := structpb.NewStruct(map[string]interface{}{
			"direct_service_preferences": values,
		})
		require.NoError(t, err)
		return &pbv1beta2.AddResourceRelationshipsRequest{
			Type:               "workspace",
			ReporterType:       "features",
			ReporterInstanceId: "delta-e2e-instance",
			LocalResourceId:    workspaceId,
			Reporter:           prefs,
		}
	}

	removePrefs := func(services ...string) *pbv1beta2.RemoveResourceRelationshipsRequest {
		values := make([]interface{}, len(services))
		for i, s := range services {
			values[i] = s
		}
		prefs, err := structpb.NewStruct(map[string]interface{}{
			"direct_service_preferences": values,
		})
		require.NoError(t, err)
		return &pbv1beta2.RemoveResourceRelationshipsRequest{
			Type:               "workspace",
			ReporterType:       "features",
			ReporterInstanceId: "delta-e2e-instance",
			LocalResourceId:    workspaceId,
			Reporter:           prefs,
		}
	}

	// Step 1: auto-create the resource with multiple relationship values.
	t.Logf("1. Adding preferences [%s, %s] to never-reported workspace %s (auto-create)", svcA, svcB, workspaceId)
	_, err := client.AddResourceRelationships(ctx, addPrefs(svcA, svcB))
	require.NoError(t, err, "AddResourceRelationships (auto-create) failed")

	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcA), pbv1beta2.Allowed_ALLOWED_TRUE, "auto-created tuple for svc-a")
	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcB), pbv1beta2.Allowed_ALLOWED_TRUE, "auto-created tuple for svc-b")

	// Step 2: add another value; existing values must be untouched.
	t.Logf("2. Adding preference %s (merge; existing values must be untouched)", svcC)
	_, err = client.AddResourceRelationships(ctx, addPrefs(svcC))
	require.NoError(t, err, "AddResourceRelationships (merge) failed")

	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcC), pbv1beta2.Allowed_ALLOWED_TRUE, "merged tuple for svc-c")
	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcA), pbv1beta2.Allowed_ALLOWED_TRUE, "existing tuple for svc-a after merge")
	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcB), pbv1beta2.Allowed_ALLOWED_TRUE, "existing tuple for svc-b after merge")

	// Step 3: adding an existing value is an idempotent no-op.
	t.Logf("3. Adding existing preference %s again (idempotent)", svcA)
	_, err = client.AddResourceRelationships(ctx, addPrefs(svcA))
	require.NoError(t, err, "AddResourceRelationships (idempotent) failed")
	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcA), pbv1beta2.Allowed_ALLOWED_TRUE, "tuple for svc-a after idempotent add")

	// Step 4: remove one value; the others must be untouched.
	t.Logf("4. Removing preference %s (others must be untouched)", svcA)
	_, err = client.RemoveResourceRelationships(ctx, removePrefs(svcA))
	require.NoError(t, err, "RemoveResourceRelationships failed")

	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcA), pbv1beta2.Allowed_ALLOWED_FALSE, "removed tuple for svc-a")
	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcB), pbv1beta2.Allowed_ALLOWED_TRUE, "tuple for svc-b after removing svc-a")
	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcC), pbv1beta2.Allowed_ALLOWED_TRUE, "tuple for svc-c after removing svc-a")

	// Step 5: remove multiple values in one call; the resource persists.
	t.Logf("5. Removing preferences [%s, %s] in one call (resource must persist)", svcB, svcC)
	_, err = client.RemoveResourceRelationships(ctx, removePrefs(svcB, svcC))
	require.NoError(t, err, "RemoveResourceRelationships (multiple) failed")

	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcB), pbv1beta2.Allowed_ALLOWED_FALSE, "removed tuple for svc-b")
	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcC), pbv1beta2.Allowed_ALLOWED_FALSE, "removed tuple for svc-c")

	// Removing a value that is not present is a no-op that succeeds only
	// when the resource exists (Step 8 shows missing resources return
	// NOT_FOUND), so this proves the workspace persisted the remove-all
	// and Step 6 cannot mask a failed persistence check by recreating a
	// missing workspace through AddResourceRelationships.
	t.Log("5.5. Probing the workspace still exists via a no-op removal")
	_, err = client.RemoveResourceRelationships(ctx, removePrefs(svcA))
	require.NoError(t, err, "RemoveResourceRelationships (existence probe) failed")

	t.Log("6. Re-adding a preference proves the resource still exists after all relationships were removed")
	_, err = client.AddResourceRelationships(ctx, addPrefs(svcB))
	require.NoError(t, err, "AddResourceRelationships after remove-all failed")
	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcB), pbv1beta2.Allowed_ALLOWED_TRUE, "re-added tuple for svc-b")

	// Step 7: removing a value that is not present is a no-op.
	t.Log("7. Removing preference that is not present (idempotent no-op)")
	_, err = client.RemoveResourceRelationships(ctx, removePrefs(svcA))
	require.NoError(t, err, "RemoveResourceRelationships (no-op) failed")
	waitForCheckAllowed(t, ctx, client, deltaFeaturesCheck(workspaceId, svcB), pbv1beta2.Allowed_ALLOWED_TRUE, "tuple for svc-b after no-op removal")

	// Step 8: removing from a resource that does not exist returns NOT_FOUND.
	t.Log("8. Removing from a never-reported workspace (expect NOT_FOUND)")
	_, err = client.RemoveResourceRelationships(ctx, &pbv1beta2.RemoveResourceRelationshipsRequest{
		Type:               "workspace",
		ReporterType:       "features",
		ReporterInstanceId: "delta-e2e-instance",
		LocalResourceId:    "delta-e2e-missing-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Reporter:           mustPrefsStruct(t, svcA),
	})
	require.Error(t, err, "RemoveResourceRelationships on missing resource should fail")
	assert.Equal(t, codes.NotFound, status.Code(err), "expected NOT_FOUND for missing resource")

	// Step 9: tombstoned resources reject both endpoints with NOT_FOUND.
	t.Log("9. Deleting the workspace, then adding and removing (expect NOT_FOUND)")
	_, err = client.DeleteResource(ctx, &pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "workspace",
			ResourceId:   workspaceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       "features",
				InstanceId: proto.String("delta-e2e-instance"),
			},
		},
	})
	require.NoError(t, err, "DeleteResource failed")

	_, err = client.AddResourceRelationships(ctx, addPrefs(svcA))
	require.Error(t, err, "AddResourceRelationships on tombstoned resource should fail")
	assert.Equal(t, codes.NotFound, status.Code(err), "expected NOT_FOUND for tombstoned add")

	_, err = client.RemoveResourceRelationships(ctx, removePrefs(svcB))
	require.Error(t, err, "RemoveResourceRelationships on tombstoned resource should fail")
	assert.Equal(t, codes.NotFound, status.Code(err), "expected NOT_FOUND for tombstoned remove")
}

// mustPrefsStruct builds a reporter struct carrying a single
// direct_service_preferences value, failing the test on invalid input.
func mustPrefsStruct(t *testing.T, service string) *structpb.Struct {
	t.Helper()
	prefs, err := structpb.NewStruct(map[string]interface{}{
		"direct_service_preferences": []interface{}{service},
	})
	require.NoError(t, err)
	return prefs
}

// TestInventoryAPIHTTP_v1beta2_RelationshipDeltas_Host exercises the
// relationship delta endpoints against common-stream relationship fields
// (host/hbi workspace assignment): auto-create via Add, the single-valued
// conflict rejection, and NOT_FOUND for tombstoned resources.
func TestInventoryAPIHTTP_v1beta2_RelationshipDeltas_Host(t *testing.T) {
	enableShortMode(t)
	ctx, cancel := context.WithTimeout(context.Background(), deltaTestTimeout)
	t.Cleanup(cancel)

	client, _ := deltaE2EClient(t)

	resourceId := fmt.Sprintf("delta-e2e-host-%d", time.Now().UnixNano())
	workspace := fmt.Sprintf("delta-ws-%d", time.Now().UnixNano())
	otherWorkspace := workspace + "-other"

	addWorkspace := func(ws string) *pbv1beta2.AddResourceRelationshipsRequest {
		common, err := structpb.NewStruct(map[string]interface{}{
			"workspace_id": ws,
		})
		require.NoError(t, err)
		return &pbv1beta2.AddResourceRelationshipsRequest{
			Type:               "host",
			ReporterType:       "hbi",
			ReporterInstanceId: "delta-e2e-instance",
			LocalResourceId:    resourceId,
			Common:             common,
		}
	}

	hostWorkspaceCheck := &pbv1beta2.CheckRequest{
		Object: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       "hbi",
				InstanceId: proto.String("delta-e2e-instance"),
			},
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   workspace,
				Reporter: &pbv1beta2.ReporterReference{
					Type: "rbac",
				},
			},
		},
	}

	// Step 1: auto-create the host with its workspace relationship via Add.
	t.Logf("1. Adding workspace %s to never-reported host %s (auto-create)", workspace, resourceId)
	_, err := client.AddResourceRelationships(ctx, addWorkspace(workspace))
	require.NoError(t, err, "AddResourceRelationships (auto-create) failed")

	waitForCheckAllowed(t, ctx, client, hostWorkspaceCheck, pbv1beta2.Allowed_ALLOWED_TRUE, "auto-created host workspace tuple")

	// Step 2: the workspace relationship is single-valued; adding a second
	// value while one exists is rejected.
	t.Logf("2. Adding a second workspace %s to a host that already has one (expect INVALID_ARGUMENT)", otherWorkspace)
	_, err = client.AddResourceRelationships(ctx, addWorkspace(otherWorkspace))
	require.Error(t, err, "AddResourceRelationships with conflicting single-valued relationship should fail")
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "expected INVALID_ARGUMENT for single-valued conflict")

	// Step 3: tombstoned resources reject Add with NOT_FOUND.
	t.Log("3. Deleting the host, then adding again (expect NOT_FOUND)")
	_, err = client.DeleteResource(ctx, &pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   resourceId,
			Reporter: &pbv1beta2.ReporterReference{
				Type:       "hbi",
				InstanceId: proto.String("delta-e2e-instance"),
			},
		},
	})
	require.NoError(t, err, "DeleteResource failed")

	_, err = client.AddResourceRelationships(ctx, addWorkspace(workspace))
	require.Error(t, err, "AddResourceRelationships on tombstoned resource should fail")
	assert.Equal(t, codes.NotFound, status.Code(err), "expected NOT_FOUND for tombstoned add")
}
