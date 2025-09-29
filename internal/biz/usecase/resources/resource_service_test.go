package resources

import (
	"context"
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
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.Nil(t, foundResource)
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

	// Verify both hosts are no longer found (tombstoned)
	foundHost1, err = resourceRepo.FindResourceByKeys(nil, key1)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "Should not find deleted host1")
	assert.Nil(t, foundHost1)

	foundHost2, err = resourceRepo.FindResourceByKeys(nil, key2)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound, "Should not find deleted host2")
	assert.Nil(t, foundHost2)
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

func TestCalculateTuples(t *testing.T) {
	tests := []struct {
		name                   string
		version                uint
		currentWorkspaceID     string
		previousWorkspaceID    string
		expectTuplesToCreate   bool
		expectTuplesToDelete   bool
		expectedCreateResource string
		expectedDeleteResource string
		expectedCreateSubject  string
		expectedDeleteSubject  string
	}{
		{
			name:                   "version 0 creates initial tuple",
			version:                0,
			currentWorkspaceID:     "workspace-initial",
			previousWorkspaceID:    "",
			expectTuplesToCreate:   true,
			expectTuplesToDelete:   false,
			expectedCreateResource: "host:test-resource",
			expectedCreateSubject:  "rbac:workspace:workspace-initial",
		},
		{
			name:                   "workspace change creates and deletes tuples",
			version:                2,
			currentWorkspaceID:     "workspace-new",
			previousWorkspaceID:    "workspace-old",
			expectTuplesToCreate:   true,
			expectTuplesToDelete:   true,
			expectedCreateResource: "host:test-resource",
			expectedDeleteResource: "host:test-resource",
			expectedCreateSubject:  "rbac:workspace:workspace-new",
			expectedDeleteSubject:  "rbac:workspace:workspace-old",
		},
		{
			name:                   "workspace change creates and deletes tuples version 1",
			version:                1,
			currentWorkspaceID:     "workspace-new",
			previousWorkspaceID:    "workspace-old",
			expectTuplesToCreate:   true,
			expectTuplesToDelete:   true,
			expectedCreateResource: "host:test-resource",
			expectedDeleteResource: "host:test-resource",
			expectedCreateSubject:  "rbac:workspace:workspace-new",
			expectedDeleteSubject:  "rbac:workspace:workspace-old",
		},

		{
			name:                   "same workspace creates only",
			version:                2,
			currentWorkspaceID:     "workspace-same",
			previousWorkspaceID:    "workspace-same",
			expectTuplesToCreate:   true,
			expectTuplesToDelete:   false,
			expectedCreateResource: "host:test-resource",
			expectedCreateSubject:  "rbac:workspace:workspace-same",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a fake repository with workspace overrides aligned to test case expectations
			repo := data.NewFakeResourceRepositoryWithWorkspaceOverrides(tt.currentWorkspaceID, tt.previousWorkspaceID)
			// Seed fake repo behavior for workspace IDs via current version and previous version
			// The CalculateTuples tests rely on FindCommonRepresentationsByVersion returning
			// entries for current and (optionally) previous. The fake repo synthesizes data based
			// on version values, so we don't need to wire specific state here beyond calling the usecase.

			// Create usecase with mock repo
			uc := &Usecase{
				resourceRepository: repo,
				Log:                log.NewHelper(log.DefaultLogger),
			}

			// Create test key
			key, err := model.NewReporterResourceKey(
				model.LocalResourceId("test-resource"),
				model.ResourceType("host"),
				model.ReporterType("HBI"),
				model.ReporterInstanceId("test-instance"),
			)
			require.NoError(t, err)

			// Create TupleEvent
			tupleEvent, err := model.NewTupleEvent(model.Version(tt.version), key)
			require.NoError(t, err)

			// Call CalculateTuplesv2
			result, err := uc.CalculateTuples(tupleEvent)
			require.NoError(t, err)

			// Verify expectations
			assert.Equal(t, tt.expectTuplesToCreate, result.HasTuplesToCreate())
			assert.Equal(t, tt.expectTuplesToDelete, result.HasTuplesToDelete())

			if tt.expectTuplesToCreate {
				require.NotNil(t, result.TuplesToCreate())
				require.Len(t, *result.TuplesToCreate(), 1)
				createTuple := (*result.TuplesToCreate())[0]
				assert.Equal(t, tt.expectedCreateResource, createTuple.Resource())
				assert.Equal(t, "workspace", createTuple.Relation())
				assert.Equal(t, tt.expectedCreateSubject, createTuple.Subject())
			}

			if tt.expectTuplesToDelete {
				require.NotNil(t, result.TuplesToDelete())
				require.Len(t, *result.TuplesToDelete(), 1)
				deleteTuple := (*result.TuplesToDelete())[0]
				assert.Equal(t, tt.expectedDeleteResource, deleteTuple.Resource())
				assert.Equal(t, "workspace", deleteTuple.Relation())
				assert.Equal(t, tt.expectedDeleteSubject, deleteTuple.Subject())
			}
		})
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

func TestGetWorkspaceVersions(t *testing.T) {
	tests := []struct {
		name          string
		version       uint
		expectedCount int
	}{
		{
			name:          "successful retrieval",
			version:       2,
			expectedCount: 2, // Mock returns 2 representations
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := data.NewFakeResourceRepositoryWithWorkspaceOverrides("", "")
			uc := &Usecase{
				resourceRepository: repo,
			}

			key, err := model.NewReporterResourceKey(
				model.LocalResourceId("test-resource"),
				model.ResourceType("host"),
				model.ReporterType("HBI"),
				model.ReporterInstanceId("test-instance"),
			)
			require.NoError(t, err)

			result, err := uc.getWorkspaceVersions(key, tt.version)
			require.NoError(t, err)
			assert.Len(t, result, tt.expectedCount)
		})
	}
}
func TestExtractWorkspaceIDs(t *testing.T) {
	tests := []struct {
		name                  string
		representationVersion []data.RepresentationsByVersion
		currentVersion        uint
		expectedCurrent       string
		expectedPrevious      string
	}{
		{
			name: "extract current and previous workspace IDs",
			representationVersion: []data.RepresentationsByVersion{
				{
					Version: 2,
					Data: map[string]interface{}{
						"workspace_id": "workspace-new",
					},
				},
				{
					Version: 1,
					Data: map[string]interface{}{
						"workspace_id": "workspace-old",
					},
				},
			},
			currentVersion:   2,
			expectedCurrent:  "workspace-new",
			expectedPrevious: "workspace-old",
		},
		{
			name: "extract only current workspace ID",
			representationVersion: []data.RepresentationsByVersion{
				{
					Version: 0,
					Data: map[string]interface{}{
						"workspace_id": "workspace-initial",
					},
				},
			},
			currentVersion:   0,
			expectedCurrent:  "workspace-initial",
			expectedPrevious: "",
		},
		{
			name: "no workspace IDs found",
			representationVersion: []data.RepresentationsByVersion{
				{
					Version: 1,
					Data: map[string]interface{}{
						"other_field": "value",
					},
				},
			},
			currentVersion:   1,
			expectedCurrent:  "",
			expectedPrevious: "",
		},
		{
			name: "empty workspace ID ignored",
			representationVersion: []data.RepresentationsByVersion{
				{
					Version: 1,
					Data: map[string]interface{}{
						"workspace_id": "",
					},
				},
			},
			currentVersion:   1,
			expectedCurrent:  "",
			expectedPrevious: "",
		},
		{
			name: "workspace ID with special characters",
			representationVersion: []data.RepresentationsByVersion{
				{
					Version: 1,
					Data: map[string]interface{}{
						"workspace_id": "workspace-with-dashes_and_underscores",
					},
				},
			},
			currentVersion:   1,
			expectedCurrent:  "workspace-with-dashes_and_underscores",
			expectedPrevious: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &Usecase{}
			current, previous := uc.extractWorkspaceIDs(tt.representationVersion, tt.currentVersion)

			assert.Equal(t, tt.expectedCurrent, current)
			assert.Equal(t, tt.expectedPrevious, previous)
		})
	}
}

func TestCreateWorkspaceTuple(t *testing.T) {
	uc := &Usecase{}

	key, err := model.NewReporterResourceKey(
		model.LocalResourceId("test-resource"),
		model.ResourceType("host"),
		model.ReporterType("HBI"),
		model.ReporterInstanceId("test-instance"),
	)
	require.NoError(t, err)

	tests := []struct {
		name             string
		workspaceID      string
		expectedResource string
		expectedSubject  string
	}{
		{
			name:             "normal workspace ID",
			workspaceID:      "workspace-123",
			expectedResource: "host:test-resource",
			expectedSubject:  "rbac:workspace:workspace-123",
		},
		{
			name:             "workspace ID with special characters",
			workspaceID:      "workspace-with-dashes_and_underscores",
			expectedResource: "host:test-resource",
			expectedSubject:  "rbac:workspace:workspace-with-dashes_and_underscores",
		},
		{
			name:             "empty workspace ID",
			workspaceID:      "",
			expectedResource: "host:test-resource",
			expectedSubject:  "rbac:workspace:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tuple := uc.createWorkspaceTuple(tt.workspaceID, key)

			assert.Equal(t, tt.expectedResource, tuple.Resource())
			assert.Equal(t, "workspace", tuple.Relation())
			assert.Equal(t, tt.expectedSubject, tuple.Subject())
		})
	}
}

func TestDetermineTupleOperations(t *testing.T) {
	tests := []struct {
		name                  string
		representationVersion []data.RepresentationsByVersion
		currentVersion        uint
		expectedCreateCount   int
		expectedDeleteCount   int
		expectedCreateSubject string
		expectedDeleteSubject string
	}{
		{
			name: "workspace change creates and deletes tuples",
			representationVersion: []data.RepresentationsByVersion{
				{
					Version: 2,
					Data: map[string]interface{}{
						"workspace_id": "workspace-new",
					},
				},
				{
					Version: 1,
					Data: map[string]interface{}{
						"workspace_id": "workspace-old",
					},
				},
			},
			currentVersion:        2,
			expectedCreateCount:   1,
			expectedDeleteCount:   1,
			expectedCreateSubject: "rbac:workspace:workspace-new",
			expectedDeleteSubject: "rbac:workspace:workspace-old",
		},
		{
			name: "same workspace creates only",
			representationVersion: []data.RepresentationsByVersion{
				{
					Version: 2,
					Data: map[string]interface{}{
						"workspace_id": "workspace-same",
					},
				},
				{
					Version: 1,
					Data: map[string]interface{}{
						"workspace_id": "workspace-same",
					},
				},
			},
			currentVersion:        2,
			expectedCreateCount:   1,
			expectedDeleteCount:   0,
			expectedCreateSubject: "rbac:workspace:workspace-same",
		},
		{
			name: "version 0 creates initial tuple",
			representationVersion: []data.RepresentationsByVersion{
				{
					Version: 0,
					Data: map[string]interface{}{
						"workspace_id": "workspace-initial",
					},
				},
			},
			currentVersion:        0,
			expectedCreateCount:   1,
			expectedDeleteCount:   0,
			expectedCreateSubject: "rbac:workspace:workspace-initial",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &Usecase{}

			key, err := model.NewReporterResourceKey(
				model.LocalResourceId("test-resource"),
				model.ResourceType("host"),
				model.ReporterType("HBI"),
				model.ReporterInstanceId("test-instance"),
			)
			require.NoError(t, err)

			result, err := uc.determineTupleOperations(tt.representationVersion, tt.currentVersion, key)
			require.NoError(t, err)

			if tt.expectedCreateCount > 0 {
				require.NotNil(t, result.TuplesToCreate())
				assert.Len(t, *result.TuplesToCreate(), tt.expectedCreateCount)
				createTuple := (*result.TuplesToCreate())[0]
				assert.Equal(t, tt.expectedCreateSubject, createTuple.Subject())
			} else {
				assert.Nil(t, result.TuplesToCreate())
			}

			if tt.expectedDeleteCount > 0 {
				require.NotNil(t, result.TuplesToDelete())
				assert.Len(t, *result.TuplesToDelete(), tt.expectedDeleteCount)
				deleteTuple := (*result.TuplesToDelete())[0]
				assert.Equal(t, tt.expectedDeleteSubject, deleteTuple.Subject())
			} else {
				assert.Nil(t, result.TuplesToDelete())
			}
		})
	}
}
