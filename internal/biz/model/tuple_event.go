package model

import (
	"encoding/json"
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

// MarshalJSON implements json.Marshaler interface
func (te TupleEvent) MarshalJSON() ([]byte, error) {
	type tupleEventJSON struct {
		ReporterResourceKey           ReporterResourceKey    `json:"reporter_resource_key"`
		OperationType                 biz.EventOperationType `json:"operation_type"`
		CommonVersion                 *Version               `json:"common_version,omitempty"`
		ReporterRepresentationVersion *Version               `json:"reporter_representation_version,omitempty"`
	}

	return json.Marshal(tupleEventJSON{
		ReporterResourceKey:           te.reporterResourceKey,
		OperationType:                 te.operationType,
		CommonVersion:                 te.commonVersion,
		ReporterRepresentationVersion: te.reporterRepresentationVersion,
	})
}

// UnmarshalJSON implements json.Unmarshaler interface
func (te *TupleEvent) UnmarshalJSON(data []byte) error {
	type tupleEventJSON struct {
		ReporterResourceKey           ReporterResourceKey `json:"reporter_resource_key"`
		OperationType                 string              `json:"operation_type"`
		CommonVersion                 *Version            `json:"common_version,omitempty"`
		ReporterRepresentationVersion *Version            `json:"reporter_representation_version,omitempty"`
	}

	var temp tupleEventJSON
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Convert string to concrete EventOperationType
	switch temp.OperationType {
	case "created":
		te.operationType = biz.NewOperationTypeCreated()
	case "updated":
		te.operationType = biz.NewOperationTypeUpdated()
	case "deleted":
		te.operationType = biz.NewOperationTypeDeleted()
	default:
		return fmt.Errorf("invalid operation type: %s", temp.OperationType)
	}

	te.reporterResourceKey = temp.ReporterResourceKey
	te.commonVersion = temp.CommonVersion
	te.reporterRepresentationVersion = temp.ReporterRepresentationVersion

	return nil
}
