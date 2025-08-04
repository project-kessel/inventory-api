//go:build test

package model

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/project-kessel/inventory-api/internal"
)

func TestResource_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewResourceTestFixture()

	t.Run("should create resource with single reporter resource", func(t *testing.T) {
		t.Parallel()

		resource, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceIdType(),
			fixture.ValidResourceTypeType(),
			fixture.ValidReporterTypeType(),
			fixture.ValidReporterInstanceIdType(),
			fixture.ValidReporterResourceIdType(),
			fixture.ValidApiHrefType(),
			fixture.ValidConsoleHrefType(),
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		assertValidResource(t, resource, err, "single reporter resource")
		assertInitialResourceState(t, resource)
		assertResourceEvent(t, resource, "single reporter resource")
	})

	t.Run("should create resource with multiple reporter resources", func(t *testing.T) {
		t.Parallel()

		// Note: The new signature creates only one ReporterResource
		// This test now validates that we can create another resource with different values
		resource, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.AnotherLocalResourceIdType(),
			fixture.AnotherResourceTypeType(),
			fixture.AnotherReporterTypeType(),
			fixture.AnotherReporterInstanceIdType(),
			fixture.AnotherReporterResourceIdType(),
			fixture.AnotherApiHrefType(),
			fixture.EmptyConsoleHrefType(),
			fixture.AnotherReporterRepresentationType(),
			fixture.AnotherCommonRepresentationType(),
		)

		assertValidResource(t, resource, err, "different reporter resource")
		assertInitialResourceState(t, resource)
		assertResourceEvent(t, resource, "different reporter resource")
		if len(resource.reporterResources) != 1 {
			t.Errorf("Expected 1 reporter resource, got %d", len(resource.reporterResources))
		}
	})

	t.Run("should create resource with different resource type", func(t *testing.T) {
		t.Parallel()

		resource, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.AnotherLocalResourceIdType(),
			fixture.AnotherResourceTypeType(),
			fixture.AnotherReporterTypeType(),
			fixture.AnotherReporterInstanceIdType(),
			fixture.AnotherReporterResourceIdType(),
			fixture.AnotherApiHrefType(),
			fixture.EmptyConsoleHrefType(),
			fixture.AnotherReporterRepresentationType(),
			fixture.AnotherCommonRepresentationType(),
		)

		assertValidResource(t, resource, err, "different resource type")
		assertInitialResourceState(t, resource)
		assertResourceEvent(t, resource, "different resource type")
	})

	t.Run("should reject nil resource id", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.NilId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty resource type", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.EmptyResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject whitespace-only resource type", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.WhitespaceResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty local resource id", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.EmptyLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty reporter type", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.EmptyReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty reporter instance id", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.EmptyReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty api href", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.EmptyApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty reporter representation data", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.EmptyRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		assertInvalidResource(t, err, "Resource invalid ResourceEvent")
	})

	t.Run("should reject empty common representation data", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.EmptyRepresentationType(),
		)

		assertInvalidResource(t, err, "Resource invalid ResourceEvent")
	})
}

func TestResource_AggregateRootBehavior(t *testing.T) {
	t.Parallel()
	fixture := NewResourceTestFixture()

	t.Run("should enforce initial state invariants", func(t *testing.T) {
		t.Parallel()

		resource, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		assertInitialResourceState(t, resource)
		if len(resource.reporterResources) == 0 {
			t.Error("Resource aggregate must contain ReporterResources")
		}
	})

	t.Run("should validate required fields as aggregate root", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name                       string
			idVal                      uuid.UUID
			localResourceIdVal         string
			resourceTypeVal            string
			reporterTypeVal            string
			reporterInstanceIdVal      string
			resourceIdVal              uuid.UUID
			apiHrefVal                 string
			consoleHrefVal             string
			reporterRepresentationData internal.JsonObject
			commonRepresentationData   internal.JsonObject
			expectedError              string
		}{
			{
				name:                       "nil id",
				idVal:                      fixture.NilId,
				localResourceIdVal:         fixture.ValidLocalResourceId,
				resourceTypeVal:            fixture.ValidResourceType,
				reporterTypeVal:            fixture.ValidReporterType,
				reporterInstanceIdVal:      fixture.ValidReporterInstanceId,
				resourceIdVal:              fixture.ValidResourceId,
				apiHrefVal:                 fixture.ValidApiHref,
				consoleHrefVal:             fixture.ValidConsoleHref,
				reporterRepresentationData: fixture.ValidReporterRepresentationType(),
				commonRepresentationData:   fixture.ValidCommonRepresentationType(),
				expectedError:              "resource invalid ReporterResource",
			},
			{
				name:                       "empty resource type",
				idVal:                      fixture.ValidResourceIdType(),
				localResourceIdVal:         fixture.ValidLocalResourceId,
				resourceTypeVal:            fixture.EmptyResourceType,
				reporterTypeVal:            fixture.ValidReporterType,
				reporterInstanceIdVal:      fixture.ValidReporterInstanceId,
				resourceIdVal:              fixture.ValidResourceId,
				apiHrefVal:                 fixture.ValidApiHref,
				consoleHrefVal:             fixture.ValidConsoleHref,
				reporterRepresentationData: fixture.ValidReporterRepresentationType(),
				commonRepresentationData:   fixture.ValidCommonRepresentationType(),
				expectedError:              "resource invalid ReporterResource",
			},
			{
				name:                       "empty local resource id",
				idVal:                      fixture.ValidResourceIdType(),
				localResourceIdVal:         fixture.EmptyLocalResourceId,
				resourceTypeVal:            fixture.ValidResourceType,
				reporterTypeVal:            fixture.ValidReporterType,
				reporterInstanceIdVal:      fixture.ValidReporterInstanceId,
				resourceIdVal:              fixture.ValidResourceId,
				apiHrefVal:                 fixture.ValidApiHref,
				consoleHrefVal:             fixture.ValidConsoleHref,
				reporterRepresentationData: fixture.ValidReporterRepresentationType(),
				commonRepresentationData:   fixture.ValidCommonRepresentationType(),
				expectedError:              "resource invalid ReporterResource",
			},
			{
				name:                       "empty reporter type",
				idVal:                      fixture.ValidResourceIdType(),
				localResourceIdVal:         fixture.ValidLocalResourceId,
				resourceTypeVal:            fixture.ValidResourceType,
				reporterTypeVal:            fixture.EmptyReporterType,
				reporterInstanceIdVal:      fixture.ValidReporterInstanceId,
				resourceIdVal:              fixture.ValidResourceId,
				apiHrefVal:                 fixture.ValidApiHref,
				consoleHrefVal:             fixture.ValidConsoleHref,
				reporterRepresentationData: fixture.ValidReporterRepresentationType(),
				commonRepresentationData:   fixture.ValidCommonRepresentationType(),
				expectedError:              "resource invalid ReporterResource",
			},
			{
				name:                       "empty api href",
				idVal:                      fixture.ValidResourceIdType(),
				localResourceIdVal:         fixture.ValidLocalResourceId,
				resourceTypeVal:            fixture.ValidResourceType,
				reporterTypeVal:            fixture.ValidReporterType,
				reporterInstanceIdVal:      fixture.ValidReporterInstanceId,
				resourceIdVal:              fixture.ValidResourceId,
				apiHrefVal:                 fixture.EmptyApiHref,
				consoleHrefVal:             fixture.ValidConsoleHref,
				reporterRepresentationData: fixture.ValidReporterRepresentationType(),
				commonRepresentationData:   fixture.ValidCommonRepresentationType(),
				expectedError:              "resource invalid ReporterResource",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				_, err := NewResource(
					tc.idVal,
					tc.localResourceIdVal,
					tc.resourceTypeVal,
					tc.reporterTypeVal,
					tc.reporterInstanceIdVal,
					tc.resourceIdVal,
					tc.apiHrefVal,
					tc.consoleHrefVal,
					tc.reporterRepresentationData,
					tc.commonRepresentationData,
				)

				assertInvalidResource(t, err, tc.expectedError)
			})
		}
	})
}

func assertResourceEvent(t *testing.T, resource Resource, context string) {
	t.Helper()
	if len(resource.resourceEvents) == 0 {
		t.Errorf("Expected resource to have at least one ResourceEvent for %s", context)
		return
	}
	event := resource.resourceEvents[0]
	if event.id.String() == "" {
		t.Errorf("Expected ResourceEvent to have valid resource ID for %s", context)
	}
	if event.resourceType.String() == "" {
		t.Errorf("Expected ResourceEvent to have valid resource type for %s", context)
	}
}

func assertValidResource(t *testing.T, resource Resource, err error, context string) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error for %s, got %v", context, err)
	}
	if resource.id.String() == "" {
		t.Errorf("Expected valid resource ID for %s", context)
	}
	if resource.resourceType.String() == "" {
		t.Errorf("Expected valid resource type for %s", context)
	}
}

func assertInvalidResource(t *testing.T, err error, expectedErrorSubstring string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
	}
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error containing %s, got %v", expectedErrorSubstring, err)
	}
}

func assertInitialResourceState(t *testing.T, resource Resource) {
	t.Helper()

	if resource.commonVersion.Uint() != 0 {
		t.Errorf("Expected initial common version to be 0, got %d", resource.commonVersion.Uint())
	}
	if resource.consistencyToken != ConsistencyToken("") {
		t.Errorf("Expected consistency token to be zero value when first created, got %s", resource.consistencyToken.String())
	}
}

func TestResource_Update(t *testing.T) {
	t.Parallel()
	fixture := NewResourceTestFixture()

	t.Run("should update resource successfully", func(t *testing.T) {
		t.Parallel()

		// Create initial resource
		original, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		if err != nil {
			t.Fatalf("Expected no error creating Resource, got %v", err)
		}

		// Get the reporter resource key from the created resource
		reporterResourceKey := original.ReporterResources()[0].Key()

		// Update data
		newApiHref := "https://api.example.com/updated"
		newConsoleHref := "https://console.example.com/updated"
		newReporterData := internal.JsonObject{
			"name":      "updated-cluster",
			"namespace": "updated-namespace",
		}
		newCommonData := internal.JsonObject{
			"workspace_id": "updated-workspace",
			"labels":       internal.JsonObject{"env": "production"},
		}
		reporterVersion := "2.0.0"

		updated, err := original.Update(
			reporterResourceKey,
			newApiHref,
			newConsoleHref,
			&reporterVersion,
			newCommonData,
			newReporterData,
		)

		if err != nil {
			t.Fatalf("Expected no error updating Resource, got %v", err)
		}

		// Verify common version was incremented
		expectedCommonVersion := original.commonVersion.Uint() + 1
		if updated.commonVersion.Uint() != expectedCommonVersion {
			t.Errorf("Expected common version %d, got %d", expectedCommonVersion, updated.commonVersion.Uint())
		}

		// Verify reporter resource was updated
		updatedReporterResources := updated.ReporterResources()
		if len(updatedReporterResources) != 1 {
			t.Fatalf("Expected 1 reporter resource, got %d", len(updatedReporterResources))
		}

		updatedReporterResource := updatedReporterResources[0]
		if updatedReporterResource.apiHref.String() != newApiHref {
			t.Errorf("Expected apiHref %s, got %s", newApiHref, updatedReporterResource.apiHref.String())
		}
		if updatedReporterResource.consoleHref.String() != newConsoleHref {
			t.Errorf("Expected consoleHref %s, got %s", newConsoleHref, updatedReporterResource.consoleHref.String())
		}

		// Verify resource event was created
		resourceEvents := updated.ResourceEvents()
		if len(resourceEvents) != 2 { // Original + updated
			t.Fatalf("Expected 2 resource events, got %d", len(resourceEvents))
		}
	})

	t.Run("should return error for non-existent reporter resource key", func(t *testing.T) {
		t.Parallel()

		original, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		if err != nil {
			t.Fatalf("Expected no error creating Resource, got %v", err)
		}

		localResourceId, _ := NewLocalResourceId("non-existent")
		resourceType, _ := NewResourceType("unknown")
		reporterType, _ := NewReporterType("test")
		reporterInstanceId, _ := NewReporterInstanceId("instance1")

		nonExistentKey, err := NewReporterResourceKey(localResourceId, resourceType, reporterType, reporterInstanceId)
		if err != nil {
			t.Fatalf("Expected no error creating non-existent key, got %v", err)
		}

		_, err = original.Update(
			nonExistentKey,
			"https://api.example.com/updated",
			"https://console.example.com/updated",
			nil,
			internal.JsonObject{"test": "data"},
			internal.JsonObject{"test": "data"},
		)

		if err == nil {
			t.Error("Expected error for non-existent reporter resource key, got none")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected error about not found, got %v", err)
		}
	})

	t.Run("should preserve other resource fields", func(t *testing.T) {
		t.Parallel()

		original, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		if err != nil {
			t.Fatalf("Expected no error creating Resource, got %v", err)
		}

		reporterResourceKey := original.ReporterResources()[0].Key()

		updated, err := original.Update(
			reporterResourceKey,
			"https://api.example.com/updated",
			"https://console.example.com/updated",
			nil,
			internal.JsonObject{"test": "data"},
			internal.JsonObject{"test": "data"},
		)

		if err != nil {
			t.Fatalf("Expected no error updating Resource, got %v", err)
		}

		// Verify unchanged fields
		if updated.id != original.id {
			t.Errorf("Expected ID to remain unchanged")
		}
		if updated.resourceType != original.resourceType {
			t.Errorf("Expected resourceType to remain unchanged")
		}
		if updated.consistencyToken != original.consistencyToken {
			t.Errorf("Expected consistencyToken to remain unchanged")
		}
	})

	t.Run("should handle empty console href", func(t *testing.T) {
		t.Parallel()

		original, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		if err != nil {
			t.Fatalf("Expected no error creating Resource, got %v", err)
		}

		reporterResourceKey := original.ReporterResources()[0].Key()

		updated, err := original.Update(
			reporterResourceKey,
			"https://api.example.com/updated",
			"", // Empty console href
			nil,
			internal.JsonObject{"test": "data"},
			internal.JsonObject{"test": "data"},
		)

		if err != nil {
			t.Fatalf("Expected no error updating Resource with empty console href, got %v", err)
		}

		updatedReporterResource := updated.ReporterResources()[0]
		if updatedReporterResource.consoleHref.String() != "" {
			t.Errorf("Expected empty consoleHref, got %s", updatedReporterResource.consoleHref.String())
		}
	})

	t.Run("should return error for invalid apiHref", func(t *testing.T) {
		t.Parallel()

		original, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		if err != nil {
			t.Fatalf("Expected no error creating Resource, got %v", err)
		}

		reporterResourceKey := original.ReporterResources()[0].Key()

		_, err = original.Update(
			reporterResourceKey,
			"", // Invalid empty apiHref
			"https://console.example.com/updated",
			nil,
			internal.JsonObject{"test": "data"},
			internal.JsonObject{"test": "data"},
		)

		if err == nil {
			t.Error("Expected error for invalid apiHref, got none")
		}
	})

	t.Run("should handle nil reporter version", func(t *testing.T) {
		t.Parallel()

		original, err := NewResource(
			fixture.ValidResourceIdType(),
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationType(),
			fixture.ValidCommonRepresentationType(),
		)

		if err != nil {
			t.Fatalf("Expected no error creating Resource, got %v", err)
		}

		reporterResourceKey := original.ReporterResources()[0].Key()

		updated, err := original.Update(
			reporterResourceKey,
			"https://api.example.com/updated",
			"https://console.example.com/updated",
			nil, // nil reporter version
			internal.JsonObject{"test": "data"},
			internal.JsonObject{"test": "data"},
		)

		if err != nil {
			t.Fatalf("Expected no error updating Resource with nil reporter version, got %v", err)
		}

		// Should succeed without error
		if len(updated.ResourceEvents()) != 2 {
			t.Errorf("Expected 2 resource events, got %d", len(updated.ResourceEvents()))
		}
	})
}
