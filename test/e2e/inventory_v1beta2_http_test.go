package e2e

import (
	"context"
	"testing"
	"time"

	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-client-go/common"
	v1beta2 "github.com/project-kessel/inventory-client-go/v1beta2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// V1Beta2
func TestInventoryAPIHTTP_v1beta2_ResourceLifecycle_Host(t *testing.T) {
	t.Parallel()
	c := common.NewConfig(
		common.WithHTTPUrl(inventoryapi_http_url),
		common.WithTLSInsecure(insecure),
		common.WithHTTPTLSConfig(tlsConfig),
	)

	client, err := v1beta2.NewHttpClient(context.Background(), c)
	assert.NoError(t, err, "Failed to create v1beta2 HTTP client")

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"satellite_id":          "550e8400-e29b-41d4-a716-446655440000",
		"sub_manager_id":        "550e8400-e29b-41d4-a716-446655440000",
		"insights_inventory_id": "550e8400-e29b-41d4-a716-446655440000",
		"ansible_host":          "abc",
	})
	assert.NoError(t, err, "Failed to create structpb for host reporter")

	req := pbv1beta2.ReportResourceRequest{
		WaitForSync: false,
		Resource: &pbv1beta2.Resource{
			Type:               "host",
			ReporterType:       "HBI",
			ReporterInstanceId: "testuser@example.com",
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
		},
	}

	opts := getCallOptions()
	_, err = client.KesselResourceService.ReportResource(context.Background(), &req, opts...)
	assert.NoError(t, err, "Failed to Report Resource")

	delReq := pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "host",
			ResourceId:   "host-abc-123",
			Reporter: &pbv1beta2.ReporterReference{
				Type: "HBI",
			},
		},
	}

	_, err = client.KesselResourceService.DeleteResource(context.Background(), &delReq, opts...)
	assert.NoError(t, err, "Failed to Delete Resource")

}

func TestInventoryAPIHTTP_v1beta2_ResourceLifecycle_Notifications(t *testing.T) {
	t.Parallel()

	c := common.NewConfig(
		common.WithHTTPUrl(inventoryapi_http_url),
		common.WithTLSInsecure(insecure),
		common.WithHTTPTLSConfig(tlsConfig),
	)

	client, err := v1beta2.NewHttpClient(context.Background(), c)
	assert.NoError(t, err, "Failed to create v1beta2 HTTP client")

	// will likely change the notifications json schema, this is here to satisfy validation
	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"reporter_type":        "NOTIFICATIONS",
		"reporter_instance_id": "testuser@example.com",
		"local_resource_id":    "notification-abc-123",
	})
	assert.NoError(t, err, "Failed to create structpb for reporter")

	req := pbv1beta2.ReportResourceRequest{
		Resource: &pbv1beta2.Resource{
			Type:               "notifications_integration",
			ReporterType:       "NOTIFICATIONS",
			ReporterInstanceId: "testuser@example.com",
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
		},
	}

	opts := getCallOptions()
	_, err = client.KesselResourceService.ReportResource(context.Background(), &req, opts...)
	assert.NoError(t, err, "Failed to Report Resource")

	delReq := pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "integrations",
			ResourceId:   "notification-abc-123",
			Reporter: &pbv1beta2.ReporterReference{
				Type: "NOTIFICATIONS",
			},
		},
	}

	_, err = client.KesselResourceService.DeleteResource(context.Background(), &delReq, opts...)
	assert.NoError(t, err, "Failed to Delete Resource")

}

func TestInventoryAPIHTTP_v1beta2_ResourceLifecycle_K8S_Cluster(t *testing.T) {
	t.Parallel()

	c := common.NewConfig(
		common.WithHTTPUrl(inventoryapi_http_url),
		common.WithTLSInsecure(insecure),
		common.WithHTTPTLSConfig(tlsConfig),
	)

	client, err := v1beta2.NewHttpClient(context.Background(), c)
	assert.NoError(t, err, "Failed to create v1beta2 HTTP client")

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
		Resource: &pbv1beta2.Resource{
			Type:               "k8s_cluster",
			ReporterType:       "ACM",
			ReporterInstanceId: "testuser@example.com",
			Representations: &pbv1beta2.ResourceRepresentations{
				Metadata: &pbv1beta2.RepresentationMetadata{
					LocalResourceId: "k8s_cluster-abc-123",
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
		},
	}

	opts := getCallOptions()
	_, err = client.KesselResourceService.ReportResource(context.Background(), &req, opts...)
	assert.NoError(t, err, "Failed to Report Resource")

	delReq := pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "k8s_cluster",
			ResourceId:   "k8s_cluster-abc-123",
			Reporter: &pbv1beta2.ReporterReference{
				Type: "ACM",
			},
		},
	}

	_, err = client.KesselResourceService.DeleteResource(context.Background(), &delReq, opts...)
	assert.NoError(t, err, "Failed to Delete Resource")

}

func TestInventoryAPIHTTP_v1beta2_ResourceLifecycle_K8S_Policy(t *testing.T) {
	t.Parallel()

	c := common.NewConfig(
		common.WithHTTPUrl(inventoryapi_http_url),
		common.WithTLSInsecure(insecure),
		common.WithHTTPTLSConfig(tlsConfig),
	)

	client, err := v1beta2.NewHttpClient(context.Background(), c)
	assert.NoError(t, err, "Failed to create v1beta2 HTTP client")

	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"disabled": true,
		"severity": "MEDIUM",
	})
	assert.NoError(t, err, "Failed to create structpb for reporter")

	req := pbv1beta2.ReportResourceRequest{
		Resource: &pbv1beta2.Resource{
			Type:               "k8s_policy",
			ReporterType:       "ACM",
			ReporterInstanceId: "testuser@example.com",
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
		},
	}

	opts := getCallOptions()
	_, err = client.KesselResourceService.ReportResource(context.Background(), &req, opts...)
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

	_, err = client.KesselResourceService.DeleteResource(context.Background(), &delReq, opts...)
	assert.NoError(t, err, "Failed to Delete Resource")

}

//func TestInventoryAPIHTTP_v1beta2_AuthzLifecycle(t *testing.T) {
//	t.Parallel()
//
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

func TestInventoryAPIHTTP_v1beta2_Host_WaitForSync(t *testing.T) {
	t.Parallel()

	resourceId := "wait-for-sync-host-abc-123"

	c := common.NewConfig(
		common.WithHTTPUrl(inventoryapi_http_url),
		common.WithTLSInsecure(insecure),
		common.WithHTTPTLSConfig(tlsConfig),
		common.WithTimeout(10*time.Second),
	)

	client, err := v1beta2.NewHttpClient(context.Background(), c)
	assert.NoError(t, err, "Failed to create v1beta2 HTTP client")

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
		WaitForSync: true,
		Resource: &pbv1beta2.Resource{
			Type:               "host",
			ReporterType:       "HBI",
			ReporterInstanceId: "testuser@example.com",
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
		},
	}
	opts := getCallOptions()
	_, err = client.KesselResourceService.ReportResource(context.Background(), &req, opts...)
	assert.NoError(t, err, "Failed to Report Resource")

	var host model.Resource
	err = db.Where("reporter_resource_id = ?", resourceId).First(&host).Error
	assert.NoError(t, err, "Error fetching host from DB")
	assert.NotNil(t, host, "Host not found in DB")
	assert.NotEmpty(t, host.ConsistencyToken, "Consistency token is empty")

	delReq := pbv1beta2.DeleteResourceRequest{
		Reference: &pbv1beta2.ResourceReference{
			ResourceType: "HBI",
			ResourceId:   "resourceId",
			Reporter: &pbv1beta2.ReporterReference{
				Type: "ACM",
			},
		},
	}
	_, err = client.KesselResourceService.DeleteResource(context.Background(), &delReq, opts...)
	assert.NoError(t, err, "Failed to Delete Resource")
}
