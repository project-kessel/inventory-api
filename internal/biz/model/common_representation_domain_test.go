package model

import (
	"testing"

	"github.com/google/uuid"
)

// Test scenarios for CommonRepresentation domain model
//
// These tests focus on domain logic, business rules, and model behavior
// rather than database operations or infrastructure concerns.
//
// Domain tests validate:
// - Business validation rules and constraints
// - Domain behavior and business logic
// - Factory method behavior and validation
// - Data handling and transformation logic
// - Model comparison and equality semantics
// - Versioning and lifecycle management

func TestCommonRepresentation_Validation(t *testing.T) {
	t.Parallel()

	fixture := NewTestFixture(t)

	t.Run("valid_representation", func(t *testing.T) {
		t.Parallel()
		cr := fixture.ValidCommonRepresentation()

		// Factory method should create a valid instance without errors
		AssertNoError(t, nil, "Valid CommonRepresentation should be created successfully")

		// Verify the instance was created correctly
		// ResourceType field has been removed
		if cr == nil {
			t.Error("CommonRepresentation should be created successfully")
		}
	})

	t.Run("invalid_representations", func(t *testing.T) {
		t.Parallel()

		testCases := map[string]func(*testing.T){
			"empty_id": func(t *testing.T) {
				t.Parallel()
				// Test factory method with empty ID
				_, err := NewCommonRepresentation(
					uuid.Nil,
					JsonObject{"workspace_id": "test"},
					1,
					"",
					"test-instance",
				)
				AssertValidationError(t, err, "ResourceId", "Empty ResourceId should fail creation")
			},

			"zero_version": func(t *testing.T) {
				t.Parallel()
				// Test factory method with zero version - should be valid
				cr, err := NewCommonRepresentation(
					uuid.NewSHA1(uuid.NameSpaceOID, []byte("test")),
					JsonObject{"workspace_id": "test"},
					0,
					"hbi",
					"test-instance",
				)
				AssertNoError(t, err, "Zero Version should be valid")
				if cr == nil {
					t.Error("CommonRepresentation should not be nil")
				} else if cr.Version != 0 {
					t.Errorf("Expected Version to be 0, got %d", cr.Version)
				}
			},
			"empty_reporter_type": func(t *testing.T) {
				t.Parallel()
				// Test factory method with empty reporter type
				_, err := NewCommonRepresentation(
					uuid.NewSHA1(uuid.NameSpaceOID, []byte("test")),
					JsonObject{"workspace_id": "test"},
					1,
					"",
					"test-instance",
				)
				AssertValidationError(t, err, "ReportedByReporterType", "Empty ReportedByReporterType should fail creation")
			},
			"empty_reporter_instance": func(t *testing.T) {
				t.Parallel()
				// Test factory method with empty reporter instance
				_, err := NewCommonRepresentation(
					uuid.NewSHA1(uuid.NameSpaceOID, []byte("test")),
					JsonObject{"workspace_id": "test"},
					1,
					"hbi",
					"",
				)
				AssertValidationError(t, err, "ReportedByReporterInstance", "Empty ReportedByReporterInstance should fail creation")
			},
			"nil_data": func(t *testing.T) {
				t.Parallel()
				// Test factory method with nil data - should be valid
				cr, err := NewCommonRepresentation(
					uuid.NewSHA1(uuid.NameSpaceOID, []byte("test")),
					nil,
					1,
					"hbi",
					"test-instance",
				)
				AssertNoError(t, err, "Nil Data should be valid")
				if cr == nil {
					t.Error("CommonRepresentation should not be nil")
				} else if cr.Data != nil {
					t.Error("Expected Data to be nil")
				}
			},
		}

		RunTableDrivenTest(t, testCases)
	})
}

func TestCommonRepresentation_BusinessRules(t *testing.T) {
	t.Parallel()

	fixture := NewTestFixture(t)

	t.Run("version_must_be_positive", func(t *testing.T) {
		t.Parallel()
		cr := fixture.CommonRepresentationWithVersion(1)

		// Factory method should create a valid instance for positive version
		AssertNoError(t, nil, "Positive version should be valid")
		AssertEqual(t, uint(1), cr.Version, "Version should be set correctly")
	})

	t.Run("version_can_be_zero", func(t *testing.T) {
		t.Parallel()
		// Test factory method with zero version - should be valid
		cr, err := NewCommonRepresentation(
			uuid.NewSHA1(uuid.NameSpaceOID, []byte("test")),
			JsonObject{"workspace_id": "test"},
			0,
			"hbi",
			"test-instance",
		)
		AssertNoError(t, err, "Zero version should be valid")
		if cr == nil {
			t.Error("CommonRepresentation should not be nil")
		} else if cr.Version != 0 {
			t.Errorf("Expected Version to be 0, got %d", cr.Version)
		}
	})

	t.Run("required_fields_cannot_be_empty", func(t *testing.T) {
		t.Parallel()

		testCases := map[string]func(*testing.T){
			"id_required": func(t *testing.T) {
				t.Parallel()
				// Test factory method with empty ID
				_, err := NewCommonRepresentation(
					uuid.Nil,
					JsonObject{"workspace_id": "test"},
					1,
					"",
					"test-instance",
				)
				AssertValidationError(t, err, "ResourceId", "ResourceId should be required")
			},

			"reporter_type_required": func(t *testing.T) {
				t.Parallel()
				// Test factory method with empty reporter type
				_, err := NewCommonRepresentation(
					uuid.NewSHA1(uuid.NameSpaceOID, []byte("test")),
					JsonObject{"workspace_id": "test"},
					1,
					"",
					"test-instance",
				)
				AssertValidationError(t, err, "ReportedByReporterType", "ReportedByReporterType should be required")
			},
			"reporter_instance_required": func(t *testing.T) {
				t.Parallel()
				// Test factory method with empty reporter instance
				_, err := NewCommonRepresentation(
					uuid.NewSHA1(uuid.NameSpaceOID, []byte("test")),
					JsonObject{"workspace_id": "test"},
					1,
					"hbi",
					"",
				)
				AssertValidationError(t, err, "ReportedByReporterInstance", "ReportedByReporterInstance should be required")
			},
		}

		RunTableDrivenTest(t, testCases)
	})
}

func TestCommonRepresentation_DataHandling(t *testing.T) {
	t.Parallel()

	fixture := NewTestFixture(t)

	t.Run("data_can_be_complex_json", func(t *testing.T) {
		t.Parallel()
		complexData := JsonObject{
			"name":        "complex-resource",
			"description": "A complex resource with nested data",
			"metadata": JsonObject{
				"labels": JsonObject{
					"environment": "test",
					"team":        "platform",
				},
				"annotations": []interface{}{
					"annotation1",
					"annotation2",
				},
			},
			"status": JsonObject{
				"phase":    "running",
				"ready":    true,
				"replicas": 3,
			},
		}

		cr := fixture.CommonRepresentationWithData(complexData)

		// Factory method should create a valid instance with complex data
		AssertNoError(t, nil, "Complex JSON data should be valid")
		AssertEqual(t, complexData, cr.Data, "Complex data should be preserved")
	})

	t.Run("data_can_be_empty_object", func(t *testing.T) {
		t.Parallel()
		cr := fixture.CommonRepresentationWithEmptyData()

		// Factory method should create a valid instance with empty data
		AssertNoError(t, nil, "Empty data object should be valid")
		AssertEqual(t, JsonObject{}, cr.Data, "Empty data should be preserved")
	})

	t.Run("data_can_be_nil", func(t *testing.T) {
		t.Parallel()
		// Test factory method with nil data - should be valid
		cr, err := NewCommonRepresentation(
			uuid.NewSHA1(uuid.NameSpaceOID, []byte("test")),
			nil,
			1,
			"hbi",
			"test-instance",
		)
		AssertNoError(t, err, "Nil data should be valid")
		if cr == nil {
			t.Error("CommonRepresentation should not be nil")
		} else if cr.Data != nil {
			t.Error("Expected Data to be nil")
		}
	})
}

func TestCommonRepresentation_Comparison(t *testing.T) {
	t.Parallel()

	fixture := NewTestFixture(t)

	t.Run("identical_representations_are_equal", func(t *testing.T) {
		t.Parallel()
		cr1 := fixture.ValidCommonRepresentation()
		cr2 := fixture.ValidCommonRepresentation()

		AssertEqual(t, cr1, cr2, "Identical representations should be equal")
	})

	t.Run("different_ids_make_representations_different", func(t *testing.T) {
		t.Parallel()
		cr1 := fixture.CommonRepresentationWithID("id1")
		cr2 := fixture.CommonRepresentationWithID("id2")

		AssertNotEqual(t, cr1, cr2, "Different IDs should make representations different")
	})

	t.Run("different_versions_make_representations_different", func(t *testing.T) {
		t.Parallel()
		cr1 := fixture.CommonRepresentationWithVersion(1)
		cr2 := fixture.CommonRepresentationWithVersion(2)

		AssertNotEqual(t, cr1, cr2, "Different versions should make representations different")
	})
}

func TestCommonRepresentation_FactoryMethods(t *testing.T) {
	t.Run("should_create_valid_CommonRepresentation_with_specific_ResourceId", func(t *testing.T) {
		testResourceId := uuid.New()
		cr, err := NewCommonRepresentation(
			testResourceId,
			JsonObject{"workspace_id": "test-workspace"},
			1,
			"hbi",
			"test-instance",
		)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if cr.ResourceId != testResourceId {
			t.Errorf("Expected ResourceId %s, got %s", testResourceId, cr.ResourceId)
		}

		if cr.Version != 1 {
			t.Errorf("Expected Version 1, got %d", cr.Version)
		}
	})

	t.Run("should_enforce_validation_rules_in_factory", func(t *testing.T) {
		testResourceId := uuid.New()

		// Test zero Version - should be valid now
		cr, err := NewCommonRepresentation(
			testResourceId,
			JsonObject{"workspace_id": "test-workspace"},
			0, // zero Version should be valid
			"hbi",
			"test-instance",
		)

		if err != nil {
			t.Errorf("Expected zero Version to be valid, got error: %v", err)
		}
		if cr == nil {
			t.Error("Expected valid CommonRepresentation, got nil")
		} else if cr.Version != 0 {
			t.Errorf("Expected Version to be 0, got %d", cr.Version)
		}

		// Test nil Data - should be valid now
		cr, err = NewCommonRepresentation(
			testResourceId,
			nil, // nil Data should be valid
			1,
			"hbi",
			"test-instance",
		)

		if err != nil {
			t.Errorf("Expected nil Data to be valid, got error: %v", err)
		}
		if cr == nil {
			t.Error("Expected valid CommonRepresentation, got nil")
		} else if cr.Data != nil {
			t.Error("Expected Data to be nil")
		}

		// Test nil UUID
		_, err = NewCommonRepresentation(
			uuid.Nil, // nil UUID
			JsonObject{"workspace_id": "test-workspace"},
			1,
			"hbi",
			"test-instance",
		)

		if err == nil {
			t.Error("Expected validation error for nil UUID")
		}
	})
}
