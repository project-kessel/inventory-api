package model

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Helper function to check if a ReporterResource is valid
// This is used in infrastructure tests that need to verify existing objects
func isValidReporterResource(rr *ReporterResource) bool {
	return rr != nil && rr.ID != uuid.Nil && rr.LocalResourceID != "" &&
		rr.ReporterType != "" && rr.ResourceType != "" && rr.ReporterInstanceID != "" &&
		rr.ResourceID != uuid.Nil && rr.APIHref != ""
}

// Helper function to check if a ReporterResourceKey is valid
func isValidReporterResourceKey(key *ReporterResourceKey) bool {
	return key != nil && key.LocalResourceID != "" && key.ReporterType != "" &&
		key.ResourceType != "" && key.ReporterInstanceID != ""
}

func TestReporterResource_Infrastructure_Structure(t *testing.T) {
	t.Parallel()

	t.Run("should have all required fields with correct types", func(t *testing.T) {
		t.Parallel()

		rr := &ReporterResource{}

		// Test field types
		AssertFieldType(t, rr, "ID", reflect.TypeOf(uuid.UUID{}))
		AssertFieldType(t, rr, "LocalResourceID", reflect.TypeOf(""))
		AssertFieldType(t, rr, "ReporterType", reflect.TypeOf(""))
		AssertFieldType(t, rr, "ResourceType", reflect.TypeOf(""))
		AssertFieldType(t, rr, "ReporterInstanceID", reflect.TypeOf(""))
		AssertFieldType(t, rr, "ResourceID", reflect.TypeOf(uuid.UUID{}))
		AssertFieldType(t, rr, "APIHref", reflect.TypeOf(""))
		AssertFieldType(t, rr, "ConsoleHref", reflect.TypeOf(""))
		AssertFieldType(t, rr, "RepresentationVersion", reflect.TypeOf(uint(0)))
		AssertFieldType(t, rr, "Generation", reflect.TypeOf(uint(0)))
		AssertFieldType(t, rr, "Tombstone", reflect.TypeOf(false))
		AssertFieldType(t, rr, "CreatedAt", reflect.TypeOf(time.Time{}))
		AssertFieldType(t, rr, "UpdatedAt", reflect.TypeOf(time.Time{}))
	})

	t.Run("should properly embed ReporterResourceKey", func(t *testing.T) {
		t.Parallel()

		rr := &ReporterResource{}

		// Check if ReporterResource embeds ReporterResourceKey
		rrType := reflect.TypeOf(rr).Elem()
		field, found := rrType.FieldByName("ReporterResourceKey")
		if !found {
			t.Error("ReporterResource should embed ReporterResourceKey")
			return
		}

		if field.Type != reflect.TypeOf(ReporterResourceKey{}) {
			t.Errorf("Expected ReporterResourceKey type, got %v", field.Type)
		}

		// Verify anonymous embedding
		if !field.Anonymous {
			t.Error("ReporterResourceKey should be anonymously embedded")
		}
	})

	t.Run("should have correct GORM tags for primary key", func(t *testing.T) {
		t.Parallel()

		rr := &ReporterResource{}

		// Check primary key field has correct GORM tags
		AssertGORMTag(t, rr, "ID", "type:uuid;primaryKey")
	})

	t.Run("should have correct GORM size constraints", func(t *testing.T) {
		t.Parallel()

		rr := &ReporterResource{}

		// Verify size constraints match constants
		AssertGORMTag(t, rr, "LocalResourceID", "size:256;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:1;not null")
		AssertGORMTag(t, rr, "ReporterType", "size:128;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:2;not null")
		AssertGORMTag(t, rr, "ResourceType", "size:128;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:3;not null")
		AssertGORMTag(t, rr, "ReporterInstanceID", "size:256;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:4;not null")
		AssertGORMTag(t, rr, "APIHref", "size:512;not null")
		AssertGORMTag(t, rr, "ConsoleHref", "size:512")
	})

	t.Run("should have correct unique index constraints", func(t *testing.T) {
		t.Parallel()

		rr := &ReporterResource{}

		// Check unique index fields
		AssertGORMTag(t, rr, "RepresentationVersion", "index:reporter_resource_key_idx,unique;not null")
		AssertGORMTag(t, rr, "Generation", "index:reporter_resource_key_idx,unique;not null")
	})

	t.Run("should have correct non-nullable field types", func(t *testing.T) {
		t.Parallel()

		rr := &ReporterResource{}

		// All required fields should be non-nullable
		AssertFieldType(t, rr, "ID", reflect.TypeOf(uuid.UUID{}))
		AssertFieldType(t, rr, "LocalResourceID", reflect.TypeOf(""))
		AssertFieldType(t, rr, "ReporterType", reflect.TypeOf(""))
		AssertFieldType(t, rr, "ResourceType", reflect.TypeOf(""))
		AssertFieldType(t, rr, "ReporterInstanceID", reflect.TypeOf(""))
		AssertFieldType(t, rr, "ResourceID", reflect.TypeOf(uuid.UUID{}))
		AssertFieldType(t, rr, "APIHref", reflect.TypeOf(""))
		AssertFieldType(t, rr, "Tombstone", reflect.TypeOf(false))
	})
}

func TestReporterResourceKey_Infrastructure_Structure(t *testing.T) {
	t.Parallel()

	t.Run("should have all required fields with correct types", func(t *testing.T) {
		t.Parallel()

		key := &ReporterResourceKey{}

		// Test field types
		AssertFieldType(t, key, "LocalResourceID", reflect.TypeOf(""))
		AssertFieldType(t, key, "ReporterType", reflect.TypeOf(""))
		AssertFieldType(t, key, "ResourceType", reflect.TypeOf(""))
		AssertFieldType(t, key, "ReporterInstanceID", reflect.TypeOf(""))
	})

	t.Run("should have correct GORM index tags", func(t *testing.T) {
		t.Parallel()

		key := &ReporterResourceKey{}

		// Check index tags for composite key
		AssertGORMTag(t, key, "LocalResourceID", "size:256;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:1;not null")
		AssertGORMTag(t, key, "ReporterType", "size:128;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:2;not null")
		AssertGORMTag(t, key, "ResourceType", "size:128;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:3;not null")
		AssertGORMTag(t, key, "ReporterInstanceID", "size:256;index:reporter_resource_key_idx,unique;index:reporter_resource_search_idx,priority:4;not null")
	})
}

func TestReporterResource_Infrastructure_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle zero values", func(t *testing.T) {
		t.Parallel()

		rr := &ReporterResource{}

		// Zero values should be acceptable for optional fields
		AssertEqual(t, uuid.Nil, rr.ID, "ID should default to nil UUID")
		AssertEqual(t, "", rr.LocalResourceID, "LocalResourceID should default to empty string")
		AssertEqual(t, "", rr.ConsoleHref, "ConsoleHref should default to empty string")
		AssertEqual(t, uint(0), rr.RepresentationVersion, "RepresentationVersion should default to 0")
		AssertEqual(t, uint(0), rr.Generation, "Generation should default to 0")
		AssertEqual(t, false, rr.Tombstone, "Tombstone should default to false")
	})

	t.Run("should handle unicode characters", func(t *testing.T) {
		t.Parallel()

		rr := &ReporterResource{
			ID: uuid.New(),
			ReporterResourceKey: ReporterResourceKey{
				LocalResourceID:    "资源-123",
				ReporterType:       "类型",
				ResourceType:       "k8s_cluster",
				ReporterInstanceID: "实例-abc",
			},
			ResourceID:            uuid.New(),
			APIHref:               "https://api.example.com/资源",
			ConsoleHref:           "https://console.example.com/资源",
			RepresentationVersion: 1,
			Generation:            0,
			Tombstone:             false,
		}

		if !isValidReporterResource(rr) {
			t.Error("ReporterResource with unicode characters should be valid")
		}
	})

	t.Run("should handle maximum length strings", func(t *testing.T) {
		t.Parallel()

		// Test with maximum allowed sizes
		maxLocalResourceID := make([]byte, 256)
		for i := range maxLocalResourceID {
			maxLocalResourceID[i] = 'A'
		}

		maxReporterType := make([]byte, 128)
		for i := range maxReporterType {
			maxReporterType[i] = 'B'
		}

		maxResourceType := make([]byte, 128)
		for i := range maxResourceType {
			maxResourceType[i] = 'C'
		}

		maxReporterInstanceID := make([]byte, 256)
		for i := range maxReporterInstanceID {
			maxReporterInstanceID[i] = 'D'
		}

		maxAPIHref := make([]byte, 512)
		for i := range maxAPIHref {
			maxAPIHref[i] = 'E'
		}

		maxConsoleHref := make([]byte, 512)
		for i := range maxConsoleHref {
			maxConsoleHref[i] = 'F'
		}

		rr := &ReporterResource{
			ID: uuid.New(),
			ReporterResourceKey: ReporterResourceKey{
				LocalResourceID:    string(maxLocalResourceID),
				ReporterType:       string(maxReporterType),
				ResourceType:       string(maxResourceType),
				ReporterInstanceID: string(maxReporterInstanceID),
			},
			ResourceID:            uuid.New(),
			APIHref:               string(maxAPIHref),
			ConsoleHref:           string(maxConsoleHref),
			RepresentationVersion: 999999,
			Generation:            999999,
			Tombstone:             true,
		}

		if !isValidReporterResource(rr) {
			t.Error("ReporterResource with maximum length strings should be valid")
		}
	})
}

func TestReporterResource_Infrastructure_Validation(t *testing.T) {
	t.Parallel()

	t.Run("should validate required fields", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name             string
			reporterResource ReporterResource
			isValid          bool
		}{
			{
				name: "valid reporter resource",
				reporterResource: ReporterResource{
					ID: uuid.New(),
					ReporterResourceKey: ReporterResourceKey{
						LocalResourceID:    "resource-123",
						ReporterType:       "acm",
						ResourceType:       "k8s_cluster",
						ReporterInstanceID: "instance-abc",
					},
					ResourceID:            uuid.New(),
					APIHref:               "https://api.example.com/resource",
					RepresentationVersion: 1,
					Generation:            0,
					Tombstone:             false,
				},
				isValid: true,
			},
			{
				name: "nil ID",
				reporterResource: ReporterResource{
					ID: uuid.Nil,
					ReporterResourceKey: ReporterResourceKey{
						LocalResourceID:    "resource-123",
						ReporterType:       "acm",
						ResourceType:       "k8s_cluster",
						ReporterInstanceID: "instance-abc",
					},
					ResourceID: uuid.New(),
					APIHref:    "https://api.example.com/resource",
				},
				isValid: false,
			},
			{
				name: "empty LocalResourceID",
				reporterResource: ReporterResource{
					ID: uuid.New(),
					ReporterResourceKey: ReporterResourceKey{
						LocalResourceID:    "",
						ReporterType:       "acm",
						ResourceType:       "k8s_cluster",
						ReporterInstanceID: "instance-abc",
					},
					ResourceID: uuid.New(),
					APIHref:    "https://api.example.com/resource",
				},
				isValid: false,
			},
			{
				name: "empty APIHref",
				reporterResource: ReporterResource{
					ID: uuid.New(),
					ReporterResourceKey: ReporterResourceKey{
						LocalResourceID:    "resource-123",
						ReporterType:       "acm",
						ResourceType:       "k8s_cluster",
						ReporterInstanceID: "instance-abc",
					},
					ResourceID: uuid.New(),
					APIHref:    "",
				},
				isValid: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				actual := isValidReporterResource(&tc.reporterResource)
				if actual != tc.isValid {
					t.Errorf("Expected isValid=%v, got %v for %s", tc.isValid, actual, tc.name)
				}
			})
		}
	})

	t.Run("should validate ReporterResourceKey fields", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name    string
			key     ReporterResourceKey
			isValid bool
		}{
			{
				name: "valid key",
				key: ReporterResourceKey{
					LocalResourceID:    "resource-123",
					ReporterType:       "acm",
					ResourceType:       "k8s_cluster",
					ReporterInstanceID: "instance-abc",
				},
				isValid: true,
			},
			{
				name: "empty LocalResourceID",
				key: ReporterResourceKey{
					LocalResourceID:    "",
					ReporterType:       "acm",
					ResourceType:       "k8s_cluster",
					ReporterInstanceID: "instance-abc",
				},
				isValid: false,
			},
			{
				name: "empty ReporterType",
				key: ReporterResourceKey{
					LocalResourceID:    "resource-123",
					ReporterType:       "",
					ResourceType:       "k8s_cluster",
					ReporterInstanceID: "instance-abc",
				},
				isValid: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				actual := isValidReporterResourceKey(&tc.key)
				if actual != tc.isValid {
					t.Errorf("Expected isValid=%v, got %v for %s", tc.isValid, actual, tc.name)
				}
			})
		}
	})
}

func TestReporterResource_Infrastructure_Timestamps(t *testing.T) {
	t.Parallel()

	t.Run("should have timestamp fields", func(t *testing.T) {
		t.Parallel()

		rr := &ReporterResource{}

		// Verify timestamp fields exist
		AssertFieldType(t, rr, "CreatedAt", reflect.TypeOf(time.Time{}))
		AssertFieldType(t, rr, "UpdatedAt", reflect.TypeOf(time.Time{}))
	})

	t.Run("should handle timestamp operations", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		rr := &ReporterResource{
			ID: uuid.New(),
			ReporterResourceKey: ReporterResourceKey{
				LocalResourceID:    "resource-123",
				ReporterType:       "acm",
				ResourceType:       "k8s_cluster",
				ReporterInstanceID: "instance-abc",
			},
			ResourceID: uuid.New(),
			APIHref:    "https://api.example.com/resource",
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		AssertEqual(t, now, rr.CreatedAt, "CreatedAt should be set correctly")
		AssertEqual(t, now, rr.UpdatedAt, "UpdatedAt should be set correctly")
	})
}

func TestReporterResource_Infrastructure_Constructor(t *testing.T) {
	t.Parallel()

	t.Run("should have NewReporterResource constructor", func(t *testing.T) {
		t.Parallel()

		id := uuid.New()
		resourceID := uuid.New()

		rr, err := NewReporterResource(
			id,
			"resource-123",
			"acm",
			"k8s_cluster",
			"instance-abc",
			resourceID,
			"https://api.example.com/resource",
			"https://console.example.com/resource",
			1,
			0,
			false,
		)

		AssertNoError(t, err, "NewReporterResource should not return error for valid inputs")
		AssertEqual(t, id, rr.ID, "ID should be set correctly")
		AssertEqual(t, "resource-123", rr.LocalResourceID, "LocalResourceID should be set correctly")
		AssertEqual(t, "acm", rr.ReporterType, "ReporterType should be set correctly")
		AssertEqual(t, "k8s_cluster", rr.ResourceType, "ResourceType should be set correctly")
		AssertEqual(t, "instance-abc", rr.ReporterInstanceID, "ReporterInstanceID should be set correctly")
		AssertEqual(t, resourceID, rr.ResourceID, "ResourceID should be set correctly")
		AssertEqual(t, "https://api.example.com/resource", rr.APIHref, "APIHref should be set correctly")
		AssertEqual(t, "https://console.example.com/resource", rr.ConsoleHref, "ConsoleHref should be set correctly")
		AssertEqual(t, uint(1), rr.RepresentationVersion, "RepresentationVersion should be set correctly")
		AssertEqual(t, uint(0), rr.Generation, "Generation should be set correctly")
		AssertEqual(t, false, rr.Tombstone, "Tombstone should be set correctly")
	})

	t.Run("should validate constructor inputs", func(t *testing.T) {
		t.Parallel()

		// Test with invalid inputs
		_, err := NewReporterResource(
			uuid.Nil, // Invalid ID
			"resource-123",
			"acm",
			"k8s_cluster",
			"instance-abc",
			uuid.New(),
			"https://api.example.com/resource",
			"https://console.example.com/resource",
			1,
			0,
			false,
		)

		AssertError(t, err, "NewReporterResource should return error for nil ID")

		// Test with empty required string
		_, err = NewReporterResource(
			uuid.New(),
			"", // Invalid empty LocalResourceID
			"acm",
			"k8s_cluster",
			"instance-abc",
			uuid.New(),
			"https://api.example.com/resource",
			"https://console.example.com/resource",
			1,
			0,
			false,
		)

		AssertError(t, err, "NewReporterResource should return error for empty LocalResourceID")
	})
}
