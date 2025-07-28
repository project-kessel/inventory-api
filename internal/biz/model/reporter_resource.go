package model

import "github.com/google/uuid"

type ReporterResource struct {
	// Surrogate Id for ReporterResourceKey
	id uuid.UUID
	// Actual Id
	ReporterResourceKey

	// Fields that do not need versioning, only latest state matters
	resourceID  uuid.UUID
	apiHref     string
	consoleHref string

	// Normalized Latest values
	representationVersion uint
	generation            uint
	tombstone             bool
}

type ReporterResourceKey struct {
	localResourceID    string
	reporterType       string
	resourceType       string
	reporterInstanceID string
}
