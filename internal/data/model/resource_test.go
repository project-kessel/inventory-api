package model

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Helper function to check if a Resource is valid
// This is used in infrastructure tests that need to verify existing objects
func isValidResource(r *Resource) bool {
	return r != nil && r.ID != uuid.Nil && r.Type != ""
}

func TestResource_Infrastructure_Structure(t *testing.T) {
	t.Parallel()

	t.Run("should have all required fields with correct types", func(t *testing.T) {
		t.Parallel()

		r := &Resource{}

		// Test field types
		AssertFieldType(t, r, "ID", reflect.TypeOf(uuid.UUID{}))
		AssertFieldType(t, r, "Type", reflect.TypeOf(""))
		AssertFieldType(t, r, "CommonVersion", reflect.TypeOf(uint(0)))
		AssertFieldType(t, r, "ConsistencyToken", reflect.TypeOf(""))
		AssertFieldType(t, r, "CreatedAt", reflect.TypeOf(time.Time{}))
		AssertFieldType(t, r, "UpdatedAt", reflect.TypeOf(time.Time{}))
	})

	t.Run("should have correct GORM tags for primary key", func(t *testing.T) {
		t.Parallel()

		r := &Resource{}

		// Check primary key fields have correct GORM tags
		AssertGORMTag(t, r, "ID", "type:uuid;primaryKey")
		AssertGORMTag(t, r, "CommonVersion", "type:bigint;check:common_version >= 0")
	})

	t.Run("should have correct GORM size constraints", func(t *testing.T) {
		t.Parallel()

		r := &Resource{}

		// Verify size constraints
		AssertGORMTag(t, r, "Type", "size:128;not null;")
		AssertGORMTag(t, r, "ConsistencyToken", "size:1024;column:ktn;")
	})

	t.Run("should have correct non-nullable field types", func(t *testing.T) {
		t.Parallel()

		r := &Resource{}

		// All required fields should be non-nullable
		AssertFieldType(t, r, "ID", reflect.TypeOf(uuid.UUID{}))
		AssertFieldType(t, r, "Type", reflect.TypeOf(""))
		AssertFieldType(t, r, "CommonVersion", reflect.TypeOf(uint(0)))
	})

}

func TestResource_Infrastructure_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("should handle zero values", func(t *testing.T) {
		t.Parallel()

		r := &Resource{}

		// Zero values should be acceptable for optional fields
		AssertEqual(t, uuid.Nil, r.ID, "ID should default to nil UUID")
		AssertEqual(t, "", r.Type, "Type should default to empty string")
		AssertEqual(t, uint(0), r.CommonVersion, "CommonVersion should default to 0")
		AssertEqual(t, "", r.ConsistencyToken, "ConsistencyToken should default to empty string")
	})

	t.Run("should handle unicode characters in Type", func(t *testing.T) {
		t.Parallel()

		r := &Resource{
			ID:               uuid.New(),
			Type:             "资源类型",
			CommonVersion:    1,
			ConsistencyToken: "token-with-unicode-字符",
		}

		if !isValidResource(r) {
			t.Error("Resource with unicode characters should be valid")
		}
	})

	t.Run("should handle maximum length strings", func(t *testing.T) {
		t.Parallel()

		// Test with maximum allowed sizes
		maxType := make([]byte, 128)
		for i := range maxType {
			maxType[i] = 'A'
		}

		maxToken := make([]byte, 1024)
		for i := range maxToken {
			maxToken[i] = 'B'
		}

		r := &Resource{
			ID:               uuid.New(),
			Type:             string(maxType),
			CommonVersion:    999999,
			ConsistencyToken: string(maxToken),
		}

		if !isValidResource(r) {
			t.Error("Resource with maximum length strings should be valid")
		}
	})
}

func TestResource_Infrastructure_Validation(t *testing.T) {
	t.Parallel()

	t.Run("should validate required fields", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name     string
			resource Resource
			isValid  bool
		}{
			{
				name: "valid resource",
				resource: Resource{
					ID:            uuid.New(),
					Type:          "k8s_cluster",
					CommonVersion: 1,
				},
				isValid: true,
			},
			{
				name: "nil ID",
				resource: Resource{
					ID:            uuid.Nil,
					Type:          "k8s_cluster",
					CommonVersion: 1,
				},
				isValid: false,
			},
			{
				name: "empty type",
				resource: Resource{
					ID:            uuid.New(),
					Type:          "",
					CommonVersion: 1,
				},
				isValid: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				actual := isValidResource(&tc.resource)
				if actual != tc.isValid {
					t.Errorf("Expected isValid=%v, got %v for %s", tc.isValid, actual, tc.name)
				}
			})
		}
	})
}

func TestResource_Infrastructure_Timestamps(t *testing.T) {
	t.Parallel()

	t.Run("should have timestamp fields", func(t *testing.T) {
		t.Parallel()

		r := &Resource{}

		// Verify timestamp fields exist
		AssertFieldType(t, r, "CreatedAt", reflect.TypeOf(time.Time{}))
		AssertFieldType(t, r, "UpdatedAt", reflect.TypeOf(time.Time{}))
	})

	t.Run("should handle timestamp operations", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		r := &Resource{
			ID:            uuid.New(),
			Type:          "test_resource",
			CommonVersion: 1,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		AssertEqual(t, now, r.CreatedAt, "CreatedAt should be set correctly")
		AssertEqual(t, now, r.UpdatedAt, "UpdatedAt should be set correctly")
	})
}
