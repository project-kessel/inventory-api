package model

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
)

const initialReporterRepresentationVersion = 0
const initialGeneration = 0
const initialTombstone = false

// Generic serialization interfaces
type Serializable[T any] interface {
	Serialize() T
}

type TinyType[T any] interface {
	Serializable[T]
}

// Generic serialization implementations
func SerializeString[U ~string](tinyType U) string {
	return string(tinyType)
}

func SerializeUint[U ~uint](tinyType U) uint {
	return uint(tinyType)
}

func SerializeUUID[U interface {
	~[16]byte
}](tinyType U) uuid.UUID {
	return uuid.UUID(tinyType)
}

func SerializeBool[U ~bool](tinyType U) bool {
	return bool(tinyType)
}

func Deserialize[U ~string](value string) U {
	return U(value)
}

func DeserializeUint[U ~uint](value uint) U {
	return U(value)
}

func DeserializeUUID[U interface {
	~[16]byte
}](value uuid.UUID) U {
	return U(value)
}

func DeserializeBool[U ~bool](value bool) U {
	return U(value)
}

// Type-specific deserialize functions for idiomatic usage
func DeserializeVersion(value uint) Version {
	return DeserializeUint[Version](value)
}

func DeserializeResourceId(value uuid.UUID) ResourceId {
	return DeserializeUUID[ResourceId](value)
}

func DeserializeReporterResourceId(value uuid.UUID) ReporterResourceId {
	return DeserializeUUID[ReporterResourceId](value)
}

func DeserializeResourceType(value string) ResourceType {
	return Deserialize[ResourceType](value)
}

func DeserializeReporterType(value string) ReporterType {
	return Deserialize[ReporterType](value)
}

func DeserializeReporterInstanceId(value string) ReporterInstanceId {
	return Deserialize[ReporterInstanceId](value)
}

func DeserializeConsistencyToken(value string) ConsistencyToken {
	return Deserialize[ConsistencyToken](value)
}

func DeserializeGeneration(value uint) Generation {
	return DeserializeUint[Generation](value)
}

func DeserializeReporterVersion(value string) ReporterVersion {
	return Deserialize[ReporterVersion](value)
}

func DeserializeTombstone(value bool) Tombstone {
	return DeserializeBool[Tombstone](value)
}

func DeserializeURI(value string) URI {
	return Deserialize[URI](value)
}

func DeserializeApiHref(value string) ApiHref {
	return ApiHref(DeserializeURI(value))
}

func DeserializeConsoleHref(value string) ConsoleHref {
	return ConsoleHref(DeserializeURI(value))
}

func DeserializeLocalResourceId(value string) LocalResourceId {
	return Deserialize[LocalResourceId](value)
}

// Helper for types that need special increment behavior
type Incrementable interface {
	Increment() Incrementable
}

type Version uint

func NewVersion(version uint) Version {
	return Version(version)
}

func (v Version) Uint() uint {
	return uint(v)
}

// Increment returns a new Version with the value incremented by 1.
// Note: This will rollover to 0 if the maximum uint value is reached
// (18,446,744,073,709,551,615 on 64-bit systems or 4,294,967,295 on 32-bit systems).
func (v Version) Increment() Version {
	return Version(uint(v) + 1)
}

func (v Version) Serialize() uint {
	return SerializeUint(v)
}

type ResourceId uuid.UUID

func NewResourceId(id uuid.UUID) (ResourceId, error) {
	if id == uuid.Nil {
		return ResourceId(uuid.Nil), fmt.Errorf("%w: ResourceId", ErrInvalidUUID)
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
	return SerializeUUID(r)
}

type ReporterResourceId uuid.UUID

func NewReporterResourceId(id uuid.UUID) (ReporterResourceId, error) {
	if id == uuid.Nil {
		return ReporterResourceId(uuid.Nil), fmt.Errorf("%w: ReporterResourceId", ErrInvalidUUID)
	}
	return ReporterResourceId(id), nil
}

func NewReporterResourceIdFromString(id string) (ReporterResourceId, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ReporterResourceId(uuid.Nil), fmt.Errorf("%w: ReporterResourceId", ErrEmpty)
	}

	parsedUUID, err := uuid.Parse(id)
	if err != nil {
		return ReporterResourceId(uuid.Nil), fmt.Errorf("%w: ReporterResourceId (%v)", ErrInvalidUUID, err)
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
	return SerializeUUID(rr)
}

type ResourceType string

func NewResourceType(resourceType string) (ResourceType, error) {
	resourceType = strings.TrimSpace(resourceType)
	if resourceType == "" {
		return ResourceType(""), fmt.Errorf("%w: ResourceType", ErrEmpty)
	}
	return ResourceType(resourceType), nil
}

func (rt ResourceType) String() string {
	return string(rt)
}

func (rt ResourceType) Serialize() string {
	return SerializeString(rt)
}

type ReporterType string

func NewReporterType(reporterType string) (ReporterType, error) {
	reporterType = strings.TrimSpace(reporterType)
	if reporterType == "" {
		return ReporterType(""), fmt.Errorf("%w: ReporterType", ErrEmpty)
	}
	return ReporterType(reporterType), nil
}

func (rt ReporterType) String() string {
	return string(rt)
}

func (rt ReporterType) Serialize() string {
	return SerializeString(rt)
}

type ReporterInstanceId string

func NewReporterInstanceId(reporterInstanceId string) (ReporterInstanceId, error) {
	reporterInstanceId = strings.TrimSpace(reporterInstanceId)
	if reporterInstanceId == "" {
		return ReporterInstanceId(""), fmt.Errorf("%w: ReporterInstanceId", ErrEmpty)
	}
	return ReporterInstanceId(reporterInstanceId), nil
}

func (ri ReporterInstanceId) String() string {
	return string(ri)
}

func (ri ReporterInstanceId) Serialize() string {
	return SerializeString(ri)
}

type ConsistencyToken string

func NewConsistencyToken(token string) (ConsistencyToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return ConsistencyToken(""), fmt.Errorf("%w: ConsistencyToken", ErrEmpty)
	}
	return ConsistencyToken(token), nil
}

func (ct ConsistencyToken) String() string {
	return string(ct)
}

func (ct ConsistencyToken) Serialize() string {
	return SerializeString(ct)
}

type Generation uint

func NewGeneration(generation uint) Generation {
	return Generation(generation)
}

func (g Generation) Uint() uint {
	return uint(g)
}

// Increment returns a new Generation with the value incremented by 1.
// Note: This will rollover to 0 if the maximum uint value is reached
// (18,446,744,073,709,551,615 on 64-bit systems or 4,294,967,295 on 32-bit systems).
func (g Generation) Increment() Generation {
	return Generation(uint(g) + 1)
}

func (g Generation) Serialize() uint {
	return SerializeUint(g)
}

type ReporterVersion string

func NewReporterVersion(version string) (ReporterVersion, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return ReporterVersion(""), fmt.Errorf("%w: ReporterVersion", ErrEmpty)
	}
	return ReporterVersion(version), nil
}

func (rv ReporterVersion) String() string {
	return string(rv)
}

func (rv ReporterVersion) Serialize() string {
	return SerializeString(rv)
}

type Tombstone bool

func NewTombstone(tombstone bool) Tombstone {
	return Tombstone(tombstone)
}

func (t Tombstone) Bool() bool {
	return bool(t)
}

func (t Tombstone) Serialize() bool {
	return SerializeBool(t)
}

type URI string

func NewURI(uri string) (URI, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return URI(""), fmt.Errorf("%w: URI", ErrEmpty)
	}
	return URI(uri), nil
}

func (u URI) String() string {
	return string(u)
}

func (u URI) Serialize() string {
	return SerializeString(u)
}

// Type aliases for backward compatibility and semantic clarity
type ApiHref = URI
type ConsoleHref = URI

func NewApiHref(href string) (ApiHref, error) {
	uri, err := NewURI(href)
	if err != nil {
		return ApiHref(""), fmt.Errorf("%w: ApiHref", ErrEmpty)
	}
	return ApiHref(uri), nil
}

func NewConsoleHref(href string) (ConsoleHref, error) {
	uri, err := NewURI(href)
	if err != nil {
		return ConsoleHref(""), fmt.Errorf("%w: ConsoleHref", ErrEmpty)
	}
	return ConsoleHref(uri), nil
}

type LocalResourceId string

func NewLocalResourceId(id string) (LocalResourceId, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return LocalResourceId(""), fmt.Errorf("%w: LocalResourceId", ErrEmpty)
	}
	return LocalResourceId(id), nil
}

func (lr LocalResourceId) String() string {
	return string(lr)
}

func (lr LocalResourceId) Serialize() string {
	return SerializeString(lr)
}

type Representation internal.JsonObject

func NewRepresentation(data internal.JsonObject) (Representation, error) {
	if data == nil {
		return nil, fmt.Errorf("representation data cannot be nil")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("representation data cannot be empty")
	}
	return Representation(data), nil
}

func (r Representation) Data() internal.JsonObject {
	return internal.JsonObject(r)
}

func (r Representation) Serialize() internal.JsonObject {
	return internal.JsonObject(r)
}

func DeserializeRepresentation(data internal.JsonObject) Representation {
	return Representation(data)
}
