package model

import (
	"github.com/google/uuid"
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
	ValidData   JsonObject
	AnotherData JsonObject
	EmptyData   JsonObject
	NilData     JsonObject
}

func NewJsonObjectTestFixture() JsonObjectTestFixture {
	return JsonObjectTestFixture{
		ValidData: JsonObject{
			"name":      "test-cluster",
			"status":    "active",
			"nodeCount": 3,
			"region":    "us-east-1",
			"labels":    map[string]interface{}{"env": "prod", "team": "platform"},
		},
		AnotherData: JsonObject{
			"hostname": "test-host",
			"os":       "linux",
			"memory":   "16GB",
			"cpu":      "4 cores",
			"uptime":   "30 days",
		},
		EmptyData: JsonObject{},
		NilData:   nil,
	}
}

type CommonRepresentationTestFixture struct {
	ValidResourceId                 uuid.UUID
	ValidData                       JsonObject
	ValidVersion                    uint
	ValidReportedByReporterType     string
	ValidReportedByReporterInstance string
	NilResourceId                   uuid.UUID
	EmptyData                       JsonObject
	NilData                         JsonObject
	ZeroVersion                     uint
	EmptyReporterType               string
	WhitespaceReporterType          string
	EmptyReporterInstance           string
	WhitespaceReporterInstance      string
}

func NewCommonRepresentationTestFixture() CommonRepresentationTestFixture {
	return CommonRepresentationTestFixture{
		ValidResourceId: uuid.MustParse("550e8400-e29b-41d4-a716-446655440123"),
		ValidData: JsonObject{
			"name":        "test-resource",
			"description": "A test resource for CommonRepresentation",
			"metadata":    map[string]interface{}{"version": "1.0", "type": "test"},
		},
		ValidVersion:                    1,
		ValidReportedByReporterType:     "test-reporter",
		ValidReportedByReporterInstance: "test-instance-001",
		NilResourceId:                   uuid.Nil,
		EmptyData:                       JsonObject{},
		NilData:                         nil,
		ZeroVersion:                     0,
		EmptyReporterType:               "",
		WhitespaceReporterType:          "  \t\n  ",
		EmptyReporterInstance:           "",
		WhitespaceReporterInstance:      "  \t\n  ",
	}
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
