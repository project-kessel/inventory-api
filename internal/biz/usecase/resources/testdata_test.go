package resources

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/require"
)

// fixture returns a ReportResourceCommandFixture for creating test commands.
func fixture(t *testing.T) ReportResourceCommandFixture {
	t.Helper()
	return ReportResourceCommandFixture{t: t}
}

// ReportResourceCommandFixture provides test fixtures for ReportResourceCommand.
type ReportResourceCommandFixture struct {
	t *testing.T
}

// Basic creates a basic ReportResourceCommand with default values.
func (f ReportResourceCommandFixture) Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId string) ReportResourceCommand {
	return f.WithData(resourceType, reporterType, reporterInstance, localResourceId,
		map[string]interface{}{
			"local_resource_id": localResourceId,
		},
		map[string]interface{}{
			"workspace_id": workspaceId,
		},
	)
}

// WithData creates a ReportResourceCommand with custom reporter and common data.
func (f ReportResourceCommandFixture) WithData(resourceType, reporterType, reporterInstance, localResourceId string, reporterData, commonData map[string]interface{}) ReportResourceCommand {
	f.t.Helper()

	localResId, err := model.NewLocalResourceId(localResourceId)
	require.NoError(f.t, err)
	resType, err := model.NewResourceType(resourceType)
	require.NoError(f.t, err)
	repType, err := model.NewReporterType(reporterType)
	require.NoError(f.t, err)
	repInstanceId, err := model.NewReporterInstanceId(reporterInstance)
	require.NoError(f.t, err)
	apiHref, err := model.NewApiHref("https://api.example.com/resource/123")
	require.NoError(f.t, err)
	consoleHref, err := model.NewConsoleHref("https://console.example.com/resource/123")
	require.NoError(f.t, err)
	reporterRep, err := model.NewRepresentation(reporterData)
	require.NoError(f.t, err)
	commonRep, err := model.NewRepresentation(commonData)
	require.NoError(f.t, err)

	return ReportResourceCommand{
		LocalResourceId:        localResId,
		ResourceType:           resType,
		ReporterType:           repType,
		ReporterInstanceId:     repInstanceId,
		ApiHref:                apiHref,
		ConsoleHref:            &consoleHref,
		ReporterRepresentation: reporterRep,
		CommonRepresentation:   commonRep,
		WriteVisibility:        WriteVisibilityMinimizeLatency,
	}
}

// WithTransactionId creates a ReportResourceCommand with a transaction ID.
func (f ReportResourceCommandFixture) WithTransactionId(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, transactionId string) ReportResourceCommand {
	cmd := f.Basic(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
	txId := model.NewTransactionId(transactionId)
	cmd.TransactionId = &txId
	return cmd
}

// Updated creates a ReportResourceCommand representing an update with modified data.
func (f ReportResourceCommandFixture) Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId string) ReportResourceCommand {
	return f.WithData(resourceType, reporterType, reporterInstance, localResourceId,
		map[string]interface{}{
			"hostname": "updated-hostname",
			"status":   "running",
		},
		map[string]interface{}{
			"workspace_id": workspaceId,
			"name":         "updated-resource",
			"environment":  "production",
		},
	)
}

// ReporterRich creates a ReportResourceCommand with rich reporter data and minimal common data.
func (f ReportResourceCommandFixture) ReporterRich(resourceType, reporterType, reporterInstance, localResourceId, workspaceId string) ReportResourceCommand {
	return f.WithData(resourceType, reporterType, reporterInstance, localResourceId,
		map[string]interface{}{
			"cluster_name":    "reporter-focused-cluster",
			"cluster_version": "1.28.0",
			"node_count":      10,
			"cpu_total":       "40 cores",
			"memory_total":    "160Gi",
		},
		map[string]interface{}{
			"workspace_id": workspaceId,
		},
	)
}

// CommonRich creates a ReportResourceCommand with minimal reporter data and rich common data.
func (f ReportResourceCommandFixture) CommonRich(resourceType, reporterType, reporterInstance, localResourceId, workspaceId string) ReportResourceCommand {
	return f.WithData(resourceType, reporterType, reporterInstance, localResourceId,
		map[string]interface{}{
			"name": "minimal-cluster",
		},
		map[string]interface{}{
			"workspace_id": workspaceId,
			"environment":  "production",
			"region":       "us-east-1",
			"cost_center":  "engineering",
			"owner":        "platform-team",
		},
	)
}

// WithCycleData creates a ReportResourceCommand with cycle-specific data for testing iterations.
func (f ReportResourceCommandFixture) WithCycleData(resourceType, reporterType, reporterInstance, localResourceId, workspaceId string, cycle int) ReportResourceCommand {
	return f.WithData(resourceType, reporterType, reporterInstance, localResourceId,
		map[string]interface{}{
			"cycle": cycle,
		},
		map[string]interface{}{
			"workspace_id": workspaceId,
			"cycle":        cycle,
		},
	)
}

// UpdatedWithTransactionId creates an updated ReportResourceCommand with a transaction ID.
func (f ReportResourceCommandFixture) UpdatedWithTransactionId(resourceType, reporterType, reporterInstance, localResourceId, workspaceId, transactionId string) ReportResourceCommand {
	cmd := f.Updated(resourceType, reporterType, reporterInstance, localResourceId, workspaceId)
	txId := model.NewTransactionId(transactionId)
	cmd.TransactionId = &txId
	return cmd
}
