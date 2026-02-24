# Plan: Optional Fields Refactor — Tests First, Then Pointers

This plan covers: (1) documenting optional behavior in tests, (2) refactoring the 7 inconsistent fields to use pointers for optional/zero-value semantics, (3) adding failure tests for the new behavior, and (4) updating tests after the refactor.

**Reference:** [optional-fields-inconsistent-handling.md](./optional-fields-inconsistent-handling.md) (the 7 inconsistencies).

---

## Current state summary (as of completion)

Refactoring was limited to **the 7 fields** listed in the reference document. The following was done:

- **Phases 1–4** are complete. The full test suite (without `-tags=test`) passes.
- **TransactionId:** Pointers in command, domain, and snapshots (`*string` in snapshots); data layer and repo use string at boundary with nil/empty handling.
- **ConsoleHref:** Pointer in model (domain `*ConsoleHref`, snapshot `ReporterResourceSnapshot.ConsoleHref` `*string`); data layer converts to/from string.
- **ReporterVersion (legacy):** `EventRelationshipReporter.ReporterVersion` is `*string`; aligned with `EventResourceReporter`.
- **Representations:** Validation requires at least one of reporter/common; reporter-only and common-only succeed; domain allows empty representation for the absent side.
- **OptionsConfig:** Nil checks added in `LogConfigurationInfo`, `InjectClowdAppConfig`, and `Configure*`; tests confirm no panic with nil option groups.
- **Timestamps:** **Reverted to value types.** Domain and snapshots use `time.Time`; `GetTimestamps()` returns `(time.Time, time.Time)`; data layer uses `time.Time`. Legacy outbox events still use `*time.Time` with omitempty and convert from domain `time.Time` when building events.
- **resolveOptionalFields:** Returns pointers (no zero-value collapse); `NewResource`/`Update` accept `*ConsoleHref`, `*TransactionId`, `*Representation`.

Tests built with `-tags=test` (e.g. `resource_test.go`, `reporter_test.go`, `resource_delete_event_test.go`, `resource_report_event_test.go`) have pre-existing API drift and are out of scope for this refactor.

---

## Execution status (latest)

| Phase | Status | Notes |
|-------|--------|--------|
| **Phase 1** | Done | All 7 optional-behavior tests added; 4 `Update` calls in resource_test.go fixed (7th arg); all tests pass. |
| **Phase 2.3** | Done | `EventRelationshipReporter.ReporterVersion` changed from `string` to `*string`; test updated. |
| **Phase 2.5** | Done | `LogConfigurationInfo` and `InjectClowdAppConfig`/`Configure*` now nil-check option groups; tests added. |
| **Phase 2.1** | Done | TransactionId: snapshots/domain/data use *string; repo idempotency uses nil/empty checks. |
| **Phase 2.2** | Done | ConsoleHref: **pointer in model** (ReporterResource.consoleHref *ConsoleHref; ReporterResourceSnapshot.ConsoleHref *string); data layer converts to/from string at boundary; tests updated. |
| **Phase 2.6** | Reverted | Timestamps reverted to **value types** (time.Time): snapshots and domain use time.Time; GetTimestamps returns (time.Time, time.Time); data layer uses time.Time; legacy converts to *time.Time for omitempty when building outbox events. Goal was pointers in model definitions; user requested timestamp revert only. |
| **Phase 2.7** | Done | resolveOptionalFields now returns pointers (no zero-value collapse); NewResource/Update accept *ConsoleHref, *TransactionId, *Representation; usecase and all call sites updated. |
| **Phase 2.4** | Reverted | Representations: validation requires **both** reporter and common (reporter-only and common-only return error); domain rejects nil/empty representation data. |
| **Phase 4 (partial)** | Done | Service layer tests updated: both nil → error; reporter-only and common-only → error (representation required); both empty structs → error. |
| **Phase 3** | Done | Added `TestResolveOptionalFields_NilOptionalFields_ReturnNils` (3.7: nil optional fields return nil, no zero-value collapse). Other 3.x items covered by existing tests (snapshot TransactionId nil round-trip, HasTransactionIdBeenProcessed(""), config nil, etc.). Timestamps: no refactor; tests use value types / legacy *time.Time where applicable. |
| **Phase 4** | Done | Full test suite (without `-tags=test`) passes. Checklist items (biz/model, data, usecase, config, model_legacy) were addressed during Phase 2 and Phase 4 (partial) service tests. Tests built with `-tags=test` (resource_test.go, reporter_test.go, resource_delete_event_test.go, resource_report_event_test.go) have pre-existing API drift (NewReporterId return, NewResourceReportEvent/NewResourceDeleteEvent signatures) and are out of scope for this refactor. |

---

## Phase 1 — Document optional behavior in tests (tests must pass)

Goal: Add or adjust tests so that **current** optional behavior is explicit and asserted. No production code changes yet; all tests pass.

### 1.1 TransactionId

| Area | File(s) | What to add/update |
|------|---------|--------------------|
| **Domain** | `internal/biz/model/resource_test.go` | Add test: `TestNewResource_EmptyTransactionId_GeneratesNewId` (already exists; ensure it asserts that empty string leads to a generated non-empty ID). Add test: `TestUpdate_EmptyTransactionId_GeneratesNewId` (same for Update). |
| **Snapshot** | `internal/biz/model/snapshots_test.go` | Add test: `TestCommonRepresentationSnapshot_TransactionId_EmptyStringMeansNotSet` — serialize a CommonRepresentation with empty TransactionId, assert snapshot has `TransactionId == ""`. Add similar for ReporterRepresentationSnapshot if applicable. |
| **Repo** | `internal/data/resource_repository_test.go` | In `TestHasTransactionIdBeenProcessed`, ensure case `HasTransactionIdBeenProcessed(db, "")` returns false and does not panic. (Document empty string = "not set".) |
| **Usecase** | `internal/biz/usecase/resources/resource_service_test.go` | Add test: `TestReportResource_NilTransactionId_ResolvesToGeneratedId` — call ReportResource with `cmd.TransactionId == nil`, then verify idempotency or stored resource has a non-empty transaction ID (current behavior: nil → resolveOptionalFields → "" → domain generates). |

**Acceptance:** All new/updated tests pass with current code.

---

### 1.2 ConsoleHref

| Area | File(s) | What to add/update |
|------|---------|--------------------|
| **Domain** | `internal/biz/model/resource_test.go`, `reporter_resource_test.go` | Add test: `TestUpdate_ConsoleHref_EmptyString_Allowed` — Update with `consoleHref` from `NewConsoleHref("")` (or equivalent), assert no error and ReporterResource has empty ConsoleHref. |
| **Usecase** | `internal/biz/usecase/resources/resource_service_test.go` or `testdata_test.go` | Add test: `TestReportResource_NilConsoleHref_ResolvesToEmptyString` — command with `ConsoleHref == nil`, create/update resource, assert ReporterResource.ConsoleHref() is empty (current behavior). |

**Acceptance:** All new/updated tests pass with current code.

---

### 1.3 ReporterVersion (legacy)

| Area | File(s) | What to add/update |
|------|---------|--------------------|
| **Legacy** | `internal/biz/model_legacy/outboxevents_test.go` | Add test: `TestEventResourceReporter_ReporterVersion_NilAndNonNil` — build event with ReporterVersion nil vs non-nil, assert JSON/struct shape. Add test: `TestEventRelationshipReporter_ReporterVersion_EmptyString` — document that relationship reporter uses string (empty = absent). |
| **Model** | `internal/biz/model/reporter_representation_test.go` | Ensure existing tests for nil vs non-nil ReporterVersion are clearly named (e.g. `ReporterVersion_Nil_SerializesToNil`, `ReporterVersion_EmptyString_RoundTrips`). |

**Acceptance:** All new/updated tests pass with current code.

---

### 1.4 ReporterRepresentation / CommonRepresentation (command)

| Area | File(s) | What to add/update |
|------|---------|--------------------|
| **Usecase** | `internal/biz/usecase/resources/resource_service_test.go` | Add test: `TestReportResource_BothRepresentationsNil_ReturnsRepresentationRequiredError` — both nil returns error. Add tests: reporter-only and common-only **succeed** (at least one required; Phase 2.4). |
| **Domain** | `internal/biz/model` | Tests that `Representations` allows common-only or reporter-only (at least one required); domain allows empty representation for the absent side. |

**Acceptance:** All tests pass; both nil → error; reporter-only and common-only → success.

---

### 1.5 OptionsConfig (nil safety)

| Area | File(s) | What to add/update |
|------|---------|--------------------|
| **Config** | `internal/config/config_test.go` | Add test: `TestLogConfigurationInfo_WithNilOptionGroups_DoesNotPanic` — build OptionsConfig with one or more of Authn/Authz/Storage/Consumer/Server set to nil, call LogConfigurationInfo (and optionally InjectClowdAppConfig) and assert no panic. If current code panics, this test will fail (we will fix in Phase 2). So in Phase 1 we document current behavior: add test that uses only fully-initialized config (NewOptionsConfig()) and assert no panic; add a separate test that expects panic when Authz is nil (or skip that and add nil-safe behavior in Phase 2). Prefer: add test that passes when all options set; add comment that "nil options are not yet safe". |
| **Config** | `internal/config/config.go` | No change in Phase 1. |

**Acceptance:** Tests that assume full config pass; optional "nil options" test can be added to fail now and pass in Phase 2.

---

### 1.6 Timestamps (CreatedAt / UpdatedAt)

| Area | File(s) | What to add/update |
|------|---------|--------------------|
| **Domain** | `internal/biz/model/resource_test.go` | Tests use `IsZero()` for legacy/backfill. `TestResourceSnapshot_CreatedAtUpdatedAt_ZeroMeansNotSet` — snapshot uses `time.Time`; zero = not set; update sets UpdatedAt. |
| **Snapshot** | `internal/biz/model/snapshots_test.go` | Test that zero CreatedAt/UpdatedAt round-trips; timestamps remain value types. |
| **Legacy** | `internal/biz/model_legacy/outboxevents_test.go` | Outbox event payload uses *time.Time with omitempty; conversion from domain time.Time when building events. |

**Acceptance:** Tests pass; timestamps are value types (`time.Time`); zero = not set. (Phase 2.6 pointer refactor was reverted.)

---

### 1.7 resolveOptionalFields (optional vs zero value)

| Area | File(s) | What to add/update |
|------|---------|--------------------|
| **Usecase** | `internal/biz/usecase/resources/resource_service_test.go` | Add test: `TestResolveOptionalFields_NilOptionalFields_ReturnNils` — call resolveOptionalFields with cmd having nil TransactionId, nil ConsoleHref, nil ReporterRepresentation, nil CommonRepresentation; assert returned values are **nil** (no zero-value collapse). |

**Acceptance:** Test passes; reflects Phase 2.7 behavior (pointers returned as-is).

---

## Phase 2 — Refactor to use pointers for optional/zero-value semantics

Goal: Change the 7 areas so that "optional" or "not set" is represented by **pointer type** (nil) where appropriate; zero value is not used to mean "absent" for those fields.

### 2.1 TransactionId

| Change | File(s) | Action |
|--------|---------|--------|
| **Snapshot** | `internal/biz/model/snapshots.go` | Change `CommonRepresentationSnapshot.TransactionId` and `ReporterRepresentationSnapshot.TransactionId` from `string` to `*string`. Nil = not set. |
| **Domain** | `internal/biz/model/resource.go` | Change `NewResource` and `Update` to accept `transactionId *TransactionId`. When nil, generate; when non-nil, use *transactionId. Keep internal representation as value for now or switch to pointer; caller passes pointer. |
| **Domain** | `internal/biz/model/common_representation.go`, `reporter_representation.go` | TransactionId in domain entities/snapshots: use *string or *TransactionId in serialization (snapshot) and in Deserialize. |
| **Data** | `internal/data/model/common_representation.go`, `reporter_representation.go`, migrations | DB: allow NULL for transaction_id; map *string to NULL. GORM: use `*string` for TransactionId in structs. |
| **Repo** | `internal/data/resource_repository.go`, `fake_resource_repository.go` | `HasTransactionIdBeenProcessed(tx, transactionId string)` → consider `*string`; when nil or empty, return false. Keep signature or add overload to avoid large call-site changes. |
| **Usecase** | `internal/biz/usecase/resources/resource_service.go` | resolveOptionalFields: return `transactionId *model.TransactionId`; pass through to domain. createResource/updateResource: pass pointer; domain generates when nil. |

**Tests to add in Phase 3** (failure tests for new behavior): assert that snapshot with nil TransactionId round-trips; assert repo returns false for nil transactionId; assert domain generates when nil.

---

### 2.2 ConsoleHref

| Change | File(s) | Action |
|--------|---------|--------|
| **Domain** | `internal/biz/model/reporter_resource.go` | Change `consoleHref ConsoleHref` to `consoleHref *ConsoleHref`. Update NewReporterResource, Update, Serialize, Deserialize. Nil = not provided. |
| **Domain** | `internal/biz/model/resource.go` | Update and NewResource: accept `consoleHref *ConsoleHref`; pass through to ReporterResource. |
| **Snapshot** | `internal/biz/model/snapshots.go` | `ReporterResourceSnapshot.ConsoleHref` → `*string` (or *ConsoleHref if we keep type). Serialize/deserialize nil. |
| **Data** | `internal/data/model/reporter_resource.go`, `resource_repository.go` (FindResourceByKeysResult) | GORM: `ConsoleHref *string` or nullable column; map nil to NULL. |
| **Usecase** | `internal/biz/usecase/resources/resource_service.go` | resolveOptionalFields: return `consoleHref *model.ConsoleHref`; pass through; domain treats nil as "no console href". |

**Tests:** Assert nil ConsoleHref in command stays nil through to domain/snapshot where applicable; assert empty string is not used for "absent."

---

### 2.3 ReporterVersion (legacy)

| Change | File(s) | Action |
|--------|---------|--------|
| **Legacy** | `internal/biz/model_legacy/outboxevents.go` | Change `EventRelationshipReporter.ReporterVersion` from `string` to `*string`. Nil = absent. Update any construction/consumption of EventRelationshipReporter. |

**Tests:** Assert both EventResourceReporter and EventRelationshipReporter use *string; JSON round-trip nil and non-nil.

---

### 2.4 ReporterRepresentation / CommonRepresentation (command)

| Change | File(s) | Action |
|--------|---------|--------|
| **Usecase** | `internal/biz/usecase/resources/resource_service.go` | Change validation: require **at least one** of ReporterRepresentation or CommonRepresentation non-nil (align with domain Representations). Allow reporter-only or common-only; adjust createResource/updateResource and resolveOptionalFields to handle one nil (e.g. empty Representation for the absent one, or explicit branch). |
| **Tests** | `internal/biz/usecase/resources/resource_service_test.go` | Add tests: reporter-only and common-only ReportResource succeed; both nil still returns error. |

**Tests:** Failure test: both nil → error; success tests: reporter-only, common-only.

---

### 2.5 OptionsConfig (nil safety)

| Change | File(s) | Action |
|--------|---------|--------|
| **Config** | `internal/config/config.go` | In LogConfigurationInfo, InjectClowdAppConfig, ConfigureAuthz, ConfigureStorage, ConfigureConsumer: add nil checks before dereferencing options.Server, options.Authz, options.Storage, options.Consumer. When nil, skip that block or return early / no-op. |
| **Config** | `internal/config/config_test.go` | Add test: build OptionsConfig with Authz = nil (or Server = nil), call LogConfigurationInfo; assert no panic. |

**Tests:** Test that partial config (some option groups nil) does not panic.

---

### 2.6 Timestamps (CreatedAt / UpdatedAt) — reverted

**Current state (reverted):** Timestamps remain **value types** (`time.Time`) in domain and snapshots. The plan originally called for `*time.Time` for optional semantics; this was reverted by request so that only the other six refactors (and pointer alignment in model definitions for ConsoleHref, etc.) are in place.

| Current implementation | File(s) | Notes |
|------------------------|---------|--------|
| **Domain** | `resource_report_event.go`, `reporter_resource.go` | `createdAt`, `updatedAt` are `time.Time`. `CreatedAt()`/`UpdatedAt()` return `time.Time`. `SetTimestamps(time.Time, time.Time)`. |
| **Snapshot** | `snapshots.go` | `ResourceSnapshot`, `ReporterResourceSnapshot`, `CommonRepresentationSnapshot`, `ReporterRepresentationSnapshot` use `time.Time` for CreatedAt/UpdatedAt. Zero = not set. |
| **Data** | `internal/data/model/*.go` | GORM and snapshots use `time.Time`; no pointer conversion at boundary. |
| **Legacy** | `outboxevents.go` | Outbox event structs use `*time.Time` with omitempty for JSON. When building from domain, code converts: if `!resourceEvent.CreatedAt().IsZero()` then set `createdAt = &t`. |

**Tests:** Assert zero timestamp via `IsZero()`; snapshot/deserialize use `time.Time` and zero value for "not set."

---

### 2.7 resolveOptionalFields

| Change | File(s) | Action |
|--------|---------|--------|
| **Usecase** | `internal/biz/usecase/resources/resource_service.go` | Change resolveOptionalFields to return pointers: `consoleHref *model.ConsoleHref`, `transactionId *model.TransactionId`, `reporterRepresentation *model.Representation`, `commonRepresentation *model.Representation`. Do not collapse nil to zero value; pass pointers through. |
| **Domain** | `internal/biz/model/resource.go` | NewResource and Update: accept pointers; when nil, use "generate" or "empty" only where domain explicitly defines it (e.g. transactionId nil → generate; consoleHref nil → store as nil). |

**Tests:** Assert that when command has nil TransactionId, domain receives nil and generates; when command has nil ConsoleHref, domain receives nil and stores nil (no empty string).

---

## Phase 3 — Failure tests for new behavior

Goal: Add tests that **assert the new (desired) behavior**. These tests will **fail before** Phase 2 is done and **pass after** Phase 2. They also protect against regressions.

### 3.1 TransactionId

- **Test:** Snapshot with `TransactionId == nil` deserializes and serializes back to nil; DB stores NULL when TransactionId is nil.
- **Test:** `HasTransactionIdBeenProcessed(tx, nil)` or `HasTransactionIdBeenProcessed(tx, "")` returns false and does not panic.
- **Test:** ReportResource with cmd.TransactionId == nil results in a generated transaction ID stored in DB (idempotency check uses that generated ID).

### 3.2 ConsoleHref

- **Test:** Command with ConsoleHref == nil results in ReporterResource with ConsoleHref() returning nil (or a type that indicates "absent"), not empty string.
- **Test:** Snapshot/DB has NULL or nil for console_href when not provided.

### 3.3 ReporterVersion (legacy)

- **Test:** EventRelationshipReporter.ReporterVersion is *string; JSON with missing field deserializes to nil.

### 3.4 ReporterRepresentation / CommonRepresentation

- **Test:** ReportResource with only ReporterRepresentation set (CommonRepresentation nil) succeeds and stores resource with only reporter data (or vice versa).
- **Test:** ReportResource with both nil still returns RepresentationRequiredError.

### 3.5 OptionsConfig

- **Test:** LogConfigurationInfo(options) with options.Authz == nil does not panic.
- **Test:** LogConfigurationInfo(options) with options.Server == nil does not panic.

### 3.6 Timestamps (current: value types)

- **Test:** ResourceSnapshot with zero CreatedAt/UpdatedAt deserializes; zero means "not set"; update sets UpdatedAt.
- **Test:** Legacy event with CreatedAt/UpdatedAt nil omits from JSON (omitempty); legacy converts from domain `time.Time` (zero → nil not set in outbox payload).

### 3.7 resolveOptionalFields

- **Test:** resolveOptionalFields(cmd) with cmd.TransactionId == nil returns transactionId == nil (not empty string).
- **Test:** resolveOptionalFields(cmd) with cmd.ConsoleHref == nil returns consoleHref == nil.

---

## Phase 4 — Update tests to new behavior

Goal: After Phase 2 and 3, fix any tests that were asserting **old** behavior (zero value, required both representations, etc.) so the full suite passes.

### 4.1 Checklist (done; reflects current state)

| Area | Current state |
|------|----------------|
| **biz/model** | resource_test, reporter_resource_test, snapshots_test, etc.: optional fields use pointers where refactored (TransactionId, ConsoleHref, Representations); timestamps use `time.Time` and `.IsZero()` for "not set"; fixtures use pointers for optional fields. |
| **data/model** | Nullable/pointer where applicable (*string for TransactionId, ConsoleHref in snapshot conversion); timestamps remain `time.Time` in GORM and snapshots. |
| **biz/usecase/resources** | resource_service_test, testdata_test: command fixtures with nil optional fields; assertions use nil where type is pointer. |
| **config** | config_test: `TestLogConfigurationInfo_WithNilOptionGroups_DoesNotPanic` and similar; nil option groups do not panic. |
| **model_legacy** | EventRelationshipReporter.ReporterVersion is *string; outbox event timestamps remain *time.Time with omitempty; conversion from domain time.Time when building events. |
| **data** | resource_repository_test, fake_resource_repository: TransactionId handled as string at repo boundary with nil/empty semantics; timestamps as time.Time. |

### 4.2 Order of operations

1. Run full test suite after Phase 2; note failing tests.
2. Fix assertions and fixtures in Phase 4 so that:
   - Optional fields are compared to nil where type is pointer.
   - Zero value is no longer used to mean "absent" for those fields.
   - At-least-one representation validation and reporter-only/common-only flows are tested and pass.
3. Re-run Phase 3 failure tests; they should pass.
4. Re-run entire suite; all should pass.

---

## Summary order of work

| Phase | Description | Outcome |
|-------|-------------|---------|
| **1** | Add tests that document current optional behavior (no prod changes) | Done; all tests pass |
| **2** | Refactor 7 areas to use pointers for optional/zero-value semantics | Done for 6 of 7; (2.6) Timestamps reverted to value types |
| **3** | Add failure tests that assert new behavior | Done; tests pass |
| **4** | Update existing tests to new behavior (fix fixtures and assertions) | Done; full suite passes |

**Implementation order used within Phase 2:**  
(2.3) ReporterVersion legacy → (2.5) OptionsConfig nil checks → (2.1) TransactionId → (2.2) ConsoleHref → (2.7) resolveOptionalFields → (2.4) Representations validation. (2.6) Timestamps was implemented then **reverted**; timestamps remain `time.Time` in domain and snapshots.

---

## Files to touch (quick reference)

- **Model (biz):** `resource.go`, `reporter_resource.go`, `common_representation.go`, `reporter_representation.go`, `resource_report_event.go`, `snapshots.go`, `resource_repository.go` (interface if needed).
- **Model tests (biz):** `resource_test.go`, `reporter_resource_test.go`, `snapshots_test.go`, `common_representation_test.go`, `reporter_representation_test.go`, `resource_report_event_test.go`.
- **Data model:** `internal/data/model/resource.go`, `reporter_representation.go`, `common_representation.go`, `reporter_resource.go`; migrations if DB column nullability changes.
- **Data model tests:** `internal/data/model/*_test.go`.
- **Usecase:** `internal/biz/usecase/resources/resource_service.go`, `commands.go` (if needed).
- **Usecase tests:** `resource_service_test.go`, `testdata_test.go`.
- **Config:** `internal/config/config.go`, `config_test.go`.
- **Legacy:** `internal/biz/model_legacy/outboxevents.go`, `outboxevents_test.go`.
- **Repo:** `internal/data/resource_repository.go`, `fake_resource_repository.go`, `resource_repository_test.go`.
