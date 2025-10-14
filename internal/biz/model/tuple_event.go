package model

import (
	"encoding/json"
	"fmt"
)

type TupleEvent struct {
	reporterResourceKey           ReporterResourceKey
	commonVersion                 *Version
	reporterRepresentationVersion *Version
}

func NewTupleEvent(
	reporterResourceKey ReporterResourceKey,
	commonVersion *Version,
	reporterRepresentationVersion *Version,
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

// MarshalJSON implements json.Marshaler interface
func (te TupleEvent) MarshalJSON() ([]byte, error) {
	type tupleEventJSON struct {
		ReporterResourceKey           ReporterResourceKey `json:"reporter_resource_key"`
		CommonVersion                 *Version            `json:"common_version,omitempty"`
		ReporterRepresentationVersion *Version            `json:"reporter_representation_version,omitempty"`
	}

	return json.Marshal(tupleEventJSON{
		ReporterResourceKey:           te.reporterResourceKey,
		CommonVersion:                 te.commonVersion,
		ReporterRepresentationVersion: te.reporterRepresentationVersion,
	})
}

// UnmarshalJSON implements json.Unmarshaler interface
func (te *TupleEvent) UnmarshalJSON(data []byte) error {
	type tupleEventJSON struct {
		ReporterResourceKey           ReporterResourceKey `json:"reporter_resource_key"`
		CommonVersion                 *Version            `json:"common_version,omitempty"`
		ReporterRepresentationVersion *Version            `json:"reporter_representation_version,omitempty"`
	}

	var temp tupleEventJSON
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	te.reporterResourceKey = temp.ReporterResourceKey
	te.commonVersion = temp.CommonVersion
	te.reporterRepresentationVersion = temp.ReporterRepresentationVersion

	return nil
}
