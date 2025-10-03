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

func (te *TupleEvent) UnmarshalJSON(data []byte) error {
	var temp struct {
		ReporterResourceKey struct {
			LocalResourceID string `json:"localResourceID"`
			ResourceType    string `json:"resourceType"`
			Reporter        struct {
				ReporterType       string `json:"reporterType"`
				ReporterInstanceId string `json:"reporterInstanceId"`
			} `json:"reporter"`
		} `json:"reporterResourceKey"`
		OperationType                 string `json:"operationType"`
		CommonVersion                 *uint  `json:"commonVersion"`
		ReporterRepresentationVersion *uint  `json:"reporterRepresentationVersion"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Convert to domain types
	localResourceId, _ := NewLocalResourceId(temp.ReporterResourceKey.LocalResourceID)
	resourceType, _ := NewResourceType(temp.ReporterResourceKey.ResourceType)
	reporterType, _ := NewReporterType(temp.ReporterResourceKey.Reporter.ReporterType)
	reporterInstanceId, _ := NewReporterInstanceId(temp.ReporterResourceKey.Reporter.ReporterInstanceId)

	reporterResourceKey, _ := NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)

	// Convert operation type
	var operationType biz.EventOperationType
	switch temp.OperationType {
	case "created":
		operationType = biz.OperationTypeCreated
	case "updated":
		operationType = biz.OperationTypeUpdated
	case "deleted":
		operationType = biz.OperationTypeDeleted
	default:
		operationType = biz.OperationTypeCreated
	}

	var commonVersion *Version
	if temp.CommonVersion != nil {
		v := NewVersion(*temp.CommonVersion)
		commonVersion = &v
	}

	var reporterRepresentationVersion *Version
	if temp.ReporterRepresentationVersion != nil {
		v := NewVersion(*temp.ReporterRepresentationVersion)
		reporterRepresentationVersion = &v
	}

	te.reporterResourceKey = reporterResourceKey
	te.operationType = operationType
	te.commonVersion = commonVersion
	te.reporterRepresentationVersion = reporterRepresentationVersion

	return nil
}
