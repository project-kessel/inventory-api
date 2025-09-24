package model

import (
	"fmt"

	"github.com/project-kessel/inventory-api/internal/biz"
)

type TupleEvent struct {
	reporterResourceKey           ReporterResourceKey
	operationType                 biz.EventOperationType
	commonVersion                 *Version
	reporterRepresentationVersion *Version
}

func NewTupleEvent(
	reporterResourceKey ReporterResourceKey,
	operationType biz.EventOperationType,
	commonVersion *Version,
	reporterRepresentationVersion *Version,
) (TupleEvent, error) {
	// Validate required fields
	if reporterResourceKey == (ReporterResourceKey{}) {
		return TupleEvent{}, fmt.Errorf("%w: reporterResourceKey", ErrEmpty)
	}

	if operationType == nil {
		return TupleEvent{}, fmt.Errorf("%w: operationType", ErrEmpty)
	}

	// Enforce invariant: at least one version must be present
	if commonVersion == nil && reporterRepresentationVersion == nil {
		return TupleEvent{}, fmt.Errorf("at least one version (commonVersion or reporterRepresentationVersion) must be present")
	}

	return TupleEvent{
		reporterResourceKey:           reporterResourceKey,
		operationType:                 operationType,
		commonVersion:                 commonVersion,
		reporterRepresentationVersion: reporterRepresentationVersion,
	}, nil
}

// Version returns the common version (for backward compatibility)
// Returns zero version if common version is not set
func (te TupleEvent) Version() Version {
	if te.commonVersion != nil {
		return *te.commonVersion
	}
	return NewVersion(0)
}

func (te TupleEvent) ReporterResourceKey() ReporterResourceKey {
	return te.reporterResourceKey
}

func (te TupleEvent) OperationType() biz.EventOperationType {
	return te.operationType
}

func (te TupleEvent) CommonVersion() *Version {
	return te.commonVersion
}

func (te TupleEvent) ReporterRepresentationVersion() *Version {
	return te.reporterRepresentationVersion
}
