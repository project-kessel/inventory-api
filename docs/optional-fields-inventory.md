# Optional Fields by Structure (inventory-api)

This document lists **optional** fields in each struct across the module `github.com/project-kessel/inventory-api`. A field is considered optional if it is:

- A **pointer type** (`*T`) — can be nil
- Tagged with **`omitempty`** in JSON/other struct tags
- **Proto3 optional** or **oneof** in generated API types (noted in API section)

Use this list for planning and refactoring (e.g. validation, API contracts, DB nullability).

---

## 1. internal/biz/model (domain models & snapshots)

### CommonRepresentation (biz)
- No optional **exported** fields (struct uses unexported fields; snapshot type below carries the data).

### CommonRepresentationSnapshot
- **None** — all fields required in snapshot (TransactionId is string, can be empty).

### RepresentationSnapshot
- **None** — single field `Data` (JsonObject) is required.

### ResourceSnapshot
- **None** — all fields required.

### ReporterResourceKeySnapshot
- **None** — all fields required.

### ReporterResourceSnapshot
- **None** — all fields required.

### ReporterRepresentationSnapshot
- **ReporterVersion** `*string` — optional; nil when not provided.

### Representations
- **commonVersion** `*uint` — optional; paired with common data (both present or both absent).
- **reporterRepresentationVersion** `*uint` — optional; paired with reporter data (both present or both absent).

### ReportResourceCommand (internal/biz/usecase/resources/commands.go)
- **ConsoleHref** `*model.ConsoleHref`
- **ReporterVersion** `*model.ReporterVersion`
- **TransactionId** `*model.TransactionId`
- **ReporterRepresentation** `*model.Representation`
- **CommonRepresentation** `*model.Representation`

### SubjectReference
- **relation** (unexported) `*Relation` — nil means no relation (direct subject reference).

### SchemaService
- **Log** `*log.Helper` — optional logger.

### Resource (domain)
- Uses **reporterVersion** `*ReporterVersion` in `NewResource` and `Update` (optional).

### ResourceReportEvent / DeserializeResourceEvent
- **reporterRepresentationSnapshot** `*ReporterRepresentationSnapshot` — optional argument.
- **commonRepresentationSnapshot** `*CommonRepresentationSnapshot` — optional argument.

### DeserializeResource
- **resourceSnapshot** `*ResourceSnapshot` — can be nil (returns nil).
- **reporterRepresentationSnapshot** `*ReporterRepresentationSnapshot` — optional.
- **commonRepresentationSnapshot** `*CommonRepresentationSnapshot` — optional.

### TupleEvent (tuple_event.go)
- **CommonVersion** `*Version` `json:"common_version,omitempty"`
- **ReporterRepresentationVersion** `*Version` `json:"reporter_representation_version,omitempty"`

### ValidationError
- No optional fields (Field, Message both required).

### Consistency (consistency.go)
- No optional **fields**; semantics use zero value (empty token = minimize latency).

---

## 2. internal/data/model (GORM / persistence)

### CommonRepresentation (data)
- **None** — TransactionId is string; DB allows NULL via unique index `where: transaction_id IS NOT NULL AND transaction_id != ''`.

### ReporterRepresentation (data)
- **ReporterVersion** `*string` — optional; nullable in DB.

### Resource (data)
- **None** — all fields required (ConsistencyToken is string).

### ReporterResource (data)
- **None** — all required in schema.

---

## 3. internal/config

### OptionsConfig
- **Authn** `*authn.Options`
- **Authz** `*authz.Options`
- **Storage** `*storage.Options`
- **Consumer** `*consumer.Options`
- **Server** `*server.Options`
- **Consistency** `*consistency.Options`
- **Service** `*service.Options`
- **Schema** `*schema.Options`
- **SelfSubjectStrategy** `*selfsubject.Options`  
*(All are pointers; typically set in NewOptionsConfig but can be overridden.)*

---

## 4. internal/server

### Server
- **HttpServer** `*khttp.Server`
- **GrpcServer** `*kgrpc.Server`
- **App** `*kratos.App`

### Options (server)
- **GrpcOptions** `*grpc.Options`
- **HttpOptions** `*http.Options`
- **PprofOptions** `*pprof.Options`

### Config (server/http and server/grpc)
- **Options** `*Options`
- **TLSConfig** `*tls.Config`

---

## 5. internal/consumer

### InventoryConsumer
- **DB** `*gorm.DB`
- **MetricsCollector** `*metricscollector.MetricsCollector`
- **Logger** `*log.Helper`
- **AuthOptions** `*auth.Options`
- **RetryOptions** `*retry.Options`
- **SchemaService** `*model.SchemaService`

---

## 6. internal/biz/usecase/resources

### Usecase
- **schemaService** `*model.SchemaService`
- **waitForNotifBreaker** `*gobreaker.CircuitBreaker`
- **Log** `*log.Helper`
- **Config** `*UsecaseConfig`
- **MetricsCollector** `*metricscollector.MetricsCollector`

---

## 7. internal/service/resources

### InventoryService
- **Ctl** `*resources.Usecase`

---

## 8. internal/authn

### Options (authn)
- **Authenticator** `*AuthenticatorOptions`
- **AllowUnauthenticated** `*bool`
- **OIDC** `*oidc.Options`

### ChainEntryOptions (authn)
- **Enable** `*bool` — optional; defaults to true.
- **Transport** `*Transport` — optional; defaults to both HTTP and gRPC.

### Transport (authn)
- **HTTP** `*bool`
- **GRPC** `*bool`

### Config (authn)
- **Authenticator** `*AuthenticatorConfig`

### AuthenticatorCompletedConfig
- **OIDCConfig** `*oidc.CompletedConfig`

### AuthzContext (authn/api)
- **Subject** `*Claims` — optional; unauthenticated when nil.

### OAuth2Authenticator (oidc)
- **Verifier** `*coreosoidc.IDTokenVerifier`

### FirstMatchAuthenticator
- **logger** `*log.Helper`

---

## 9. internal/authz

### Options (authz)
- **Kessel** `*kessel.Options`

### Config (authz)
- **Kessel** `*kessel.Config`

### AllowAllAuthz
- **Logger** `*log.Helper`

### KesselAuthz
- **tokenClient** `*tokenClient`
- **Logger** `*log.Helper`

---

## 10. internal/subject/selfsubject

### Options (selfsubject)
- **RedHatRbac** `*RedHatRbacOptions`

---

## 11. internal/config/schema

### Options (schema)
- **InMemory** `*inmemory.Options`

### Config (schema)
- **InMemory** `*inmemory.Config`

---

## 12. internal/biz/model_legacy (outbox events / legacy)

### EventResourceData
- **ResourceData** `internal.JsonObject` `json:"resource_data,omitempty"`

### EventRelationshipData
- **ResourceData** `internal.JsonObject` `json:"resource_data,omitempty"`

### EventResourceMetadata
- **CreatedAt** `*time.Time` `json:"created_at,omitempty"`
- **UpdatedAt** `*time.Time` `json:"updated_at,omitempty"`
- **DeletedAt** `*time.Time` `json:"deleted_at,omitempty"`
- **Labels** `[]EventResourceLabel` `json:"labels,omitempty"`

### EventResourceReporter
- **ReporterVersion** `*string` `json:"reporter_version"` (no omitempty but pointer => optional)

### EventRelationshipMetadata
- **CreatedAt** `*time.Time` `json:"created_at,omitempty"`
- **UpdatedAt** `*time.Time` `json:"updated_at,omitempty"`
- **DeletedAt** `*time.Time` `json:"deleted_at,omitempty"`

---

## 13. internal/data (repositories, migrations)

### resourceRepository
- **db** `*gorm.DB`

### healthRepo
- **DB** `*gorm.DB`

### gormTransactionManager
- **metricsCollector** `*metricscollector.MetricsCollector`

### Migration schema (20251210120000_initial_schema.go) — ReporterRepresentation
- **ReporterVersion** `*string`

### connWrapper (locks.go)
- **conn** `*sql.Conn`

---

## 14. internal/middleware

### StreamAuthInterceptor
- **cfg** `*StreamAuthConfig`

---

## 15. internal/server/pprof

### Server (pprof)
- **server** `*http.Server`
- **logger** `*log.Helper`

---

## 16. API (generated protobuf — api/kessel/inventory/v1beta2, v1)

Generated `.pb.go` types use pointers and `omitempty` for optional/oneof fields. Summary of **optional or oneof** fields:

- **CheckSelfResponse**: ConsistencyToken `*ConsistencyToken`
- **CheckSelfBulkResponse(Item/Pair)**: Request, Item, Error (oneof), Pairs, ConsistencyToken
- **SubjectReference**: Relation (oneof), Resource `*ResourceReference`
- **StreamedListObjectsResponse**: Object, Pagination, ConsistencyToken
- **StreamedListObjectsRequest**: ObjectType, Subject, Pagination (oneof), Consistency (oneof)
- **ResourceRepresentations**: Metadata, Common (oneof), Reporter (oneof)
- **ResourceReference**: Reporter `*ReporterReference` (oneof)
- **RequestPagination**: ContinuationToken `*string` (oneof)
- **RepresentationType**: ReporterType `*string` (oneof)
- **RepresentationMetadata**: ConsoleHref (oneof), ReporterVersion (oneof)
- **ReporterReference**: InstanceId `*string` (oneof)
- **ReportResourceRequest**: InventoryId (oneof), Representations `*ResourceRepresentations`
- **DeleteResourceRequest**: Reference `*ResourceReference`
- **Consistency**: AtLeastAsFresh `*ConsistencyToken` (oneof)
- **CheckSelfRequest**: Object, Consistency (oneof)
- **CheckSelfBulkRequest**: Items, Consistency (oneof)
- **CheckRequest / CheckForUpdateRequest**: Object, Subject
- **CheckBulkRequest**: Object, Subject, Items, Consistency
- **CheckBulkResponse(Item/Pair)**: same pattern as CheckSelfBulk (Pairs, ConsistencyToken, oneof Item/Error)
- **health (v1)**: GetLivezResponse / GetReadyzResponse — Status, Code (proto3 optional style with omitempty)

*(Full field-level detail is in the respective `*.pb.go` files.)*

---

## Summary counts (non-generated, project structs)

| Area              | Structs with optional fields |
|-------------------|-------------------------------|
| biz/model         | Snapshots (ReporterRepresentationSnapshot), Representations, SubjectReference, TupleEvent, SchemaService, Resource (reporterVersion), DeserializeResource/DeserializeResourceEvent params |
| biz/usecase       | ReportResourceCommand (5 optional), Usecase (5 pointer deps) |
| data/model        | ReporterRepresentation (ReporterVersion) |
| config            | OptionsConfig (all 9 option groups as pointers) |
| server            | Server (3), Options (3), Config (Options, TLSConfig) |
| consumer          | InventoryConsumer (6 pointer deps) |
| authn             | Options, ChainEntry, Transport, Config, AuthzContext, OIDC, FirstMatch |
| authz             | Options, Config, AllowAllAuthz, KesselAuthz |
| model_legacy      | EventResourceData, EventRelationshipData, EventResourceMetadata, EventResourceReporter, EventRelationshipMetadata |
| data (internal)   | Repos, transaction manager, migration schema, connWrapper |
| middleware/pprof  | StreamAuthInterceptor, pprof.Server |

This list can be used to cross-verify with code and to plan validation, API defaults, and DB nullability.

---

**Related:** For optional fields whose **usage is inconsistent** with their struct definition (e.g. zero value vs nil, or pointer vs value across layers), see [optional-fields-inconsistent-handling.md](./optional-fields-inconsistent-handling.md).
