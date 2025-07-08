package model

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestFixture provides a centralized way to create test data for domain models
type TestFixture struct {
	t *testing.T
}

// NewTestFixture creates a new test fixture instance
func NewTestFixture(t *testing.T) *TestFixture {
	return &TestFixture{t: t}
}

// ValidationError represents a domain validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Common Representation Fixtures

// ValidCommonRepresentation returns a valid CommonRepresentation for testing
func (f *TestFixture) ValidCommonRepresentation() *CommonRepresentation {
	// Use a deterministic UUID for consistent test results
	deterministicUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("test-common-representation"))

	return &CommonRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{
				"name":        "test-resource",
				"description": "Test resource description",
				"status":      "active",
			},
		},
		ID:                         deterministicUUID,
		ResourceType:               "test-resource-type",
		Version:                    1,
		ReportedByReporterType:     "test-reporter",
		ReportedByReporterInstance: "test-instance",
	}
}

// CommonRepresentationWithID returns a CommonRepresentation with specified ID
func (f *TestFixture) CommonRepresentationWithID(id string) *CommonRepresentation {
	cr := f.ValidCommonRepresentation()
	if id == "" {
		cr.ID = uuid.Nil
	} else {
		// Try to parse as UUID, if it fails, generate a deterministic one
		if parsedUUID, err := uuid.Parse(id); err == nil {
			cr.ID = parsedUUID
		} else {
			// For test cases that pass non-UUID strings, we'll use a deterministic UUID
			cr.ID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(id))
		}
	}
	return cr
}

// CommonRepresentationWithVersion returns a CommonRepresentation with specified version
func (f *TestFixture) CommonRepresentationWithVersion(version uint) *CommonRepresentation {
	cr := f.ValidCommonRepresentation()
	cr.Version = version
	return cr
}

// CommonRepresentationWithResourceType returns a CommonRepresentation with specified resource type
func (f *TestFixture) CommonRepresentationWithResourceType(resourceType string) *CommonRepresentation {
	cr := f.ValidCommonRepresentation()
	cr.ResourceType = resourceType
	return cr
}

// CommonRepresentationWithReporterType returns a CommonRepresentation with specified reporter type
func (f *TestFixture) CommonRepresentationWithReporterType(reporterType string) *CommonRepresentation {
	cr := f.ValidCommonRepresentation()
	cr.ReportedByReporterType = reporterType
	return cr
}

// CommonRepresentationWithReporterInstance returns a CommonRepresentation with specified reporter instance
func (f *TestFixture) CommonRepresentationWithReporterInstance(reporterInstance string) *CommonRepresentation {
	cr := f.ValidCommonRepresentation()
	cr.ReportedByReporterInstance = reporterInstance
	return cr
}

// CommonRepresentationWithData returns a CommonRepresentation with specified data
func (f *TestFixture) CommonRepresentationWithData(data JsonObject) *CommonRepresentation {
	cr := f.ValidCommonRepresentation()
	cr.Data = data
	return cr
}

// CommonRepresentationWithEmptyData returns a CommonRepresentation with empty data
func (f *TestFixture) CommonRepresentationWithEmptyData() *CommonRepresentation {
	cr := f.ValidCommonRepresentation()
	cr.Data = JsonObject{}
	return cr
}

// CommonRepresentationWithNilData returns a CommonRepresentation with nil data
func (f *TestFixture) CommonRepresentationWithNilData() *CommonRepresentation {
	cr := f.ValidCommonRepresentation()
	cr.Data = nil
	return cr
}

// MinimalCommonRepresentation returns a CommonRepresentation with minimal valid data
func (f *TestFixture) MinimalCommonRepresentation() *CommonRepresentation {
	return &CommonRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{},
		},
		ID:                         uuid.NewSHA1(uuid.NameSpaceOID, []byte("minimal-common-representation")),
		ResourceType:               "minimal-type",
		Version:                    1,
		ReportedByReporterType:     "minimal-reporter",
		ReportedByReporterInstance: "minimal-instance",
	}
}

// MaximalCommonRepresentation returns a CommonRepresentation with maximum length values
func (f *TestFixture) MaximalCommonRepresentation() *CommonRepresentation {
	return &CommonRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{
				"very_long_key_name_for_testing": "very long value with lots of content to test JSON storage limits",
				"nested_object": JsonObject{
					"level1": JsonObject{
						"level2": "deep nesting test",
					},
				},
				"array_field": []interface{}{"item1", "item2", "item3"},
			},
		},
		ID:                         uuid.NewSHA1(uuid.NameSpaceOID, []byte("maximal-common-representation")),
		ResourceType:               "very-long-resource-type-name-that-exceeds-normal-expectations-and-tests-size-constraints",
		Version:                    4294967295, // Max uint32
		ReportedByReporterType:     "very-long-reporter-type-name-for-testing-maximum-length-constraints",
		ReportedByReporterInstance: "very-long-reporter-instance-name-for-testing-maximum-length-constraints",
	}
}

// UnicodeCommonRepresentation returns a CommonRepresentation with unicode characters
func (f *TestFixture) UnicodeCommonRepresentation() *CommonRepresentation {
	return &CommonRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{
				"unicode_field": "测试数据 🌟 emoji test",
				"special_chars": "!@#$%^&*()_+-=[]{}|;':\",./<>?",
			},
		},
		ID:                         uuid.NewSHA1(uuid.NameSpaceOID, []byte("测试-id-🌟")),
		ResourceType:               "测试-resource-type",
		Version:                    1,
		ReportedByReporterType:     "测试-reporter",
		ReportedByReporterInstance: "测试-instance",
	}
}

// SpecialCharsCommonRepresentation returns a CommonRepresentation with special characters
func (f *TestFixture) SpecialCharsCommonRepresentation() *CommonRepresentation {
	return &CommonRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{
				"special_data": "Data with special characters: !@#$%^&*()_+-=[]{}|;':\",./<>?",
			},
		},
		ID:                         uuid.NewSHA1(uuid.NameSpaceOID, []byte("special-!@#$%^&*()-id")),
		ResourceType:               "special-!@#$%^&*()-type",
		Version:                    1,
		ReportedByReporterType:     "special-reporter",
		ReportedByReporterInstance: "special-instance",
	}
}

// Reporter Representation Fixtures

// ValidReporterRepresentation returns a valid ReporterRepresentation for testing
func (f *TestFixture) ValidReporterRepresentation() *ReporterRepresentation {
	return &ReporterRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{
				"name":        "test-resource",
				"description": "Test resource description",
				"status":      "active",
			},
		},
		LocalResourceID:    "local-123",
		ReporterType:       "test-reporter",
		ResourceType:       "test-resource-type",
		Version:            1,
		ReporterInstanceID: "reporter-instance-123",
		Generation:         1,
		APIHref:            "https://api.example.com/resource/123",
		ConsoleHref:        stringPtr("https://console.example.com/resource/123"),
		CommonVersion:      1,
		Tombstone:          false,
		ReporterVersion:    stringPtr("1.0.0"),
	}
}

// ReporterRepresentationWithLocalResourceID returns a ReporterRepresentation with specified local resource ID
func (f *TestFixture) ReporterRepresentationWithLocalResourceID(localResourceID string) *ReporterRepresentation {
	rr := f.ValidReporterRepresentation()
	rr.LocalResourceID = localResourceID
	return rr
}

// ReporterRepresentationWithTombstone returns a ReporterRepresentation with tombstone flag
func (f *TestFixture) ReporterRepresentationWithTombstone(tombstone bool) *ReporterRepresentation {
	rr := f.ValidReporterRepresentation()
	rr.Tombstone = tombstone
	if tombstone {
		rr.Data = JsonObject{
			"deleted_at": "2023-01-01T00:00:00Z",
			"reason":     "Resource deleted",
		}
	}
	return rr
}

// ReporterRepresentationWithAPIHref returns a ReporterRepresentation with specified API href
func (f *TestFixture) ReporterRepresentationWithAPIHref(apiHref string) *ReporterRepresentation {
	rr := f.ValidReporterRepresentation()
	rr.APIHref = apiHref
	return rr
}

// ReporterRepresentationWithConsoleHref returns a ReporterRepresentation with specified console href
func (f *TestFixture) ReporterRepresentationWithConsoleHref(consoleHref string) *ReporterRepresentation {
	rr := f.ValidReporterRepresentation()
	if consoleHref == "" {
		rr.ConsoleHref = nil
	} else {
		rr.ConsoleHref = &consoleHref
	}
	return rr
}

// ReporterRepresentationWithReporterVersion returns a ReporterRepresentation with specified reporter version
func (f *TestFixture) ReporterRepresentationWithReporterVersion(reporterVersion *string) *ReporterRepresentation {
	rr := f.ValidReporterRepresentation()
	rr.ReporterVersion = reporterVersion
	return rr
}

// ReporterRepresentationWithNilReporterVersion returns a ReporterRepresentation with nil reporter version
func (f *TestFixture) ReporterRepresentationWithNilReporterVersion() *ReporterRepresentation {
	rr := f.ValidReporterRepresentation()
	rr.ReporterVersion = nil
	return rr
}

// ReporterRepresentationWithNilConsoleHref returns a ReporterRepresentation with nil console href
func (f *TestFixture) ReporterRepresentationWithNilConsoleHref() *ReporterRepresentation {
	rr := f.ValidReporterRepresentation()
	rr.ConsoleHref = nil
	return rr
}

// Validation Functions

// ValidateCommonRepresentation validates a CommonRepresentation instance
func ValidateCommonRepresentation(cr *CommonRepresentation) error {
	if cr.ID == uuid.Nil {
		return ValidationError{Field: "ID", Message: "cannot be empty"}
	}
	if cr.ResourceType == "" {
		return ValidationError{Field: "ResourceType", Message: "cannot be empty"}
	}
	if cr.Version == 0 {
		return ValidationError{Field: "Version", Message: "must be positive"}
	}
	if cr.ReportedByReporterType == "" {
		return ValidationError{Field: "ReportedByReporterType", Message: "cannot be empty"}
	}
	if cr.ReportedByReporterInstance == "" {
		return ValidationError{Field: "ReportedByReporterInstance", Message: "cannot be empty"}
	}
	if cr.Data == nil {
		return ValidationError{Field: "Data", Message: "cannot be nil"}
	}
	return nil
}

// ValidateReporterRepresentation validates a ReporterRepresentation instance
func ValidateReporterRepresentation(rr *ReporterRepresentation) error {
	if rr.LocalResourceID == "" || strings.TrimSpace(rr.LocalResourceID) == "" {
		return ValidationError{Field: "LocalResourceID", Message: "cannot be empty"}
	}
	if len(rr.LocalResourceID) > 128 {
		return ValidationError{Field: "LocalResourceID", Message: "exceeds maximum length of 128 characters"}
	}
	if rr.ReporterType == "" || strings.TrimSpace(rr.ReporterType) == "" {
		return ValidationError{Field: "ReporterType", Message: "cannot be empty"}
	}
	if len(rr.ReporterType) > 128 {
		return ValidationError{Field: "ReporterType", Message: "exceeds maximum length of 128 characters"}
	}
	if rr.ResourceType == "" || strings.TrimSpace(rr.ResourceType) == "" {
		return ValidationError{Field: "ResourceType", Message: "cannot be empty"}
	}
	if len(rr.ResourceType) > 128 {
		return ValidationError{Field: "ResourceType", Message: "exceeds maximum length of 128 characters"}
	}
	if rr.Version == 0 {
		return ValidationError{Field: "Version", Message: "must be positive"}
	}
	if rr.ReporterInstanceID == "" || strings.TrimSpace(rr.ReporterInstanceID) == "" {
		return ValidationError{Field: "ReporterInstanceID", Message: "cannot be empty"}
	}
	if len(rr.ReporterInstanceID) > 128 {
		return ValidationError{Field: "ReporterInstanceID", Message: "exceeds maximum length of 128 characters"}
	}
	if rr.Generation == 0 {
		return ValidationError{Field: "Generation", Message: "must be positive"}
	}
	if rr.APIHref != "" {
		if len(rr.APIHref) > 512 {
			return ValidationError{Field: "APIHref", Message: "exceeds maximum length of 512 characters"}
		}
		if err := validateURL(rr.APIHref); err != nil {
			return ValidationError{Field: "APIHref", Message: err.Error()}
		}
	}
	if rr.ConsoleHref != nil && *rr.ConsoleHref != "" {
		if len(*rr.ConsoleHref) > 512 {
			return ValidationError{Field: "ConsoleHref", Message: "exceeds maximum length of 512 characters"}
		}
		if err := validateURL(*rr.ConsoleHref); err != nil {
			return ValidationError{Field: "ConsoleHref", Message: err.Error()}
		}
	}
	if rr.CommonVersion == 0 {
		return ValidationError{Field: "CommonVersion", Message: "must be positive"}
	}
	if rr.ReporterVersion != nil && len(*rr.ReporterVersion) > 128 {
		return ValidationError{Field: "ReporterVersion", Message: "exceeds maximum length of 128 characters"}
	}
	if rr.Data == nil {
		return ValidationError{Field: "Data", Message: "cannot be nil"}
	}
	return nil
}

// validateURL validates that a URL has proper format with scheme and host
func validateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %v", err)
	}

	if parsedURL.Scheme == "" {
		return fmt.Errorf("URL must have a scheme (http/https)")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	return nil
}

// Test Helper Functions

// AssertEqual compares two values and fails the test if they're not equal
func AssertEqual(t *testing.T, expected, actual interface{}, message string) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("%s: expected %+v, got %+v", message, expected, actual)
	}
}

// AssertNotEqual compares two values and fails the test if they're equal
func AssertNotEqual(t *testing.T, expected, actual interface{}, message string) {
	t.Helper()
	if reflect.DeepEqual(expected, actual) {
		t.Errorf("%s: expected values to be different, but both were %+v", message, expected)
	}
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error, message string) {
	t.Helper()
	if err != nil {
		t.Errorf("%s: unexpected error: %v", message, err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error but got nil", message)
	}
}

// AssertValidationError checks if the error is a ValidationError with the expected field
func AssertValidationError(t *testing.T, err error, expectedField string, message string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected ValidationError but got nil", message)
		return
	}

	validationErr, ok := err.(ValidationError)
	if !ok {
		t.Errorf("%s: expected ValidationError but got %T: %v", message, err, err)
		return
	}

	if validationErr.Field != expectedField {
		t.Errorf("%s: expected ValidationError for field '%s' but got '%s'", message, expectedField, validationErr.Field)
	}
}

// AssertTableName checks if the model has the expected table name
func AssertTableName(t *testing.T, model interface{}, expectedTableName string) {
	t.Helper()

	// Check if the model has a TableName method
	value := reflect.ValueOf(model)
	method := value.MethodByName("TableName")
	if !method.IsValid() {
		t.Errorf("Model %T does not have a TableName method", model)
		return
	}

	// Call the TableName method
	results := method.Call(nil)
	if len(results) != 1 {
		t.Errorf("TableName method should return exactly one value")
		return
	}

	actualTableName := results[0].String()
	if actualTableName != expectedTableName {
		t.Errorf("Expected table name '%s', got '%s'", expectedTableName, actualTableName)
	}
}

// AssertGORMTag checks if a field has the expected GORM tag
func AssertGORMTag(t *testing.T, model interface{}, fieldName string, expectedTag string) {
	t.Helper()

	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	field, found := modelType.FieldByName(fieldName)
	if !found {
		t.Errorf("Field '%s' not found in model %T", fieldName, model)
		return
	}

	actualTag := field.Tag.Get("gorm")
	if actualTag != expectedTag {
		t.Errorf("Field '%s' expected GORM tag '%s', got '%s'", fieldName, expectedTag, actualTag)
	}
}

// AssertFieldType checks if a field has the expected type
func AssertFieldType(t *testing.T, model interface{}, fieldName string, expectedType reflect.Type) {
	t.Helper()

	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	field, found := modelType.FieldByName(fieldName)
	if !found {
		t.Errorf("Field '%s' not found in model %T", fieldName, model)
		return
	}

	if field.Type != expectedType {
		t.Errorf("Field '%s' expected type %v, got %v", fieldName, expectedType, field.Type)
	}
}

// RunTableDrivenTest runs a set of test cases in parallel
func RunTableDrivenTest(t *testing.T, testCases map[string]func(*testing.T)) {
	t.Helper()
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			testCase(t)
		})
	}
}

// Contains checks if a string contains a substring
func Contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}
