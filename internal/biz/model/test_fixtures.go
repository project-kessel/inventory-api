package model

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
)

// TestFixture provides a centralized way to create test data for domain models
type TestFixture struct {
	t *testing.T
}

// NewTestFixture creates a new test fixture instance
func NewTestFixture(t *testing.T) *TestFixture {
	return &TestFixture{t: t}
}

// Common Representation Fixtures

// ValidCommonRepresentation returns a valid CommonRepresentation for testing
func (f *TestFixture) ValidCommonRepresentation() *CommonRepresentation {
	// Use a deterministic UUID for consistent test results based on real-world data
	deterministicUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("dd1b73b9-3e33-4264-968c-e3ce55b9afec"))

	cr, err := NewCommonRepresentation(
		deterministicUUID,
		model_legacy.JsonObject{
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
	var resourceId uuid.UUID
	if id == "" {
		resourceId = uuid.Nil
	} else {
		// Try to parse as UUID, if it fails, generate a deterministic one
		if parsedUUID, err := uuid.Parse(id); err == nil {
			resourceId = parsedUUID
		} else {
			// For test cases that pass non-UUID strings, we'll use a deterministic UUID
			resourceId = uuid.NewSHA1(uuid.NameSpaceOID, []byte(id))
		}
	}

	// Create using factory method - this will fail validation for invalid IDs
	cr, err := NewCommonRepresentation(
		resourceId,
		model_legacy.JsonObject{
			"workspace_id": "a64d17d0-aec3-410a-acd0-e0b85b22c076",
		},
		"host",
		1,
		"hbi",
		"3088be62-1c60-4884-b133-9200542d0b3f",
	)
	if err != nil {
		// For test cases expecting invalid data, return the struct anyway for testing
		return &CommonRepresentation{
			Representation: Representation{
				Data: model_legacy.JsonObject{
					"workspace_id": "a64d17d0-aec3-410a-acd0-e0b85b22c076",
				},
			},
			ResourceId:                 resourceId,
			ResourceType:               "host",
			Version:                    1,
			ReportedByReporterType:     "hbi",
			ReportedByReporterInstance: "3088be62-1c60-4884-b133-9200542d0b3f",
		}
	}
	return cr
}

// CommonRepresentationWithVersion returns a CommonRepresentation with specified version
func (f *TestFixture) CommonRepresentationWithVersion(version uint) *CommonRepresentation {
	deterministicUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("dd1b73b9-3e33-4264-968c-e3ce55b9afec"))

	cr, err := NewCommonRepresentation(
		deterministicUUID,
		model_legacy.JsonObject{
			"workspace_id": "a64d17d0-aec3-410a-acd0-e0b85b22c076",
		},
		"host",
		version,
		"hbi",
		"3088be62-1c60-4884-b133-9200542d0b3f",
	)
	if err != nil {
		f.t.Fatalf("Cannot create CommonRepresentation with version %d: %v", version, err)
	}
	return cr
}

// CommonRepresentationWithResourceType returns a CommonRepresentation with specified resource type
func (f *TestFixture) CommonRepresentationWithResourceType(resourceType string) *CommonRepresentation {
	deterministicUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("dd1b73b9-3e33-4264-968c-e3ce55b9afec"))

	cr, err := NewCommonRepresentation(
		deterministicUUID,
		model_legacy.JsonObject{
			"workspace_id": "a64d17d0-aec3-410a-acd0-e0b85b22c076",
		},
		resourceType,
		1,
		"hbi",
		"3088be62-1c60-4884-b133-9200542d0b3f",
	)
	if err != nil {
		f.t.Fatalf("Cannot create CommonRepresentation with resource type %q: %v", resourceType, err)
	}
	return cr
}

// CommonRepresentationWithReporterType returns a CommonRepresentation with specified reporter type
func (f *TestFixture) CommonRepresentationWithReporterType(reporterType string) *CommonRepresentation {
	deterministicUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("dd1b73b9-3e33-4264-968c-e3ce55b9afec"))

	cr, err := NewCommonRepresentation(
		deterministicUUID,
		model_legacy.JsonObject{
			"workspace_id": "a64d17d0-aec3-410a-acd0-e0b85b22c076",
		},
		"host",
		1,
		reporterType,
		"3088be62-1c60-4884-b133-9200542d0b3f",
	)
	if err != nil {
		f.t.Fatalf("Cannot create CommonRepresentation with reporter type %q: %v", reporterType, err)
	}
	return cr
}

// CommonRepresentationWithReporterInstance returns a CommonRepresentation with specified reporter instance
func (f *TestFixture) CommonRepresentationWithReporterInstance(reporterInstance string) *CommonRepresentation {
	deterministicUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("dd1b73b9-3e33-4264-968c-e3ce55b9afec"))

	cr, err := NewCommonRepresentation(
		deterministicUUID,
		model_legacy.JsonObject{
			"workspace_id": "a64d17d0-aec3-410a-acd0-e0b85b22c076",
		},
		"host",
		1,
		"hbi",
		reporterInstance,
	)
	if err != nil {
		f.t.Fatalf("Cannot create CommonRepresentation with reporter instance %q: %v", reporterInstance, err)
	}
	return cr
}

// CommonRepresentationWithData returns a CommonRepresentation with specified data
func (f *TestFixture) CommonRepresentationWithData(data model_legacy.JsonObject) *CommonRepresentation {
	deterministicUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("dd1b73b9-3e33-4264-968c-e3ce55b9afec"))

	cr, err := NewCommonRepresentation(
		deterministicUUID,
		data,
		"host",
		1,
		"hbi",
		"3088be62-1c60-4884-b133-9200542d0b3f",
	)
	if err != nil {
		f.t.Fatalf("Cannot create CommonRepresentation with data %+v: %v", data, err)
	}
	return cr
}

// CommonRepresentationWithEmptyData returns a CommonRepresentation with empty data
func (f *TestFixture) CommonRepresentationWithEmptyData() *CommonRepresentation {
	deterministicUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("dd1b73b9-3e33-4264-968c-e3ce55b9afec"))

	cr, err := NewCommonRepresentation(
		deterministicUUID,
		model_legacy.JsonObject{},
		"host",
		1,
		"hbi",
		"3088be62-1c60-4884-b133-9200542d0b3f",
	)
	if err != nil {
		f.t.Fatalf("Cannot create CommonRepresentation with empty data: %v", err)
	}
	return cr
}

// CommonRepresentationWithNilData returns a CommonRepresentation with nil data
func (f *TestFixture) CommonRepresentationWithNilData() *CommonRepresentation {
	deterministicUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("dd1b73b9-3e33-4264-968c-e3ce55b9afec"))

	// This will always fail validation since nil data is invalid
	return &CommonRepresentation{
		Representation: Representation{
			Data: nil,
		},
		ResourceId:                 deterministicUUID,
		ResourceType:               "host",
		Version:                    1,
		ReportedByReporterType:     "hbi",
		ReportedByReporterInstance: "3088be62-1c60-4884-b133-9200542d0b3f",
	}
}

// MinimalCommonRepresentation returns a CommonRepresentation with minimal valid data
func (f *TestFixture) MinimalCommonRepresentation() *CommonRepresentation {
	cr, err := NewCommonRepresentation(
		uuid.NewSHA1(uuid.NameSpaceOID, []byte("cdcebe29-67fb-4ac6-ba03-703a22ff4bc0")),
		model_legacy.JsonObject{
			"workspace_id": "1c0753fe-48c1-44d8-823c-95d04cff5f91",
		},
		"k8s_policy",
		1,
		"ACM",
		"57a317b1-4040-4c26-8d41-dd589ba1d2eb",
	)
	if err != nil {
		f.t.Fatalf("Failed to create minimal CommonRepresentation: %v", err)
	}
	return cr
}

// MaximalCommonRepresentation returns a CommonRepresentation with maximum length values
func (f *TestFixture) MaximalCommonRepresentation() *CommonRepresentation {
	cr, err := NewCommonRepresentation(
		uuid.NewSHA1(uuid.NameSpaceOID, []byte("ae5c7a82-cb3b-4591-9b10-3ae1506d4f3d")),
		model_legacy.JsonObject{
			"workspace_id": "aee8f698-9d43-49a1-b458-680a7c9dc046",
		},
		"k8s_cluster",
		4294967295, // Max uint32
		"ACM",
		"14c6b63e-49b2-4cc2-99de-5d914b657548",
	)
	if err != nil {
		f.t.Fatalf("Failed to create maximal CommonRepresentation: %v", err)
	}
	return cr
}

// UnicodeCommonRepresentation returns a CommonRepresentation with unicode characters
func (f *TestFixture) UnicodeCommonRepresentation() *CommonRepresentation {
	cr, err := NewCommonRepresentation(
		uuid.NewSHA1(uuid.NameSpaceOID, []byte("测试-id-🌟")),
		model_legacy.JsonObject{
			"unicode_field": "测试数据 🌟 emoji test",
			"japanese":      "こんにちは世界",
			"arabic":        "مرحبا بالعالم",
			"russian":       "Привет мир",
			"emoji_data":    "🚀🌟💻🔥⚡",
		},
		"测试-resource-type",
		1,
		"测试-reporter",
		"测试-instance",
	)
	if err != nil {
		// Unicode should be valid, but if not, create directly for testing
		return &CommonRepresentation{
			Representation: Representation{
				Data: model_legacy.JsonObject{
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
	return cr
}

// SpecialCharsCommonRepresentation returns a CommonRepresentation with special characters
func (f *TestFixture) SpecialCharsCommonRepresentation() *CommonRepresentation {
	cr, err := NewCommonRepresentation(
		uuid.NewSHA1(uuid.NameSpaceOID, []byte("special-!@#$%^&*()-id")),
		model_legacy.JsonObject{
			"special_field":  "Data with special characters: !@#$%^&*()_+-=[]{}|;':\",./<>?",
			"symbols":        "™®©§¶†‡•…‰‹›",
			"math_symbols":   "±×÷≤≥≠≈∞∑∏∆√∫",
			"currency":       "$€£¥₹₽¢₩₪₨",
			"arrows":         "←→↑↓↔↕⇄⇅⇆⇇⇈⇉⇊⇋⇌",
			"punctuation":    "¡¿¨´`˜ˆ¸˛˚°",
			"brackets_mixed": "([{<>}])",
			"quotes_mixed":   "\"'`‹›«»",
		},
		"special-!@#$%^&*()-type",
		1,
		"special-†‡•-reporter",
		"special-™®©-instance",
	)
	if err != nil {
		// Special characters should be valid, but if not, create directly for testing
		return &CommonRepresentation{
			Representation: Representation{
				Data: model_legacy.JsonObject{
					"special_field":  "Data with special characters: !@#$%^&*()_+-=[]{}|;':\",./<>?",
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
	return cr
}

// Reporter Representation Fixtures

// ValidReporterRepresentation returns a valid ReporterRepresentation for testing
func (f *TestFixture) ValidReporterRepresentation() *ReporterRepresentation {
	rr, err := NewReporterRepresentation(
		model_legacy.JsonObject{
			"satellite_id":            "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
			"subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
			"insights_inventory_id":   "05707922-7b0a-4fe6-982d-6adbc7695b8f",
			"ansible_host":            "host-1",
		},
		"dd1b73b9-3e33-4264-968c-e3ce55b9afec",
		"hbi",
		"host",
		1,
		"3088be62-1c60-4884-b133-9200542d0b3f",
		1,
		"https://apiHref.com/",
		stringPtr("https://www.console.com/"),
		1,
		false,
		stringPtr("2.7.16"),
	)
	if err != nil {
		f.t.Fatalf("Failed to create valid ReporterRepresentation: %v", err)
	}
	return rr
}

// ReporterRepresentationWithLocalResourceID returns a ReporterRepresentation with specified local resource ID
func (f *TestFixture) ReporterRepresentationWithLocalResourceID(localResourceID string) (*ReporterRepresentation, error) {
	rr, err := NewReporterRepresentation(
		model_legacy.JsonObject{
			"satellite_id":            "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
			"subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
			"insights_inventory_id":   "05707922-7b0a-4fe6-982d-6adbc7695b8f",
			"ansible_host":            "host-1",
		},
		localResourceID,
		"hbi",
		"host",
		1,
		"3088be62-1c60-4884-b133-9200542d0b3f",
		1,
		"https://apiHref.com/",
		stringPtr("https://www.console.com/"),
		1,
		false,
		stringPtr("2.7.16"),
	)
	if err != nil {
		return nil, err
	}
	return rr, nil
}

// ReporterRepresentationWithResourceType returns a ReporterRepresentation with specified local resource ID
func (f *TestFixture) ReporterRepresentationWithResourceType(resourceType string) (*ReporterRepresentation, error) {
	rr, err := NewReporterRepresentation(
		model_legacy.JsonObject{
			"satellite_id":            "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
			"subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
			"insights_inventory_id":   "05707922-7b0a-4fe6-982d-6adbc7695b8f",
			"ansible_host":            "host-1",
		},
		"dd1b73b9-3e33-4264-968c-e3ce55b9afec",
		"hbi",
		resourceType,
		1,
		"3088be62-1c60-4884-b133-9200542d0b3f",
		1,
		"https://apiHref.com/",
		stringPtr("https://www.console.com/"),
		1,
		false,
		stringPtr("2.7.16"),
	)
	if err != nil {
		return nil, err
	}
	return rr, nil
}

// ReporterRepresentationWithTombstone returns a ReporterRepresentation with tombstone flag
func (f *TestFixture) ReporterRepresentationWithTombstone(tombstone bool) *ReporterRepresentation {
	data := model_legacy.JsonObject{
		"external_cluster_id": "9414df93-aefe-4153-ba8a-8765373d39b9",
		"cluster_status":      "READY",
		"cluster_reason":      "reflect",
		"kube_version":        "2.7.0",
		"kube_vendor":         "KUBE_VENDOR_UNSPECIFIED",
		"vendor_version":      "3.3.1",
		"cloud_platform":      "BAREMETAL_IPI",
		"nodes": []interface{}{
			model_legacy.JsonObject{
				"name":   "www.example.com",
				"cpu":    "7500m",
				"memory": "30973224Ki",
			},
		},
	}

	if tombstone {
		data = model_legacy.JsonObject{
			"deleted_at": "2023-01-01T00:00:00Z",
			"reason":     "Resource deleted",
		}
	}

	rr, err := NewReporterRepresentation(
		data,
		"ae5c7a82-cb3b-4591-9b10-3ae1506d4f3d",
		"ACM",
		"k8s_cluster",
		1,
		"14c6b63e-49b2-4cc2-99de-5d914b657548",
		1,
		"https://apiHref.com/",
		stringPtr("https://www.console.com/"),
		1,
		tombstone,
		stringPtr("0.2.0"),
	)
	if err != nil {
		f.t.Fatalf("Failed to create ReporterRepresentation with tombstone: %v", err)
	}
	return rr
}

// ReporterRepresentationWithAPIHref returns a ReporterRepresentation with specified API href
func (f *TestFixture) ReporterRepresentationWithAPIHref(apiHref string) *ReporterRepresentation {
	rr, err := NewReporterRepresentation(
		model_legacy.JsonObject{
			"satellite_id":            "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
			"subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
			"insights_inventory_id":   "05707922-7b0a-4fe6-982d-6adbc7695b8f",
			"ansible_host":            "host-1",
		},
		"dd1b73b9-3e33-4264-968c-e3ce55b9afec",
		"hbi",
		"host",
		1,
		"3088be62-1c60-4884-b133-9200542d0b3f",
		1,
		apiHref,
		stringPtr("https://www.console.com/"),
		1,
		false,
		stringPtr("2.7.16"),
	)
	if err != nil {
		// For test cases expecting invalid data, return the struct anyway for testing
		return &ReporterRepresentation{
			Representation: Representation{
				Data: model_legacy.JsonObject{
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
			APIHref:            apiHref,
			ConsoleHref:        stringPtr("https://www.console.com/"),
			CommonVersion:      1,
			Tombstone:          false,
			ReporterVersion:    stringPtr("2.7.16"),
		}
	}
	return rr
}

// ReporterRepresentationWithConsoleHref returns a ReporterRepresentation with specified console href
func (f *TestFixture) ReporterRepresentationWithConsoleHref(consoleHref string) *ReporterRepresentation {
	var consoleHrefPtr *string
	if consoleHref != "" {
		consoleHrefPtr = &consoleHref
	}

	rr, err := NewReporterRepresentation(
		model_legacy.JsonObject{
			"satellite_id":            "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
			"subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
			"insights_inventory_id":   "05707922-7b0a-4fe6-982d-6adbc7695b8f",
			"ansible_host":            "host-1",
		},
		"dd1b73b9-3e33-4264-968c-e3ce55b9afec",
		"hbi",
		"host",
		1,
		"3088be62-1c60-4884-b133-9200542d0b3f",
		1,
		"https://apiHref.com/",
		consoleHrefPtr,
		1,
		false,
		stringPtr("2.7.16"),
	)
	if err != nil {
		// For test cases expecting invalid data, return the struct anyway for testing
		return &ReporterRepresentation{
			Representation: Representation{
				Data: model_legacy.JsonObject{
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
			ConsoleHref:        consoleHrefPtr,
			CommonVersion:      1,
			Tombstone:          false,
			ReporterVersion:    stringPtr("2.7.16"),
		}
	}
	return rr
}

// ReporterRepresentationWithReporterVersion returns a ReporterRepresentation with specified reporter version
func (f *TestFixture) ReporterRepresentationWithReporterVersion(reporterVersion *string) *ReporterRepresentation {
	rr, err := NewReporterRepresentation(
		model_legacy.JsonObject{
			"satellite_id":            "2c4196f1-0371-4f4c-8913-e113cfaa6e67",
			"subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
			"insights_inventory_id":   "05707922-7b0a-4fe6-982d-6adbc7695b8f",
			"ansible_host":            "host-1",
		},
		"dd1b73b9-3e33-4264-968c-e3ce55b9afec",
		"hbi",
		"host",
		1,
		"3088be62-1c60-4884-b133-9200542d0b3f",
		1,
		"https://apiHref.com/",
		stringPtr("https://www.console.com/"),
		1,
		false,
		reporterVersion,
	)
	if err != nil {
		// For test cases expecting invalid data, return the struct anyway for testing
		return &ReporterRepresentation{
			Representation: Representation{
				Data: model_legacy.JsonObject{
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
			ReporterVersion:    reporterVersion,
		}
	}
	return rr
}

// ReporterRepresentationWithNilReporterVersion returns a ReporterRepresentation with nil reporter version
func (f *TestFixture) ReporterRepresentationWithNilReporterVersion() *ReporterRepresentation {
	rr, err := NewReporterRepresentation(
		model_legacy.JsonObject{
			"disabled": true,
			"severity": "CRITICAL",
		},
		"cdcebe29-67fb-4ac6-ba03-703a22ff4bc0",
		"ACM",
		"k8s_policy",
		1,
		"57a317b1-4040-4c26-8d41-dd589ba1d2eb",
		1,
		"https://apiHref.com/",
		stringPtr("https://www.console.com/"),
		1,
		false,
		nil, // This is the key difference - nil reporter version
	)
	if err != nil {
		f.t.Fatalf("Failed to create ReporterRepresentation with nil reporter version: %v", err)
	}
	return rr
}

// ReporterRepresentationWithNilConsoleHref returns a ReporterRepresentation with nil console href
func (f *TestFixture) ReporterRepresentationWithNilConsoleHref() *ReporterRepresentation {
	rr, err := NewReporterRepresentation(
		model_legacy.JsonObject{
			"reporter_type":        "NOTIFICATIONS",
			"reporter_instance_id": "f2e4e735-3936-4ee6-a881-b2e1f9326991",
			"local_resource_id":    "cbc86170-e959-42d8-bd2a-964a5a558475",
		},
		"03c923f9-6747-4177-ae35-d36493a1c88e",
		"NOTIFICATIONS",
		"notifications_integration",
		1,
		"cc38fb9e-251d-4abe-9eaf-b71607558b2a",
		1,
		"https://www.campbell-butler.biz/",
		nil, // This is the key difference - nil console href
		1,
		false,
		stringPtr("1.5.7"),
	)
	if err != nil {
		f.t.Fatalf("Failed to create ReporterRepresentation with nil console href: %v", err)
	}
	return rr
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

	validationErr, ok := err.(model_legacy.ValidationError)
	if !ok {
		t.Errorf("%s: expected ValidationError but got %T: %v", message, err, err)
		return
	}

	if validationErr.Field != expectedField {
		t.Errorf("%s: expected ValidationError for field '%s' but got '%s'", message, expectedField, validationErr.Field)
	}
}

// AssertTableName checks if the model_legacy has the expected table name
func AssertTableName(t *testing.T, model interface{}, expectedTableName string) {
	t.Helper()

	// Check if the model_legacy has a TableName method
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
		t.Errorf("Field '%s' not found in model_legacy %T", fieldName, model)
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
		t.Errorf("Field '%s' not found in model_legacy %T", fieldName, model)
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
