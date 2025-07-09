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
	// Use a deterministic UUID for consistent test results based on real-world data
	deterministicUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("dd1b73b9-3e33-4264-968c-e3ce55b9afec"))

	cr, err := NewCommonRepresentation(
		deterministicUUID,
		JsonObject{
			"workspace_id": "a64d17d0-aec3-410a-acd0-e0b85b22c076",
		},
		"host",
		1,
		"hbi",
		"3088be62-1c60-4884-b133-9200542d0b3f",
	)
	if err != nil {
		f.t.Fatalf("Failed to create valid CommonRepresentation: %v", err)
	}
	return cr
}

// CommonRepresentationWithID returns a CommonRepresentation with specified ID
func (f *TestFixture) CommonRepresentationWithID(id string) *CommonRepresentation {
	cr := f.ValidCommonRepresentation()
	if id == "" {
		cr.ResourceId = uuid.Nil
	} else {
		// Try to parse as UUID, if it fails, generate a deterministic one
		if parsedUUID, err := uuid.Parse(id); err == nil {
			cr.ResourceId = parsedUUID
		} else {
			// For test cases that pass non-UUID strings, we'll use a deterministic UUID
			cr.ResourceId = uuid.NewSHA1(uuid.NameSpaceOID, []byte(id))
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
			Data: JsonObject{
				"workspace_id": "1c0753fe-48c1-44d8-823c-95d04cff5f91",
			},
		},
		ResourceId:                 uuid.NewSHA1(uuid.NameSpaceOID, []byte("cdcebe29-67fb-4ac6-ba03-703a22ff4bc0")),
		ResourceType:               "k8s_policy",
		Version:                    1,
		ReportedByReporterType:     "ACM",
		ReportedByReporterInstance: "57a317b1-4040-4c26-8d41-dd589ba1d2eb",
	}
}

// MaximalCommonRepresentation returns a CommonRepresentation with maximum length values
func (f *TestFixture) MaximalCommonRepresentation() *CommonRepresentation {
	return &CommonRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{
				"workspace_id": "aee8f698-9d43-49a1-b458-680a7c9dc046",
			},
		},
		ResourceId:                 uuid.NewSHA1(uuid.NameSpaceOID, []byte("ae5c7a82-cb3b-4591-9b10-3ae1506d4f3d")),
		ResourceType:               "k8s_cluster",
		Version:                    4294967295, // Max uint32
		ReportedByReporterType:     "ACM",
		ReportedByReporterInstance: "14c6b63e-49b2-4cc2-99de-5d914b657548",
	}
}

// UnicodeCommonRepresentation returns a CommonRepresentation with unicode characters
func (f *TestFixture) UnicodeCommonRepresentation() *CommonRepresentation {
	return &CommonRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{
				"unicode_field": "测试数据 🌟 emoji test",
				"japanese":      "こんにちは世界",
				"arabic":        "مرحبا بالعالم",
				"russian":       "Привет мир",
				"emoji_data":    "🚀🌟💻🔥⚡",
			},
		},
		ResourceId:                 uuid.NewSHA1(uuid.NameSpaceOID, []byte("测试-id-🌟")),
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
				"special_data":   "Data with special characters: !@#$%^&*()_+-=[]{}|;':\",./<>?",
				"symbols":        "™®©§¶†‡•…‰‹›",
				"math_symbols":   "±×÷≤≥≠≈∞∑∏∆√∫",
				"currency":       "$€£¥₹₽¢₩₪₨",
				"arrows":         "←→↑↓↔↕⇄⇅⇆⇇⇈⇉⇊⇋⇌",
				"punctuation":    "¡¿¨´`˜ˆ¸˛˚°",
				"brackets_mixed": "([{<>}])",
				"quotes_mixed":   "\"'`‹›«»",
			},
		},
		ResourceId:                 uuid.NewSHA1(uuid.NameSpaceOID, []byte("special-!@#$%^&*()-id")),
		ResourceType:               "special-!@#$%^&*()-type",
		Version:                    1,
		ReportedByReporterType:     "special-†‡•-reporter",
		ReportedByReporterInstance: "special-™®©-instance",
	}
}

// Reporter Representation Fixtures

// ValidReporterRepresentation returns a valid ReporterRepresentation for testing
func (f *TestFixture) ValidReporterRepresentation() *ReporterRepresentation {
	return &ReporterRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{
				"satellite_id":            "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
				"subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
				"insights_inventory_id":   "05707922-7b0a-4fe6-982d-6adbc7695b8f",
				"ansible_host":            "host-1",
			},
		},
		LocalResourceID:    "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
		ReporterType:       "hbi",
		ResourceType:       "host",
		Version:            1,
		ReporterInstanceID: "3088be62-1c60-4884-b133-9200542d0b3f",
		Generation:         1,
		APIHref:            "https://apiHref.com/",
		ConsoleHref:        stringPtr("https://www.console.com/"),
		CommonVersion:      1,
		Tombstone:          false,
		ReporterVersion:    stringPtr("2.7.16"),
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
	rr := &ReporterRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{
				"external_cluster_id": "9414df93-aefe-4153-ba8a-8765373d39b9",
				"cluster_status":      "READY",
				"cluster_reason":      "reflect",
				"kube_version":        "2.7.0",
				"kube_vendor":         "KUBE_VENDOR_UNSPECIFIED",
				"vendor_version":      "3.3.1",
				"cloud_platform":      "BAREMETAL_IPI",
				"nodes": []interface{}{
					JsonObject{
						"name":   "www.example.com",
						"cpu":    "7500m",
						"memory": "30973224Ki",
					},
				},
			},
		},
		LocalResourceID:    "ae5c7a82-cb3b-4591-9b10-3ae1506d4f3d",
		ReporterType:       "ACM",
		ResourceType:       "k8s_cluster",
		Version:            1,
		ReporterInstanceID: "14c6b63e-49b2-4cc2-99de-5d914b657548",
		Generation:         1,
		APIHref:            "https://apiHref.com/",
		ConsoleHref:        stringPtr("https://www.console.com/"),
		CommonVersion:      1,
		Tombstone:          tombstone,
		ReporterVersion:    stringPtr("0.2.0"),
	}

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
	return &ReporterRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{
				"disabled": true,
				"severity": "CRITICAL",
			},
		},
		LocalResourceID:    "cdcebe29-67fb-4ac6-ba03-703a22ff4bc0",
		ReporterType:       "ACM",
		ResourceType:       "k8s_policy",
		Version:            1,
		ReporterInstanceID: "57a317b1-4040-4c26-8d41-dd589ba1d2eb",
		Generation:         1,
		APIHref:            "https://apiHref.com/",
		ConsoleHref:        stringPtr("https://www.console.com/"),
		CommonVersion:      1,
		Tombstone:          false,
		ReporterVersion:    nil, // This is the key difference - nil reporter version
	}
}

// ReporterRepresentationWithNilConsoleHref returns a ReporterRepresentation with nil console href
func (f *TestFixture) ReporterRepresentationWithNilConsoleHref() *ReporterRepresentation {
	return &ReporterRepresentation{
		BaseRepresentation: BaseRepresentation{
			Data: JsonObject{
				"reporter_type":        "NOTIFICATIONS",
				"reporter_instance_id": "f2e4e735-3936-4ee6-a881-b2e1f9326991",
				"local_resource_id":    "cbc86170-e959-42d8-bd2a-964a5a558475",
			},
		},
		LocalResourceID:    "03c923f9-6747-4177-ae35-d36493a1c88e",
		ReporterType:       "NOTIFICATIONS",
		ResourceType:       "notifications_integration",
		Version:            1,
		ReporterInstanceID: "cc38fb9e-251d-4abe-9eaf-b71607558b2a",
		Generation:         1,
		APIHref:            "https://www.campbell-butler.biz/",
		ConsoleHref:        nil, // This is the key difference - nil console href
		CommonVersion:      1,
		Tombstone:          false,
		ReporterVersion:    stringPtr("1.5.7"),
	}
}

// Validation Functions

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
