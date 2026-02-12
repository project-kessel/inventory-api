package e2e

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
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
	assert.NoError(t, err, "Failed to create gRPC client")
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
	assert.NoError(t, err, "Failed to create structpb for host reporter")

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
	assert.NoError(t, err, "Failed to Report Resource")

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
	assert.NoError(t, err, "Failed to create gRPC client")
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
	assert.NoError(t, err, "Failed to create structpb for reporter")

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
	assert.NoError(t, err, "Failed to Report Resource")

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
	assert.NoError(t, err, "Failed to create gRPC client")
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
	assert.NoError(t, err, "Failed to create structpb for cluster reporter")

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
	assert.NoError(t, err, "Failed to Report Resource")

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
	assert.NoError(t, err, "Failed to create gRPC client")
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
	assert.NoError(t, err, "Failed to create structpb for reporter")

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
	assert.NoError(t, err, "Failed to Report Resource")

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
	assert.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	assert.NoError(t, err, "Failed to connect gRPC client")

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	commonData := &structpb.Struct{}
	commonData.Fields = map[string]*structpb.Value{
		"workspace_id": structpb.NewStringValue("workspace-v2"),
	}

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"disabled": true,
		"severity": "MEDIUM",
	})
	assert.NoError(t, err, "Failed to create structpb for reporter")

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
	assert.NoError(t, err, "Failed to Report Resource")

	var host datamodel.Resource
	err = db.Table("resource r").
		Select("r.*").
		Joins("JOIN reporter_resources rr ON r.id = rr.resource_id").
		Where("rr.local_resource_id = ?", resourceId).
		First(&host).Error
	assert.NoError(t, err, "Error fetching host from DB")
	assert.NotNil(t, host, "Host not found in DB")
	assert.NotEmpty(t, host.ConsistencyToken, "Consistency token is empty")

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
	assert.NoError(t, err, "Failed to create gRPC client")
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
	assert.NoError(t, err, "Failed to create structpb for host reporter")

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
	t.Log("Waiting for outbox events to be processed (polling up to 10s)...")

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

	// Poll for up to 10 seconds
	oldWorkspaceAllowed := false
	for i := 0; i < 10; i++ {
		checkResp, err := client.Check(ctx, checkOldWorkspace)
		if err == nil && checkResp.GetAllowed() == pbv1beta2.Allowed_ALLOWED_FALSE {
			t.Logf("✓ Old workspace check returned ALLOWED_FALSE (attempt %d)", i+1)
			oldWorkspaceAllowed = true
			break
		}
		if err != nil {
			t.Logf("Check request failed (attempt %d): %v", i+1, err)
		} else {
			t.Logf("Old workspace check returned %v (attempt %d), expected ALLOWED_FALSE", checkResp.GetAllowed(), i+1)
		}
		if i < 9 {
			t.Log("Waiting 1s before retry...")
			time.Sleep(1 * time.Second)
		}
	}
	assert.True(t, oldWorkspaceAllowed, "Old workspace authorization check did not return ALLOWED_FALSE within timeout")

	// Step 4: Check authorization for NEW workspace (should be ALLOWED_TRUE)
	t.Logf("4. Checking authorization for NEW workspace %s (should be allowed: true)", newWorkspace)
	t.Log("Waiting for outbox events to be processed (polling up to 10s)...")

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

	// Poll for up to 10 seconds
	newWorkspaceAllowed := false
	for i := 0; i < 10; i++ {
		checkResp, err := client.Check(ctx, checkNewWorkspace)
		if err == nil && checkResp.GetAllowed() == pbv1beta2.Allowed_ALLOWED_TRUE {
			t.Logf("✓ New workspace check returned ALLOWED_TRUE (attempt %d)", i+1)
			newWorkspaceAllowed = true
			break
		}
		if err != nil {
			t.Logf("Check request failed (attempt %d): %v", i+1, err)
		} else {
			t.Logf("New workspace check returned %v (attempt %d), expected ALLOWED_TRUE", checkResp.GetAllowed(), i+1)
		}
		if i < 9 {
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
	assert.NoError(t, err, "Failed to create gRPC client")
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
	assert.NoError(t, err, "Failed to create structpb for reporter")

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
	assert.NoError(t, err, "Failed to Report Resource")
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

	// Poll up to 10 seconds to allow eventual consistency and error mapping
	observed := false
	for i := 0; i < 10; i++ {
		resp, err := client.CheckBulk(ctx, checkReq)
		if err == nil && resp != nil && len(resp.GetPairs()) == 3 {
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
		}
		if i < 9 {
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
	assert.NoError(t, err, "Failed to create gRPC client")
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
	assert.NoError(t, err, "Failed to create structpb for reporter")

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
	assert.NoError(t, err, "Failed to Report Resource")

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
	for i := 0; i < 10; i++ {
		checkResp, err := client.Check(ctx, checkAfterCreate)
		if err == nil && checkResp.GetAllowed() == pbv1beta2.Allowed_ALLOWED_TRUE {
			t.Logf("✓ Create-check returned ALLOWED_TRUE (attempt %d)", i+1)
			allowedTrueObserved = true
			break
		}
		if err != nil {
			t.Logf("Check request failed (attempt %d): %v", i+1, err)
		} else {
			t.Logf("Create-check returned %v (attempt %d), expected ALLOWED_TRUE", checkResp.GetAllowed(), i+1)
		}
		if i < 9 {
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
	for i := 0; i < 10; i++ {
		checkResp, err := client.Check(ctx, checkAfterDelete)
		if err == nil && checkResp.GetAllowed() == pbv1beta2.Allowed_ALLOWED_FALSE {
			t.Logf("✓ Delete-check returned ALLOWED_FALSE (attempt %d)", i+1)
			allowedFalseObserved = true
			break
		}
		if err != nil {
			t.Logf("Check request failed (attempt %d): %v", i+1, err)
		} else {
			t.Logf("Delete-check returned %v (attempt %d), expected ALLOWED_FALSE", checkResp.GetAllowed(), i+1)
		}
		if i < 9 {
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

func TestInventoryAPIHTTP_v1beta2_CheckBulk_SingleTrueAndFalse(t *testing.T) {
	enableShortMode(t)

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	assert.NoError(t, err, "Failed to create gRPC client")
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
	assert.NoError(t, err, "Failed to create structpb for reporter")

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
	assert.NoError(t, err, "Failed to Report Resource")
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

	// Poll up to 10 seconds to account for eventual consistency
	observed := false
	for i := 0; i < 10; i++ {
		resp, err := client.CheckBulk(ctx, checkBulkReq)
		if err == nil && resp != nil && len(resp.GetPairs()) == 2 {
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
		}
		if i < 9 {
			time.Sleep(1 * time.Second)
		}
	}
	assert.True(t, observed, "CheckBulk didn't return [TRUE, FALSE] within timeout")

}

func TestInventoryAPIHTTP_v1beta2_CheckBulk_OrderAndEcho(t *testing.T) {
	enableShortMode(t)

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	assert.NoError(t, err, "Failed to create gRPC client")
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
	assert.NoError(t, err, "Failed to create structpb for reporter")

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
	assert.NoError(t, err, "Failed to Report Resource")
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

	// Poll up to 10 seconds; verify order is preserved and request is echoed
	observed := false
	for i := 0; i < 10; i++ {
		resp, err := client.CheckBulk(ctx, &pbv1beta2.CheckBulkRequest{
			Items: []*pbv1beta2.CheckBulkRequestItem{item1, item2},
		})
		if err == nil && resp != nil && len(resp.GetPairs()) == 2 {
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
		}
		if i < 9 {
			time.Sleep(1 * time.Second)
		}
	}
	assert.True(t, observed, "CheckBulk order/echo expectations not met within timeout")
}

func TestInventoryAPIHTTP_v1beta2_StreamedListObjects(t *testing.T) {
	enableShortMode(t)

	ctx := context.Background()

	conn, err := grpc.NewClient(
		inventoryapi_grpc_url,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	assert.NoError(t, err, "Failed to create gRPC client")
	defer func() {
		if connErr := conn.Close(); connErr != nil {
			t.Logf("Failed to close gRPC connection: %v", connErr)
		}
	}()

	conn.Connect()
	assert.NoError(t, err, "Failed to connect gRPC client")

	client := pbv1beta2.NewKesselInventoryServiceClient(conn)

	resourceId := "streamedlistobjects-host-0001"
	reporterType := "hbi"
	reporterInstanceId := "testuser-example-com"
	workspace := "workspace-streamedlist"

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
		"ansible_host": "streamedlistobjects-host.example.com",
	})
	assert.NoError(t, err, "Failed to create structpb for reporter")

	req := &pbv1beta2.ReportResourceRequest{
		WriteVisibility:    pbv1beta2.WriteVisibility_MINIMIZE_LATENCY,
		Type:               "host",
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstanceId,
		Representations: &pbv1beta2.ResourceRepresentations{
			Metadata: &pbv1beta2.RepresentationMetadata{
				LocalResourceId: resourceId,
				ApiHref:         "https://api.example.com/hosts/streamedlistobjects-host-0001",
				ConsoleHref:     proto.String("https://console.example.com/hosts/streamedlistobjects-host-0001"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue(workspace),
				},
			},
			Reporter: reporterStruct,
		},
	}
	t.Logf("ReportResource: type=host resource_id=%s workspace=%s", resourceId, workspace)
	_, err = client.ReportResource(ctx, req)
	assert.NoError(t, err, "Failed to Report Resource")
	resourceCreated = true

	baseListReq := &pbv1beta2.StreamedListObjectsRequest{
		ObjectType: &pbv1beta2.RepresentationType{
			ResourceType: "host",
			ReporterType: proto.String(reporterType),
		},
		Relation: "workspace",
		Subject: &pbv1beta2.SubjectReference{
			Resource: &pbv1beta2.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   workspace,
				Reporter:     &pbv1beta2.ReporterReference{Type: "rbac"},
			},
		},
	}

	runStreamAndPoll := func(t *testing.T, listReq *pbv1beta2.StreamedListObjectsRequest, consistencyLabel string) []string {
		var resourceIDs []string
		for attempt := 0; attempt < 10; attempt++ {
			stream, err := client.StreamedListObjects(ctx, listReq)
			assert.NoError(t, err, "StreamedListObjects failed")
			resourceIDs = nil
			for {
				resp, recvErr := stream.Recv()
				if recvErr == io.EOF {
					break
				}
				assert.NoError(t, recvErr, "StreamedListObjects Recv failed")
				if resp.GetObject() != nil {
					resourceIDs = append(resourceIDs, resp.GetObject().GetResourceId())
				}
			}
			t.Logf("StreamedListObjects consistency=%s attempt %d: got %d objects, ids=%v", consistencyLabel, attempt+1, len(resourceIDs), resourceIDs)
			if len(resourceIDs) > 0 && contains(resourceIDs, resourceId) {
				t.Logf("StreamedListObjects consistency=%s: found resource in stream (attempt %d)", consistencyLabel, attempt+1)
				break
			}
			if attempt < 9 {
				time.Sleep(1 * time.Second)
			}
		}
		return resourceIDs
	}

	t.Run("MinimizeLatency", func(t *testing.T) {
		// Default: no Consistency set means minimize_latency
		t.Logf("StreamedListObjects test case: consistency=minimize_latency (default)")
		resourceIDs := runStreamAndPoll(t, baseListReq, "minimize_latency")
		assert.Contains(t, resourceIDs, resourceId, "stream should contain reported resource (got: %v)", resourceIDs)
	})

	t.Run("AtLeastAsFresh", func(t *testing.T) {
		// Optional: at_least_as_fresh with a consistency token from the resource
		var host datamodel.Resource
		for attempt := 0; attempt < 10; attempt++ {
			err := db.Table("resource r").
				Select("r.*").
				Joins("JOIN reporter_resources rr ON r.id = rr.resource_id").
				Where("rr.local_resource_id = ?", resourceId).
				First(&host).Error
			if err == nil && host.ConsistencyToken != "" {
				break
			}
			if attempt < 9 {
				time.Sleep(1 * time.Second)
			}
		}
		assert.NotEmpty(t, host.ConsistencyToken, "resource must have consistency token for at_least_as_fresh")
		listReq := proto.Clone(baseListReq).(*pbv1beta2.StreamedListObjectsRequest)
		listReq.Consistency = &pbv1beta2.Consistency{
			Requirement: &pbv1beta2.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &pbv1beta2.ConsistencyToken{Token: host.ConsistencyToken},
			},
		}
		t.Logf("StreamedListObjects test case: consistency=at_least_as_fresh (token=%s...)", truncateToken(host.ConsistencyToken))
		resourceIDs := runStreamAndPoll(t, listReq, "at_least_as_fresh")
		assert.Contains(t, resourceIDs, resourceId, "stream should contain reported resource (got: %v)", resourceIDs)
	})
}

func truncateToken(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12] + "..."
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
