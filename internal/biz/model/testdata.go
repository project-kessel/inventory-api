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
