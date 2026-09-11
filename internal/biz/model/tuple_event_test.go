package model

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestTupleEvent_NewTupleEvent_RequiresAtLeastOneVersion(t *testing.T) {
	localResourceId, _ := NewLocalResourceId("test-resource")
	resourceType, _ := NewResourceType("test-type")
	reporterType, _ := NewReporterType("test-reporter")
	reporterInstanceId, _ := NewReporterInstanceId("test-instance")
	key, _ := NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)

	t.Run("valid with both versions", func(t *testing.T) {
		commonVer := NewVersion(1)
		reporterVer := NewVersion(2)
		_, err := NewTupleEvent(key, &commonVer, &reporterVer)
		if err != nil {
			t.Errorf("expected no error with both versions present, got: %v", err)
		}
	})

	t.Run("valid with only common version", func(t *testing.T) {
		commonVer := NewVersion(1)
		_, err := NewTupleEvent(key, &commonVer, nil)
		if err != nil {
			t.Errorf("expected no error with only common version, got: %v", err)
		}
	})

	t.Run("valid with only reporter version", func(t *testing.T) {
		reporterVer := NewVersion(2)
		_, err := NewTupleEvent(key, nil, &reporterVer)
		if err != nil {
			t.Errorf("expected no error with only reporter version, got: %v", err)
		}
	})

	t.Run("invalid with both versions nil", func(t *testing.T) {
		_, err := NewTupleEvent(key, nil, nil)
		if err == nil {
			t.Error("expected error when both versions are nil")
		}
		expectedMsg := "at least one version"
		if err != nil && !contains(err.Error(), expectedMsg) {
			t.Errorf("expected error message to contain %q, got: %v", expectedMsg, err)
		}
	})
}

func TestTupleEvent_UnmarshalJSON_ValidatesInvariant(t *testing.T) {
	localResourceId, _ := NewLocalResourceId("test-resource")
	resourceType, _ := NewResourceType("test-type")
	reporterType, _ := NewReporterType("test-reporter")
	reporterInstanceId, _ := NewReporterInstanceId("test-instance")
	key, _ := NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)

	t.Run("valid JSON with both versions", func(t *testing.T) {
		commonVer := NewVersion(1)
		reporterVer := NewVersion(2)
		event, _ := NewTupleEvent(key, &commonVer, &reporterVer)

		jsonData, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled TupleEvent
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if unmarshaled.CommonVersion() == nil || unmarshaled.CommonVersion().Uint() != 1 {
			t.Errorf("expected common version 1, got: %v", unmarshaled.CommonVersion())
		}
		if unmarshaled.ReporterRepresentationVersion() == nil || unmarshaled.ReporterRepresentationVersion().Uint() != 2 {
			t.Errorf("expected reporter version 2, got: %v", unmarshaled.ReporterRepresentationVersion())
		}
	})

	t.Run("valid JSON with only common version", func(t *testing.T) {
		commonVer := NewVersion(1)
		event, _ := NewTupleEvent(key, &commonVer, nil)

		jsonData, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled TupleEvent
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if unmarshaled.CommonVersion() == nil || unmarshaled.CommonVersion().Uint() != 1 {
			t.Errorf("expected common version 1, got: %v", unmarshaled.CommonVersion())
		}
		if unmarshaled.ReporterRepresentationVersion() != nil {
			t.Errorf("expected nil reporter version, got: %v", unmarshaled.ReporterRepresentationVersion())
		}
	})

	t.Run("valid JSON with only reporter version", func(t *testing.T) {
		reporterVer := NewVersion(2)
		event, _ := NewTupleEvent(key, nil, &reporterVer)

		jsonData, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled TupleEvent
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if unmarshaled.CommonVersion() != nil {
			t.Errorf("expected nil common version, got: %v", unmarshaled.CommonVersion())
		}
		if unmarshaled.ReporterRepresentationVersion() == nil || unmarshaled.ReporterRepresentationVersion().Uint() != 2 {
			t.Errorf("expected reporter version 2, got: %v", unmarshaled.ReporterRepresentationVersion())
		}
	})

	t.Run("invalid JSON with both versions nil - poison message scenario", func(t *testing.T) {
		// Simulate a poison message from Kafka with both versions omitted/nil
		poisonJSON := `{
			"reporter_resource_key": {
				"local_resource_id": "test-resource",
				"resource_type": "test-type",
				"reporter_type": "test-reporter",
				"reporter_instance_id": "test-instance"
			}
		}`

		var unmarshaled TupleEvent
		err := json.Unmarshal([]byte(poisonJSON), &unmarshaled)
		if err == nil {
			t.Error("expected error when unmarshaling poison message with both versions nil")
		}

		expectedMsg := "at least one version"
		if err != nil && !contains(err.Error(), expectedMsg) {
			t.Errorf("expected error message to contain %q, got: %v", expectedMsg, err)
		}
	})

	t.Run("invalid JSON with empty reporter resource key", func(t *testing.T) {
		invalidJSON := `{
			"reporter_resource_key": {},
			"common_version": 1
		}`

		var unmarshaled TupleEvent
		err := json.Unmarshal([]byte(invalidJSON), &unmarshaled)
		if err == nil {
			t.Error("expected error when reporter resource key is empty")
		}
	})
}

func TestTupleEvent_RoundTrip(t *testing.T) {
	localResourceId, _ := NewLocalResourceId("test-resource")
	resourceType, _ := NewResourceType("test-type")
	reporterType, _ := NewReporterType("test-reporter")
	reporterInstanceId, _ := NewReporterInstanceId(uuid.New().String())
	key, _ := NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
	commonVer := NewVersion(5)
	reporterVer := NewVersion(10)

	original, err := NewTupleEvent(key, &commonVer, &reporterVer)
	if err != nil {
		t.Fatalf("failed to create tuple event: %v", err)
	}

	// Marshal
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal
	var roundtripped TupleEvent
	err = json.Unmarshal(jsonData, &roundtripped)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify key fields
	if roundtripped.ReporterResourceKey().LocalResourceId().Serialize() != localResourceId.Serialize() {
		t.Errorf("local resource id mismatch")
	}
	if roundtripped.ReporterResourceKey().ResourceType().Serialize() != resourceType.Serialize() {
		t.Errorf("resource type mismatch")
	}
	if roundtripped.ReporterResourceKey().ReporterType().Serialize() != reporterType.Serialize() {
		t.Errorf("reporter type mismatch")
	}

	// Verify versions
	if roundtripped.CommonVersion() == nil || roundtripped.CommonVersion().Uint() != 5 {
		t.Errorf("common version mismatch: expected 5, got %v", roundtripped.CommonVersion())
	}
	if roundtripped.ReporterRepresentationVersion() == nil || roundtripped.ReporterRepresentationVersion().Uint() != 10 {
		t.Errorf("reporter version mismatch: expected 10, got %v", roundtripped.ReporterRepresentationVersion())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findInString(s, substr))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
