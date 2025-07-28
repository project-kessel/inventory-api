package model

import (
	"testing"

	"github.com/google/uuid"
	model2 "github.com/project-kessel/inventory-api/internal/data/model"
)

func TestReporterResource_NewReporterResource(t *testing.T) {
	t.Run("should create valid ReporterResource", func(t *testing.T) {
		id := uuid.New()
		resourceID := uuid.New()

		rr, err := model2.NewReporterResource(
			id,
			"local-123",
			"hbi",
			"host",
			"instance-456",
			resourceID,
			"https://api.example.com",
			"https://console.example.com",
			1,
			2,
			false,
		)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if rr.ID != id {
			t.Errorf("Expected ID %v, got %v", id, rr.ID)
		}

		if rr.LocalResourceID != "local-123" {
			t.Errorf("Expected LocalResourceID 'local-123', got '%s'", rr.LocalResourceID)
		}

		if rr.ReporterType != "hbi" {
			t.Errorf("Expected ReporterType 'hbi', got '%s'", rr.ReporterType)
		}

		if rr.ResourceType != "host" {
			t.Errorf("Expected ResourceType 'host', got '%s'", rr.ResourceType)
		}

		if rr.ReporterInstanceID != "instance-456" {
			t.Errorf("Expected ReporterInstanceID 'instance-456', got '%s'", rr.ReporterInstanceID)
		}

		if rr.ResourceID != resourceID {
			t.Errorf("Expected ResourceID %v, got %v", resourceID, rr.ResourceID)
		}

		if rr.RepresentationVersion != 1 {
			t.Errorf("Expected RepresentationVersion 1, got %d", rr.RepresentationVersion)
		}

		if rr.Generation != 2 {
			t.Errorf("Expected Generation 2, got %d", rr.Generation)
		}

		if rr.Tombstone != false {
			t.Errorf("Expected Tombstone false, got %t", rr.Tombstone)
		}
	})

	t.Run("should validate required fields", func(t *testing.T) {
		validID := uuid.New()
		validResourceID := uuid.New()

		testCases := []struct {
			name               string
			id                 uuid.UUID
			localResourceID    string
			reporterType       string
			resourceType       string
			reporterInstanceID string
			resourceID         uuid.UUID
			expectedError      string
		}{
			{
				name:               "empty ID",
				id:                 uuid.Nil,
				localResourceID:    "valid",
				reporterType:       "valid",
				resourceType:       "valid",
				reporterInstanceID: "valid",
				resourceID:         validResourceID,
				expectedError:      "ID",
			},
			{
				name:               "empty LocalResourceID",
				id:                 validID,
				localResourceID:    "",
				reporterType:       "valid",
				resourceType:       "valid",
				reporterInstanceID: "valid",
				resourceID:         validResourceID,
				expectedError:      "LocalResourceID",
			},
			{
				name:               "empty ReporterType",
				id:                 validID,
				localResourceID:    "valid",
				reporterType:       "",
				resourceType:       "valid",
				reporterInstanceID: "valid",
				resourceID:         validResourceID,
				expectedError:      "ReporterType",
			},
			{
				name:               "empty ResourceType",
				id:                 validID,
				localResourceID:    "valid",
				reporterType:       "valid",
				resourceType:       "",
				reporterInstanceID: "valid",
				resourceID:         validResourceID,
				expectedError:      "ResourceType",
			},
			{
				name:               "empty ReporterInstanceID",
				id:                 validID,
				localResourceID:    "valid",
				reporterType:       "valid",
				resourceType:       "valid",
				reporterInstanceID: "",
				resourceID:         validResourceID,
				expectedError:      "ReporterInstanceID",
			},
			{
				name:               "empty ResourceID",
				id:                 validID,
				localResourceID:    "valid",
				reporterType:       "valid",
				resourceType:       "valid",
				reporterInstanceID: "valid",
				resourceID:         uuid.Nil,
				expectedError:      "ResourceID",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := model2.NewReporterResource(
					tc.id,
					tc.localResourceID,
					tc.reporterType,
					tc.resourceType,
					tc.reporterInstanceID,
					tc.resourceID,
					"https://api.example.com",
					"https://console.example.com",
					1,
					1,
					false,
				)

				if err == nil {
					t.Errorf("Expected validation error for %s", tc.name)
				}

				if err != nil && !contains(err.Error(), tc.expectedError) {
					t.Errorf("Expected error to contain '%s', got: %v", tc.expectedError, err)
				}
			})
		}
	})
}

// Helper function to check if a string contains another string
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
