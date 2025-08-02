//go:build test

package model

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestResource_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewResourceTestFixture()

	t.Run("should create resource with single reporter resource", func(t *testing.T) {
		t.Parallel()

		resource, err := NewResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationData,
			fixture.ValidCommonRepresentationData,
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
			fixture.ValidId,
			fixture.AnotherLocalResourceId,
			fixture.AnotherResourceType,
			fixture.AnotherReporterType,
			fixture.AnotherReporterInstanceId,
			fixture.AnotherResourceId,
			fixture.AnotherApiHref,
			fixture.EmptyConsoleHref,
			fixture.AnotherReporterRepresentationData,
			fixture.AnotherCommonRepresentationData,
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
			fixture.ValidId,
			fixture.AnotherLocalResourceId,
			fixture.AnotherResourceType,
			fixture.AnotherReporterType,
			fixture.AnotherReporterInstanceId,
			fixture.AnotherResourceId,
			fixture.AnotherApiHref,
			fixture.EmptyConsoleHref,
			fixture.AnotherReporterRepresentationData,
			fixture.AnotherCommonRepresentationData,
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
			fixture.ValidReporterRepresentationData,
			fixture.ValidCommonRepresentationData,
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty resource type", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.EmptyResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationData,
			fixture.ValidCommonRepresentationData,
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject whitespace-only resource type", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.WhitespaceResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationData,
			fixture.ValidCommonRepresentationData,
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty local resource id", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidId,
			fixture.EmptyLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationData,
			fixture.ValidCommonRepresentationData,
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty reporter type", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.EmptyReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationData,
			fixture.ValidCommonRepresentationData,
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty reporter instance id", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.EmptyReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationData,
			fixture.ValidCommonRepresentationData,
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty api href", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.EmptyApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationData,
			fixture.ValidCommonRepresentationData,
		)

		assertInvalidResource(t, err, "resource invalid ReporterResource")
	})

	t.Run("should reject empty reporter representation data", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.EmptyReporterRepresentationData,
			fixture.ValidCommonRepresentationData,
		)

		assertInvalidResource(t, err, "Resource invalid ResourceEvent")
	})

	t.Run("should reject empty common representation data", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationData,
			fixture.EmptyCommonRepresentationData,
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
			fixture.ValidId,
			fixture.ValidLocalResourceId,
			fixture.ValidResourceType,
			fixture.ValidReporterType,
			fixture.ValidReporterInstanceId,
			fixture.ValidResourceId,
			fixture.ValidApiHref,
			fixture.ValidConsoleHref,
			fixture.ValidReporterRepresentationData,
			fixture.ValidCommonRepresentationData,
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
			reporterRepresentationData JsonObject
			commonRepresentationData   JsonObject
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
				reporterRepresentationData: fixture.ValidReporterRepresentationData,
				commonRepresentationData:   fixture.ValidCommonRepresentationData,
				expectedError:              "resource invalid ReporterResource",
			},
			{
				name:                       "empty resource type",
				idVal:                      fixture.ValidId,
				localResourceIdVal:         fixture.ValidLocalResourceId,
				resourceTypeVal:            fixture.EmptyResourceType,
				reporterTypeVal:            fixture.ValidReporterType,
				reporterInstanceIdVal:      fixture.ValidReporterInstanceId,
				resourceIdVal:              fixture.ValidResourceId,
				apiHrefVal:                 fixture.ValidApiHref,
				consoleHrefVal:             fixture.ValidConsoleHref,
				reporterRepresentationData: fixture.ValidReporterRepresentationData,
				commonRepresentationData:   fixture.ValidCommonRepresentationData,
				expectedError:              "resource invalid ReporterResource",
			},
			{
				name:                       "empty local resource id",
				idVal:                      fixture.ValidId,
				localResourceIdVal:         fixture.EmptyLocalResourceId,
				resourceTypeVal:            fixture.ValidResourceType,
				reporterTypeVal:            fixture.ValidReporterType,
				reporterInstanceIdVal:      fixture.ValidReporterInstanceId,
				resourceIdVal:              fixture.ValidResourceId,
				apiHrefVal:                 fixture.ValidApiHref,
				consoleHrefVal:             fixture.ValidConsoleHref,
				reporterRepresentationData: fixture.ValidReporterRepresentationData,
				commonRepresentationData:   fixture.ValidCommonRepresentationData,
				expectedError:              "resource invalid ReporterResource",
			},
			{
				name:                       "empty reporter type",
				idVal:                      fixture.ValidId,
				localResourceIdVal:         fixture.ValidLocalResourceId,
				resourceTypeVal:            fixture.ValidResourceType,
				reporterTypeVal:            fixture.EmptyReporterType,
				reporterInstanceIdVal:      fixture.ValidReporterInstanceId,
				resourceIdVal:              fixture.ValidResourceId,
				apiHrefVal:                 fixture.ValidApiHref,
				consoleHrefVal:             fixture.ValidConsoleHref,
				reporterRepresentationData: fixture.ValidReporterRepresentationData,
				commonRepresentationData:   fixture.ValidCommonRepresentationData,
				expectedError:              "resource invalid ReporterResource",
			},
			{
				name:                       "empty api href",
				idVal:                      fixture.ValidId,
				localResourceIdVal:         fixture.ValidLocalResourceId,
				resourceTypeVal:            fixture.ValidResourceType,
				reporterTypeVal:            fixture.ValidReporterType,
				reporterInstanceIdVal:      fixture.ValidReporterInstanceId,
				resourceIdVal:              fixture.ValidResourceId,
				apiHrefVal:                 fixture.EmptyApiHref,
				consoleHrefVal:             fixture.ValidConsoleHref,
				reporterRepresentationData: fixture.ValidReporterRepresentationData,
				commonRepresentationData:   fixture.ValidCommonRepresentationData,
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
