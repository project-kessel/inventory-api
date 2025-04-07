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
func TestInventoryAPIHTTP_v1beta2_ResourceLifecycle(t *testing.T) {
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
