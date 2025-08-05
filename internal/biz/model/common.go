package model

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/project-kessel/inventory-api/internal"
)

const initialReporterRepresentationVersion = 0
const initialGeneration = 0
const initialTombstone = true

type Version uint

func NewVersion(version uint) Version {
	return Version(version)
}

func (v Version) Uint() uint {
	return uint(v)
}

func (v Version) Increment() Version {
	return Version(uint(v) + 1)
}

func (v Version) Serialize() uint {
	return uint(v)
}

func DeserializeVersion(value uint) Version {
	return Version(value)
}

type ResourceId uuid.UUID

func NewResourceId(id uuid.UUID) (ResourceId, error) {
	if id == uuid.Nil {
		return ResourceId(uuid.Nil), fmt.Errorf("ResourceId cannot be empty")
	}
	return ResourceId(id), nil
}

func (r ResourceId) UUID() uuid.UUID {
	return uuid.UUID(r)
}

func (r ResourceId) String() string {
	return uuid.UUID(r).String()
}

func (r ResourceId) Serialize() uuid.UUID {
	return uuid.UUID(r)
}

func DeserializeResourceId(value uuid.UUID) ResourceId {
	return ResourceId(value)
}

type ReporterResourceId uuid.UUID

func NewReporterResourceId(id uuid.UUID) (ReporterResourceId, error) {
	if id == uuid.Nil {
		return ReporterResourceId(uuid.Nil), fmt.Errorf("ReporterResourceId cannot be empty")
	}
	return ReporterResourceId(id), nil
}

func NewReporterResourceIdFromString(id string) (ReporterResourceId, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ReporterResourceId(uuid.Nil), fmt.Errorf("ReporterResourceId cannot be empty")
	}

	parsedUUID, err := uuid.Parse(id)
	if err != nil {
		return ReporterResourceId(uuid.Nil), fmt.Errorf("invalid ReporterResourceId format: %w", err)
	}

	return NewReporterResourceId(parsedUUID)
}

func (rr ReporterResourceId) UUID() uuid.UUID {
	return uuid.UUID(rr)
}

func (rr ReporterResourceId) String() string {
	return uuid.UUID(rr).String()
}

func (rr ReporterResourceId) Serialize() uuid.UUID {
	return uuid.UUID(rr)
}

func DeserializeReporterResourceId(value uuid.UUID) ReporterResourceId {
	return ReporterResourceId(value)
}

type ResourceType string

func NewResourceType(resourceType string) (ResourceType, error) {
	resourceType = strings.TrimSpace(resourceType)
	if resourceType == "" {
		return ResourceType(""), fmt.Errorf("ResourceType cannot be empty")
	}
	return ResourceType(resourceType), nil
}

func (rt ResourceType) String() string {
	return string(rt)
}

func (rt ResourceType) Serialize() string {
	return string(rt)
}

func DeserializeResourceType(value string) ResourceType {
	return ResourceType(value)
}

type ReporterType string

func NewReporterType(reporterType string) (ReporterType, error) {
	reporterType = strings.TrimSpace(reporterType)
	if reporterType == "" {
		return ReporterType(""), fmt.Errorf("ReportedByReporterType cannot be empty")
	}
	return ReporterType(reporterType), nil
}

func (rt ReporterType) String() string {
	return string(rt)
}

func (rt ReporterType) Serialize() string {
	return string(rt)
}

func DeserializeReporterType(value string) ReporterType {
	return ReporterType(value)
}

type ReporterInstanceId string

func NewReporterInstanceId(reporterInstanceId string) (ReporterInstanceId, error) {
	reporterInstanceId = strings.TrimSpace(reporterInstanceId)
	if reporterInstanceId == "" {
		return ReporterInstanceId(""), fmt.Errorf("ReporterInstanceId cannot be empty")
	}
	return ReporterInstanceId(reporterInstanceId), nil
}

func (ri ReporterInstanceId) String() string {
	return string(ri)
}

func (ri ReporterInstanceId) Serialize() string {
	return string(ri)
}

func DeserializeReporterInstanceId(value string) ReporterInstanceId {
	return ReporterInstanceId(value)
}

type ConsistencyToken string

func NewConsistencyToken(token string) (ConsistencyToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return ConsistencyToken(""), fmt.Errorf("ConsistencyToken cannot be empty")
	}
	return ConsistencyToken(token), nil
}

func (ct ConsistencyToken) String() string {
	return string(ct)
}

func (ct ConsistencyToken) Serialize() string {
	return string(ct)
}

func DeserializeConsistencyToken(value string) ConsistencyToken {
	return ConsistencyToken(value)
}

type Generation uint

func NewGeneration(generation uint) Generation {
	return Generation(generation)
}

func (g Generation) Uint() uint {
	return uint(g)
}

func (g Generation) Serialize() uint {
	return uint(g)
}

func DeserializeGeneration(value uint) Generation {
	return Generation(value)
}

type ReporterVersion string

func NewReporterVersion(version string) (ReporterVersion, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return ReporterVersion(""), fmt.Errorf("ReporterVersion cannot be empty")
	}
	return ReporterVersion(version), nil
}

func (rv ReporterVersion) String() string {
	return string(rv)
}

func (rv ReporterVersion) Serialize() string {
	return string(rv)
}

func DeserializeReporterVersion(value string) ReporterVersion {
	return ReporterVersion(value)
}

type Tombstone bool

func NewTombstone(tombstone bool) Tombstone {
	return Tombstone(tombstone)
}

func (t Tombstone) Bool() bool {
	return bool(t)
}

func (t Tombstone) Serialize() bool {
	return bool(t)
}

func DeserializeTombstone(value bool) Tombstone {
	return Tombstone(value)
}

type ApiHref string

func NewApiHref(href string) (ApiHref, error) {
	href = strings.TrimSpace(href)
	if href == "" {
		return ApiHref(""), fmt.Errorf("ApiHref cannot be empty")
	}
	return ApiHref(href), nil
}

func (ah ApiHref) String() string {
	return string(ah)
}

func (ah ApiHref) Serialize() string {
	return string(ah)
}

func DeserializeApiHref(value string) ApiHref {
	return ApiHref(value)
}

type ConsoleHref string

func NewConsoleHref(href string) (ConsoleHref, error) {
	href = strings.TrimSpace(href)
	if href == "" {
		return ConsoleHref(""), fmt.Errorf("ConsoleHref cannot be empty")
	}
	return ConsoleHref(href), nil
}

func (ch ConsoleHref) String() string {
	return string(ch)
}

func (ch ConsoleHref) Serialize() string {
	return string(ch)
}

func DeserializeConsoleHref(value string) ConsoleHref {
	return ConsoleHref(value)
}

type LocalResourceId string

func NewLocalResourceId(id string) (LocalResourceId, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return LocalResourceId(""), fmt.Errorf("LocalResourceId cannot be empty")
	}
	return LocalResourceId(id), nil
}

func (lr LocalResourceId) String() string {
	return string(lr)
}

func (lr LocalResourceId) Serialize() string {
	return string(lr)
}

func DeserializeLocalResourceId(value string) LocalResourceId {
	return LocalResourceId(value)
}

// JsonObject is an alias to internal.JsonObject for consistency
type JsonObject = internal.JsonObject

type Representation struct {
	data JsonObject
}
