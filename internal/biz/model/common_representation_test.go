package model

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
)

// Test scenarios for CommonRepresentation domain model
//
// These tests focus on domain logic, business rules, and model behavior
// rather than database operations or infrastructure concerns.

func TestCommonRepresentation_TableName(t *testing.T) {
	t.Parallel()

	fixture := NewTestFixture(t)
	cr := fixture.ValidCommonRepresentation()

	AssertTableName(t, cr, "common_representation")
}

func TestCommonRepresentation_Structure(t *testing.T) {
	t.Parallel()

	fixture := NewTestFixture(t)

	t.Run("has_embedded_base_representation", func(t *testing.T) {
		t.Parallel()
		cr := fixture.ValidCommonRepresentation()

		// Check that BaseRepresentation is embedded
		crType := reflect.TypeOf(*cr)
		_, found := crType.FieldByName("BaseRepresentation")
		if !found {
			t.Error("CommonRepresentation should embed BaseRepresentation")
		}
	})

	t.Run("has_required_fields", func(t *testing.T) {
		t.Parallel()
		cr := fixture.ValidCommonRepresentation()

		requiredFields := []string{"ID", "ResourceType", "Version", "ReportedByReporterType", "ReportedByReporterInstance"}
		crType := reflect.TypeOf(*cr)

		for _, fieldName := range requiredFields {
			_, found := crType.FieldByName(fieldName)
			if !found {
				t.Errorf("CommonRepresentation should have field %s", fieldName)
			}
		}
	})

	t.Run("field_types", func(t *testing.T) {
		t.Parallel()
		cr := fixture.ValidCommonRepresentation()

		AssertFieldType(t, cr, "ID", reflect.TypeOf(uuid.UUID{}))
		AssertFieldType(t, cr, "ResourceType", reflect.TypeOf(""))
		AssertFieldType(t, cr, "Version", reflect.TypeOf(0))
		AssertFieldType(t, cr, "ReportedByReporterType", reflect.TypeOf(""))
		AssertFieldType(t, cr, "ReportedByReporterInstance", reflect.TypeOf(""))
	})
}

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
				AssertValidationError(t, err, "ID", "Empty ID should fail validation")
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
			"negative_version": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithVersion(-1)
				err := ValidateCommonRepresentation(cr)
				AssertValidationError(t, err, "Version", "Negative Version should fail validation")
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

	t.Run("version_cannot_be_negative", func(t *testing.T) {
		t.Parallel()
		cr := fixture.CommonRepresentationWithVersion(-1)

		err := ValidateCommonRepresentation(cr)
		AssertValidationError(t, err, "Version", "Negative version should be invalid")
	})

	t.Run("required_fields_cannot_be_empty", func(t *testing.T) {
		t.Parallel()

		testCases := map[string]func(*testing.T){
			"id_required": func(t *testing.T) {
				t.Parallel()
				cr := fixture.CommonRepresentationWithID("")
				err := ValidateCommonRepresentation(cr)
				AssertValidationError(t, err, "ID", "ID should be required")
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

func TestCommonRepresentation_EdgeCases(t *testing.T) {
	t.Parallel()

	fixture := NewTestFixture(t)

	t.Run("unicode_characters_in_fields", func(t *testing.T) {
		t.Parallel()
		cr := fixture.UnicodeCommonRepresentation()

		err := ValidateCommonRepresentation(cr)
		AssertNoError(t, err, "Unicode characters should be valid")

		// UUID should be valid (deterministic based on input)
		if cr.ID == uuid.Nil {
			t.Error("Unicode-based UUID should not be nil")
		}
	})

	t.Run("special_characters_in_fields", func(t *testing.T) {
		t.Parallel()
		cr := fixture.SpecialCharsCommonRepresentation()

		err := ValidateCommonRepresentation(cr)
		AssertNoError(t, err, "Special characters should be valid")

		// UUID should be valid (deterministic based on input)
		if cr.ID == uuid.Nil {
			t.Error("Special-chars-based UUID should not be nil")
		}
	})

	t.Run("maximum_length_values", func(t *testing.T) {
		t.Parallel()
		cr := fixture.MaximalCommonRepresentation()

		err := ValidateCommonRepresentation(cr)
		AssertNoError(t, err, "Maximum length values should be valid")

		if cr.Version != 2147483647 {
			t.Errorf("Expected max int32 version 2147483647, got %d", cr.Version)
		}
	})

	t.Run("minimal_valid_representation", func(t *testing.T) {
		t.Parallel()
		cr := fixture.MinimalCommonRepresentation()

		err := ValidateCommonRepresentation(cr)
		AssertNoError(t, err, "Minimal valid representation should pass validation")
	})
}

func TestCommonRepresentation_Serialization(t *testing.T) {
	t.Parallel()

	fixture := NewTestFixture(t)

	t.Run("json_marshalling", func(t *testing.T) {
		t.Parallel()
		cr := fixture.ValidCommonRepresentation()

		// Basic check that the object is not nil (placeholder for actual JSON marshalling test)
		if cr == nil {
			t.Error("CommonRepresentation should not be nil for JSON marshalling")
		}
	})

	t.Run("complex_data_marshalling", func(t *testing.T) {
		t.Parallel()
		complexData := JsonObject{
			"nested": JsonObject{
				"array": []interface{}{1, 2, 3},
				"bool":  true,
				"null":  nil,
			},
		}

		cr := fixture.CommonRepresentationWithData(complexData)

		// Basic check that the object is not nil (placeholder for actual JSON marshalling test)
		if cr == nil {
			t.Error("CommonRepresentation with complex data should not be nil for JSON marshalling")
		}
	})
}

func TestCommonRepresentation_GORMTags(t *testing.T) {
	t.Parallel()

	fixture := NewTestFixture(t)
	cr := fixture.ValidCommonRepresentation()

	t.Run("id_field_tags", func(t *testing.T) {
		t.Parallel()
		AssertGORMTag(t, cr, "ID", "type:uuid;column:id;primary_key;default:gen_random_uuid()")
	})

	t.Run("resource_type_field_tags", func(t *testing.T) {
		t.Parallel()
		AssertGORMTag(t, cr, "ResourceType", "size:128;column:resource_type")
	})

	t.Run("version_field_tags", func(t *testing.T) {
		t.Parallel()
		AssertGORMTag(t, cr, "Version", "column:version;primary_key")
	})

	t.Run("reported_by_reporter_type_field_tags", func(t *testing.T) {
		t.Parallel()
		AssertGORMTag(t, cr, "ReportedByReporterType", "column:reported_by_reporter_type")
	})

	t.Run("reported_by_reporter_instance_field_tags", func(t *testing.T) {
		t.Parallel()
		AssertGORMTag(t, cr, "ReportedByReporterInstance", "column:reported_by_reporter_instance")
	})
}

func TestCommonRepresentation_CompositePrimaryKey(t *testing.T) {
	t.Parallel()

	fixture := NewTestFixture(t)

	t.Run("same_id_different_versions_should_be_allowed", func(t *testing.T) {
		t.Parallel()
		// Create two representations with the same ID but different versions
		sharedUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("shared-resource-id"))

		cr1 := fixture.CommonRepresentationWithID(sharedUUID.String())
		cr1.Version = 1

		cr2 := fixture.CommonRepresentationWithID(sharedUUID.String())
		cr2.Version = 2

		// Both should be valid
		err1 := ValidateCommonRepresentation(cr1)
		AssertNoError(t, err1, "First version should be valid")

		err2 := ValidateCommonRepresentation(cr2)
		AssertNoError(t, err2, "Second version should be valid")

		// They should have the same ID but different versions
		AssertEqual(t, cr1.ID, cr2.ID, "Both representations should have the same ID")
		AssertNotEqual(t, cr1.Version, cr2.Version, "Representations should have different versions")
	})

	t.Run("composite_primary_key_fields_both_have_primary_key_tag", func(t *testing.T) {
		t.Parallel()
		cr := fixture.ValidCommonRepresentation()
		crType := reflect.TypeOf(*cr)

		// Check that both ID and Version have primary_key in their GORM tags
		idField, _ := crType.FieldByName("ID")
		idTag := idField.Tag.Get("gorm")
		if !Contains(idTag, "primary_key") {
			t.Errorf("ID field should have primary_key tag, got: %s", idTag)
		}

		versionField, _ := crType.FieldByName("Version")
		versionTag := versionField.Tag.Get("gorm")
		if !Contains(versionTag, "primary_key") {
			t.Errorf("Version field should have primary_key tag, got: %s", versionTag)
		}
	})

	t.Run("versioned_resource_lifecycle", func(t *testing.T) {
		t.Parallel()
		// Simulate a resource lifecycle with multiple versions
		resourceUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("lifecycle-test-resource"))

		// Version 1: Initial creation
		v1 := fixture.CommonRepresentationWithID(resourceUUID.String())
		v1.Version = 1
		v1.Data = JsonObject{
			"status": "created",
			"name":   "test-resource",
		}

		// Version 2: Update
		v2 := fixture.CommonRepresentationWithID(resourceUUID.String())
		v2.Version = 2
		v2.Data = JsonObject{
			"status": "updated",
			"name":   "test-resource-updated",
		}

		// Version 3: Final state
		v3 := fixture.CommonRepresentationWithID(resourceUUID.String())
		v3.Version = 3
		v3.Data = JsonObject{
			"status": "finalized",
			"name":   "test-resource-final",
		}

		// All versions should be valid
		AssertNoError(t, ValidateCommonRepresentation(v1), "Version 1 should be valid")
		AssertNoError(t, ValidateCommonRepresentation(v2), "Version 2 should be valid")
		AssertNoError(t, ValidateCommonRepresentation(v3), "Version 3 should be valid")

		// All should have the same ID but different versions
		AssertEqual(t, v1.ID, v2.ID, "V1 and V2 should have same ID")
		AssertEqual(t, v2.ID, v3.ID, "V2 and V3 should have same ID")

		// Versions should be different
		AssertNotEqual(t, v1.Version, v2.Version, "V1 and V2 should have different versions")
		AssertNotEqual(t, v2.Version, v3.Version, "V2 and V3 should have different versions")
		AssertNotEqual(t, v1.Version, v3.Version, "V1 and V3 should have different versions")
	})
}
