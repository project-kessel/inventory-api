package model

import (
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
)

type VersionTestFixture struct {
	ZeroValue     uint
	PositiveValue uint
	LargeValue    uint
}

func NewVersionTestFixture() VersionTestFixture {
	return VersionTestFixture{
		ZeroValue:     0,
		PositiveValue: 42,
		LargeValue:    ^uint(0), // Maximum uint value
	}
}

type GenerationTestFixture struct {
	ZeroValue     uint
	PositiveValue uint
	LargeValue    uint
}

func NewGenerationTestFixture() GenerationTestFixture {
	return GenerationTestFixture{
		ZeroValue:     0,
		PositiveValue: 15,
		LargeValue:    ^uint(0), // Maximum uint value
	}
}

type TombstoneTestFixture struct {
	TrueValue  bool
	FalseValue bool
}

func NewTombstoneTestFixture() TombstoneTestFixture {
	return TombstoneTestFixture{
		TrueValue:  true,
		FalseValue: false,
	}
}

type ResourceIdTestFixture struct {
	ValidUUID   uuid.UUID
	AnotherUUID uuid.UUID
	NilUUID     uuid.UUID
}

func NewResourceIdTestFixture() ResourceIdTestFixture {
	return ResourceIdTestFixture{
		ValidUUID:   uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		AnotherUUID: uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		NilUUID:     uuid.Nil,
	}
}

type ReporterResourceIdTestFixture struct {
	ValidUUID        uuid.UUID
	AnotherUUID      uuid.UUID
	NilUUID          uuid.UUID
	ValidString      string
	InvalidString    string
	EmptyString      string
	WhitespaceString string
}

func NewReporterResourceIdTestFixture() ReporterResourceIdTestFixture {
	return ReporterResourceIdTestFixture{
		ValidUUID:        uuid.MustParse("550e8400-e29b-41d4-a716-446655440001"),
		AnotherUUID:      uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c9"),
		NilUUID:          uuid.Nil,
		ValidString:      "550e8400-e29b-41d4-a716-446655440002",
		InvalidString:    "invalid-uuid",
		EmptyString:      "",
		WhitespaceString: "  \t\n  ",
	}
}

type ResourceTypeTestFixture struct {
	ValidType        string
	AnotherType      string
	EmptyString      string
	WhitespaceString string
}

func NewResourceTypeTestFixture() ResourceTypeTestFixture {
	return ResourceTypeTestFixture{
		ValidType:        "k8s_cluster",
		AnotherType:      "host",
		EmptyString:      "",
		WhitespaceString: "  \t\n  ",
	}
}

type ReporterTypeTestFixture struct {
	ValidType        string
	AnotherType      string
	EmptyString      string
	WhitespaceString string
}

func NewReporterTypeTestFixture() ReporterTypeTestFixture {
	return ReporterTypeTestFixture{
		ValidType:        "acm",
		AnotherType:      "ocm",
		EmptyString:      "",
		WhitespaceString: "  \t\n  ",
	}
}

type ReporterInstanceIdTestFixture struct {
	ValidId          string
	AnotherId        string
	EmptyString      string
	WhitespaceString string
}

func NewReporterInstanceIdTestFixture() ReporterInstanceIdTestFixture {
	return ReporterInstanceIdTestFixture{
		ValidId:          "instance-123",
		AnotherId:        "instance-456",
		EmptyString:      "",
		WhitespaceString: "  \t\n  ",
	}
}

type ReporterTestFixture struct {
	ValidReporterType         string
	ValidReporterInstanceId   string
	AnotherReporterType       string
	AnotherReporterInstanceId string
	EmptyString               string
	WhitespaceString          string
}

func NewReporterTestFixture() ReporterTestFixture {
	return ReporterTestFixture{
		ValidReporterType:         "acm",
		ValidReporterInstanceId:   "instance-123",
		AnotherReporterType:       "ocm",
		AnotherReporterInstanceId: "instance-456",
		EmptyString:               "",
		WhitespaceString:          "  \t\n  ",
	}
}

type ConsistencyTokenTestFixture struct {
	ValidToken       string
	AnotherToken     string
	EmptyString      string
	WhitespaceString string
}

func NewConsistencyTokenTestFixture() ConsistencyTokenTestFixture {
	return ConsistencyTokenTestFixture{
		ValidToken:       "token-123",
		AnotherToken:     "token-456",
		EmptyString:      "",
		WhitespaceString: "  \t\n  ",
	}
}

type ReporterVersionTestFixture struct {
	ValidVersion     string
	AnotherVersion   string
	EmptyString      string
	WhitespaceString string
	NilPointer       *string
	ValidPointer     *string
}

func NewReporterVersionTestFixture() ReporterVersionTestFixture {
	validPtr := "v1.2.3"
	return ReporterVersionTestFixture{
		ValidVersion:     "v1.0.0",
		AnotherVersion:   "v2.1.0",
		EmptyString:      "",
		WhitespaceString: "  \t\n  ",
		NilPointer:       nil,
		ValidPointer:     &validPtr,
	}
}

type ApiHrefTestFixture struct {
	ValidHref        string
	AnotherHref      string
	EmptyString      string
	WhitespaceString string
}

func NewApiHrefTestFixture() ApiHrefTestFixture {
	return ApiHrefTestFixture{
		ValidHref:        "/api/v1/resources/123",
		AnotherHref:      "/api/v1/resources/456",
		EmptyString:      "",
		WhitespaceString: "  \t\n  ",
	}
}

type ConsoleHrefTestFixture struct {
	ValidHref        string
	AnotherHref      string
	EmptyString      string
	WhitespaceString string
}

func NewConsoleHrefTestFixture() ConsoleHrefTestFixture {
	return ConsoleHrefTestFixture{
		ValidHref:        "/console/resources/123",
		AnotherHref:      "/console/resources/456",
		EmptyString:      "",
		WhitespaceString: "  \t\n  ",
	}
}

type LocalResourceIdTestFixture struct {
	ValidId          string
	AnotherId        string
	EmptyString      string
	WhitespaceString string
}

func NewLocalResourceIdTestFixture() LocalResourceIdTestFixture {
	return LocalResourceIdTestFixture{
		ValidId:          "local-123",
		AnotherId:        "local-456",
		EmptyString:      "",
		WhitespaceString: "  \t\n  ",
	}
}

type JsonObjectTestFixture struct {
	ValidData   internal.JsonObject
	AnotherData internal.JsonObject
	EmptyData   internal.JsonObject
	NilData     internal.JsonObject
}

func NewJsonObjectTestFixture() JsonObjectTestFixture {
	return JsonObjectTestFixture{
		ValidData: internal.JsonObject{
			"name":      "test-cluster",
			"status":    "active",
			"nodeCount": 3,
			"region":    "us-east-1",
			"labels":    map[string]interface{}{"env": "prod", "team": "platform"},
		},
		AnotherData: internal.JsonObject{
			"hostname": "test-host",
			"os":       "linux",
			"memory":   "16GB",
			"cpu":      "4 cores",
			"uptime":   "30 days",
		},
		EmptyData: internal.JsonObject{},
		NilData:   nil,
	}
}

type CommonRepresentationTestFixture struct {
	ValidResourceId                 uuid.UUID
	ValidData                       internal.JsonObject
	ValidVersion                    uint
	ValidReportedByReporterType     string
	ValidReportedByReporterInstance string
	NilResourceId                   uuid.UUID
	EmptyData                       internal.JsonObject
	NilData                         internal.JsonObject
	ZeroVersion                     uint
	EmptyReporterType               string
	WhitespaceReporterType          string
	EmptyReporterInstance           string
	WhitespaceReporterInstance      string
}

func NewCommonRepresentationTestFixture() CommonRepresentationTestFixture {
	return CommonRepresentationTestFixture{
		ValidResourceId: uuid.MustParse("550e8400-e29b-41d4-a716-446655440123"),
		ValidData: internal.JsonObject{
			"name":        "test-resource",
			"description": "A test resource for CommonRepresentation",
			"metadata":    map[string]interface{}{"version": "1.0", "type": "test"},
		},
		ValidVersion:                    1,
		ValidReportedByReporterType:     "test-reporter",
		ValidReportedByReporterInstance: "test-instance-001",
		NilResourceId:                   uuid.Nil,
		EmptyData:                       internal.JsonObject{},
		NilData:                         nil,
		ZeroVersion:                     0,
		EmptyReporterType:               "",
		WhitespaceReporterType:          "  \t\n  ",
		EmptyReporterInstance:           "",
		WhitespaceReporterInstance:      "  \t\n  ",
	}
}

// Helper methods for creating domain types from test fixture
func (f CommonRepresentationTestFixture) ValidResourceIdType() ResourceId {
	resourceId, _ := NewResourceId(f.ValidResourceId)
	return resourceId
}

func (f CommonRepresentationTestFixture) ValidRepresentationType() Representation {
	representation, _ := NewRepresentation(f.ValidData)
	return representation
}

func (f CommonRepresentationTestFixture) ValidVersionType() Version {
	return NewVersion(f.ValidVersion)
}

func (f CommonRepresentationTestFixture) ZeroVersionType() Version {
	return NewVersion(f.ZeroVersion)
}

func (f CommonRepresentationTestFixture) ValidReporterTypeType() ReporterType {
	reporterType, _ := NewReporterType(f.ValidReportedByReporterType)
	return reporterType
}

func (f CommonRepresentationTestFixture) ValidReporterInstanceIdType() ReporterInstanceId {
	reporterInstanceId, _ := NewReporterInstanceId(f.ValidReportedByReporterInstance)
	return reporterInstanceId
}

func (f CommonRepresentationTestFixture) NilResourceIdType() ResourceId {
	return DeserializeResourceId(f.NilResourceId)
}

func (f CommonRepresentationTestFixture) EmptyRepresentationType() Representation {
	return DeserializeRepresentation(f.EmptyData)
}

func (f CommonRepresentationTestFixture) NilRepresentationType() Representation {
	return DeserializeRepresentation(f.NilData)
}

func (f CommonRepresentationTestFixture) EmptyReporterTypeType() ReporterType {
	return DeserializeReporterType(f.EmptyReporterType)
}

func (f CommonRepresentationTestFixture) WhitespaceReporterTypeType() ReporterType {
	return DeserializeReporterType(f.WhitespaceReporterType)
}

func (f CommonRepresentationTestFixture) EmptyReporterInstanceIdType() ReporterInstanceId {
	return DeserializeReporterInstanceId(f.EmptyReporterInstance)
}

func (f CommonRepresentationTestFixture) WhitespaceReporterInstanceIdType() ReporterInstanceId {
	return DeserializeReporterInstanceId(f.WhitespaceReporterInstance)
}

type ReporterResourceTestFixture struct {
	ValidId                      uuid.UUID
	ValidLocalResourceId         string
	ValidLocalResourceIdUUID     string
	ValidLocalResourceIdString   string
	ValidResourceType            string
	ValidReporterType            string
	ValidReporterInstanceId      string
	ValidResourceId              uuid.UUID
	ValidApiHref                 string
	ValidConsoleHref             string
	EmptyConsoleHref             string
	NilId                        uuid.UUID
	EmptyLocalResourceId         string
	WhitespaceLocalResourceId    string
	EmptyResourceType            string
	WhitespaceResourceType       string
	EmptyReporterType            string
	WhitespaceReporterType       string
	EmptyReporterInstanceId      string
	WhitespaceReporterInstanceId string
	NilResourceId                uuid.UUID
	EmptyApiHref                 string
	WhitespaceApiHref            string
}

func NewReporterResourceTestFixture() ReporterResourceTestFixture {
	return ReporterResourceTestFixture{
		ValidId:                      uuid.MustParse("550e8400-e29b-41d4-a716-446655440200"),
		ValidLocalResourceId:         "local-resource-123",
		ValidLocalResourceIdUUID:     "550e8400-e29b-41d4-a716-446655440300",
		ValidLocalResourceIdString:   "my-cluster-name-prod",
		ValidResourceType:            "k8s_cluster",
		ValidReporterType:            "acm",
		ValidReporterInstanceId:      "acm-instance-001",
		ValidResourceId:              uuid.MustParse("550e8400-e29b-41d4-a716-446655440201"),
		ValidApiHref:                 "/api/v1/resources/123",
		ValidConsoleHref:             "/console/resources/123",
		EmptyConsoleHref:             "",
		NilId:                        uuid.Nil,
		EmptyLocalResourceId:         "",
		WhitespaceLocalResourceId:    "  \t\n  ",
		EmptyResourceType:            "",
		WhitespaceResourceType:       "  \t\n  ",
		EmptyReporterType:            "",
		WhitespaceReporterType:       "  \t\n  ",
		EmptyReporterInstanceId:      "",
		WhitespaceReporterInstanceId: "  \t\n  ",
		NilResourceId:                uuid.Nil,
		EmptyApiHref:                 "",
		WhitespaceApiHref:            "  \t\n  ",
	}
}

// Helper methods to create tiny types from primitive values for NewReporterResource
func (f ReporterResourceTestFixture) ValidIdType() ReporterResourceId {
	id, _ := NewReporterResourceId(f.ValidId)
	return id
}

func (f ReporterResourceTestFixture) ValidLocalResourceIdType() LocalResourceId {
	id, _ := NewLocalResourceId(f.ValidLocalResourceId)
	return id
}

func (f ReporterResourceTestFixture) ValidResourceTypeType() ResourceType {
	rt, _ := NewResourceType(f.ValidResourceType)
	return rt
}

func (f ReporterResourceTestFixture) ValidReporterTypeType() ReporterType {
	rt, _ := NewReporterType(f.ValidReporterType)
	return rt
}

func (f ReporterResourceTestFixture) ValidReporterInstanceIdType() ReporterInstanceId {
	ri, _ := NewReporterInstanceId(f.ValidReporterInstanceId)
	return ri
}

func (f ReporterResourceTestFixture) ValidResourceIdType() ResourceId {
	id, _ := NewResourceId(f.ValidResourceId)
	return id
}

func (f ReporterResourceTestFixture) ValidApiHrefType() ApiHref {
	ah, _ := NewApiHref(f.ValidApiHref)
	return ah
}

func (f ReporterResourceTestFixture) ValidConsoleHrefType() ConsoleHref {
	ch, _ := NewConsoleHref(f.ValidConsoleHref)
	return ch
}

func (f ReporterResourceTestFixture) EmptyConsoleHrefType() ConsoleHref {
	return ConsoleHref("")
}

// Helper methods for invalid values (using deserialize to bypass validation)
func (f ReporterResourceTestFixture) NilIdType() ReporterResourceId {
	return DeserializeReporterResourceId(f.NilId)
}

func (f ReporterResourceTestFixture) EmptyLocalResourceIdType() LocalResourceId {
	return DeserializeLocalResourceId(f.EmptyLocalResourceId)
}

func (f ReporterResourceTestFixture) WhitespaceLocalResourceIdType() LocalResourceId {
	return DeserializeLocalResourceId(f.WhitespaceLocalResourceId)
}

func (f ReporterResourceTestFixture) EmptyResourceTypeType() ResourceType {
	return DeserializeResourceType(f.EmptyResourceType)
}

func (f ReporterResourceTestFixture) WhitespaceResourceTypeType() ResourceType {
	return DeserializeResourceType(f.WhitespaceResourceType)
}

func (f ReporterResourceTestFixture) EmptyReporterTypeType() ReporterType {
	return DeserializeReporterType(f.EmptyReporterType)
}

func (f ReporterResourceTestFixture) WhitespaceReporterTypeType() ReporterType {
	return DeserializeReporterType(f.WhitespaceReporterType)
}

func (f ReporterResourceTestFixture) EmptyReporterInstanceIdType() ReporterInstanceId {
	return DeserializeReporterInstanceId(f.EmptyReporterInstanceId)
}

func (f ReporterResourceTestFixture) WhitespaceReporterInstanceIdType() ReporterInstanceId {
	return DeserializeReporterInstanceId(f.WhitespaceReporterInstanceId)
}

func (f ReporterResourceTestFixture) NilResourceIdType() ResourceId {
	return DeserializeResourceId(f.NilResourceId)
}

func (f ReporterResourceTestFixture) EmptyApiHrefType() ApiHref {
	return DeserializeApiHref(f.EmptyApiHref)
}

func (f ReporterResourceTestFixture) WhitespaceApiHrefType() ApiHref {
	return DeserializeApiHref(f.WhitespaceApiHref)
}

func (f ReporterResourceTestFixture) ValidLocalResourceIdUUIDType() LocalResourceId {
	id, _ := NewLocalResourceId(f.ValidLocalResourceIdUUID)
	return id
}

func (f ReporterResourceTestFixture) ValidLocalResourceIdStringType() LocalResourceId {
	id, _ := NewLocalResourceId(f.ValidLocalResourceIdString)
	return id
}

type ReporterRepresentationTestFixture struct {
	ValidData                    internal.JsonObject
	ValidReporterResourceId      string
	ValidVersion                 uint
	ValidGeneration              uint
	ValidCommonVersion           uint
	ValidReporterVersion         *string
	NilReporterVersion           *string
	EmptyData                    internal.JsonObject
	NilData                      internal.JsonObject
	EmptyReporterResourceId      string
	WhitespaceReporterResourceId string
	InvalidReporterResourceId    string
}

func NewReporterRepresentationTestFixture() ReporterRepresentationTestFixture {
	validReporterVersion := "v1.2.3"
	return ReporterRepresentationTestFixture{
		ValidData: internal.JsonObject{
			"name":        "test-reporter-resource",
			"description": "A test resource for ReporterRepresentation",
			"metadata":    map[string]interface{}{"version": "2.0", "type": "reporter"},
		},
		ValidReporterResourceId:      "550e8400-e29b-41d4-a716-446655440400",
		ValidVersion:                 1,
		ValidGeneration:              2,
		ValidCommonVersion:           3,
		ValidReporterVersion:         &validReporterVersion,
		NilReporterVersion:           nil,
		EmptyData:                    internal.JsonObject{},
		NilData:                      nil,
		EmptyReporterResourceId:      "",
		WhitespaceReporterResourceId: "  \t\n  ",
		InvalidReporterResourceId:    "invalid-uuid-format",
	}
}

// Helper methods for creating domain types from test fixture
func (f ReporterRepresentationTestFixture) ValidRepresentationType() Representation {
	representation, _ := NewRepresentation(f.ValidData)
	return representation
}

func (f ReporterRepresentationTestFixture) ValidReporterResourceIdType() ReporterResourceId {
	reporterResourceId, _ := NewReporterResourceIdFromString(f.ValidReporterResourceId)
	return reporterResourceId
}

func (f ReporterRepresentationTestFixture) ValidVersionType() Version {
	return NewVersion(f.ValidVersion)
}

func (f ReporterRepresentationTestFixture) ValidGenerationType() Generation {
	return NewGeneration(f.ValidGeneration)
}

func (f ReporterRepresentationTestFixture) ValidCommonVersionType() Version {
	return NewVersion(f.ValidCommonVersion)
}

func (f ReporterRepresentationTestFixture) ValidReporterVersionType() *ReporterVersion {
	if f.ValidReporterVersion == nil {
		return nil
	}
	reporterVersion, _ := NewReporterVersion(*f.ValidReporterVersion)
	return &reporterVersion
}

func (f ReporterRepresentationTestFixture) NilReporterVersionType() *ReporterVersion {
	return nil
}

func (f ReporterRepresentationTestFixture) EmptyRepresentationType() Representation {
	return DeserializeRepresentation(f.EmptyData)
}

func (f ReporterRepresentationTestFixture) NilRepresentationType() Representation {
	return DeserializeRepresentation(f.NilData)
}

func (f ReporterRepresentationTestFixture) EmptyReporterResourceIdType() ReporterResourceId {
	// For empty string, use nil UUID since that's what we'd get from parsing empty string
	return DeserializeReporterResourceId(uuid.Nil)
}

func (f ReporterRepresentationTestFixture) WhitespaceReporterResourceIdType() ReporterResourceId {
	// For whitespace string, use nil UUID since that's what we'd get from parsing whitespace
	return DeserializeReporterResourceId(uuid.Nil)
}

func (f ReporterRepresentationTestFixture) InvalidReporterResourceIdType() ReporterResourceId {
	// For invalid string, use nil UUID since parsing would fail
	return DeserializeReporterResourceId(uuid.Nil)
}

type ResourceTestFixture struct {
	ValidId                   uuid.UUID
	ValidResourceType         string
	AnotherResourceType       string
	ValidReporterResource     ReporterResource
	AnotherReporterResource   ReporterResource
	MultipleReporterResources []ReporterResource
	EmptyReporterResources    []ReporterResource
	NilId                     uuid.UUID
	EmptyResourceType         string
	WhitespaceResourceType    string

	// Individual values for NewResource function
	ValidLocalResourceId            string
	ValidReporterType               string
	ValidReporterInstanceId         string
	ValidResourceId                 uuid.UUID
	ValidApiHref                    string
	ValidConsoleHref                string
	ValidReporterRepresentationData internal.JsonObject
	ValidCommonRepresentationData   internal.JsonObject

	AnotherLocalResourceId            string
	AnotherReporterType               string
	AnotherReporterInstanceId         string
	AnotherResourceId                 uuid.UUID
	AnotherApiHref                    string
	EmptyConsoleHref                  string
	AnotherReporterRepresentationData internal.JsonObject
	AnotherCommonRepresentationData   internal.JsonObject

	EmptyLocalResourceId            string
	EmptyReporterType               string
	EmptyReporterInstanceId         string
	EmptyApiHref                    string
	WhitespaceLocalResourceId       string
	WhitespaceReporterType          string
	WhitespaceReporterInstanceId    string
	WhitespaceApiHref               string
	EmptyReporterRepresentationData internal.JsonObject
	EmptyCommonRepresentationData   internal.JsonObject
}

func NewResourceTestFixture() ResourceTestFixture {
	reporterResourceId1, _ := NewReporterResourceId(uuid.MustParse("550e8400-e29b-41d4-a716-446655440400"))
	localResourceId1, _ := NewLocalResourceId("local-resource-123")
	resourceType1, _ := NewResourceType("k8s_cluster")
	reporterType1, _ := NewReporterType("acm")
	reporterInstanceId1, _ := NewReporterInstanceId("acm-instance-001")
	resourceId1, _ := NewResourceId(uuid.MustParse("550e8400-e29b-41d4-a716-446655440401"))
	apiHref1, _ := NewApiHref("/api/v1/resources/123")
	consoleHref1, _ := NewConsoleHref("/console/resources/123")

	validReporterResource, _ := NewReporterResource(
		reporterResourceId1,
		localResourceId1,
		resourceType1,
		reporterType1,
		reporterInstanceId1,
		resourceId1,
		apiHref1,
		consoleHref1,
	)

	reporterResourceId2, _ := NewReporterResourceId(uuid.MustParse("550e8400-e29b-41d4-a716-446655440500"))
	localResourceId2, _ := NewLocalResourceId("local-resource-456")
	resourceType2, _ := NewResourceType("host")
	reporterType2, _ := NewReporterType("ocm")
	reporterInstanceId2, _ := NewReporterInstanceId("ocm-instance-001")
	resourceId2, _ := NewResourceId(uuid.MustParse("550e8400-e29b-41d4-a716-446655440501"))
	apiHref2, _ := NewApiHref("/api/v1/resources/456")
	consoleHref2 := ConsoleHref("")

	anotherReporterResource, _ := NewReporterResource(
		reporterResourceId2,
		localResourceId2,
		resourceType2,
		reporterType2,
		reporterInstanceId2,
		resourceId2,
		apiHref2,
		consoleHref2,
	)

	return ResourceTestFixture{
		ValidId:                   uuid.MustParse("550e8400-e29b-41d4-a716-446655440600"),
		ValidResourceType:         "k8s_cluster",
		AnotherResourceType:       "host",
		ValidReporterResource:     validReporterResource,
		AnotherReporterResource:   anotherReporterResource,
		MultipleReporterResources: []ReporterResource{validReporterResource, anotherReporterResource},
		EmptyReporterResources:    []ReporterResource{},
		NilId:                     uuid.Nil,
		EmptyResourceType:         "",
		WhitespaceResourceType:    "  \t\n  ",

		// Individual values for NewResource function
		ValidLocalResourceId:            "local-resource-123",
		ValidReporterType:               "acm",
		ValidReporterInstanceId:         "acm-instance-001",
		ValidResourceId:                 uuid.MustParse("550e8400-e29b-41d4-a716-446655440401"),
		ValidApiHref:                    "/api/v1/resources/123",
		ValidConsoleHref:                "/console/resources/123",
		ValidReporterRepresentationData: internal.JsonObject{"name": "test-reporter-resource", "description": "A test resource for ReporterRepresentation", "metadata": map[string]interface{}{"version": "2.0", "type": "reporter"}},
		ValidCommonRepresentationData:   internal.JsonObject{"id": "550e8400-e29b-41d4-a716-446655440400", "type": "reporter", "version": 1, "generation": 2, "commonVersion": 3},

		AnotherLocalResourceId:            "local-resource-456",
		AnotherReporterType:               "ocm",
		AnotherReporterInstanceId:         "ocm-instance-001",
		AnotherResourceId:                 uuid.MustParse("550e8400-e29b-41d4-a716-446655440501"),
		AnotherApiHref:                    "/api/v1/resources/456",
		EmptyConsoleHref:                  "",
		AnotherReporterRepresentationData: internal.JsonObject{"name": "test-reporter-resource", "description": "A test resource for ReporterRepresentation", "metadata": map[string]interface{}{"version": "2.0", "type": "reporter"}},
		AnotherCommonRepresentationData:   internal.JsonObject{"id": "550e8400-e29b-41d4-a716-446655440500", "type": "reporter", "version": 1, "generation": 2, "commonVersion": 3},

		EmptyLocalResourceId:            "",
		EmptyReporterType:               "",
		EmptyReporterInstanceId:         "",
		EmptyApiHref:                    "",
		WhitespaceLocalResourceId:       "  \t\n  ",
		WhitespaceReporterType:          "  \t\n  ",
		WhitespaceReporterInstanceId:    "  \t\n  ",
		WhitespaceApiHref:               "  \t\n  ",
		EmptyReporterRepresentationData: internal.JsonObject{},
		EmptyCommonRepresentationData:   internal.JsonObject{},
	}
}

// Helper methods to create tiny types from primitive values for NewResource
func (f ResourceTestFixture) ValidResourceIdType() ResourceId {
	id, _ := NewResourceId(f.ValidId)
	return id
}

func (f ResourceTestFixture) ValidLocalResourceIdType() LocalResourceId {
	id, _ := NewLocalResourceId(f.ValidLocalResourceId)
	return id
}

func (f ResourceTestFixture) ValidResourceTypeType() ResourceType {
	rt, _ := NewResourceType(f.ValidResourceType)
	return rt
}

func (f ResourceTestFixture) ValidReporterTypeType() ReporterType {
	rt, _ := NewReporterType(f.ValidReporterType)
	return rt
}

func (f ResourceTestFixture) ValidReporterInstanceIdType() ReporterInstanceId {
	ri, _ := NewReporterInstanceId(f.ValidReporterInstanceId)
	return ri
}

func (f ResourceTestFixture) ValidReporterResourceIdType() ReporterResourceId {
	id, _ := NewReporterResourceId(f.ValidResourceId)
	return id
}

func (f ResourceTestFixture) ValidApiHrefType() ApiHref {
	ah, _ := NewApiHref(f.ValidApiHref)
	return ah
}

func (f ResourceTestFixture) ValidConsoleHrefType() ConsoleHref {
	ch, _ := NewConsoleHref(f.ValidConsoleHref)
	return ch
}

// Helper methods for "another" values
func (f ResourceTestFixture) AnotherResourceTypeType() ResourceType {
	rt, _ := NewResourceType(f.AnotherResourceType)
	return rt
}

func (f ResourceTestFixture) AnotherLocalResourceIdType() LocalResourceId {
	id, _ := NewLocalResourceId(f.AnotherLocalResourceId)
	return id
}

func (f ResourceTestFixture) AnotherReporterTypeType() ReporterType {
	rt, _ := NewReporterType(f.AnotherReporterType)
	return rt
}

func (f ResourceTestFixture) AnotherReporterInstanceIdType() ReporterInstanceId {
	ri, _ := NewReporterInstanceId(f.AnotherReporterInstanceId)
	return ri
}

func (f ResourceTestFixture) AnotherReporterResourceIdType() ReporterResourceId {
	id, _ := NewReporterResourceId(f.AnotherResourceId)
	return id
}

func (f ResourceTestFixture) AnotherApiHrefType() ApiHref {
	ah, _ := NewApiHref(f.AnotherApiHref)
	return ah
}

func (f ResourceTestFixture) EmptyConsoleHrefType() ConsoleHref {
	return ConsoleHref("")
}

func (f ResourceTestFixture) ValidReporterRepresentationType() Representation {
	rep, _ := NewRepresentation(f.ValidReporterRepresentationData)
	return rep
}

func (f ResourceTestFixture) ValidCommonRepresentationType() Representation {
	rep, _ := NewRepresentation(f.ValidCommonRepresentationData)
	return rep
}

func (f ResourceTestFixture) AnotherReporterRepresentationType() Representation {
	rep, _ := NewRepresentation(f.AnotherReporterRepresentationData)
	return rep
}

func (f ResourceTestFixture) AnotherCommonRepresentationType() Representation {
	rep, _ := NewRepresentation(f.AnotherCommonRepresentationData)
	return rep
}

func (f ResourceTestFixture) EmptyRepresentationType() Representation {
	return DeserializeRepresentation(f.EmptyReporterRepresentationData)
}

func (f ResourceTestFixture) NilIdType() ResourceId {
	return DeserializeResourceId(f.NilId)
}

func (f ResourceTestFixture) EmptyLocalResourceIdType() LocalResourceId {
	return DeserializeLocalResourceId(f.EmptyLocalResourceId)
}

func (f ResourceTestFixture) EmptyResourceTypeType() ResourceType {
	return DeserializeResourceType(f.EmptyResourceType)
}

func (f ResourceTestFixture) EmptyReporterTypeType() ReporterType {
	return DeserializeReporterType(f.EmptyReporterType)
}

func (f ResourceTestFixture) EmptyReporterInstanceIdType() ReporterInstanceId {
	return DeserializeReporterInstanceId(f.EmptyReporterInstanceId)
}

func (f ResourceTestFixture) EmptyApiHrefType() ApiHref {
	return DeserializeApiHref(f.EmptyApiHref)
}

func (f ResourceTestFixture) WhitespaceResourceTypeType() ResourceType {
	return DeserializeResourceType(f.WhitespaceResourceType)
}

func (f ResourceTestFixture) WhitespaceReporterTypeType() ReporterType {
	return DeserializeReporterType(f.WhitespaceReporterType)
}

func (f ResourceTestFixture) WhitespaceReporterInstanceIdType() ReporterInstanceId {
	return DeserializeReporterInstanceId(f.WhitespaceReporterInstanceId)
}

func (f ResourceTestFixture) WhitespaceLocalResourceIdType() LocalResourceId {
	return DeserializeLocalResourceId(f.WhitespaceLocalResourceId)
}

type ResourceEventTestFixture struct {
	ValidResourceId         uuid.UUID
	ValidResourceType       string
	ValidReporterType       string
	ValidReporterInstanceId string
	ValidReporterData       internal.JsonObject
	ValidReporterResourceID string
	ValidReporterVersion    uint
	ValidReporterGeneration uint
	ValidCommonData         internal.JsonObject
	ValidCommonVersion      uint
	ValidReporterVersionStr *string

	AnotherResourceId     uuid.UUID
	AnotherResourceType   string
	AnotherReporterData   internal.JsonObject
	AnotherCommonData     internal.JsonObject
	NilReporterVersionStr *string

	InvalidResourceId       uuid.UUID
	EmptyResourceType       string
	EmptyReporterType       string
	EmptyReporterInstanceId string
	EmptyReporterData       internal.JsonObject
	EmptyReporterResourceID string
	EmptyCommonData         internal.JsonObject
	WhitespaceResourceType  string
}

func NewResourceEventTestFixture() ResourceEventTestFixture {
	validVersionStr := "1.2.3"

	return ResourceEventTestFixture{
		ValidResourceId:         uuid.MustParse("550e8400-e29b-41d4-a716-446655440700"),
		ValidResourceType:       "k8s_cluster",
		ValidReporterType:       "acm",
		ValidReporterInstanceId: "acm-instance-001",
		ValidReporterData:       internal.JsonObject{"name": "test-cluster", "status": "active"},
		ValidReporterResourceID: "550e8400-e29b-41d4-a716-446655440701",
		ValidReporterVersion:    1,
		ValidReporterGeneration: 0,
		ValidCommonData:         internal.JsonObject{"id": "test-id", "type": "cluster"},
		ValidCommonVersion:      1,
		ValidReporterVersionStr: &validVersionStr,

		AnotherResourceId:     uuid.MustParse("550e8400-e29b-41d4-a716-446655440800"),
		AnotherResourceType:   "host",
		AnotherReporterData:   internal.JsonObject{"hostname": "test-host", "os": "linux"},
		AnotherCommonData:     internal.JsonObject{"id": "host-id", "type": "host"},
		NilReporterVersionStr: nil,

		InvalidResourceId:       uuid.Nil,
		EmptyResourceType:       "",
		EmptyReporterType:       "",
		EmptyReporterInstanceId: "",
		EmptyReporterData:       internal.JsonObject{},
		EmptyReporterResourceID: "",
		EmptyCommonData:         internal.JsonObject{},
		WhitespaceResourceType:  "  \t\n  ",
	}
}

// Helper methods for creating domain types from test fixture
func (f ResourceEventTestFixture) ValidResourceIdType() ResourceId {
	resourceId, _ := NewResourceId(f.ValidResourceId)
	return resourceId
}

func (f ResourceEventTestFixture) ValidResourceTypeType() ResourceType {
	resourceType, _ := NewResourceType(f.ValidResourceType)
	return resourceType
}

func (f ResourceEventTestFixture) ValidReporterTypeType() ReporterType {
	reporterType, _ := NewReporterType(f.ValidReporterType)
	return reporterType
}

func (f ResourceEventTestFixture) ValidReporterInstanceIdType() ReporterInstanceId {
	reporterInstanceId, _ := NewReporterInstanceId(f.ValidReporterInstanceId)
	return reporterInstanceId
}

func (f ResourceEventTestFixture) ValidReporterDataType() Representation {
	representation, _ := NewRepresentation(f.ValidReporterData)
	return representation
}

func (f ResourceEventTestFixture) ValidReporterResourceIdType() ReporterResourceId {
	reporterResourceId, _ := NewReporterResourceIdFromString(f.ValidReporterResourceID)
	return reporterResourceId
}

func (f ResourceEventTestFixture) ValidReporterVersionType() Version {
	return NewVersion(f.ValidReporterVersion)
}

func (f ResourceEventTestFixture) ValidReporterGenerationType() Generation {
	return NewGeneration(f.ValidReporterGeneration)
}

func (f ResourceEventTestFixture) ValidCommonDataType() Representation {
	representation, _ := NewRepresentation(f.ValidCommonData)
	return representation
}

func (f ResourceEventTestFixture) ValidCommonVersionType() Version {
	return NewVersion(f.ValidCommonVersion)
}

func (f ResourceEventTestFixture) ValidReporterVersionStrType() *ReporterVersion {
	if f.ValidReporterVersionStr == nil {
		return nil
	}
	reporterVersion, _ := NewReporterVersion(*f.ValidReporterVersionStr)
	return &reporterVersion
}

func (f ResourceEventTestFixture) NilReporterVersionStrType() *ReporterVersion {
	return nil
}

func (f ResourceEventTestFixture) AnotherResourceIdType() ResourceId {
	resourceId, _ := NewResourceId(f.AnotherResourceId)
	return resourceId
}

func (f ResourceEventTestFixture) AnotherResourceTypeType() ResourceType {
	resourceType, _ := NewResourceType(f.AnotherResourceType)
	return resourceType
}

func (f ResourceEventTestFixture) AnotherReporterDataType() Representation {
	representation, _ := NewRepresentation(f.AnotherReporterData)
	return representation
}

func (f ResourceEventTestFixture) AnotherCommonDataType() Representation {
	representation, _ := NewRepresentation(f.AnotherCommonData)
	return representation
}

func (f ResourceEventTestFixture) InvalidResourceIdType() ResourceId {
	return DeserializeResourceId(f.InvalidResourceId)
}

func (f ResourceEventTestFixture) EmptyResourceTypeType() ResourceType {
	return DeserializeResourceType(f.EmptyResourceType)
}

func (f ResourceEventTestFixture) WhitespaceResourceTypeType() ResourceType {
	return DeserializeResourceType(f.WhitespaceResourceType)
}

func (f ResourceEventTestFixture) EmptyReporterTypeType() ReporterType {
	return DeserializeReporterType(f.EmptyReporterType)
}

func (f ResourceEventTestFixture) EmptyReporterInstanceIdType() ReporterInstanceId {
	return DeserializeReporterInstanceId(f.EmptyReporterInstanceId)
}

func (f ResourceEventTestFixture) EmptyReporterDataType() Representation {
	return DeserializeRepresentation(f.EmptyReporterData)
}

func (f ResourceEventTestFixture) EmptyReporterResourceIdType() ReporterResourceId {
	return DeserializeReporterResourceId(uuid.Nil)
}

func (f ResourceEventTestFixture) EmptyCommonDataType() Representation {
	return DeserializeRepresentation(f.EmptyCommonData)
}
