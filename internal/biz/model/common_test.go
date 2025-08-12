package model

import (
	"testing"

	"github.com/google/uuid"

	"github.com/project-kessel/inventory-api/internal/errors"
)

func TestVersion_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewVersionTestFixture()

	t.Run("should create version with zero value", func(t *testing.T) {
		t.Parallel()

		version := NewVersion(fixture.ZeroValue)

		if version.Uint() != fixture.ZeroValue {
			t.Errorf("Expected version %d, got %d", fixture.ZeroValue, version.Uint())
		}
	})

	t.Run("should create version with positive value", func(t *testing.T) {
		t.Parallel()

		version := NewVersion(fixture.PositiveValue)

		if version.Uint() != fixture.PositiveValue {
			t.Errorf("Expected version %d, got %d", fixture.PositiveValue, version.Uint())
		}
	})

	t.Run("should create version with large value", func(t *testing.T) {
		t.Parallel()

		version := NewVersion(fixture.LargeValue)

		if version.Uint() != fixture.LargeValue {
			t.Errorf("Expected version %d, got %d", fixture.LargeValue, version.Uint())
		}
	})

	t.Run("should prevent negative values through uint type safety", func(t *testing.T) {
		t.Parallel()

		version := NewVersion(0)
		if version.Uint() != 0 {
			t.Errorf("Expected version 0, got %d", version.Uint())
		}

		maxUint := ^uint(0)
		versionMax := NewVersion(maxUint)
		if versionMax.Uint() != maxUint {
			t.Errorf("Expected version %d, got %d", maxUint, versionMax.Uint())
		}
	})
}

func TestVersion_Increment(t *testing.T) {
	t.Parallel()

	t.Run("should increment zero version to one", func(t *testing.T) {
		t.Parallel()

		version := NewVersion(0)
		incremented := version.Increment()

		if incremented.Uint() != 1 {
			t.Errorf("Expected incremented version to be 1, got %d", incremented.Uint())
		}
	})

	t.Run("should increment positive version by one", func(t *testing.T) {
		t.Parallel()

		version := NewVersion(5)
		incremented := version.Increment()

		if incremented.Uint() != 6 {
			t.Errorf("Expected incremented version to be 6, got %d", incremented.Uint())
		}
	})

	t.Run("should increment large version by one", func(t *testing.T) {
		t.Parallel()

		version := NewVersion(1000)
		incremented := version.Increment()

		if incremented.Uint() != 1001 {
			t.Errorf("Expected incremented version to be 1001, got %d", incremented.Uint())
		}
	})

	t.Run("should not modify original version", func(t *testing.T) {
		t.Parallel()

		original := NewVersion(10)
		incremented := original.Increment()

		if original.Uint() != 10 {
			t.Errorf("Expected original version to remain 10, got %d", original.Uint())
		}
		if incremented.Uint() != 11 {
			t.Errorf("Expected incremented version to be 11, got %d", incremented.Uint())
		}
	})

	t.Run("should handle maximum value gracefully", func(t *testing.T) {
		t.Parallel()

		maxUint := ^uint(0)
		version := NewVersion(maxUint)
		incremented := version.Increment()

		if incremented.Uint() != 0 {
			t.Errorf("Expected incremented max version to wrap to 0, got %d", incremented.Uint())
		}
	})
}

func TestGeneration_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewGenerationTestFixture()

	t.Run("should create generation with zero value", func(t *testing.T) {
		t.Parallel()

		generation := NewGeneration(fixture.ZeroValue)

		if generation.Uint() != fixture.ZeroValue {
			t.Errorf("Expected generation %d, got %d", fixture.ZeroValue, generation.Uint())
		}
	})

	t.Run("should create generation with positive value", func(t *testing.T) {
		t.Parallel()

		generation := NewGeneration(fixture.PositiveValue)

		if generation.Uint() != fixture.PositiveValue {
			t.Errorf("Expected generation %d, got %d", fixture.PositiveValue, generation.Uint())
		}
	})

	t.Run("should create generation with large value", func(t *testing.T) {
		t.Parallel()

		generation := NewGeneration(fixture.LargeValue)

		if generation.Uint() != fixture.LargeValue {
			t.Errorf("Expected generation %d, got %d", fixture.LargeValue, generation.Uint())
		}
	})

	t.Run("should prevent negative values through uint type safety", func(t *testing.T) {
		t.Parallel()

		generation := NewGeneration(0)
		if generation.Uint() != 0 {
			t.Errorf("Expected generation 0, got %d", generation.Uint())
		}

		maxUint := ^uint(0)
		generationMax := NewGeneration(maxUint)
		if generationMax.Uint() != maxUint {
			t.Errorf("Expected generation %d, got %d", maxUint, generationMax.Uint())
		}
	})
}

func TestTombstone_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewTombstoneTestFixture()

	t.Run("should create tombstone with true value", func(t *testing.T) {
		t.Parallel()

		tombstone := NewTombstone(fixture.TrueValue)

		if tombstone.Bool() != fixture.TrueValue {
			t.Errorf("Expected tombstone %t, got %t", fixture.TrueValue, tombstone.Bool())
		}
	})

	t.Run("should create tombstone with false value", func(t *testing.T) {
		t.Parallel()

		tombstone := NewTombstone(fixture.FalseValue)

		if tombstone.Bool() != fixture.FalseValue {
			t.Errorf("Expected tombstone %t, got %t", fixture.FalseValue, tombstone.Bool())
		}
	})
}

func TestResourceId_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewResourceIdTestFixture()

	t.Run("should create resource id with valid UUID", func(t *testing.T) {
		t.Parallel()

		resourceId, err := NewResourceId(fixture.ValidUUID)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if resourceId.UUID() != fixture.ValidUUID {
			t.Errorf("Expected resource id %v, got %v", fixture.ValidUUID, resourceId.UUID())
		}
		if resourceId.String() != fixture.ValidUUID.String() {
			t.Errorf("Expected resource id string %s, got %s", fixture.ValidUUID.String(), resourceId.String())
		}
	})

	t.Run("should create resource id with another valid UUID", func(t *testing.T) {
		t.Parallel()

		resourceId, err := NewResourceId(fixture.AnotherUUID)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if resourceId.UUID() != fixture.AnotherUUID {
			t.Errorf("Expected resource id %v, got %v", fixture.AnotherUUID, resourceId.UUID())
		}
	})

	t.Run("should reject nil UUID", func(t *testing.T) {
		t.Parallel()

		_, err := NewResourceId(fixture.NilUUID)

		if err == nil {
			t.Error("Expected error for nil UUID, got none")
		}
	})
}

func TestReporterResourceId_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewReporterResourceIdTestFixture()

	t.Run("should create reporter resource id with valid UUID", func(t *testing.T) {
		t.Parallel()

		reporterResourceId, err := NewReporterResourceId(fixture.ValidUUID)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if reporterResourceId.UUID() != fixture.ValidUUID {
			t.Errorf("Expected reporter resource id %v, got %v", fixture.ValidUUID, reporterResourceId.UUID())
		}
		if reporterResourceId.String() != fixture.ValidUUID.String() {
			t.Errorf("Expected reporter resource id string %s, got %s", fixture.ValidUUID.String(), reporterResourceId.String())
		}
	})

	t.Run("should create reporter resource id from valid string", func(t *testing.T) {
		t.Parallel()

		reporterResourceId, err := NewReporterResourceIdFromString(fixture.ValidString)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if reporterResourceId.String() != fixture.ValidString {
			t.Errorf("Expected reporter resource id string %s, got %s", fixture.ValidString, reporterResourceId.String())
		}
	})

	t.Run("should reject nil UUID", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterResourceId(fixture.NilUUID)

		if err == nil {
			t.Error("Expected error for nil UUID, got none")
		}
	})

	t.Run("should reject invalid string", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterResourceIdFromString(fixture.InvalidString)

		if err == nil {
			t.Error("Expected error for invalid string, got none")
		}
	})

	t.Run("should reject empty string", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterResourceIdFromString(fixture.EmptyString)

		if err == nil {
			t.Error("Expected error for empty string, got none")
		}
	})

	t.Run("should reject whitespace string", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterResourceIdFromString(fixture.WhitespaceString)

		if err == nil {
			t.Error("Expected error for whitespace string, got none")
		}
	})
}

func TestResourceType_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewResourceTypeTestFixture()

	t.Run("should create resource type with valid type", func(t *testing.T) {
		t.Parallel()

		resourceType, err := NewResourceType(fixture.ValidType)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if resourceType.String() != fixture.ValidType {
			t.Errorf("Expected resource type %s, got %s", fixture.ValidType, resourceType.String())
		}
	})

	t.Run("should create resource type with another valid type", func(t *testing.T) {
		t.Parallel()

		resourceType, err := NewResourceType(fixture.AnotherType)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if resourceType.String() != fixture.AnotherType {
			t.Errorf("Expected resource type %s, got %s", fixture.AnotherType, resourceType.String())
		}
	})

	t.Run("should reject empty string", func(t *testing.T) {
		t.Parallel()

		_, err := NewResourceType(fixture.EmptyString)

		if err == nil {
			t.Error("Expected error for empty string, got none")
		}
	})

	t.Run("should reject whitespace string", func(t *testing.T) {
		t.Parallel()

		_, err := NewResourceType(fixture.WhitespaceString)

		if err == nil {
			t.Error("Expected error for whitespace string, got none")
		}
	})
}

func TestReporterType_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewReporterTypeTestFixture()

	t.Run("should create reporter type with valid type", func(t *testing.T) {
		t.Parallel()

		reporterType, err := NewReporterType(fixture.ValidType)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if reporterType.String() != fixture.ValidType {
			t.Errorf("Expected reporter type %s, got %s", fixture.ValidType, reporterType.String())
		}
	})

	t.Run("should create reporter type with another valid type", func(t *testing.T) {
		t.Parallel()

		reporterType, err := NewReporterType(fixture.AnotherType)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if reporterType.String() != fixture.AnotherType {
			t.Errorf("Expected reporter type %s, got %s", fixture.AnotherType, reporterType.String())
		}
	})

	t.Run("should reject empty string", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterType(fixture.EmptyString)

		if err == nil {
			t.Error("Expected error for empty string, got none")
		}
	})

	t.Run("should reject whitespace string", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterType(fixture.WhitespaceString)

		if err == nil {
			t.Error("Expected error for whitespace string, got none")
		}
	})
}

func TestReporterInstanceId_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewReporterInstanceIdTestFixture()

	t.Run("should create reporter instance id with valid id", func(t *testing.T) {
		t.Parallel()

		reporterInstanceId, err := NewReporterInstanceId(fixture.ValidId)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if reporterInstanceId.String() != fixture.ValidId {
			t.Errorf("Expected reporter instance id %s, got %s", fixture.ValidId, reporterInstanceId.String())
		}
	})

	t.Run("should create reporter instance id with another valid id", func(t *testing.T) {
		t.Parallel()

		reporterInstanceId, err := NewReporterInstanceId(fixture.AnotherId)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if reporterInstanceId.String() != fixture.AnotherId {
			t.Errorf("Expected reporter instance id %s, got %s", fixture.AnotherId, reporterInstanceId.String())
		}
	})

	t.Run("should reject empty string", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterInstanceId(fixture.EmptyString)

		if err == nil {
			t.Error("Expected error for empty string, got none")
		}
	})

	t.Run("should reject whitespace string", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterInstanceId(fixture.WhitespaceString)

		if err == nil {
			t.Error("Expected error for whitespace string, got none")
		}
	})
}

func TestConsistencyToken_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewConsistencyTokenTestFixture()

	t.Run("should create consistency token with valid token", func(t *testing.T) {
		t.Parallel()

		consistencyToken, err := NewConsistencyToken(fixture.ValidToken)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if consistencyToken.String() != fixture.ValidToken {
			t.Errorf("Expected consistency token %s, got %s", fixture.ValidToken, consistencyToken.String())
		}
	})

	t.Run("should create consistency token with another valid token", func(t *testing.T) {
		t.Parallel()

		consistencyToken, err := NewConsistencyToken(fixture.AnotherToken)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if consistencyToken.String() != fixture.AnotherToken {
			t.Errorf("Expected consistency token %s, got %s", fixture.AnotherToken, consistencyToken.String())
		}
	})

	t.Run("should reject empty string", func(t *testing.T) {
		t.Parallel()

		_, err := NewConsistencyToken(fixture.EmptyString)

		if err == nil {
			t.Error("Expected error for empty string, got none")
		}
	})

	t.Run("should reject whitespace string", func(t *testing.T) {
		t.Parallel()

		_, err := NewConsistencyToken(fixture.WhitespaceString)

		if err == nil {
			t.Error("Expected error for whitespace string, got none")
		}
	})
}

func TestReporterVersion_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewReporterVersionTestFixture()

	t.Run("should create reporter version with valid version", func(t *testing.T) {
		t.Parallel()

		reporterVersion, err := NewReporterVersion(fixture.ValidVersion)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if reporterVersion.String() != fixture.ValidVersion {
			t.Errorf("Expected reporter version %s, got %s", fixture.ValidVersion, reporterVersion.String())
		}
	})

	t.Run("should create reporter version with another valid version", func(t *testing.T) {
		t.Parallel()

		reporterVersion, err := NewReporterVersion(fixture.AnotherVersion)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if reporterVersion.String() != fixture.AnotherVersion {
			t.Errorf("Expected reporter version %s, got %s", fixture.AnotherVersion, reporterVersion.String())
		}
	})

	t.Run("should reject empty string", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterVersion(fixture.EmptyString)

		if err == nil {
			t.Error("Expected error for empty string, got none")
		}
	})

	t.Run("should reject whitespace string", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterVersion(fixture.WhitespaceString)

		if err == nil {
			t.Error("Expected error for whitespace string, got none")
		}
	})

	t.Run("should create reporter version with valid string", func(t *testing.T) {
		t.Parallel()

		reporterVersion, err := NewReporterVersion(*fixture.ValidPointer)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if reporterVersion.String() != *fixture.ValidPointer {
			t.Errorf("Expected reporter version %s, got %s", *fixture.ValidPointer, reporterVersion.String())
		}
	})

	t.Run("should enforce non-null values in tiny types", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterVersion("")

		if err == nil {
			t.Error("Expected error for empty string, got none")
		}
		errors.AssertIs(t, err, ErrEmpty)
	})
}

func TestApiHref_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewApiHrefTestFixture()

	t.Run("should create api href with valid href", func(t *testing.T) {
		t.Parallel()

		apiHref, err := NewApiHref(fixture.ValidHref)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if apiHref.String() != fixture.ValidHref {
			t.Errorf("Expected api href %s, got %s", fixture.ValidHref, apiHref.String())
		}
	})

	t.Run("should create api href with another valid href", func(t *testing.T) {
		t.Parallel()

		apiHref, err := NewApiHref(fixture.AnotherHref)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if apiHref.String() != fixture.AnotherHref {
			t.Errorf("Expected api href %s, got %s", fixture.AnotherHref, apiHref.String())
		}
	})

	t.Run("should reject empty string", func(t *testing.T) {
		t.Parallel()

		_, err := NewApiHref(fixture.EmptyString)

		if err == nil {
			t.Error("Expected error for empty string, got none")
		}
	})

	t.Run("should reject whitespace string", func(t *testing.T) {
		t.Parallel()

		_, err := NewApiHref(fixture.WhitespaceString)

		if err == nil {
			t.Error("Expected error for whitespace string, got none")
		}
	})
}

func TestConsoleHref_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewConsoleHrefTestFixture()

	t.Run("should create console href with valid href", func(t *testing.T) {
		t.Parallel()

		consoleHref, err := NewConsoleHref(fixture.ValidHref)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if consoleHref.String() != fixture.ValidHref {
			t.Errorf("Expected console href %s, got %s", fixture.ValidHref, consoleHref.String())
		}
	})

	t.Run("should create console href with another valid href", func(t *testing.T) {
		t.Parallel()

		consoleHref, err := NewConsoleHref(fixture.AnotherHref)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if consoleHref.String() != fixture.AnotherHref {
			t.Errorf("Expected console href %s, got %s", fixture.AnotherHref, consoleHref.String())
		}
	})

	t.Run("should reject empty string", func(t *testing.T) {
		t.Parallel()

		_, err := NewConsoleHref(fixture.EmptyString)

		if err == nil {
			t.Error("Expected error for empty string, got none")
		}
	})

	t.Run("should reject whitespace string", func(t *testing.T) {
		t.Parallel()

		_, err := NewConsoleHref(fixture.WhitespaceString)

		if err == nil {
			t.Error("Expected error for whitespace string, got none")
		}
	})
}

func TestLocalResourceId_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewLocalResourceIdTestFixture()

	t.Run("should create local resource id with valid id", func(t *testing.T) {
		t.Parallel()

		localResourceId, err := NewLocalResourceId(fixture.ValidId)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if localResourceId.String() != fixture.ValidId {
			t.Errorf("Expected local resource id %s, got %s", fixture.ValidId, localResourceId.String())
		}
	})

	t.Run("should create local resource id with another valid id", func(t *testing.T) {
		t.Parallel()

		localResourceId, err := NewLocalResourceId(fixture.AnotherId)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if localResourceId.String() != fixture.AnotherId {
			t.Errorf("Expected local resource id %s, got %s", fixture.AnotherId, localResourceId.String())
		}
	})

	t.Run("should reject empty string", func(t *testing.T) {
		t.Parallel()

		_, err := NewLocalResourceId(fixture.EmptyString)

		if err == nil {
			t.Error("Expected error for empty string, got none")
		}
	})

	t.Run("should reject whitespace string", func(t *testing.T) {
		t.Parallel()

		_, err := NewLocalResourceId(fixture.WhitespaceString)

		if err == nil {
			t.Error("Expected error for whitespace string, got none")
		}
	})
}

func TestSerializationRoundtrip(t *testing.T) {
	t.Parallel()

	t.Run("Version roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []uint{0, 1, 42, 1000, ^uint(0)} // zero, small, medium, large, max
		for _, original := range testCases {
			version := NewVersion(original)
			serialized := version.Serialize()
			deserialized := DeserializeVersion(serialized)

			if deserialized.Uint() != original {
				t.Errorf("Version roundtrip failed: %d -> %d -> %d", original, serialized, deserialized.Uint())
			}
		}
	})

	t.Run("ResourceId roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []uuid.UUID{
			uuid.New(),
			uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
			uuid.MustParse("00000000-0000-0000-0000-000000000000"),
		}
		for _, original := range testCases {
			resourceId, err := NewResourceId(original)
			if err != nil {
				if original == uuid.Nil {
					continue // Skip nil UUID as it's invalid
				}
				t.Fatalf("Failed to create ResourceId: %v", err)
			}
			serialized := resourceId.Serialize()
			deserialized := DeserializeResourceId(serialized)

			if deserialized.UUID() != original {
				t.Errorf("ResourceId roundtrip failed: %s -> %s -> %s", original, serialized, deserialized.UUID())
			}
		}
	})

	t.Run("ReporterResourceId roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []uuid.UUID{
			uuid.New(),
			uuid.MustParse("987fcdeb-51a2-43d8-b123-456789abcdef"),
		}
		for _, original := range testCases {
			reporterResourceId, err := NewReporterResourceId(original)
			if err != nil {
				t.Fatalf("Failed to create ReporterResourceId: %v", err)
			}
			serialized := reporterResourceId.Serialize()
			deserialized := DeserializeReporterResourceId(serialized)

			if deserialized.UUID() != original {
				t.Errorf("ReporterResourceId roundtrip failed: %s -> %s -> %s", original, serialized, deserialized.UUID())
			}
		}
	})

	t.Run("ResourceType roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []string{"rhel-host", "k8s-cluster", "notifications-integration", "test-type"}
		for _, original := range testCases {
			resourceType, err := NewResourceType(original)
			if err != nil {
				t.Fatalf("Failed to create ResourceType: %v", err)
			}
			serialized := resourceType.Serialize()
			deserialized := DeserializeResourceType(serialized)

			if deserialized.String() != original {
				t.Errorf("ResourceType roundtrip failed: %s -> %s -> %s", original, serialized, deserialized.String())
			}
		}
	})

	t.Run("ReporterType roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []string{"hbi", "acm", "acs", "ocm"}
		for _, original := range testCases {
			reporterType, err := NewReporterType(original)
			if err != nil {
				t.Fatalf("Failed to create ReporterType: %v", err)
			}
			serialized := reporterType.Serialize()
			deserialized := DeserializeReporterType(serialized)

			if deserialized.String() != original {
				t.Errorf("ReporterType roundtrip failed: %s -> %s -> %s", original, serialized, deserialized.String())
			}
		}
	})

	t.Run("ReporterInstanceId roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []string{"instance-1", "test-instance", "reporter-instance-123"}
		for _, original := range testCases {
			reporterInstanceId, err := NewReporterInstanceId(original)
			if err != nil {
				t.Fatalf("Failed to create ReporterInstanceId: %v", err)
			}
			serialized := reporterInstanceId.Serialize()
			deserialized := DeserializeReporterInstanceId(serialized)

			if deserialized.String() != original {
				t.Errorf("ReporterInstanceId roundtrip failed: %s -> %s -> %s", original, serialized, deserialized.String())
			}
		}
	})

	t.Run("ConsistencyToken roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []string{"token-123", "consistency-token", "abcd-efgh-ijkl"}
		for _, original := range testCases {
			consistencyToken, err := NewConsistencyToken(original)
			if err != nil {
				t.Fatalf("Failed to create ConsistencyToken: %v", err)
			}
			serialized := consistencyToken.Serialize()
			deserialized := DeserializeConsistencyToken(serialized)

			if deserialized.String() != original {
				t.Errorf("ConsistencyToken roundtrip failed: %s -> %s -> %s", original, serialized, deserialized.String())
			}
		}
	})

	t.Run("Generation roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []uint{0, 1, 100, 9999, ^uint(0)} // zero, small, medium, large, max
		for _, original := range testCases {
			generation := NewGeneration(original)
			serialized := generation.Serialize()
			deserialized := DeserializeGeneration(serialized)

			if deserialized.Uint() != original {
				t.Errorf("Generation roundtrip failed: %d -> %d -> %d", original, serialized, deserialized.Uint())
			}
		}
	})

	t.Run("ReporterVersion roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []string{"1.0.0", "v2.1.3", "latest", "snapshot-123"}
		for _, original := range testCases {
			reporterVersion, err := NewReporterVersion(original)
			if err != nil {
				t.Fatalf("Failed to create ReporterVersion: %v", err)
			}
			serialized := reporterVersion.Serialize()
			deserialized := DeserializeReporterVersion(serialized)

			if deserialized.String() != original {
				t.Errorf("ReporterVersion roundtrip failed: %s -> %s -> %s", original, serialized, deserialized.String())
			}
		}
	})

	t.Run("Tombstone roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []bool{true, false}
		for _, original := range testCases {
			tombstone := NewTombstone(original)
			serialized := tombstone.Serialize()
			deserialized := DeserializeTombstone(serialized)

			if deserialized.Bool() != original {
				t.Errorf("Tombstone roundtrip failed: %t -> %t -> %t", original, serialized, deserialized.Bool())
			}
		}
	})

	t.Run("ApiHref roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []string{
			"https://api.example.com/resource/123",
			"http://localhost:8080/api/v1/resource",
			"/api/resource/456",
		}
		for _, original := range testCases {
			apiHref, err := NewApiHref(original)
			if err != nil {
				t.Fatalf("Failed to create ApiHref: %v", err)
			}
			serialized := apiHref.Serialize()
			deserialized := DeserializeApiHref(serialized)

			if deserialized.String() != original {
				t.Errorf("ApiHref roundtrip failed: %s -> %s -> %s", original, serialized, deserialized.String())
			}
		}
	})

	t.Run("ConsoleHref roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []string{
			"https://console.example.com/resource/123",
			"http://localhost:3000/console/resource",
			"/console/resource/789",
		}
		for _, original := range testCases {
			consoleHref, err := NewConsoleHref(original)
			if err != nil {
				t.Fatalf("Failed to create ConsoleHref: %v", err)
			}
			serialized := consoleHref.Serialize()
			deserialized := DeserializeConsoleHref(serialized)

			if deserialized.String() != original {
				t.Errorf("ConsoleHref roundtrip failed: %s -> %s -> %s", original, serialized, deserialized.String())
			}
		}
	})

	t.Run("LocalResourceId roundtrip", func(t *testing.T) {
		t.Parallel()

		testCases := []string{
			"local-resource-123",
			"cluster-abcd-efgh",
			uuid.New().String(),
		}
		for _, original := range testCases {
			localResourceId, err := NewLocalResourceId(original)
			if err != nil {
				t.Fatalf("Failed to create LocalResourceId: %v", err)
			}
			serialized := localResourceId.Serialize()
			deserialized := DeserializeLocalResourceId(serialized)

			if deserialized.String() != original {
				t.Errorf("LocalResourceId roundtrip failed: %s -> %s -> %s", original, serialized, deserialized.String())
			}
		}
	})
}
