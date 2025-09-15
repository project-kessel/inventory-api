package resources

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal"
	"github.com/project-kessel/inventory-api/internal/authz/allow"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
)

func TestReportResource(t *testing.T) {
	tests := []struct {
		name             string
		resourceType     string
		reporterType     string
		reporterInstance string
		localResourceId  string
		workspaceId      string
		expectError      bool
	}{
		{
			name:             "creates new k8s cluster resource",
			resourceType:     "k8s_cluster",
			reporterType:     "ocm",
			reporterInstance: "test-instance",
			localResourceId:  "test-local-resource",
			workspaceId:      "test-workspace",
			expectError:      false,
		},
		{
			name:             "creates new host resource",
			resourceType:     "host",
			reporterType:     "hbi",
			reporterInstance: "hbi-instance",
			localResourceId:  "test-host-123",
			workspaceId:      "test-workspace-2",
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := log.DefaultLogger

			resourceRepo := data.NewFakeResourceRepository()
			authorizer := &allow.AllowAllAuthz{}
			usecaseConfig := &UsecaseConfig{
				ReadAfterWriteEnabled: false,
				ConsumerEnabled:       false,
			}

			usecase := New(resourceRepo, nil, nil, authorizer, nil, "test-topic", logger, nil, nil, usecaseConfig)

			reportRequest := createTestReportRequest(t, tt.resourceType, tt.reporterType, tt.reporterInstance, tt.localResourceId, tt.workspaceId)
			err := usecase.ReportResource(ctx, reportRequest, "test-reporter")

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			localResourceId, err := model.NewLocalResourceId(tt.localResourceId)
			require.NoError(t, err)
			resourceType, err := model.NewResourceType(tt.resourceType)
			require.NoError(t, err)
			reporterType, err := model.NewReporterType(tt.reporterType)
			require.NoError(t, err)
			reporterInstanceId, err := model.NewReporterInstanceId(tt.reporterInstance)
			require.NoError(t, err)

			key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
			require.NoError(t, err)

			foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err)
			require.NotNil(t, foundResource)
		})
	}
}

func TestReportResourceThenDelete(t *testing.T) {
	tests := []struct {
		name                     string
		resourceType             string
		reporterType             string
		reporterInstanceId       string
		localResourceId          string
		workspaceId              string
		deleteReporterInstanceId string
		expectError              bool
	}{
		{
			name:                     "deletes resource with reporterInstanceId",
			resourceType:             "k8s_cluster",
			reporterType:             "ocm",
			reporterInstanceId:       "delete-test-instance",
			localResourceId:          "delete-test-resource",
			workspaceId:              "delete-test-workspace",
			deleteReporterInstanceId: "delete-test-instance",
			expectError:              false,
		},
		{
			name:                     "deletes resource without reporterInstanceId",
			resourceType:             "host",
			reporterType:             "hbi",
			reporterInstanceId:       "delete-test-instance-2",
			localResourceId:          "delete-test-resource-2",
			workspaceId:              "delete-test-workspace-2",
			deleteReporterInstanceId: "",
			expectError:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := log.DefaultLogger

			resourceRepo := data.NewFakeResourceRepository()
			authorizer := &allow.AllowAllAuthz{}
			usecaseConfig := &UsecaseConfig{
				ReadAfterWriteEnabled: false,
				ConsumerEnabled:       false,
			}

			usecase := New(resourceRepo, nil, nil, authorizer, nil, "test-topic", logger, nil, nil, usecaseConfig)

			reportRequest := createTestReportRequest(t, tt.resourceType, tt.reporterType, tt.reporterInstanceId, tt.localResourceId, tt.workspaceId)
			err := usecase.ReportResource(ctx, reportRequest, "test-reporter")
			require.NoError(t, err)

			localResourceId, err := model.NewLocalResourceId(tt.localResourceId)
			require.NoError(t, err)
			resourceType, err := model.NewResourceType(tt.resourceType)
			require.NoError(t, err)
			reporterType, err := model.NewReporterType(tt.reporterType)
			require.NoError(t, err)
			reporterInstanceId, err := model.NewReporterInstanceId(tt.reporterInstanceId)
			require.NoError(t, err)

			key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
			require.NoError(t, err)

			foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err)
			require.NotNil(t, foundResource)

			var deleteReporterInstanceId model.ReporterInstanceId
			if tt.deleteReporterInstanceId != "" {
				deleteReporterInstanceId, err = model.NewReporterInstanceId(tt.deleteReporterInstanceId)
				require.NoError(t, err)
			} else {
				deleteReporterInstanceId = model.ReporterInstanceId("")
			}

			deleteKey, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, deleteReporterInstanceId)
			require.NoError(t, err)

			deleteFoundResource, err := resourceRepo.FindResourceByKeys(nil, deleteKey)
			require.NoError(t, err)
			require.NotNil(t, deleteFoundResource)
		})
	}
}

func createTestReportRequest(t *testing.T, resourceType, reporterType, reporterInstance, localResourceId, workspaceId string) *v1beta2.ReportResourceRequest {
	reporterData, _ := structpb.NewStruct(map[string]interface{}{
		"local_resource_id": localResourceId,
		"api_href":          "https://api.example.com/resource/123",
		"console_href":      "https://console.example.com/resource/123",
	})

	commonData, _ := structpb.NewStruct(map[string]interface{}{
		"workspace_id": workspaceId,
		"name":         "test-cluster",
		"namespace":    "default",
	})

	return &v1beta2.ReportResourceRequest{
		Type:               resourceType,
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstance,
		Representations: &v1beta2.ResourceRepresentations{
			Metadata: &v1beta2.RepresentationMetadata{
				LocalResourceId: localResourceId,
				ApiHref:         "https://api.example.com/resource/123",
				ConsoleHref:     internal.StringPtr("https://console.example.com/resource/123"),
			},
			Reporter: reporterData,
			Common:   commonData,
		},
		WriteVisibility: v1beta2.WriteVisibility_MINIMIZE_LATENCY,
	}
}

func TestDelete_ResourceNotFound(t *testing.T) {
	logger := log.DefaultLogger

	resourceRepo := data.NewFakeResourceRepository()
	authorizer := &allow.AllowAllAuthz{}
	usecaseConfig := &UsecaseConfig{
		ReadAfterWriteEnabled: false,
		ConsumerEnabled:       false,
	}

	usecase := New(resourceRepo, nil, nil, authorizer, nil, "test-topic", logger, nil, nil, usecaseConfig)

	localResourceId, err := model.NewLocalResourceId("non-existent-resource")
	require.NoError(t, err)
	resourceType, err := model.NewResourceType("k8s_cluster")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("ocm")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("test-instance")
	require.NoError(t, err)

	key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
	require.NoError(t, err)

	err = usecase.Delete(key)
	require.Error(t, err)
}
