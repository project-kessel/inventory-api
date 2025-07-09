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

		err := ValidateCommonRepresentation(cr)
		AssertNoError(t, err, "Valid CommonRepresentation should pass validation")
	})

	t.Run("invalid_representations", func(t *testing.T) {
		t.Parallel()

		testCases := map[string]func(*testing.T){
			"empty_id": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithID("")
				err := ValidateCommonRepresentation(cr)
				AssertValidationError(t, err, "ResourceId", "Empty ResourceId should fail validation")
			},
			"empty_resource_type": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithResourceType("")
				err := ValidateCommonRepresentation(cr)
				AssertValidationError(t, err, "ResourceType", "Empty ResourceType should fail validation")
			},
			"zero_version": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithVersion(0)
				err := ValidateCommonRepresentation(cr)
				AssertValidationError(t, err, "Version", "Zero Version should fail validation")
			},
			"empty_reporter_type": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithReporterType("")
				err := ValidateCommonRepresentation(cr)
				AssertValidationError(t, err, "ReportedByReporterType", "Empty ReportedByReporterType should fail validation")
			},
			"empty_reporter_instance": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithReporterInstance("")
				err := ValidateCommonRepresentation(cr)
				AssertValidationError(t, err, "ReportedByReporterInstance", "Empty ReportedByReporterInstance should fail validation")
			},
			"nil_data": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithNilData()
				err := ValidateCommonRepresentation(cr)
				AssertValidationError(t, err, "Data", "Nil Data should fail validation")
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

		err := ValidateCommonRepresentation(cr)
		AssertNoError(t, err, "Positive version should be valid")
	})

	t.Run("version_cannot_be_zero", func(t *testing.T) {
		t.Parallel()
		cr := fixture.CommonRepresentationWithVersion(0)

		err := ValidateCommonRepresentation(cr)
		AssertValidationError(t, err, "Version", "Zero version should be invalid")
	})

	t.Run("required_fields_cannot_be_empty", func(t *testing.T) {
		t.Parallel()

		testCases := map[string]func(*testing.T){
			"id_required": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithID("")
				err := ValidateCommonRepresentation(cr)
				AssertValidationError(t, err, "ResourceId", "ResourceId should be required")
			},
			"resource_type_required": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithResourceType("")
				err := ValidateCommonRepresentation(cr)
				AssertValidationError(t, err, "ResourceType", "ResourceType should be required")
			},
			"reporter_type_required": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithReporterType("")
				err := ValidateCommonRepresentation(cr)
				AssertValidationError(t, err, "ReportedByReporterType", "ReportedByReporterType should be required")
			},
			"reporter_instance_required": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithReporterInstance("")
				err := ValidateCommonRepresentation(cr)
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

		err := ValidateCommonRepresentation(cr)
		AssertNoError(t, err, "Complex JSON data should be valid")

		AssertEqual(t, complexData, cr.Data, "Complex data should be preserved")
	})

	t.Run("data_can_be_empty_object", func(t *testing.T) {
		t.Parallel()
		cr := fixture.CommonRepresentationWithEmptyData()

		err := ValidateCommonRepresentation(cr)
		AssertNoError(t, err, "Empty data object should be valid")

		AssertEqual(t, JsonObject{}, cr.Data, "Empty data should be preserved")
	})

	t.Run("data_cannot_be_nil", func(t *testing.T) {
		t.Parallel()
		cr := fixture.CommonRepresentationWithNilData()

		err := ValidateCommonRepresentation(cr)
		AssertValidationError(t, err, "Data", "Nil data should be invalid")
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
			"host",
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

		if cr.ResourceType != "host" {
			t.Errorf("Expected ResourceType 'host', got '%s'", cr.ResourceType)
		}

		if cr.Version != 1 {
			t.Errorf("Expected Version 1, got %d", cr.Version)
		}
	})

	t.Run("should_enforce_validation_rules_in_factory", func(t *testing.T) {
		testResourceId := uuid.New()

		// Test empty ResourceType
		_, err := NewCommonRepresentation(
			testResourceId,
			JsonObject{"workspace_id": "test-workspace"},
			"", // empty ResourceType
			1,
			"hbi",
			"test-instance",
		)

		if err == nil {
			t.Error("Expected validation error for empty ResourceType")
		}

		// Test zero Version
		_, err = NewCommonRepresentation(
			testResourceId,
			JsonObject{"workspace_id": "test-workspace"},
			"host",
			0, // zero Version
			"hbi",
			"test-instance",
		)

		if err == nil {
			t.Error("Expected validation error for zero Version")
		}

		// Test nil Data
		_, err = NewCommonRepresentation(
			testResourceId,
			nil, // nil Data
			"host",
			1,
			"hbi",
			"test-instance",
		)

		if err == nil {
			t.Error("Expected validation error for nil Data")
		}

		// Test nil UUID
		_, err = NewCommonRepresentation(
			uuid.Nil, // nil UUID
			JsonObject{"workspace_id": "test-workspace"},
			"host",
			1,
			"hbi",
			"test-instance",
		)

		if err == nil {
			t.Error("Expected validation error for nil UUID")
		}
	})
}
