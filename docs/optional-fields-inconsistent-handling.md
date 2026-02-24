# Optional Fields — Inconsistent Handling (Usage vs Struct Definition)

This document lists **optional fields whose usage/consumption is inconsistent with their struct definition** or with how the same concept is represented elsewhere. Use it for refactoring and to align "optional" semantics (nil vs zero value, pointer vs value type) across layers.

---

## 1. TransactionId

| Layer | Struct / API | Type | "Empty" / optional meaning |
|-------|----------------|------|----------------------------|
| Command | `ReportResourceCommand.TransactionId` | `*model.TransactionId` | `nil` = not provided |
| Domain | `NewResource(..., transactionId TransactionId, ...)`, `Update(..., transactionId TransactionId)` | `TransactionId` (value) | `""` = generate new ID |
| Snapshot | `CommonRepresentationSnapshot.TransactionId`, `ReporterRepresentationSnapshot.TransactionId` | `string` | `""` = no idempotency key |
| Repo | `HasTransactionIdBeenProcessed(tx, transactionId string)` | `string` | `""` → returns false (not processed) |

**Inconsistencies:**

- **Optionality encoding:** Command uses **pointer** (nil = absent); domain and snapshot use **value type** (empty string = absent/generate). So "optional" is expressed as nil in one layer and as empty string in others.
- **Struct definition:** Snapshots use `string`, not `*string`, so "optional" is not reflected in the type; callers must use the convention that `""` means "not set."
- **Cross-check:** `resolveOptionalFields` turns `cmd.TransactionId == nil` into `transactionId = ""`, and `resource.go` does `if transactionId == "" { transactionId, err = GenerateTransactionId() }`. Behavior is consistent; only the type convention differs.

**Recommendation:** Decide on a single convention (e.g. pointer in all DTOs/snapshots where optional, or consistently document "empty string = not set" and keep value types). If DB allows NULL for `transaction_id`, consider `*string` in snapshots and mapping nil ↔ NULL.

---

## 2. ConsoleHref

| Layer | Struct / API | Type | "Empty" / optional meaning |
|-------|----------------|------|----------------------------|
| Command | `ReportResourceCommand.ConsoleHref` | `*model.ConsoleHref` | `nil` = not provided |
| Domain | `ReporterResource.consoleHref`, `Update(..., consoleHref ConsoleHref, ...)` | `ConsoleHref` (value, alias for URI) | `""` = no console link |
| Legacy event | `EventResourceReporter.ConsoleHref` | `string` | `""` = absent |
| Data (FindResourceByKeysResult) | `ConsoleHref` | `string` | |

**Inconsistencies:**

- **Optional at API, value in domain:** Command uses **pointer** for optional; domain uses **value type** and allows empty string (`NewConsoleHref("")` is used in tests). So "absent" is represented as nil in the command but as empty string in the domain.
- **resolveOptionalFields:** When `cmd.ConsoleHref == nil`, we pass zero value `ConsoleHref("")` into the domain. Domain and `ReporterResource` accept empty string; no separate "absent" notion in the type.

**Recommendation:** Either (a) keep current semantics and document that "optional" at API means "may be empty string" in domain, or (b) introduce an explicit optional type in the domain (e.g. `*ConsoleHref`) so nil is represented consistently.

---

## 3. ReporterVersion

| Layer | Struct / API | Type | "Empty" / optional meaning |
|-------|----------------|------|----------------------------|
| Command | `ReportResourceCommand.ReporterVersion` | `*model.ReporterVersion` | `nil` = not provided |
| Domain | `ResourceReportEvent.reporterRepresentation.reporterVersion` | `*ReporterVersion` | `nil` = not provided |
| Snapshot | `ReporterRepresentationSnapshot.ReporterVersion` | `*string` | `nil` = not provided |
| Data model | `ReporterRepresentation.ReporterVersion` | `*string` | `nil` = not provided |
| Legacy | `EventResourceReporter.ReporterVersion` | `*string` | |
| Legacy | `EventRelationshipReporter.ReporterVersion` | **`string`** | |

**Inconsistencies:**

- **EventRelationshipReporter vs EventResourceReporter:** In the same legacy package, reporter version is **`*string`** in `EventResourceReporter` but **`string`** in `EventRelationshipReporter`. So "optional" is pointer in one and value (empty string) in the other.
- **Empty string vs nil:** Data model tests expect `ReporterVersion` to round-trip as nil when nil and as `""` when `&""`. So both "absent" (nil) and "present but empty" (pointer to "") are possible; consumption must handle both if that distinction is desired.

**Recommendation:** Use the same type for both legacy reporter structs (prefer `*string` with nil = absent). Document or normalize whether "" is a valid reporter version or should be treated like nil.

---

## 4. ReporterRepresentation and CommonRepresentation (command)

| Location | Definition | Usage |
|----------|------------|--------|
| `ReportResourceCommand` | `ReporterRepresentation *model.Representation`, `CommonRepresentation *model.Representation` | Optional (pointers) |
| `ReportResource` validation (resource_service.go) | | **Requires both non-nil:** `if cmd.ReporterRepresentation == nil` / `if cmd.CommonRepresentation == nil` return error |
| Domain `Representations` | At least one of common or reporter must be present | Allows "common only" or "reporter only" |

**Inconsistencies:**

- **Struct vs validation:** Command defines both as optional (pointers), but the current ReportResource path **requires both** to be non-nil. So at the struct level they are optional; at the usecase entrypoint they are effectively required.
- **Domain vs usecase:** Domain model `Representations` allows at least one of common/reporter; usecase does not currently allow reporter-only or common-only for ReportResource.

**Recommendation:** Either (a) document that for ReportResource both are required and consider making them non-pointer in the command for this flow, or (b) align with the domain and allow at least one, and handle partial updates explicitly (including validation and resolveOptionalFields).

---

## 5. Config option pointers (OptionsConfig and nested Options)

| Struct | Fields | Usage |
|--------|--------|--------|
| `OptionsConfig` | `Authn`, `Authz`, `Storage`, `Consumer`, `Server`, `Consistency`, `Service`, `Schema`, `SelfSubjectStrategy` — all `*Options` | `config.go`: `options.Server.PublicUrl`, `options.Server.HttpOptions.Addr`, `options.Authz.Authz`, `options.Authz.Kessel.URL`, `options.Storage.Postgres.Host`, `options.Consumer.BootstrapServers`, etc. — **no nil checks** |
| `server.Options` | `GrpcOptions`, `HttpOptions`, `PprofOptions` — all pointers | AddFlags/Complete/Validate use them; typically created in NewOptions() |
| `authz.Options` | `Kessel *kessel.Options` | `config.go`: `options.Authz.Authz`, `options.Authz.Kessel.URL` — no nil check on `Authz` or `Kessel` |

**Inconsistencies:**

- **Optional at struct level, required in use:** Options are pointer types (so technically optional), but `LogConfigurationInfo`, `InjectClowdAppConfig`, and `Configure*` assume they are set and dereference without nil checks. If config is ever built partially (e.g. from tests or a minimal config), this can panic.

**Recommendation:** Either (a) guarantee all option groups are always set (e.g. constructors only) and document that, or (b) add nil checks before use and define behavior when an option group is nil (e.g. skip logging, or fail fast with a clear error).

---

## 6. Authn chain entry: Enable and Transport

| Struct | Field | Type | Usage |
|--------|--------|------|--------|
| `ChainEntryOptions` / `ChainEntry` | `Enable` | `*bool` | `config.go`: `if entry.Enable != nil { baseEnabled = *entry.Enable }` — nil means default (true). Handled. |
| | `Transport` | `*Transport` | Processed in `completeChainEntry`; nil means "both HTTP and gRPC." Handled. |
| `Transport` | `HTTP`, `GRPC` | `*bool` | Used to derive EnabledHTTP / EnabledGRPC. Handled. |

**Consistency:** Optional semantics are documented and code checks for nil before dereference. No change needed for consistency; only ensure any new config paths also respect nil as "use default."

---

## 7. Timestamps: time.Time (value) vs *time.Time (pointer)

| Layer | Struct / Field | Type | "Empty" / optional meaning |
|-------|----------------|------|----------------------------|
| Domain | `ResourceReportEvent.createdAt`, `updatedAt` | `time.Time` | Value type; **IsZero()** used in tests to mean "not set" (e.g. legacy DB with no backfill) |
| Snapshot | `ResourceSnapshot.CreatedAt`, `UpdatedAt`; `ReporterResourceSnapshot`; `CommonRepresentationSnapshot` | `time.Time` | Value type; zero = "not set" |
| Legacy event | `EventResourceMetadata.CreatedAt`, `UpdatedAt`, `DeletedAt`; `EventRelationshipMetadata` same | `*time.Time` with `omitempty` | **nil** = not set |

**Inconsistencies:**

- **Two ways to mean "optional time":** Domain and snapshots use **value type + IsZero()**; legacy outbox events use **pointer + nil + omitempty**. So "optional timestamp" is represented as zero value in one place and as nil in another.
- **Consumption:** Code that builds legacy events from domain (e.g. `newResourceEvent`) uses `resourceEvent.CreatedAt()` which returns `*time.Time` (address of field), so it is never nil. Only which timestamp is set (created vs updated vs deleted) varies. So the inconsistency is mainly in the **shape** of the struct (value vs pointer) across layers, not in runtime nil dereference.

**Recommendation:** Standardize on one convention for "optional timestamp" in DTOs/snapshots/events (e.g. pointer + nil, or value + IsZero() and document it). If the system needs to distinguish "unknown" from "known zero time," prefer pointer.

---

## 8. Consistency (atLeastAsFresh)

| Struct | Field | Type | "Empty" / optional meaning |
|--------|--------|------|----------------------------|
| `model.Consistency` | `atLeastAsFresh` (unexported) | `ConsistencyToken` (value) | **Empty string** = minimize latency; non-empty = at-least-as-fresh |
| Proto | `Consistency` oneof | | MinimizeLatency vs AtLeastAsFresh(token) |

**Consistency:** Optional semantics are encoded as **zero value** (empty string) in the domain. No pointer; usage is consistent (e.g. `MinimizeLatency()` is `c.atLeastAsFresh == ""`). No change needed for consistency.

---

## 9. SubjectReference.Relation

| Struct | Field | Type | Usage |
|--------|--------|------|--------|
| `model.SubjectReference` | `relation` (unexported) | `*Relation` | **nil** = no relation (direct subject); non-nil = relation present. Documented and used consistently. |

**Consistency:** Optional relation is correctly represented as pointer; nil is checked. No change needed.

---

## 10. Representations (commonVersion / reporterRepresentationVersion)

| Struct | Fields | Type | Usage |
|--------|--------|------|--------|
| `Representations` | `commonVersion`, `reporterRepresentationVersion` | `*uint` | **nil** when that representation is absent; paired with data (both present or both absent). `NewRepresentations` validates pairing. |
| Consumer / tuple event | `TupleEvent.CommonVersion()`, `ReporterRepresentationVersion()` | `*Version` | Consumer checks `if tupleEvent.CommonVersion() != nil` before use. |

**Consistency:** Optional versions are pointers; callers check for nil. No inconsistency.

---

## 11. resolveOptionalFields and "zero value" defaults

| Location | Behavior |
|----------|----------|
| `resource_service.resolveOptionalFields` | When a command pointer is nil, returns the **zero value** for that type: empty string for `TransactionId`, empty `ConsoleHref`, empty `Representation` for reporter/common. |

**Inconsistencies:**

- **Optional (pointer) vs default (zero value):** Optional is encoded as nil in the command, but downstream we pass **value types** whose zero value is used as the default. So "optional" is not carried through the type system past the command; it collapses to "zero value" in the domain. This is consistent with the current domain design (e.g. empty TransactionId triggers generation, empty ConsoleHref is valid) but ties "optional" to zero-value semantics and makes it easy to confuse "not provided" with "provided and empty" where both are valid (e.g. ConsoleHref, or ReporterVersion if we ever allowed "").

**Recommendation:** Keep the TODO (RHCLOUD-41760) to model optional fields explicitly. Consider keeping optional as pointer (or a dedicated optional type) through the usecase layer so that "not provided" vs "provided and empty" can be distinguished where needed.

---

## Summary: Inconsistent optional fields (for refactoring)

| # | Field(s) | Inconsistency |
|---|----------|----------------|
| 1 | **TransactionId** | Optional as pointer in command; value type (empty string) in domain/snapshot/repo. Same semantics, different type convention. |
| 2 | **ConsoleHref** | Optional as pointer in command; value type (empty string) in domain. No type-level "absent" in domain. |
| 3 | **ReporterVersion** (legacy) | `EventResourceReporter.ReporterVersion` is `*string`; `EventRelationshipReporter.ReporterVersion` is `string`. Inconsistent in one package. |
| 4 | **ReporterRepresentation / CommonRepresentation** (command) | Defined as optional (pointers) but ReportResource validation requires both non-nil; domain allows at least one. |
| 5 | **OptionsConfig** (and nested Options) | All option groups are pointers but dereferenced without nil checks; "optional" at struct level, "required" in use. |
| 6 | **Timestamps** (CreatedAt/UpdatedAt/DeletedAt) | Optional as value+IsZero() in domain/snapshots vs pointer+nil+omitempty in legacy events. |
| 7 | **resolveOptionalFields** | Optional (nil) in command is turned into zero value in domain; "optional" is not carried by type past the command. |

---

## Fields that are handled consistently (no change needed)

- **SubjectReference.relation:** pointer, nil-checked.
- **Representations.commonVersion / reporterRepresentationVersion:** pointers, nil-checked; pairing validated.
- **Consistency.atLeastAsFresh:** value type, empty string = minimize latency; consistent.
- **Authn Enable/Transport:** optional pointers with explicit nil handling and defaults.

This list can be used to prioritize refactors (e.g. align TransactionId/ConsoleHref types, unify legacy ReporterVersion, add config nil checks, or standardize optional timestamps).

**Refactor plan:** A step-by-step plan (tests first, then pointers, then failure tests, then test updates) is in [plan-optional-fields-refactor.md](./plan-optional-fields-refactor.md).
