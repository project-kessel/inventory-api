---
name: RelationsRepository refactor
overview: Replace the domain port `model.Authorizer` and the `internal/authz` wiring package with a DDD-style `RelationsRepository` interface in the biz layer, implementations and a config-driven factory in `internal/data`, and a dedicated `internal/config/relations` package—mirroring how `SchemaRepository` is structured today.
todos:
  - id: biz-interface
    content: Add model.RelationsRepository (same contract as Authorizer); remove/rename authorizer.go
    status: completed
  - id: config-relations
    content: Create internal/config/relations (+ kessel subconfig); wire OptionsConfig and clowder/flags paths
    status: completed
  - id: data-factory-impls
    content: Add data.NewRelationsRepository; relocate kessel/allow/simple implementations from internal/authz
    status: completed
  - id: call-sites
    content: Update serve, health, resource usecase, tests; replace CheckAuthorizer; consumer mechanical updates only (see step 6)
    status: completed
  - id: delete-authz
    content: Remove internal/authz; run go test ./...
    status: completed
isProject: false
---

# Refactor authz into RelationsRepository (DDD repository pattern)

## Current state

- **Domain port:** [`internal/biz/model/authorizer.go`](internal/biz/model/authorizer.go) defines `Authorizer` with Health, Check*, Lookup*, tuple CRUD, lock, workspace helpers—all backed by the relations-api gRPC types.
- **Wiring + implementations:** [`internal/authz/authz.go`](internal/authz/authz.go) switches on completed config and returns `allow` or `kessel` concrete types. Subpackages [`internal/authz/kessel`](internal/authz/kessel) and [`internal/authz/allow`](internal/authz/allow) implement the interface; [`internal/authz/simple_authorizer.go`](internal/authz/simple_authorizer.go) is a large in-memory/test implementation.
- **Config:** [`internal/authz/options.go`](internal/authz/options.go) / [`internal/authz/config.go`](internal/authz/config.go) hold flags and `Complete`; [`internal/config/config.go`](internal/config/config.go) embeds `*authz.Options` and references `authz.Kessel`, etc.
- **Call sites:** [`cmd/serve/serve.go`](cmd/serve/serve.go) calls `authz.New(...)` next to `data.NewSchemaRepository(...)`; [`internal/biz/usecase/resources/resource_service.go`](internal/biz/usecase/resources/resource_service.go), [`internal/consumer/consumer.go`](internal/consumer/consumer.go), [`internal/data/health/healthrepository.go`](internal/data/health/healthrepository.go), tests, and [`internal/service/health/health.go`](internal/service/health/health.go) depend on `model.Authorizer` and/or `authz.CompletedConfig`.

## Target shape (aligned with existing repositories)

Follow the same layering as schema:

| Layer | Schema today | Relations after refactor |
|--------|----------------|---------------------------|
| Domain interface | [`internal/biz/model/schema_repository.go`](internal/biz/model/schema_repository.go) | New [`internal/biz/model/relations_repository.go`](internal/biz/model/relations_repository.go) with **`RelationsRepository`** (same method set as today’s `Authorizer`; optionally keep `type Authorizer = RelationsRepository` temporarily if you want a softer migration—prefer a clean rename unless you need compatibility). |
| Completed config | [`internal/config/schema`](internal/config/schema) | New [`internal/config/relations`](internal/config/relations): move `Options`, `Config`, `Complete`, validation, and constants (`AllowAll`, `Kessel`, `RelationsAPI` endpoint enum if still needed) out of `internal/authz`. Nested kessel options can live under `internal/config/relations/kessel` (parallel to [`internal/config/schema/inmemory`](internal/config/schema/inmemory)). |
| Factory + adapters | [`internal/data/schema_factory.go`](internal/data/schema_factory.go) | New **`data.NewRelationsRepository(ctx, relationsConfig.CompletedConfig, logger)`** in e.g. [`internal/data/relations_factory.go`](internal/data/relations_factory.go) that switches implementation like today’s `authz.New`. |
| Implementations | [`internal/data/schema_inmemory.go`](internal/data/schema_inmemory.go) (and friends) | Move **`KesselAuthz`** (rename to something like `kesselRelationsRepository` if you want naming consistency) and **`AllowAllAuthz`** into `internal/data` (either flat files `relations_kessel.go` / `relations_allow_all.go` or subpackage `internal/data/relations/...`). Move **`SimpleAuthorizer`** into `internal/data` (or `internal/data/fake_relations_repository.go`) for tests—same behavior, new package path. |

## Implementation steps

1. **Add `RelationsRepository`** in biz model: duplicate the current `Authorizer` method signatures (still using relations-api protobuf/grpc types where unavoidable), then **delete `authorizer.go`** or replace it with the new file so there is a single port name.
2. **Introduce `internal/config/relations`:** lift code from [`internal/authz/options.go`](internal/authz/options.go), [`internal/authz/config.go`](internal/authz/config.go), and [`internal/authz/kessel/config.go`](internal/authz/kessel/config.go) / options with minimal behavior change. Update [`internal/config/config.go`](internal/config/config.go) to use `*relations.Options` (field can stay named `Authz` for **mapstructure/CLI backward compatibility**, or be renamed to `Relations` if you are willing to update all `mapstructure` tags and docs in one pass).
3. **Add `data.NewRelationsRepository`:** port the switch from [`internal/authz/authz.go`](internal/authz/authz.go); ensure `KesselAuthz` / `AllowAllAuthz` still satisfy `var _ model.RelationsRepository`.
4. **Replace imports and types:** `model.Authorizer` → `model.RelationsRepository` everywhere (usecase struct field can become `RelationsRepo` for readability). Replace `authz.New` with `data.NewRelationsRepository`; replace `authz.CompletedConfig` with `relations.CompletedConfig` at boundaries (consumer, health, serve).
5. **Rehome `authz.CheckAuthorizer`:** move the function as-is into the new `internal/config/relations` package (e.g. `relations.CheckAuthorizer(CompletedConfig) string`). Update the two call sites in [`internal/data/health/healthrepository.go`](internal/data/health/healthrepository.go) to import the new path. No signature or logic changes.
6. **Consumer (minimal in this PR):** Keep the existing `switch i.Authorizer.(type)` / relations-enabled logic unchanged in structure. Only apply mechanical updates required by the move: field type `model.RelationsRepository`, new config import, and—if concrete types move under `internal/data`—update the `case` arms to the new package-qualified types (e.g. `*data.KesselAuthz` or whatever names/paths you choose). **Do not** add `RelationsRepository` methods or new abstractions for this in the relations PR; treat a cleaner consumer boundary as a **separate follow-up PR**.
7. **Remove `internal/authz`:** delete package after grep shows no references; fix [`internal/authz/options_test.go`](internal/authz/options_test.go) by relocating tests under `internal/config/relations/..._test.go`. Update any READMEs under old paths only if the repo already documents them (optional).
8. **Verify:** `go test ./...` and spot-check serve wiring in [`cmd/serve/serve.go`](cmd/serve/serve.go) (constructor order can match schema: build relations repo alongside schema repo).

## Notes and tradeoffs

- The “repository” here is an **outbound adapter to relations-api** (not GORM), which matches how this codebase already treats `SchemaRepository` (including in-memory backends)—the pattern is **interface in biz, factory + impls in data, config in `internal/config/...`**.
- Keeping **CLI keys** as `authz.*` while the type is `RelationsRepository` is a reasonable compromise to avoid breaking deployments; renaming flags is a separate operational decision.
- `SimpleAuthorizer` is large (~650+ LOC); moving it unchanged into `internal/data` keeps risk low; a later pass could trim it if desired.
- **Consumer scope:** This PR intentionally avoids redesigning [`internal/consumer/consumer.go`](internal/consumer/consumer.go) beyond what is strictly necessary for compilation and behavior parity; a dedicated consumer refactor can introduce interface-based “relations enabled” detection or other cleanup later.
