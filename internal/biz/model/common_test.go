package model

import (
	"testing"
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

	t.Run("should create reporter version pointer with valid pointer", func(t *testing.T) {
		t.Parallel()

		reporterVersionPtr, err := NewReporterVersionPtr(fixture.ValidPointer)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if reporterVersionPtr == nil {
			t.Error("Expected non-nil pointer, got nil")
		}
		if reporterVersionPtr.String() != *fixture.ValidPointer {
			t.Errorf("Expected reporter version %s, got %s", *fixture.ValidPointer, reporterVersionPtr.String())
		}
		if *reporterVersionPtr.StringPtr() != *fixture.ValidPointer {
			t.Errorf("Expected reporter version pointer %s, got %s", *fixture.ValidPointer, *reporterVersionPtr.StringPtr())
		}
	})

	t.Run("should handle nil pointer", func(t *testing.T) {
		t.Parallel()

		reporterVersionPtr, err := NewReporterVersionPtr(fixture.NilPointer)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if reporterVersionPtr != nil {
			t.Error("Expected nil pointer, got non-nil")
		}
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
