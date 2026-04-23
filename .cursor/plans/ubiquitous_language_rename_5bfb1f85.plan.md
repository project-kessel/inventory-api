---
name: Relations Ubiquitous Language
overview: "Redesign the RelationsRepository interface using ubiquitous language domain types (Relationship, ResourceReference, SubjectReference, RepresentationType) with DDD principles: tiny types, unexported fields, constructors with invariant validation, getters only if used, immutability."
todos:
  - id: define-new-types
    content: "Define new domain types in internal/biz/model/: ResourceReference, ReporterReference, RepresentationType, Relationship, LockId, LockToken. Redesign SubjectReference to wrap ResourceReference. Redesign TupleFilter, TupleSubjectFilter, ReadTuplesItem as proper domain types with tiny types. All types must use unexported fields, New* constructors with invariant checks, and getters only if used."
    status: completed
  - id: update-interface
    content: Rewrite RelationsRepository interface with Relationship, RepresentationType, multi-value returns, single DeleteTuples(filter), LookupObjects rename, tiny types
    status: completed
  - id: remove-old-types
    content: "Remove eliminated types: CheckResult, TuplesResult, AcquireLockResult, CheckBulkItem, RelationsResource, RelationsSubject, RelationsObjectType, DeleteTuplesByFilter. Redesign RelationsTuple to use ResourceReference/SubjectReference."
    status: completed
  - id: update-impls
    content: Update all RelationsRepository implementations (grpc, simple, allow_all) for new interface
    status: completed
  - id: update-callers
    content: "Update all callers: resource_service.go, consumer.go, tuple_crud_usecase.go, kesselinventoryservice.go, tuples.go, healthrepository.go"
    status: completed
  - id: update-mocks-tests
    content: Update mocks and all test files for new types, renamed methods, and multi-value returns
    status: completed
  - id: ddd-enforce-conventions
    content: "Ensure all new and redesigned types follow DDD conventions: unexported fields, New* constructors with invariant validation, getters only if used, no setters. Verify relations_results.go types are migrated to individual files with proper encapsulation. Update AGENTS.md with DDD conventions."
    status: completed
  - id: verify
    content: Ensure project compiles, lint passes, and all unit tests pass
    status: pending
isProject: false
---

# Redesign RelationsRepository with Ubiquitous Language

## Starting Point

This plan builds on the completed authorizer refactor (protobuf removal). The starting point is the current branch `RHCLOUD-45010-implement-relations-repo-refactor` which has model-typed parameters but still uses pre-DDD naming and types. The "BEFORE" below is this branch; the "AFTER" is the target state.

Main branch interface (for full context) uses raw protobuf types -- that was already addressed.

## BEFORE (current branch)

```go
type RelationsRepository interface {
    Health(ctx context.Context) (HealthResult, error)

    Check(ctx context.Context, resource ReporterResourceKey, relation Relation,
        subject SubjectReference, consistency Consistency,
    ) (CheckResult, error)

    CheckForUpdate(ctx context.Context, resource ReporterResourceKey, relation Relation,
        subject SubjectReference,
    ) (CheckResult, error)

    CheckBulk(ctx context.Context, items []CheckBulkItem, consistency Consistency,
    ) (CheckBulkResult, error)

    CheckForUpdateBulk(ctx context.Context, items []CheckBulkItem,
    ) (CheckBulkResult, error)

    LookupResources(ctx context.Context, resourceType ResourceType, reporterType ReporterType,
        relation Relation, subject SubjectReference, pagination *Pagination, consistency Consistency,
    ) (ResultStream[LookupResourcesItem], error)

    LookupSubjects(ctx context.Context, resource ReporterResourceKey, relation Relation,
        subjectType ResourceType, subjectReporter ReporterType, subjectRelation *Relation,
        pagination *Pagination, consistency Consistency,
    ) (ResultStream[LookupSubjectsItem], error)

    CreateTuples(ctx context.Context, tuples []RelationsTuple, upsert bool, fencing *FencingCheck,
    ) (TuplesResult, error)

    DeleteTuples(ctx context.Context, tuples []RelationsTuple, fencing *FencingCheck,
    ) (TuplesResult, error)

    DeleteTuplesByFilter(ctx context.Context, filter TupleFilter, fencing *FencingCheck,
    ) (TuplesResult, error)

    ReadTuples(ctx context.Context, filter TupleFilter, pagination *Pagination, consistency Consistency,
    ) (ResultStream[ReadTuplesItem], error)

    AcquireLock(ctx context.Context, lockId string) (AcquireLockResult, error)
}
```

Supporting types on the branch: `CheckResult`, `CheckBulkItem`, `CheckBulkResult`, `TuplesResult`, `AcquireLockResult`, `HealthResult`, `LookupResourcesItem`, `LookupSubjectsItem`, `FencingCheck`, `RelationsTuple` (uses deprecated `RelationsResource`/`RelationsSubject`), `SubjectReference` (wraps `ReporterResourceKey`).

## AFTER (target)

### New domain types

Defined in [`internal/biz/model/`](internal/biz/model/):

```go
// RepresentationType identifies a kind of resource, optionally scoped to a reporter.
// Matches Inventory v1beta2 RepresentationType proto semantics.
type RepresentationType struct {
    resourceType ResourceType   // required
    reporterType *ReporterType  // optional (Relations API requires it today, but domain allows omission)
}

// ResourceReference identifies a specific resource in the Relations domain.
// Distinct from ReporterResourceKey (which is the natural key for a ReporterResource entity).
type ResourceReference struct {
    resourceType ResourceType       // required
    resourceId   LocalResourceId    // required
    reporter     *ReporterReference // optional
}

type ReporterReference struct {
    reporterType ReporterType        // required
    instanceId   *ReporterInstanceId // optional
}

// SubjectReference -- redesigned to wrap ResourceReference instead of ReporterResourceKey.
type SubjectReference struct {
    resource ResourceReference // required
    relation *Relation         // optional (nil = direct subject, non-nil = subject set)
}

// Relationship is the aggregate root of the Relations domain.
// Used for Check APIs ("does this hold?") -- may be stored directly or derived via schema rules.
type Relationship struct {
    object   ResourceReference // required
    relation Relation          // required
    subject  SubjectReference  // required
}

// RelationsTuple represents a stored relationship fact.
// Structurally identical to Relationship but semantically distinct:
// a Relationship is a query ("does this hold?"), a RelationsTuple is a persisted fact.
type RelationsTuple struct {
    object   ResourceReference // required
    relation Relation          // required
    subject  SubjectReference  // required
}

// TupleFilter -- proper domain type with tiny types. All fields optional (nil = match any).
type TupleFilter struct {
    objectType   *ResourceType
    reporterType *ReporterType
    objectId     *LocalResourceId
    relation     *Relation
    subject      *TupleSubjectFilter
}

type TupleSubjectFilter struct {
    subjectType  *ResourceType
    reporterType *ReporterType
    subjectId    *LocalResourceId
    relation     *Relation
}

// ReadTuplesItem -- proper domain type replacing raw string passthrough.
type ReadTuplesItem struct {
    object            ResourceReference
    relation          Relation
    subject           SubjectReference
    continuationToken string
    consistencyToken  ConsistencyToken
}

// New tiny types
type LockId string
type LockToken string
```

### Updated interface

```go
type RelationsRepository interface {
    Health(ctx context.Context) (HealthResult, error)

    // --- Check APIs: use Relationship ---

    Check(ctx context.Context, rel Relationship, consistency Consistency,
    ) (bool, ConsistencyToken, error)

    CheckForUpdate(ctx context.Context, rel Relationship,
    ) (bool, ConsistencyToken, error)

    CheckBulk(ctx context.Context, rels []Relationship, consistency Consistency,
    ) (CheckBulkResult, error)

    CheckForUpdateBulk(ctx context.Context, rels []Relationship,
    ) (CheckBulkResult, error)

    // --- Lookup APIs: use RepresentationType for type patterns ---

    LookupObjects(ctx context.Context,
        objectType RepresentationType,
        relation Relation, subject SubjectReference,
        pagination *Pagination, consistency Consistency,
    ) (ResultStream[LookupObjectsItem], error)

    LookupSubjects(ctx context.Context,
        object ResourceReference, relation Relation,
        subjectType RepresentationType,
        subjectRelation *Relation,
        pagination *Pagination, consistency Consistency,
    ) (ResultStream[LookupSubjectsItem], error)

    // --- Tuple APIs ---

    CreateTuples(ctx context.Context, tuples []RelationsTuple,
        upsert bool, fencing *FencingCheck,
    ) (ConsistencyToken, error)

    DeleteTuples(ctx context.Context, filter TupleFilter,
        fencing *FencingCheck,
    ) (ConsistencyToken, error)

    ReadTuples(ctx context.Context, filter TupleFilter,
        pagination *Pagination, consistency Consistency,
    ) (ResultStream[ReadTuplesItem], error)

    AcquireLock(ctx context.Context, lockId LockId) (LockToken, error)
}
```

### Updated result types

All result types use unexported fields, `New*` constructors, and getters only if used.

```go
// CheckBulkItem is eliminated -- Check APIs take []Relationship directly.

type CheckBulkResultItem struct {
    allowed   bool
    err       error
    errorCode int32
}

type CheckBulkResultPair struct {
    request Relationship
    result  CheckBulkResultItem
}

type CheckBulkResult struct {
    pairs            []CheckBulkResultPair
    consistencyToken ConsistencyToken
}

type LookupObjectsItem struct {
    object            ResourceReference
    continuationToken string
}

type LookupSubjectsItem struct {
    subject           SubjectReference
    continuationToken string
}

type FencingCheck struct {
    lockId    LockId
    lockToken LockToken
}

type HealthResult struct {
    status string
    code   int
}

// TupleFilter -- all fields optional (nil = match any).
type TupleFilter struct {
    objectType   *ResourceType
    reporterType *ReporterType
    objectId     *LocalResourceId
    relation     *Relation
    subject      *TupleSubjectFilter
}

type TupleSubjectFilter struct {
    subjectType  *ResourceType
    reporterType *ReporterType
    subjectId    *LocalResourceId
    relation     *Relation
}

type ReadTuplesItem struct {
    object            ResourceReference
    relation          Relation
    subject           SubjectReference
    continuationToken string
    consistencyToken  ConsistencyToken
}
```

### Types eliminated

- `CheckResult` -- replaced by multi-value return `(bool, ConsistencyToken, error)`
- `TuplesResult` -- replaced by `(ConsistencyToken, error)`
- `AcquireLockResult` -- replaced by `(LockToken, error)`
- `CheckBulkItem` -- replaced by `Relationship` directly
- `RelationsResource`, `RelationsSubject`, `RelationsObjectType` -- deprecated types replaced by `ResourceReference`, `SubjectReference`
- `DeleteTuplesByFilter` method -- collapsed back into `DeleteTuples(filter TupleFilter, ...)`

### Types redesigned (kept but rewritten)

- `RelationsTuple` -- rewritten to use `ResourceReference`/`SubjectReference` instead of deprecated `RelationsResource`/`RelationsSubject`. Structurally identical to `Relationship` but semantically distinct (stored fact vs. query concept).
- `SubjectReference` -- wraps `ResourceReference` instead of `ReporterResourceKey`

### Types redesigned as proper domain types

- `TupleFilter` -- rewritten with tiny types (`*ResourceType`, `*ReporterType`, `*LocalResourceId`, `*Relation`) instead of raw `*string`. Domain naming replaces Relations API jargon (`ResourceNamespace` -> `ReporterType`).
- `TupleSubjectFilter` -- same treatment, typed optional fields
- `ReadTuplesItem` -- rewritten to use `ResourceReference`, `Relation`, `SubjectReference` instead of raw strings

### Types unchanged

- `ResultStream[T]` -- streaming abstraction unchanged
- `Pagination` -- unchanged

## Key design decisions

- **Relationship and RelationsTuple are distinct domain concepts** -- `Relationship` is a query concept used in Check APIs ("does this hold?"), `RelationsTuple` is a persisted fact used in tuple CRUD. Both use `ResourceReference`/`SubjectReference` but are semantically distinct types
- **ResourceReference vs ReporterResourceKey** -- distinct types; `ResourceReference` is for the Relations domain (optional reporter), `ReporterResourceKey` is the Inventory entity natural key (required reporter)
- **No ObjectReference wrapper** -- `ResourceReference` used directly for the object side; `SubjectReference` adds optional relation for the subject side
- **RepresentationType** -- domain type matching Inventory proto, with optional `reporterType` (even though Relations API requires it today)
- **Single DeleteTuples** -- takes `TupleFilter` (mirrors Relations API's actual contract), eliminates the artificial split
- **Multi-value returns** -- for simple results (Check, CreateTuples, DeleteTuples, AcquireLock), eliminating wrapper structs
- **Tiny types** -- `LockId`, `LockToken` replace raw strings; `FencingCheck` uses them

## DDD implementation conventions

Every type in this plan must follow the conventions defined in [`AGENTS.md`](AGENTS.md) under "Domain Model Conventions". Concretely:

### Rules

1. **Tiny types** -- use defined types on primitives (`type LockId string`) instead of raw `string`/`int`. Tiny types get a `New*` constructor (with invariant validation where applicable), a `Serialize() string` method, and a `String() string` method.
2. **Unexported fields** -- all struct fields are unexported. No exceptions for "simple" types.
3. **Constructor initialization** -- every struct type has a `New*` constructor. Constructors validate invariants (required fields non-zero, valid combinations) and return `(T, error)` when validation can fail. For structs where all fields are always valid (e.g., result types constructed only by infrastructure code), constructors may return `T` without error.
4. **Invariant examples**: `NewResourceReference` must require non-zero `ResourceType` and `LocalResourceId`. `NewRelationship` must require non-zero object, relation, and subject. `NewFencingCheck` must require non-zero `LockId` and `LockToken`.
5. **Getters only if used** -- add a getter only when a caller outside the type's own methods actually needs it. Do not add speculative getters. When in doubt, leave it out; a getter can always be added later.
6. **Immutability** -- no setter methods. To change a value, construct a new instance.
7. **`Deserialize*` functions** -- bypass validation; reserved for reconstructing from trusted storage or wire formats (database rows, protobuf fields). Named `Deserialize*`, not `New*`.
8. **One type per file** (for new types) -- each new domain type gets its own file (e.g., `relationship.go`, `resource_reference.go`, `fencing_check.go`). Small related types may share a file (e.g., `ReporterReference` in `resource_reference.go`).

### Type-by-type conventions

| Type | Kind | Constructor | Invariants | Getters (add only if used) |
|------|------|-------------|------------|---------------------------|
| `LockId` | tiny type | `NewLockId(s string) (LockId, error)` | non-empty | `Serialize`, `String` |
| `LockToken` | tiny type | `NewLockToken(s string) (LockToken, error)` | non-empty | `Serialize`, `String` |
| `ResourceReference` | struct value object | `NewResourceReference(...)` | non-zero `ResourceType`, `LocalResourceId` | `ResourceType`, `ResourceId`, `Reporter`, `HasReporter` |
| `ReporterReference` | struct value object | `NewReporterReference(...)` | non-zero `ReporterType` | `ReporterType`, `InstanceId`, `HasInstanceId` |
| `RepresentationType` | struct value object | `NewRepresentationType(...)` | non-zero `ResourceType` | `ResourceType`, `ReporterType`, `HasReporterType` |
| `SubjectReference` | struct value object | `NewSubjectReference(...)` | non-zero `ResourceReference` | `Resource`, `Relation`, `HasRelation` |
| `Relationship` | struct value object | `NewRelationship(...)` | non-zero object, relation, subject | `Object`, `Relation`, `Subject` |
| `RelationsTuple` | struct value object | `NewRelationsTuple(...)` | non-zero object, relation, subject | `Object`, `Relation`, `Subject` |
| `FencingCheck` | struct value object | `NewFencingCheck(lockId LockId, lockToken LockToken)` | non-zero `LockId`, `LockToken` | `LockId`, `LockToken` |
| `TupleFilter` | struct value object | `NewTupleFilter(...)` or builder | all fields optional (nil = match any) | field accessors as needed |
| `TupleSubjectFilter` | struct value object | `NewTupleSubjectFilter(...)` or builder | all fields optional | field accessors as needed |
| `HealthResult` | struct value object | `NewHealthResult(status string, code int)` | none (infrastructure result) | `Status`, `Code` |
| `CheckBulkResult` | struct value object | `NewCheckBulkResult(...)` | non-empty pairs | `Pairs`, `ConsistencyToken` |
| `CheckBulkResultPair` | struct value object | `NewCheckBulkResultPair(...)` | valid request + result | `Request`, `Result` |
| `CheckBulkResultItem` | struct value object | `NewCheckBulkResultItem(...)` | none (infrastructure result) | `Allowed`, `Error`, `ErrorCode` |
| `LookupObjectsItem` | struct value object | `NewLookupObjectsItem(...)` | non-zero `ResourceReference` | `Object`, `ContinuationToken` |
| `LookupSubjectsItem` | struct value object | `NewLookupSubjectsItem(...)` | non-zero `SubjectReference` | `Subject`, `ContinuationToken` |
| `ReadTuplesItem` | struct value object | `NewReadTuplesItem(...)` | non-zero object, relation, subject | `Object`, `Relation`, `Subject`, `ContinuationToken`, `ConsistencyToken` |
| `ResultStream[T]` | interface | N/A | N/A | `Recv() (T, error)` |

## Files to change

### Model layer ([`internal/biz/model/`](internal/biz/model/))
- New file for `ResourceReference`, `ReporterReference`, `RepresentationType`
- Update [`relations_repository.go`](internal/biz/model/relations_repository.go) -- new interface
- Update [`relations_results.go`](internal/biz/model/relations_results.go) -- remove eliminated types, update remaining types
- Update [`subject_reference.go`](internal/biz/model/subject_reference.go) -- wrap `ResourceReference` instead of `ReporterResourceKey`
- Update [`relations_tuple.go`](internal/biz/model/relations_tuple.go) -- rewrite `RelationsTuple` to use `ResourceReference`/`SubjectReference`; remove deprecated `RelationsResource`, `RelationsSubject`, `RelationsObjectType`; update `NewWorkspaceRelationsTuple`
- Add `LockId`, `LockToken` tiny types (in [`common.go`](internal/biz/model/common.go) or separate file)

### Data layer ([`internal/data/`](internal/data/))
- [`grpc_relations_repository.go`](internal/data/grpc_relations_repository.go) -- implement new interface, update protobuf conversion
- [`relations_allow_all.go`](internal/data/relations_allow_all.go) -- implement new signatures
- [`relations_simple.go`](internal/data/relations_simple.go) -- implement new signatures
- Remove `DeleteTuplesByFilter` from all implementations

### Use case layer
- [`internal/biz/usecase/resources/resource_service.go`](internal/biz/usecase/resources/resource_service.go) -- update all calls to use `Relationship`, `ResourceReference`, etc.
- [`internal/biz/usecase/resources/commands.go`](internal/biz/usecase/resources/commands.go) -- update command types (remove `CheckBulkItem`, rename `LookupResourcesCommand` to `LookupObjectsCommand`, etc.)
- [`internal/biz/usecase/tuples/commands.go`](internal/biz/usecase/tuples/commands.go) -- update `FencingCheck` to use `LockId`/`LockToken`
- [`internal/biz/usecase/tuples/tuple_crud_usecase.go`](internal/biz/usecase/tuples/tuple_crud_usecase.go) -- update `DeleteTuples` call (no more `DeleteTuplesByFilter`)

### Consumer
- [`internal/consumer/consumer.go`](internal/consumer/consumer.go) -- update to use `Relationship`, `ResourceReference`, `LockId`, `LockToken`

### Service layer
- [`internal/service/resources/kesselinventoryservice.go`](internal/service/resources/kesselinventoryservice.go) -- update `ToLookupResourcesCommand` -> `ToLookupObjectsCommand`, update response conversion
- [`internal/service/tuples/tuples.go`](internal/service/tuples/tuples.go) -- update `relationshipToRelationsTuple` to build new `RelationsTuple` (using `ResourceReference`/`SubjectReference`)

### Health
- [`internal/data/health/healthrepository.go`](internal/data/health/healthrepository.go) -- no change (already uses `HealthResult`)

### Mocks and tests
- [`internal/mocks/mocks.go`](internal/mocks/mocks.go) -- update signatures
- [`internal/data/relations_simple_test.go`](internal/data/relations_simple_test.go) -- update for new types
- [`internal/biz/usecase/resources/resource_service_test.go`](internal/biz/usecase/resources/resource_service_test.go) -- update for new types
- [`internal/service/resources/kesselinventoryservice_test.go`](internal/service/resources/kesselinventoryservice_test.go) -- update for new types
- [`internal/consumer/consumer_test.go`](internal/consumer/consumer_test.go) -- update for new types
- [`internal/biz/usecase/tuples/tuple_crud_usecase_test.go`](internal/biz/usecase/tuples/tuple_crud_usecase_test.go) -- update for new types
- [`internal/service/tuples/tuples_test.go`](internal/service/tuples/tuples_test.go) -- update for new types
- Metaauthorizer constant rename: `RelationLookupResources` -> `RelationLookupObjects`
