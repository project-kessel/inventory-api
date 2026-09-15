package model

import (
	"encoding/json"
	"fmt"
)

type TupleEvent struct {
	reporterResourceKey           ReporterResourceKey
	commonVersion                 *Version
	reporterRepresentationVersion *Version
	reporterGeneration            *Generation
}

func NewTupleEvent(
	reporterResourceKey ReporterResourceKey,
	commonVersion *Version,
	reporterRepresentationVersion *Version,
	reporterGeneration *Generation,
) (TupleEvent, error) {
	// Validate required fields
	if reporterResourceKey == (ReporterResourceKey{}) {
		return TupleEvent{}, fmt.Errorf("%w: reporterResourceKey", ErrEmpty)
	}

	// Enforce invariant: at least one version must be present
	if commonVersion == nil && reporterRepresentationVersion == nil {
		return TupleEvent{}, fmt.Errorf("at least one version (commonVersion or reporterRepresentationVersion) must be present")
	}

	return TupleEvent{
		reporterResourceKey:           reporterResourceKey,
		commonVersion:                 commonVersion,
		reporterRepresentationVersion: reporterRepresentationVersion,
		reporterGeneration:            reporterGeneration,
	}, nil
}

func (te TupleEvent) ReporterResourceKey() ReporterResourceKey {
	return te.reporterResourceKey
}

func (te TupleEvent) CommonVersion() *Version {
	return te.commonVersion
}

func (te TupleEvent) ReporterRepresentationVersion() *Version {
	return te.reporterRepresentationVersion
}

func (te TupleEvent) ReporterGeneration() *Generation {
	return te.reporterGeneration
}

// MarshalJSON implements json.Marshaler interface
func (te TupleEvent) MarshalJSON() ([]byte, error) {
	type tupleEventJSON struct {
		ReporterResourceKey           ReporterResourceKey `json:"reporter_resource_key"`
		CommonVersion                 *Version            `json:"common_version,omitempty"`
		ReporterRepresentationVersion *Version            `json:"reporter_representation_version,omitempty"`
		ReporterGeneration            *Generation         `json:"reporter_generation,omitempty"`
	}

	return json.Marshal(tupleEventJSON{
		ReporterResourceKey:           te.reporterResourceKey,
		CommonVersion:                 te.commonVersion,
		ReporterRepresentationVersion: te.reporterRepresentationVersion,
		ReporterGeneration:            te.reporterGeneration,
	})
}

// UnmarshalJSON implements json.Unmarshaler interface
func (te *TupleEvent) UnmarshalJSON(data []byte) error {
	type tupleEventJSON struct {
		ReporterResourceKey           ReporterResourceKey `json:"reporter_resource_key"`
		CommonVersion                 *Version            `json:"common_version,omitempty"`
		ReporterRepresentationVersion *Version            `json:"reporter_representation_version,omitempty"`
		ReporterGeneration            *Generation         `json:"reporter_generation,omitempty"`
	}

	var temp tupleEventJSON
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Validate required fields
	if temp.ReporterResourceKey == (ReporterResourceKey{}) {
		return fmt.Errorf("%w: reporterResourceKey", ErrEmpty)
	}

	// Enforce invariant: at least one version must be present
	// This prevents poison messages (both versions nil) from blocking the Kafka partition
	if temp.CommonVersion == nil && temp.ReporterRepresentationVersion == nil {
		return fmt.Errorf("at least one version (commonVersion or reporterRepresentationVersion) must be present")
	}

	te.reporterResourceKey = temp.ReporterResourceKey
	te.commonVersion = temp.CommonVersion
	te.reporterRepresentationVersion = temp.ReporterRepresentationVersion
	te.reporterGeneration = temp.ReporterGeneration

	return nil
}
