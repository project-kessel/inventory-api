package resources

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/gorm"

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

func TestReportFindDeleteFind_TombstoneLifecycle(t *testing.T) {
	ctx := context.Background()
	logger := log.DefaultLogger

	resourceRepo := data.NewFakeResourceRepository()
	authorizer := &allow.AllowAllAuthz{}
	usecaseConfig := &UsecaseConfig{
		ReadAfterWriteEnabled: false,
		ConsumerEnabled:       false,
	}

	usecase := New(resourceRepo, nil, nil, authorizer, nil, "test-topic", logger, nil, nil, usecaseConfig)

	reportRequest := createTestReportRequest(t, "k8s_cluster", "ocm", "lifecycle-instance", "lifecycle-resource", "lifecycle-workspace")
	err := usecase.ReportResource(ctx, reportRequest, "test-reporter")
	require.NoError(t, err)

	localResourceId, err := model.NewLocalResourceId("lifecycle-resource")
	require.NoError(t, err)
	resourceType, err := model.NewResourceType("k8s_cluster")
	require.NoError(t, err)
	reporterType, err := model.NewReporterType("ocm")
	require.NoError(t, err)
	reporterInstanceId, err := model.NewReporterInstanceId("lifecycle-instance")
	require.NoError(t, err)

	key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
	require.NoError(t, err)

	foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
	require.NoError(t, err)
	require.NotNil(t, foundResource)

	err = usecase.Delete(key)
	require.NoError(t, err)

	foundResource, err = resourceRepo.FindResourceByKeys(nil, key)
	// With tombstone filter removed, we should find the tombstoned resource
	require.NoError(t, err)
	require.NotNil(t, foundResource)
	assert.True(t, foundResource.ReporterResources()[0].Serialize().Tombstone, "Resource should be tombstoned")
}

func TestMultipleHostsLifecycle(t *testing.T) {
	ctx := context.Background()
	logger := log.DefaultLogger

	resourceRepo := data.NewFakeResourceRepository()
	authorizer := &allow.AllowAllAuthz{}
	usecaseConfig := &UsecaseConfig{
		ReadAfterWriteEnabled: false,
		ConsumerEnabled:       false,
	}

	usecase := New(resourceRepo, nil, nil, authorizer, nil, "test-topic", logger, nil, nil, usecaseConfig)

	// Create 2 hosts
	host1Request := createTestReportRequest(t, "host", "hbi", "hbi-instance-1", "host-1", "workspace-1")
	err := usecase.ReportResource(ctx, host1Request, "test-reporter")
	require.NoError(t, err, "Should create host1")

	host2Request := createTestReportRequest(t, "host", "hbi", "hbi-instance-1", "host-2", "workspace-1")
	err = usecase.ReportResource(ctx, host2Request, "test-reporter")
	require.NoError(t, err, "Should create host2")

	// Verify both hosts can be found
	key1, err := model.NewReporterResourceKey("host-1", "host", "hbi", "hbi-instance-1")
	require.NoError(t, err)
	key2, err := model.NewReporterResourceKey("host-2", "host", "hbi", "hbi-instance-1")
	require.NoError(t, err)

	foundHost1, err := resourceRepo.FindResourceByKeys(nil, key1)
	require.NoError(t, err, "Should find host1 after creation")
	require.NotNil(t, foundHost1)

	foundHost2, err := resourceRepo.FindResourceByKeys(nil, key2)
	require.NoError(t, err, "Should find host2 after creation")
	require.NotNil(t, foundHost2)

	// Update both hosts by reporting them again with updated data
	host1UpdateRequest := createTestReportRequestWithUpdatedData(t, "host", "hbi", "hbi-instance-1", "host-1", "workspace-1")
	err = usecase.ReportResource(ctx, host1UpdateRequest, "test-reporter")
	require.NoError(t, err, "Should update host1")

	host2UpdateRequest := createTestReportRequestWithUpdatedData(t, "host", "hbi", "hbi-instance-1", "host-2", "workspace-1")
	err = usecase.ReportResource(ctx, host2UpdateRequest, "test-reporter")
	require.NoError(t, err, "Should update host2")

	// Verify both updated hosts can still be found
	updatedHost1, err := resourceRepo.FindResourceByKeys(nil, key1)
	require.NoError(t, err, "Should find host1 after update")
	require.NotNil(t, updatedHost1)

	updatedHost2, err := resourceRepo.FindResourceByKeys(nil, key2)
	require.NoError(t, err, "Should find host2 after update")
	require.NotNil(t, updatedHost2)

	// Delete both hosts
	err = usecase.Delete(key1)
	require.NoError(t, err, "Should delete host1")

	err = usecase.Delete(key2)
	require.NoError(t, err, "Should delete host2")

	// Verify both hosts can be found (tombstoned) with tombstone filter removed
	foundHost1, err = resourceRepo.FindResourceByKeys(nil, key1)
	require.NoError(t, err, "Should find tombstoned host1")
	require.NotNil(t, foundHost1)
	assert.True(t, foundHost1.ReporterResources()[0].Serialize().Tombstone, "Host1 should be tombstoned")

	foundHost2, err = resourceRepo.FindResourceByKeys(nil, key2)
	require.NoError(t, err, "Should find tombstoned host2")
	require.NotNil(t, foundHost2)
	assert.True(t, foundHost2.ReporterResources()[0].Serialize().Tombstone, "Host2 should be tombstoned")
}

func createTestReportRequestWithUpdatedData(t *testing.T, resourceType, reporterType, reporterInstance, localResourceId, workspaceId string) *v1beta2.ReportResourceRequest {
	reporterData, _ := structpb.NewStruct(map[string]interface{}{
		"local_resource_id": localResourceId,
		"api_href":          "https://api.example.com/updated/123",
		"console_href":      "https://console.example.com/updated/123",
		"hostname":          "updated-hostname",
		"status":            "running",
	})

	commonData, _ := structpb.NewStruct(map[string]interface{}{
		"workspace_id": workspaceId,
		"name":         "updated-host",
		"environment":  "production",
		"tags":         map[string]interface{}{"env": "prod", "updated": "true"},
	})

	return &v1beta2.ReportResourceRequest{
		Type:               resourceType,
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstance,
		Representations: &v1beta2.ResourceRepresentations{
			Metadata: &v1beta2.RepresentationMetadata{
				LocalResourceId: localResourceId,
				ApiHref:         "https://api.example.com/updated/123",
				ConsoleHref:     internal.StringPtr("https://console.example.com/updated/123"),
			},
			Reporter: reporterData,
			Common:   commonData,
		},
		WriteVisibility: v1beta2.WriteVisibility_MINIMIZE_LATENCY,
	}
}

func TestPartialDataScenarios(t *testing.T) {
	ctx := context.Background()
	logger := log.DefaultLogger

	resourceRepo := data.NewFakeResourceRepository()
	authorizer := &allow.AllowAllAuthz{}
	usecaseConfig := &UsecaseConfig{
		ReadAfterWriteEnabled: false,
		ConsumerEnabled:       false,
	}

	usecase := New(resourceRepo, nil, nil, authorizer, nil, "test-topic", logger, nil, nil, usecaseConfig)

	t.Run("Report resource with rich reporter data and minimal common data", func(t *testing.T) {
		request := createTestReportRequestWithReporterDataOnly(t, "k8s_cluster", "ocm", "ocm-instance-1", "reporter-rich-resource", "minimal-workspace")
		err := usecase.ReportResource(ctx, request, "test-reporter")
		require.NoError(t, err, "Should create resource with rich reporter data")

		key, err := model.NewReporterResourceKey("reporter-rich-resource", "k8s_cluster", "ocm", "ocm-instance-1")
		require.NoError(t, err)

		foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource with rich reporter data")
		require.NotNil(t, foundResource)
	})

	t.Run("Report resource with minimal reporter data and rich common data", func(t *testing.T) {
		request := createTestReportRequestWithCommonDataOnly(t, "k8s_cluster", "ocm", "ocm-instance-1", "common-rich-resource", "rich-workspace")
		err := usecase.ReportResource(ctx, request, "test-reporter")
		require.NoError(t, err, "Should create resource with rich common data")

		key, err := model.NewReporterResourceKey("common-rich-resource", "k8s_cluster", "ocm", "ocm-instance-1")
		require.NoError(t, err)

		foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource with rich common data")
		require.NotNil(t, foundResource)
	})

	t.Run("Report resource with both data, then reporter-focused update, then common-focused update", func(t *testing.T) {
		// 1. Initial report with both reporter and common data
		initialRequest := createTestReportRequest(t, "k8s_cluster", "ocm", "ocm-instance-1", "progressive-resource", "initial-workspace")
		err := usecase.ReportResource(ctx, initialRequest, "test-reporter")
		require.NoError(t, err, "Should create resource with both data types")

		key, err := model.NewReporterResourceKey("progressive-resource", "k8s_cluster", "ocm", "ocm-instance-1")
		require.NoError(t, err)

		foundResource, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after initial creation")
		require.NotNil(t, foundResource)

		// 2. Reporter-focused update
		reporterFocusedRequest := createTestReportRequestWithReporterDataOnly(t, "k8s_cluster", "ocm", "ocm-instance-1", "progressive-resource", "initial-workspace")
		err = usecase.ReportResource(ctx, reporterFocusedRequest, "test-reporter")
		require.NoError(t, err, "Should update resource with reporter-focused data")

		foundResource, err = resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after reporter-focused update")
		require.NotNil(t, foundResource)

		// 3. Common-focused update
		commonFocusedRequest := createTestReportRequestWithCommonDataOnly(t, "k8s_cluster", "ocm", "ocm-instance-1", "progressive-resource", "updated-workspace")
		err = usecase.ReportResource(ctx, commonFocusedRequest, "test-reporter")
		require.NoError(t, err, "Should update resource with common-focused data")

		finalResource, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after all updates")
		require.NotNil(t, finalResource)
	})
}

func createTestReportRequestWithReporterDataOnly(t *testing.T, resourceType, reporterType, reporterInstance, localResourceId, workspaceId string) *v1beta2.ReportResourceRequest {
	// Rich reporter data
	reporterData, _ := structpb.NewStruct(map[string]interface{}{
		"local_resource_id":  localResourceId,
		"api_href":           "https://api.example.com/reporter-rich",
		"console_href":       "https://console.example.com/reporter-rich",
		"cluster_name":       "reporter-focused-cluster",
		"cluster_version":    "1.28.0",
		"node_count":         10,
		"cpu_total":          "40 cores",
		"memory_total":       "160Gi",
		"storage_total":      "1Ti",
		"network_plugin":     "calico",
		"ingress_controller": "nginx",
	})

	// Minimal common data
	commonData, _ := structpb.NewStruct(map[string]interface{}{
		"workspace_id": workspaceId,
	})

	return &v1beta2.ReportResourceRequest{
		Type:               resourceType,
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstance,
		Representations: &v1beta2.ResourceRepresentations{
			Metadata: &v1beta2.RepresentationMetadata{
				LocalResourceId: localResourceId,
				ApiHref:         "https://api.example.com/reporter-rich",
				ConsoleHref:     internal.StringPtr("https://console.example.com/reporter-rich"),
			},
			Reporter: reporterData,
			Common:   commonData,
		},
		WriteVisibility: v1beta2.WriteVisibility_MINIMIZE_LATENCY,
	}
}

func createTestReportRequestWithCommonDataOnly(t *testing.T, resourceType, reporterType, reporterInstance, localResourceId, workspaceId string) *v1beta2.ReportResourceRequest {
	// Minimal reporter data
	reporterData, _ := structpb.NewStruct(map[string]interface{}{
		"local_resource_id": localResourceId,
		"api_href":          "https://api.example.com/common-rich",
		"console_href":      "https://console.example.com/common-rich",
		"name":              "minimal-cluster",
	})

	// Rich common data
	commonData, _ := structpb.NewStruct(map[string]interface{}{
		"workspace_id": workspaceId,
		"environment":  "production",
		"region":       "us-east-1",
		"cost_center":  "engineering",
		"owner":        "platform-team",
		"project":      "inventory-system",
		"labels": map[string]interface{}{
			"env":        "prod",
			"team":       "platform",
			"monitoring": "enabled",
			"backup":     "daily",
			"tier":       "critical",
		},
		"compliance": map[string]interface{}{
			"sox":   "required",
			"hipaa": "not-applicable",
			"gdpr":  "compliant",
		},
		"security": map[string]interface{}{
			"encryption": "enabled",
			"scanning":   "continuous",
			"access":     "restricted",
		},
	})

	return &v1beta2.ReportResourceRequest{
		Type:               resourceType,
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstance,
		Representations: &v1beta2.ResourceRepresentations{
			Metadata: &v1beta2.RepresentationMetadata{
				LocalResourceId: localResourceId,
				ApiHref:         "https://api.example.com/common-rich",
				ConsoleHref:     internal.StringPtr("https://console.example.com/common-rich"),
			},
			Reporter: reporterData,
			Common:   commonData,
		},
		WriteVisibility: v1beta2.WriteVisibility_MINIMIZE_LATENCY,
	}
}
func TestResourceLifecycle_ReportUpdateDeleteReport(t *testing.T) {
	t.Run("report new -> update -> delete -> report new", func(t *testing.T) {
		ctx := context.Background()
		logger := log.DefaultLogger

		resourceRepo := data.NewFakeResourceRepository()
		authorizer := &allow.AllowAllAuthz{}
		usecaseConfig := &UsecaseConfig{
			ReadAfterWriteEnabled: false,
			ConsumerEnabled:       false,
		}

		usecase := New(resourceRepo, nil, nil, authorizer, nil, "test-topic", logger, nil, nil, usecaseConfig)

		resourceType := "host"
		reporterType := "hbi"
		reporterInstance := "test-instance"
		localResourceId := "lifecycle-test-host"
		workspaceId := "test-workspace"

		// 1. REPORT NEW: Initial resource creation
		log.Info("Report New ---------------------")
		reportRequest1 := createTestReportRequest(t, resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := usecase.ReportResource(ctx, reportRequest1, "test-reporter")
		require.NoError(t, err, "Initial report should succeed")

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Verify initial state: generation = 0, representationVersion = 0
		afterCreate, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after creation")
		require.NotNil(t, afterCreate)
		initialGeneration := afterCreate.ReporterResources()[0].Serialize().Generation
		initialRepVersion := afterCreate.ReporterResources()[0].Serialize().RepresentationVersion
		initialTombstone := afterCreate.ReporterResources()[0].Serialize().Tombstone
		assert.Equal(t, uint(0), initialGeneration, "Initial generation should be 0")
		assert.Equal(t, uint(0), initialRepVersion, "Initial representationVersion should be 0")
		assert.False(t, initialTombstone, "Initial tombstone should be false")

		log.Info("Update 1 ---------------------")
		// 2. UPDATE: Update the resource
		reportRequest2 := createTestReportRequestWithUpdatedData(t, resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = usecase.ReportResource(ctx, reportRequest2, "test-reporter")
		require.NoError(t, err, "Update should succeed")

		// Verify state after update: representationVersion incremented, generation unchanged
		afterUpdate, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after update")
		require.NotNil(t, afterUpdate)
		updateGeneration := afterUpdate.ReporterResources()[0].Serialize().Generation
		updateRepVersion := afterUpdate.ReporterResources()[0].Serialize().RepresentationVersion
		updateTombstone := afterUpdate.ReporterResources()[0].Serialize().Tombstone
		assert.Equal(t, uint(0), updateGeneration, "Generation should remain 0 after update (tombstone=false)")
		assert.Equal(t, uint(1), updateRepVersion, "RepresentationVersion should increment to 1 after update")
		assert.False(t, updateTombstone, "Tombstone should remain false after update")

		// 3. DELETE: Delete the resource
		log.Info("Delete ---------------------")
		err = usecase.Delete(key)
		require.NoError(t, err, "Delete should succeed")

		// Verify state after delete: representationVersion incremented, generation unchanged, tombstoned
		afterDelete, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete)
		deleteGeneration := afterDelete.ReporterResources()[0].Serialize().Generation
		deleteRepVersion := afterDelete.ReporterResources()[0].Serialize().RepresentationVersion
		deleteTombstone := afterDelete.ReporterResources()[0].Serialize().Tombstone
		assert.Equal(t, uint(0), deleteGeneration, "Generation should remain 0 after delete")
		assert.Equal(t, uint(2), deleteRepVersion, "RepresentationVersion should increment to 2 after delete")
		assert.True(t, deleteTombstone, "Resource should be tombstoned after delete")

		// 4. REPORT NEW: Report the same resource again after deletion (this should be an update)
		log.Info("Revive again ---------------------")
		reportRequest3 := createTestReportRequestWithUpdatedData(t, resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = usecase.ReportResource(ctx, reportRequest3, "test-reporter")
		require.NoError(t, err, "Report after delete should succeed")

		// Verify final state after update on tombstoned resource
		afterRevive, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after revival")
		require.NotNil(t, afterRevive)
		reviveGeneration := afterRevive.ReporterResources()[0].Serialize().Generation
		reviveRepVersion := afterRevive.ReporterResources()[0].Serialize().RepresentationVersion
		reviveTombstone := afterRevive.ReporterResources()[0].Serialize().Tombstone
		assert.Equal(t, uint(1), reviveGeneration, "Generation should increment to 1 after update on tombstoned resource")
		assert.Equal(t, uint(0), reviveRepVersion, "RepresentationVersion should start fresh at 0 for revival (new generation)")
		assert.False(t, reviveTombstone, "Resource should no longer be tombstoned after revival update")
	})
}

func TestResourceLifecycle_ReportUpdateDeleteReportDelete(t *testing.T) {
	t.Run("report new -> update -> delete -> report new -> delete", func(t *testing.T) {
		ctx := context.Background()
		logger := log.DefaultLogger

		resourceRepo := data.NewFakeResourceRepository()
		authorizer := &allow.AllowAllAuthz{}
		usecaseConfig := &UsecaseConfig{
			ReadAfterWriteEnabled: false,
			ConsumerEnabled:       false,
		}

		usecase := New(resourceRepo, nil, nil, authorizer, nil, "test-topic", logger, nil, nil, usecaseConfig)

		resourceType := "k8s_cluster"
		reporterType := "ocm"
		reporterInstance := "ocm-instance"
		localResourceId := "lifecycle-test-cluster"
		workspaceId := "test-workspace-2"

		// 1. REPORT NEW: Initial resource creation
		reportRequest1 := createTestReportRequest(t, resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := usecase.ReportResource(ctx, reportRequest1, "test-reporter")
		require.NoError(t, err, "Initial report should succeed")

		// 2. UPDATE: Update the resource
		reportRequest2 := createTestReportRequestWithUpdatedData(t, resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = usecase.ReportResource(ctx, reportRequest2, "test-reporter")
		require.NoError(t, err, "Update should succeed")

		// 3. DELETE: Delete the resource
		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)
		err = usecase.Delete(key)
		require.NoError(t, err, "First delete should succeed")

		// 4. REPORT NEW: Report the same resource again after deletion
		reportRequest3 := createTestReportRequestWithUpdatedData(t, resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = usecase.ReportResource(ctx, reportRequest3, "test-reporter")
		require.NoError(t, err, "Report after delete should succeed")

		// 5. DELETE: Delete the recreated resource
		err = usecase.Delete(key)
		require.NoError(t, err, "Second delete should succeed")

		// Verify final state - resource should be tombstoned
		finalResource, err := resourceRepo.FindResourceByKeys(nil, key)
		if err == gorm.ErrRecordNotFound {
			// This is expected with current tombstone filtering
			assert.Nil(t, finalResource, "Resource should not be found if tombstone filter is active")
		} else {
			// If tombstone filter is removed, we should find the resource
			require.NoError(t, err, "Should find tombstoned resource if filter is removed")
			require.NotNil(t, finalResource)
		}
	})
}

func createReporterResourceKey(t *testing.T, localResourceId, resourceType, reporterType, reporterInstance string) model.ReporterResourceKey {
	localResourceIdType, err := model.NewLocalResourceId(localResourceId)
	require.NoError(t, err)
	resourceTypeType, err := model.NewResourceType(resourceType)
	require.NoError(t, err)
	reporterTypeType, err := model.NewReporterType(reporterType)
	require.NoError(t, err)
	reporterInstanceIdType, err := model.NewReporterInstanceId(reporterInstance)
	require.NoError(t, err)

	key, err := model.NewReporterResourceKey(localResourceIdType, resourceTypeType, reporterTypeType, reporterInstanceIdType)
	require.NoError(t, err)
	return key
}

func TestResourceLifecycle_ReportDeleteResubmitDelete(t *testing.T) {
	t.Run("report -> delete -> resubmit same delete", func(t *testing.T) {
		ctx := context.Background()
		logger := log.DefaultLogger

		resourceRepo := data.NewFakeResourceRepository()
		authorizer := &allow.AllowAllAuthz{}
		usecaseConfig := &UsecaseConfig{
			ReadAfterWriteEnabled: false,
			ConsumerEnabled:       false,
		}

		usecase := New(resourceRepo, nil, nil, authorizer, nil, "test-topic", logger, nil, nil, usecaseConfig)

		resourceType := "k8s_cluster"
		reporterType := "ocm"
		reporterInstance := "idempotent-instance"
		localResourceId := "idempotent-test-resource"
		workspaceId := "idempotent-workspace"

		// 1. REPORT: Initial resource creation
		log.Info("1. Initial Report ---------------------")
		reportRequest1 := createTestReportRequest(t, resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := usecase.ReportResource(ctx, reportRequest1, "test-reporter")
		require.NoError(t, err, "Initial report should succeed")

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Verify initial state
		afterReport1, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after initial report")
		require.NotNil(t, afterReport1)
		initialState := afterReport1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(0), initialState.RepresentationVersion, "Initial representationVersion should be 0")
		assert.Equal(t, uint(0), initialState.Generation, "Initial generation should be 0")
		assert.False(t, initialState.Tombstone, "Initial tombstone should be false")

		// 2. DELETE: Delete the resource
		log.Info("2. Delete ---------------------")
		err = usecase.Delete(key)
		require.NoError(t, err, "Delete should succeed")

		// Verify delete state
		afterDelete1, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete1)
		deleteState1 := afterDelete1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(1), deleteState1.RepresentationVersion, "RepresentationVersion should increment after delete")
		assert.Equal(t, uint(0), deleteState1.Generation, "Generation should remain 0 after delete")
		assert.True(t, deleteState1.Tombstone, "Resource should be tombstoned")

		// 3. RESUBMIT SAME DELETE: Should be idempotent
		log.Info("3. Resubmit Delete ---------------------")
		err = usecase.Delete(key)
		require.NoError(t, err, "Resubmitted delete should succeed (idempotent)")

		// Verify state after duplicate delete (operations are idempotent - no changes for tombstoned resources)
		afterDelete2, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should still find tombstoned resource")
		require.NotNil(t, afterDelete2)
		deleteState2 := afterDelete2.ReporterResources()[0].Serialize()
		assert.Equal(t, deleteState1.RepresentationVersion, deleteState2.RepresentationVersion, "RepresentationVersion should remain unchanged for duplicate delete on tombstoned resource")
		assert.Equal(t, deleteState1.Generation, deleteState2.Generation, "Generation should be unchanged by duplicate delete")
		assert.True(t, deleteState2.Tombstone, "Resource should still be tombstoned")

	})
}

func TestResourceLifecycle_ReportResubmitDeleteResubmit(t *testing.T) {
	t.Run("report -> resubmit same report -> delete -> resubmit same delete", func(t *testing.T) {
		ctx := context.Background()
		logger := log.DefaultLogger

		resourceRepo := data.NewFakeResourceRepository()
		authorizer := &allow.AllowAllAuthz{}
		usecaseConfig := &UsecaseConfig{
			ReadAfterWriteEnabled: false,
			ConsumerEnabled:       false,
		}

		usecase := New(resourceRepo, nil, nil, authorizer, nil, "test-topic", logger, nil, nil, usecaseConfig)

		resourceType := "host"
		reporterType := "hbi"
		reporterInstance := "idempotent-instance-2"
		localResourceId := "idempotent-test-resource-2"
		workspaceId := "idempotent-workspace-2"

		// 1. REPORT: Initial resource creation
		log.Info("1. Initial Report ---------------------")
		reportRequest1 := createTestReportRequest(t, resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err := usecase.ReportResource(ctx, reportRequest1, "test-reporter")
		require.NoError(t, err, "Initial report should succeed")

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Verify initial state
		afterReport1, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after initial report")
		require.NotNil(t, afterReport1)
		initialState := afterReport1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(0), initialState.RepresentationVersion, "Initial representationVersion should be 0")
		assert.Equal(t, uint(0), initialState.Generation, "Initial generation should be 0")
		assert.False(t, initialState.Tombstone, "Initial tombstone should be false")

		// 2. RESUBMIT SAME REPORT: Should be idempotent
		log.Info("2. Resubmit Same Report ---------------------")
		reportRequest2 := createTestReportRequest(t, resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
		err = usecase.ReportResource(ctx, reportRequest2, "test-reporter")
		require.NoError(t, err, "Resubmitted report should succeed (idempotent)")

		// Verify state after duplicate report (should increment representation version)
		afterReport2, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find resource after duplicate report")
		require.NotNil(t, afterReport2)
		duplicateState := afterReport2.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(1), duplicateState.RepresentationVersion, "RepresentationVersion should increment after duplicate report")
		assert.Equal(t, uint(0), duplicateState.Generation, "Generation should remain 0")
		assert.False(t, duplicateState.Tombstone, "Resource should remain active")

		// 3. DELETE: Delete the resource
		log.Info("3. Delete ---------------------")
		err = usecase.Delete(key)
		require.NoError(t, err, "Delete should succeed")

		// Verify delete state
		afterDelete1, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find tombstoned resource after delete")
		require.NotNil(t, afterDelete1)
		deleteState1 := afterDelete1.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(2), deleteState1.RepresentationVersion, "RepresentationVersion should increment after delete")
		assert.Equal(t, uint(0), deleteState1.Generation, "Generation should remain 0 after delete")
		assert.True(t, deleteState1.Tombstone, "Resource should be tombstoned")

		// 4. RESUBMIT SAME DELETE: Should be idempotent
		log.Info("4. Resubmit Delete ---------------------")
		err = usecase.Delete(key)
		require.NoError(t, err, "Resubmitted delete should succeed (idempotent)")

		// Verify final state after duplicate delete (operations are idempotent - no changes for tombstoned resources)
		afterDelete2, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should still find tombstoned resource")
		require.NotNil(t, afterDelete2)
		finalDeleteState := afterDelete2.ReporterResources()[0].Serialize()
		assert.Equal(t, deleteState1.RepresentationVersion, finalDeleteState.RepresentationVersion, "RepresentationVersion should remain unchanged for duplicate delete on tombstoned resource")
		assert.Equal(t, deleteState1.Generation, finalDeleteState.Generation, "Generation should be unchanged by duplicate delete")
		assert.True(t, finalDeleteState.Tombstone, "Resource should still be tombstoned")
	})
}

func TestResourceLifecycle_ComplexIdempotency(t *testing.T) {
	t.Run("3 cycles of create+update+delete for same resource", func(t *testing.T) {
		ctx := context.Background()
		logger := log.DefaultLogger

		resourceRepo := data.NewFakeResourceRepository()
		authorizer := &allow.AllowAllAuthz{}
		usecaseConfig := &UsecaseConfig{
			ReadAfterWriteEnabled: false,
			ConsumerEnabled:       false,
		}

		usecase := New(resourceRepo, nil, nil, authorizer, nil, "test-topic", logger, nil, nil, usecaseConfig)

		resourceType := "k8s_cluster"
		reporterType := "ocm"
		reporterInstance := "complex-idempotent-instance"
		localResourceId := "complex-idempotent-resource"
		workspaceId := "complex-idempotent-workspace"

		key := createReporterResourceKey(t, localResourceId, resourceType, reporterType, reporterInstance)

		// Run 3 cycles of create+update+delete
		for cycle := 0; cycle < 3; cycle++ {
			t.Logf("=== Cycle %d: Create+Update+Delete ===", cycle)

			// 1. REPORT (CREATE or UPDATE): Should find existing or create new
			log.Infof("Cycle %d: Report Resource", cycle)
			reportRequest := createTestReportRequestWithCycleData(t, resourceType, reporterType, reporterInstance, localResourceId, workspaceId, cycle)
			err := usecase.ReportResource(ctx, reportRequest, "test-reporter")
			require.NoError(t, err, "Report should succeed in cycle %d", cycle)

			// Verify state after report
			afterReport, err := resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err, "Should find resource after report in cycle %d", cycle)
			require.NotNil(t, afterReport)
			reportState := afterReport.ReporterResources()[0].Serialize()

			expectedGeneration := uint(cycle) // Generation should be 0, 1, 2 for cycles 0, 1, 2
			assert.Equal(t, expectedGeneration, reportState.Generation, "Generation should be %d in cycle %d", expectedGeneration, cycle)
			assert.Equal(t, uint(0), reportState.RepresentationVersion, "RepresentationVersion should reset to 0 for new generation in cycle %d", cycle)
			assert.False(t, reportState.Tombstone, "Resource should be active after report in cycle %d", cycle)

			// 2. UPDATE: Update the resource
			log.Infof("Cycle %d: Update Resource", cycle)
			updateRequest := createTestReportRequestWithUpdatedData(t, resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
			err = usecase.ReportResource(ctx, updateRequest, "test-reporter")
			require.NoError(t, err, "Update should succeed in cycle %d", cycle)

			// Verify state after update
			afterUpdate, err := resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err, "Should find resource after update in cycle %d", cycle)
			require.NotNil(t, afterUpdate)
			updateState := afterUpdate.ReporterResources()[0].Serialize()
			assert.Equal(t, expectedGeneration, updateState.Generation, "Generation should remain %d after update in cycle %d", expectedGeneration, cycle)
			assert.Equal(t, uint(1), updateState.RepresentationVersion, "RepresentationVersion should increment to 1 after update in cycle %d", cycle)
			assert.False(t, updateState.Tombstone, "Resource should remain active after update in cycle %d", cycle)

			// 3. DELETE: Delete the resource
			log.Infof("Cycle %d: Delete Resource", cycle)
			err = usecase.Delete(key)
			require.NoError(t, err, "Delete should succeed in cycle %d", cycle)

			// Verify state after delete
			afterDelete, err := resourceRepo.FindResourceByKeys(nil, key)
			require.NoError(t, err, "Should find tombstoned resource after delete in cycle %d", cycle)
			require.NotNil(t, afterDelete)
			deleteState := afterDelete.ReporterResources()[0].Serialize()
			assert.Equal(t, expectedGeneration, deleteState.Generation, "Generation should remain %d after delete in cycle %d", expectedGeneration, cycle)
			assert.Equal(t, uint(2), deleteState.RepresentationVersion, "RepresentationVersion should increment to 2 after delete in cycle %d", cycle)
			assert.True(t, deleteState.Tombstone, "Resource should be tombstoned after delete in cycle %d", cycle)

			t.Logf("Cycle %d complete: Final state {Generation: %d, RepVersion: %d, Tombstone: %t}",
				cycle, deleteState.Generation, deleteState.RepresentationVersion, deleteState.Tombstone)
		}

		// Final verification: Resource should be in generation 2 after 3 cycles
		finalResource, err := resourceRepo.FindResourceByKeys(nil, key)
		require.NoError(t, err, "Should find final resource")
		require.NotNil(t, finalResource)
		finalState := finalResource.ReporterResources()[0].Serialize()
		assert.Equal(t, uint(2), finalState.Generation, "Final generation should be 2 after 3 cycles")
		assert.True(t, finalState.Tombstone, "Final resource should be tombstoned")
	})
}

func createTestReportRequestWithCycleData(t *testing.T, resourceType, reporterType, reporterInstance, localResourceId, workspaceId string, cycle int) *v1beta2.ReportResourceRequest {
	reporterData, _ := structpb.NewStruct(map[string]interface{}{
		"local_resource_id": localResourceId,
		"api_href":          fmt.Sprintf("https://api.example.com/cycle-%d", cycle),
		"console_href":      fmt.Sprintf("https://console.example.com/cycle-%d", cycle),
		"cycle":             cycle,
	})

	commonData, _ := structpb.NewStruct(map[string]interface{}{
		"workspace_id": workspaceId,
		"name":         fmt.Sprintf("test-cluster-cycle-%d", cycle),
		"namespace":    "default",
		"cycle":        cycle,
	})

	return &v1beta2.ReportResourceRequest{
		Type:               resourceType,
		ReporterType:       reporterType,
		ReporterInstanceId: reporterInstance,
		Representations: &v1beta2.ResourceRepresentations{
			Metadata: &v1beta2.RepresentationMetadata{
				LocalResourceId: localResourceId,
				ApiHref:         fmt.Sprintf("https://api.example.com/cycle-%d", cycle),
				ConsoleHref:     internal.StringPtr(fmt.Sprintf("https://console.example.com/cycle-%d", cycle)),
			},
			Reporter: reporterData,
			Common:   commonData,
		},
		WriteVisibility: v1beta2.WriteVisibility_MINIMIZE_LATENCY,
	}
}
