package e2e

import (
	"context"
	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-client-go/common"
	v1beta2 "github.com/project-kessel/inventory-client-go/v1beta2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
	"testing"
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

	resourceData := &structpb.Struct{}
	commonData := &structpb.Struct{}

	commonData.Fields = map[string]*structpb.Value{
		"workspace_id": structpb.NewStringValue("workspace-v2"),
	}

	req := pbv1beta2.ReportResourceRequest{
		Resource: &pbv1beta2.Resource{
			ResourceType: "host",
			ReporterData: &pbv1beta2.ReporterData{
				ReporterType:       "HBI",
				ReporterInstanceId: "testuser@example.com",
				ReporterVersion:    "0.1",
				LocalResourceId:    "host-abc-123",
				ApiHref:            "https://example.com/api",
				ConsoleHref:        "https://example.com/console",
				ResourceData:       resourceData,
			},
			CommonResourceData: commonData,
		},
	}
	opts := getCallOptions()
	_, err = client.KesselResourceService.ReportResource(context.Background(), &req, opts...)
	assert.NoError(t, err, "Failed to Report Resource")

	delReq := pbv1beta2.DeleteResourceRequest{
		LocalResourceId: "host-abc-123",
		ReporterType:    "HBI",
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

	resourceData := &structpb.Struct{}
	commonData := &structpb.Struct{}

	commonData.Fields = map[string]*structpb.Value{
		"workspace_id": structpb.NewStringValue("workspace-v2"),
	}

	req := pbv1beta2.ReportResourceRequest{
		Resource: &pbv1beta2.Resource{
			ResourceType: "notifications_integration",
			ReporterData: &pbv1beta2.ReporterData{
				ReporterType:       "NOTIFICATIONS",
				ReporterInstanceId: "testuser@example.com",
				ReporterVersion:    "0.1",
				LocalResourceId:    "notification-abc-123",
				ApiHref:            "https://example.com/api",
				ConsoleHref:        "https://example.com/console",
				ResourceData:       resourceData,
			},
			CommonResourceData: commonData,
		},
	}
	opts := getCallOptions()
	_, err = client.KesselResourceService.ReportResource(context.Background(), &req, opts...)
	assert.NoError(t, err, "Failed to Report Resource")

	delReq := pbv1beta2.DeleteResourceRequest{
		LocalResourceId: "notification-abc-123",
		ReporterType:    "NOTIFICATIONS",
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

	resourceData := &structpb.Struct{}
	commonData := &structpb.Struct{}

	commonData.Fields = map[string]*structpb.Value{
		"workspace_id": structpb.NewStringValue("workspace-v2"),
	}

	req := pbv1beta2.ReportResourceRequest{
		Resource: &pbv1beta2.Resource{
			ResourceType: "k8s_cluster",
			ReporterData: &pbv1beta2.ReporterData{
				ReporterType:       "ACM",
				ReporterInstanceId: "testuser@example.com",
				ReporterVersion:    "0.1",
				LocalResourceId:    "k8s_cluster-abc-123",
				ApiHref:            "https://example.com/api",
				ConsoleHref:        "https://example.com/console",
				ResourceData:       resourceData,
			},
			CommonResourceData: commonData,
		},
	}
	opts := getCallOptions()
	_, err = client.KesselResourceService.ReportResource(context.Background(), &req, opts...)
	assert.NoError(t, err, "Failed to Report Resource")

	delReq := pbv1beta2.DeleteResourceRequest{
		LocalResourceId: "k8s_cluster-abc-123",
		ReporterType:    "ACM",
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

	resourceData := &structpb.Struct{}
	commonData := &structpb.Struct{}

	commonData.Fields = map[string]*structpb.Value{
		"workspace_id": structpb.NewStringValue("workspace-v2"),
	}

	req := pbv1beta2.ReportResourceRequest{
		Resource: &pbv1beta2.Resource{
			ResourceType: "k8s_policy",
			ReporterData: &pbv1beta2.ReporterData{
				ReporterType:       "ACM",
				ReporterInstanceId: "testuser@example.com",
				ReporterVersion:    "0.1",
				LocalResourceId:    "k8s_policy-abc-123",
				ApiHref:            "https://example.com/api",
				ConsoleHref:        "https://example.com/console",
				ResourceData:       resourceData,
			},
			CommonResourceData: commonData,
		},
	}
	opts := getCallOptions()
	_, err = client.KesselResourceService.ReportResource(context.Background(), &req, opts...)
	assert.NoError(t, err, "Failed to Report Resource")

	delReq := pbv1beta2.DeleteResourceRequest{
		LocalResourceId: "k8s_policy-abc-123",
		ReporterType:    "ACM",
	}

	_, err = client.KesselResourceService.DeleteResource(context.Background(), &delReq, opts...)
	assert.NoError(t, err, "Failed to Delete Resource")

}

func TestInventoryAPIHTTP_v1beta2_AuthzLifecycle(t *testing.T) {
	t.Parallel()

	c := common.NewConfig(
		common.WithHTTPUrl(inventoryapi_http_url),
		common.WithTLSInsecure(insecure),
		common.WithHTTPTLSConfig(tlsConfig),
	)

	client, err := v1beta2.NewHttpClient(context.Background(), c)
	assert.NoError(t, err, "Failed to create v1beta2 HTTP client")

	ctx := context.Background()

	subject := &pbv1beta2.SubjectReference{
		Subject: &pbv1beta2.ResourceReference{
			ResourceId:   "bob",
			ResourceType: "principal",
			Reporter: &pbv1beta2.ReporterReference{
				Type: "rbac",
			},
		},
	}

	parent := &pbv1beta2.ResourceReference{
		ResourceId: "bob_club",
		Reporter: &pbv1beta2.ReporterReference{
			Type: "rbac",
		},
		ResourceType: "group",
	}

	// /authz/check
	checkReq := &pbv1beta2.CheckRequest{
		Subject:  subject,
		Relation: "member",
		Parent:   parent,
	}

	checkResp, err := client.KesselCheckService.Check(ctx, checkReq)
	assert.NoError(t, err, "check endpoint failed")
	assert.NotNil(t, checkResp, "check response should not be nil")
	assert.Equal(t, pbv1beta2.Allowed_ALLOWED_FALSE, checkResp.GetAllowed())

	// /authz/checkforupdate
	checkUpdateReq := &pbv1beta2.CheckForUpdateRequest{
		Subject:  subject,
		Relation: "member",
		Parent:   parent,
	}

	checkUpdateResp, err := client.KesselCheckService.CheckForUpdate(ctx, checkUpdateReq)
	assert.NoError(t, err, "checkforupdate endpoint failed")
	assert.NotNil(t, checkUpdateResp, "checkforupdate response should not be nil")
	assert.Equal(t, pbv1beta2.Allowed_ALLOWED_FALSE, checkUpdateResp.GetAllowed())
}
