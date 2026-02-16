package resources

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// fixture returns a ReportResourceCommandFixture for creating test commands.
func fixture() ReportResourceCommandFixture {
	return ReportResourceCommandFixture{}
}

// ReportResourceCommandFixture provides test fixtures for ReportResourceCommand.
type ReportResourceCommandFixture struct{}

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
	localResId, _ := model.NewLocalResourceId(localResourceId)
	resType, _ := model.NewResourceType(resourceType)
	repType, _ := model.NewReporterType(reporterType)
	repInstanceId, _ := model.NewReporterInstanceId(reporterInstance)
	apiHref, _ := model.NewApiHref("https://api.example.com/resource/123")
	consoleHref, _ := model.NewConsoleHref("https://console.example.com/resource/123")
	reporterRep, _ := model.NewRepresentation(reporterData)
	commonRep, _ := model.NewRepresentation(commonData)

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
