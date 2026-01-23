package model

import "github.com/project-kessel/inventory-api/internal"

// WriteVisibility specifies how the system should handle write visibility guarantees.
type WriteVisibility int

const (
	// WriteVisibilityUnspecified indicates no specific write visibility requirement.
	WriteVisibilityUnspecified WriteVisibility = iota
	// WriteVisibilityMinimizeLatency prioritizes low latency over immediate visibility.
	WriteVisibilityMinimizeLatency
	// WriteVisibilityCommitPending waits for the write to be committed before returning.
	WriteVisibilityCommitPending
)

// ReportResourceCommand encapsulates all data needed to report a resource.
// This is a command object that represents the intent to create or update a resource.
type ReportResourceCommand struct {
	key                    ReporterResourceKey
	apiHref                ApiHref
	consoleHref            ConsoleHref
	reporterVersion        *ReporterVersion
	reporterRepresentation Representation
	commonRepresentation   Representation
	transactionId          TransactionId
	writeVisibility        WriteVisibility
}

// NewReportResourceCommand creates a new ReportResourceCommand with all required fields.
func NewReportResourceCommand(
	localResourceId LocalResourceId,
	resourceType ResourceType,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
	apiHref ApiHref,
	consoleHref ConsoleHref,
	reporterVersion *ReporterVersion,
	reporterRepresentation internal.JsonObject,
	commonRepresentation internal.JsonObject,
	transactionId TransactionId,
	writeVisibility WriteVisibility,
) (ReportResourceCommand, error) {
	key, err := NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
	if err != nil {
		return ReportResourceCommand{}, err
	}

	reporterRep, err := NewRepresentation(reporterRepresentation)
	if err != nil {
		return ReportResourceCommand{}, err
	}

	commonRep, err := NewRepresentation(commonRepresentation)
	if err != nil {
		return ReportResourceCommand{}, err
	}

	return ReportResourceCommand{
		key:                    key,
		apiHref:                apiHref,
		consoleHref:            consoleHref,
		reporterVersion:        reporterVersion,
		reporterRepresentation: reporterRep,
		commonRepresentation:   commonRep,
		transactionId:          transactionId,
		writeVisibility:        writeVisibility,
	}, nil
}

// Key returns the reporter resource key that uniquely identifies the resource.
func (c ReportResourceCommand) Key() ReporterResourceKey {
	return c.key
}

// ApiHref returns the API href for this resource.
func (c ReportResourceCommand) ApiHref() ApiHref {
	return c.apiHref
}

// ConsoleHref returns the console href for this resource (may be empty).
func (c ReportResourceCommand) ConsoleHref() ConsoleHref {
	return c.consoleHref
}

// ReporterVersion returns the reporter version, or nil if not specified.
func (c ReportResourceCommand) ReporterVersion() *ReporterVersion {
	return c.reporterVersion
}

// ReporterRepresentation returns the reporter-specific representation data.
func (c ReportResourceCommand) ReporterRepresentation() Representation {
	return c.reporterRepresentation
}

// CommonRepresentation returns the common representation data.
func (c ReportResourceCommand) CommonRepresentation() Representation {
	return c.commonRepresentation
}

// TransactionId returns the transaction ID for idempotency.
func (c ReportResourceCommand) TransactionId() TransactionId {
	return c.transactionId
}

// WriteVisibility returns the write visibility setting.
func (c ReportResourceCommand) WriteVisibility() WriteVisibility {
	return c.writeVisibility
}

// WantsCommitPending returns true if the caller wants to wait for commit confirmation.
func (c ReportResourceCommand) WantsCommitPending() bool {
	return c.writeVisibility == WriteVisibilityCommitPending
}
