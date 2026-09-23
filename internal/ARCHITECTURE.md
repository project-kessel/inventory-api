# Application Architecture (Hexagonal / DDD)

This document is the committed source of truth for Inventory API's **target**
architecture — hexagonal (ports & adapters) with Domain-Driven Design tactical
patterns. It is read by both human/AI contributors (via `AGENTS.md`) and by
CodeRabbit (via `knowledge_base.code_guidelines` in `.coderabbit.yaml`) as
review criteria for every PR.

Judge new code against the target below, not the current baseline — this repo
has pre-existing violations (noted where relevant) that are not blockers on
their own. Only new violations, or PRs that copy/extend an existing one, should
be flagged.

Reference: *Implementing Domain-Driven Design* (Vaughn Vernon, 2013) — cited
below as "IDDD Ch. N".

## Bounded contexts

Inventory participates in several context-mapping relationships:

| Context | Upstream/Downstream | Relationship (IDDD Ch. 2–3) |
|---|---|---|
| SpiceDB relational schema | Upstream (authority) | Anti-Corruption Layer (we protect ourselves) |
| Kafka/CDC events | Upstream (events) | Published Language (event schema) |
| PostgreSQL/GORM persistence | Downstream (owned store) | Shared Kernel (model ↔ persistence) |
| Console/gRPC clients | Upstream (callers) | Open Host Service + Published Language (protobuf) |

SpiceDB speaks relational tuples (`type/id#relation@subject_type/id`); Inventory's
domain speaks logical types (`features/workspace`, reporter-scoped relations).
The translation between these two languages is **domain knowledge**, not
infrastructure plumbing.

## Package → layer

Kratos' `service`/`biz`/`data` naming is not sufficient on its own: Kratos
`service` is overloaded (it mixes Presentation and Application). Inventory
splits them explicitly.

| Layer | Owns | Packages |
|---|---|---|
| Main (bootstrap) | Config, object graph, process start/stop. No business logic. | `cmd/`, `internal/config/` |
| Presentation | Protocol mapping, credential presentation, pagination, input preconditions. | `internal/service/`, `internal/server/`, `internal/middleware/`, `internal/authn/` |
| Application | Commands/queries, transaction boundary, (meta) authz, observability of use cases. | `internal/biz/usecase/`, `internal/biz/health/` |
| Domain | Nouns/verbs, invariants, schema rules, tuple calculation, ports. No I/O. | `internal/biz/model/` |
| Infrastructure | Adapters for ports. I/O. Little business logic. | `internal/data/`, `internal/storage/`, `internal/pubsub/` |

**Mixed / transitional packages** — treat carefully, do not extend the mixing:

| Package | Treat as | Notes |
|---|---|---|
| `internal/consumer/` | Infrastructure inbound adapter that currently also hosts application logic | Kafka I/O belongs here. Tuple-replication *use case* belongs in `biz/usecase`. Do not add new type-switches on `data.*` here. |
| `internal/metricscollector/` | Infrastructure / cross-cutting | Prefer Domain-Oriented Observability (ports for probes) over leaking OTEL into domain. |
| `internal/biz/model_legacy/` | Legacy domain | Do not extend. New logic goes in `model`. |

## Dependency Direction Rule (DDR)

Source dependencies point **inward**. Only Main may import Infrastructure.

```
Main ─────────────────────────────────────────────► Infrastructure
  │                                                      ▲
  ├─ Presentation ──► Application ──► Domain ────────────┤
  │                         │                            │
  └─────────────────────────┴──── ports defined here     │
                                    adapters implement ──┘
```

Allowed imports (arrows mean "may import"):

```
cmd/*, internal/config          → anything (Main is the composition root)
internal/service, server,
  middleware, authn             → usecase, model, authn  (not data)
internal/biz/usecase, health    → model                  (not data, not service)
internal/biz/model              → stdlib + tiny helpers  (not data, not usecase,
                                   not proto, not gorm, not grpc)
internal/data, storage,
  consumer, pubsub              → model                  (adapters implement ports)
```

Forbidden:

- Domain importing `internal/data`, `internal/service`, `internal/consumer`, protobuf, GORM, gRPC
- Application importing `internal/data` or `internal/service` (use ports from `model`)
- Presentation importing `internal/data` (go through usecase)
- Type-switching on concrete adapter types (`*data.SpiceDBRelationsRepository`, …) outside `cmd/`
- Asking an adapter "what type are you?" instead of reading a config/capability method

Tests follow the same direction with one exception: infrastructure tests may
construct other adapters (in-memory schema repo, simple relations repo).
Application- and Presentation-layer tests also commonly construct concrete
`internal/data` fakes/adapters (`data.NewFakeResourceRepository()`,
`data.AllowAllRelationsRepository{}`, `data.NewSimpleRelationsRepository()`,
`data.NewInMemorySchemaRepository()`) as **test doubles**, since this repo has
no dedicated in-`model` fakes for these ports yet — this is the established
convention, not a new violation. **Domain tests must not import
`internal/data`.** Inject `nil` or a domain-level fake when the port is unused.

Known pre-existing exceptions (do not re-file unless a PR adds *more* of these):

- `internal/biz/model/resource_repository.go` and `transaction_manager.go` import GORM
- `internal/biz/model/schema_service.go` and `resource.go` import Kratos `log`
- `internal/biz/usecase/resources/resource_service.go` imports GORM and gRPC status
- `internal/biz/model/schema_service_test.go` imports `internal/data`
- `internal/biz/usecase/resources/resource_service_test.go` and
  `internal/biz/usecase/tuples/tuple_crud_usecase_test.go` import `internal/data`
  to construct fakes (see note above)
- `internal/service/resources/kesselinventoryservice_test.go` imports `internal/data`
  for the same reason, at the Presentation layer
- `internal/consumer` mixes Kafka I/O with use-case orchestration and imports `data`/`gorm`

## Ports vs adapters (IDDD Ch. 4, Ch. 12)

Ports (interfaces) live in **domain** — the domain defines the contract, not
infrastructure:

- `RelationsRepository`, `SchemaRepository`, `ResourceRepository`, `TransactionManager`, `Schema`

Adapters live in **infrastructure** — they implement the port and perform I/O:

- `SpiceDBRelationsRepository`, `GRPCRelationsRepository`, `AllowAllRelationsRepository`
- `RelationsRepositoryDecorator` (ACL decorator — see below)
- `InMemorySchemaRepository`, GORM resource repos, Kafka consumer

Domain may implement a port only when the implementation has **no I/O**
(strategy, in-memory fake simple enough to live next to the port).

### Anti-Corruption Layer (ACL) decorator pattern (IDDD Ch. 3, Ch. 13)

```
Caller → [port: RelationsRepository] → RelationsRepositoryDecorator (ACL adapter)
                                              │
                                              ├── calls SchemaService.Translate* (domain rules)
                                              │
                                              └── delegates to inner RelationsRepository (raw adapter)
                                                       │
                                                       └── SpiceDB / Kessel / AllowAll
```

The decorator:

1. Receives a request in the **downstream's ubiquitous language** (logical types)
2. Translates to the **upstream's language** using domain rules (`SchemaService`)
3. Forwards to the raw upstream adapter
4. Translates results **back** to the downstream's language
5. Returns to the caller, who never sees the upstream's model

The translation *logic* (what maps to what) is a domain rule and lives in a
domain service. The decorator that *applies* it at the port boundary is
infrastructure. Wiring the decorator together is Main's job.

## DDD tactical patterns to check on a diff

**Value Objects (IDDD Ch. 6):** Immutable, equality by value, created through
`New*` constructors that validate and normalize. `Deserialize*` reconstructs
from trusted persistence/wire formats and bypasses validation — domain logic
that *computes* a new value must go through `New*`, never a bare type
conversion (`ReporterType("HBI")`).

**Domain Services (IDDD Ch. 7):** Stateless, no I/O, no framework
dependencies, operate on domain types, don't belong to a single aggregate
(e.g. `SchemaService`). They own the "what maps to what" knowledge; adapters
only own "when to apply it."

**Aggregates and invariants (IDDD Ch. 10):** Invariants are protected inside
the aggregate/domain service, not in adapters. An unsafe operation (e.g. an
unscoped delete that folds to a parent type) must be rejected by the domain
*before* the adapter is even called — the adapter's job is to forward or
not-forward, never to decide business safety.

**Repositories / Ports (IDDD Ch. 12):** A port speaks the **ubiquitous
language** of its bounded context, never the upstream system's language. A
method that exposes raw upstream semantics (e.g. `ReadTuples` with no
translation) is a Conformist backdoor — acceptable only as a clearly
deprecated, documented escape hatch.

**Application Services (IDDD Ch. 14):** Orchestrate use cases; contain no
domain logic. Transaction boundary lives here, not in domain or
infrastructure. Must not depend on infrastructure directly — use ports.

**Modules (IDDD Ch. 9):** Flat and wide domain package, no deep nesting. Each
module speaks one ubiquitous language.

## Semantic checks (apply to the diff, not the whole file)

| ID | Check | Severity if new |
|---|---|---|
| H1 | Business rule (rewrite table, permission-name policy, "which types are derived") implemented in `internal/data` instead of domain | blocker |
| H2 | Same translation/rewrite repeated in usecase, consumer, *and* service instead of one chokepoint (decorator + domain service) | should-fix |
| H3 | Domain port has a method documented "raw SpiceDB, do not translate" (bilingual port) | should-fix |
| H4 | Caller does `switch repo.(type)` on a concrete adapter instead of gating on config/capability | blocker |
| H5 | Adapter translates a request but doesn't restore the caller's model on the result (inverse mapping) | should-fix |
| DDD1 | Tiny type built via direct conversion instead of `New*` constructor | suggestion |
| DDD2 | Adapter enforces an invariant instead of rejecting in domain first | blocker |
| DDD3 | New translator invented in `internal/data` duplicating a domain service's mapping knowledge | should-fix |
| DDD4 | ACL only translates one direction (leaks upstream model through results) | blocker |
| DDD5 | Callers must know SpiceDB type names/relation prefixes (conformist leak through the port) | blocker |
| DDD6 | Domain relies on the adapter to reject an unsafe operation | blocker |

Mechanical checks (`internal/biz/model` importing `internal/data`/`service`/
`consumer`, GORM, protobuf, or gRPC; `internal/biz/usecase` importing `data`/
`service`; `internal/service` importing `data`; protobuf types in model/
repository interfaces/usecase) can be run today via a personal, user-global
Cursor skill (`application-arch`, not checked into this repo) that includes a
`check-import-direction.sh` script. That script is not currently committed
here. Consider adding an equivalent script under this repo's own tooling
(e.g. a new `scripts/` directory) and wiring it into CI as a hard,
deterministic gate on the mechanical subset of these rules — a
`.coderabbit.yaml` code-guidelines doc alone cannot guarantee this the way a
compiled check can.

## What each layer's tests look like

| Layer | Tests |
|---|---|
| Main | Config meaning/validation; rare hermetic command tests |
| Presentation | Protocol, error mapping, serialization; in-memory transport (bufconn) |
| Application | Use cases with fake repos; hit branches reachable from the external API |
| Domain | Small, instant unit tests; full-object `assert.Equal`; no I/O |
| Infrastructure | Adapter tests; contract tests defined next to the port, run per adapter |

No method-verifying mocks (`EXPECT().Times(1)`). Prefer real instances, then
fakes, then recording fakes/spies for decorator argument capture.

## Report format for reviewers (human or AI)

Lead with a verdict: **aligned**, **aligned with suggestions**, or
**violations**. For each finding: severity (blocker / should-fix / suggestion
/ pre-existing), which rule it breaks (DDR, hexagonal placement, DDD pattern,
testing), where (file + symbol), why (layer it's in vs. layer it belongs in),
and a one-sentence fix direction — no drive-by refactors. Celebrate correct
placement (e.g. an ACL decorator in `internal/data` calling
`SchemaService.Translate*` is a success, not a smell).

Do not: treat Kratos `biz`/`data`/`service` names as sufficient; block on
pre-existing GORM/Kratos-log in domain unless the PR adds more of it; move
domain rules into adapters "because they're about SpiceDB" (SpiceDB *shape* is
infrastructure, *which types fold and how relations are prefixed* is domain).
