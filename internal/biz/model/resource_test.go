//go:build test

package model

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func createResourceWithFixture(fixture ResourceTestFixture, id uuid.UUID, resourceType string, reporterResources []ReporterResource) (Resource, error) {
	resourceId, err := NewResourceId(id)
	if err != nil {
		return Resource{}, err
	}
	resourceTypeObj, err := NewResourceType(resourceType)
	if err != nil {
		return Resource{}, err
	}
	return NewResource(resourceId, resourceTypeObj, reporterResources)
}

func TestResource_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewResourceTestFixture()

	t.Run("should create resource with single reporter resource", func(t *testing.T) {
		t.Parallel()

		resourceId, _ := NewResourceId(fixture.ValidId)
		resourceType, _ := NewResourceType(fixture.ValidResourceType)

		resource, err := NewResource(
			resourceId,
			resourceType,
			[]ReporterResource{fixture.ValidReporterResource},
		)

		assertValidResource(t, resource, err, "single reporter resource")
		assertInitialResourceState(t, resource)
	})

	t.Run("should create resource with multiple reporter resources", func(t *testing.T) {
		t.Parallel()

		resourceId, _ := NewResourceId(fixture.ValidId)
		resourceType, _ := NewResourceType(fixture.ValidResourceType)

		resource, err := NewResource(
			resourceId,
			resourceType,
			fixture.MultipleReporterResources,
		)

		assertValidResource(t, resource, err, "multiple reporter resources")
		assertInitialResourceState(t, resource)
		if len(resource.reporterResources) != 2 {
			t.Errorf("Expected 2 reporter resources, got %d", len(resource.reporterResources))
		}
	})

	t.Run("should create resource with different resource type", func(t *testing.T) {
		t.Parallel()

		resource, err := NewResource(
			fixture.ValidId,
			fixture.AnotherResourceType,
			[]ReporterResource{fixture.AnotherReporterResource},
		)

		assertValidResource(t, resource, err, "different resource type")
		assertInitialResourceState(t, resource)
	})

	t.Run("should reject nil resource id", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.NilId,
			fixture.ValidResourceType,
			[]ReporterResource{fixture.ValidReporterResource},
		)

		assertInvalidResource(t, err, "Resource invalid ID")
	})

	t.Run("should reject empty resource type", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidId,
			fixture.EmptyResourceType,
			[]ReporterResource{fixture.ValidReporterResource},
		)

		assertInvalidResource(t, err, "Resource invalid type")
	})

	t.Run("should reject whitespace-only resource type", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidId,
			fixture.WhitespaceResourceType,
			[]ReporterResource{fixture.ValidReporterResource},
		)

		assertInvalidResource(t, err, "Resource invalid type")
	})

	t.Run("should reject empty reporter resources", func(t *testing.T) {
		t.Parallel()

		_, err := NewResource(
			fixture.ValidId,
			fixture.ValidResourceType,
			fixture.EmptyReporterResources,
		)

		assertInvalidResource(t, err, "Resource must have at least one ReporterResource")
	})
}

func TestResource_AggregateRootBehavior(t *testing.T) {
	t.Parallel()
	fixture := NewResourceTestFixture()

	t.Run("should enforce initial state invariants", func(t *testing.T) {
		t.Parallel()

		resource, err := NewResource(
			fixture.ValidId,
			fixture.ValidResourceType,
			[]ReporterResource{fixture.ValidReporterResource},
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
			name              string
			id                uuid.UUID
			resourceType      string
			reporterResources []ReporterResource
			expectedError     string
		}{
			{
				name:              "nil id",
				id:                fixture.NilId,
				resourceType:      fixture.ValidResourceType,
				reporterResources: []ReporterResource{fixture.ValidReporterResource},
				expectedError:     "Resource invalid ID",
			},
			{
				name:              "empty resource type",
				id:                fixture.ValidId,
				resourceType:      fixture.EmptyResourceType,
				reporterResources: []ReporterResource{fixture.ValidReporterResource},
				expectedError:     "Resource invalid type",
			},
			{
				name:              "empty reporter resources",
				id:                fixture.ValidId,
				resourceType:      fixture.ValidResourceType,
				reporterResources: fixture.EmptyReporterResources,
				expectedError:     "Resource must have at least one ReporterResource",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				_, err := NewResource(tc.id, tc.resourceType, tc.reporterResources)

				assertInvalidResource(t, err, tc.expectedError)
			})
		}
	})
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
