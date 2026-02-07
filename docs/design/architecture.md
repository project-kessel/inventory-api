# Inventory Application Architecture

# History

Inventory has evolved quickly and changed many times. It was a prototype. Then it shipped with ACM Global Hub. Then it was part of Kessel. Then we added persistence. Then we decided to combine it with Inventory. We did all of this in a hurry. We were under tight deadlines. Core teammates changed several times. There was little time to stop and assess the architecture.

The current architecture is therefore mostly incoherent. Inventory has a maintainability crisis. Boundaries are not enforced between layers. Significant business logic exists outside of a domain model. This design and refactoring defined here is intended to resolve this cleanly, clearly, and efficiently.

# Layered architecture: Review

These \~5 layers are ubiquitous to any nontrivial, service-oriented application:

1. **Main (Bootstrap / Application):** Process entry and exit point. Config parsing and initial object graph. It is the only layer that depends on Infrastructure.  
2. **Presentation / Transport:**  Protocol specifics, request validation, credential validation, mapping to value objects. Depends on application/domain.  
3. **Application Services:** Transaction boundary, (meta) authorization, observability, published available commands and queries. Used by Main (CLI commands) and Presentation (commands available over the wire).  
4. **Domain:** The domain model. 95% of business logic is here. Interfaces for "external" dependencies are here.  
5. **Infrastructure:** Implementations of interfaces used across layers. Not depended on directly by anything except Main.

*See also: [https://www.alechenninger.com/2020/11/the-secret-world-of-testing-without.html](https://www.alechenninger.com/2020/11/the-secret-world-of-testing-without.html)*  

# Details

## Main (Bootstrap / Application)

This is the layer that contains the process entrypoint. It deals with configuration and bootstrapping the application. A dependency graph is instantiated here.

* main.go  
* CLI commands  
* Config schemas for deserialization from CLI options, config files, env vars, etc. (this is stable API)  
* Object graph instantiation and lifecycle (e.g. hot reload, if supported)

### Tests

Tests here are focused on the meaning and validation of config options, and maybe a few tests that run substantial commands with in-memory (hermetic) dependencies. Tests are few because the point of this module is to couple to the outside world, and this makes it difficult to test. We push that complexity here in order to make everything else more testable.

## Presentation / Transport

This layer is responsible for supporting inbound communication over a network. For example, with gRPC, this is where the implementations of the generated gRPC servers go. Middleware is used for cross cutting and protocol specific concerns like authentication (as credential presentation is necessarily protocol specific) or pagination.

Server implementations here are *small*. They only care about mapping between *Application Services* and the *protocol*. There is no business logic here. It depends on Application Services and the Domain, as Application Services' interfaces speak in terms of the Domain Model.

### Tests

Tests here focus on protocol specifics, such as handling of errors and serialization edge cases. It should attempt to stand up the presentation layer as production-like as possible, with the same middleware. They instantiate in memory dependencies where I/O may be normally involved.

### But what about request validation or normalization? Isn't that business logic?

It's presentation logic. Let's look at why and how to avoid confusing this with what *is* business logic.

Whenever you invoke another method in an application, that method has certain preconditions that must be true. If those preconditions are not true, it returns an error. If you can prevent this error ahead of time, you should. You'll have more context about what is wrong. You won't leak an underlying API's details.

Invoking the Application Services from Presentation is an example of this. The Application Services' commands and queries will have certain preconditions. The presentation layer should ensure these conditions are met *before* invoking the Application Service. Errors such as these from the application service may be considered server errors. Errors caught in Presentation can be clearly classified as bad input. It does not mean the caller is responsible for preemptively identifying all error scenarios. Of course, that is the job of the logic of the method. Only *preconditions* that are clearly documented as part of the method's contract should be checked.

This is not to be confused with *domain model* validation or normalization. This is not a *substitute* for validation in the domain. It is in addition to it. Only as a little as necessary is done in presentation to make for useful errors according to the protocol.

## Application Services

All of the commands and queries which can be invoked directly from outside the process are defined here. This includes the public API. It also includes commands that may be only available through CLI tools and admin interfaces and the like.

It makes the end to end use cases of an application testable, without any coupling to the I/O concerns of external process communication (e.g. CLI input or requests over the network). It makes them reusable from different protocols.

### Tests

Tests here focus on the use cases of the application. They may not hit every branch in the domain model, but should hit nearly every branch possible from the external API. Use fakes for I/O (e.g. repositories).

## Domain

This is the primary home for business logic. For inventory, this looks like logic for resources, schema validation, tuple calculation, and replication. Domain is usually *flat and wide*. It is a large package, with little to no nesting. Nesting quickly gets arbitrary, requires duplication, or creates import cycles. A domain model reuses its rich types throughout, almost never relying on primitives except for internal implementation detail.

### Domain Model vs Data Model

A Domain Model is commonly confused with a database's *data model*. In some communities (e.g. Java), it is common that they are even defined together. They are not the same thing.

A domain model defines the nouns and verbs of the business. We express business rules as directly and simply as possible in imperative code. It matches how the entire team, from coders to UX, speak about the problem domain. It necessarily contains data (state), but it is not the same thing as the data model used by a database to persist this state. It is the responsibility of the *repository implementation* to define how it translates the state of the domain model into database state, and back. However, domain models often participate in *enabling this* by exposing a serialized form.

This can be explicit or declarative. In an explicit model, the domain model has explicit serialize/deserialize functions that expose the raw state of the model, and hydrate from raw state. In a declarative model, the domain model is annotated (in some language dependent way) with how it can be serialized. Declarative metadata can "get away" with describing repository implementation details in the domain model, because it is just that: declarative metadata. Good metadata frameworks are flexible enough such that they don't otherwise impose themselves on the domain model. An ideal domain model makes no concessions to frameworks. Its only concern is modeling the business problem accurately and correctly.

In Inventory, we use an explicit approach. The domain model can be serialized and deserialized to/from generic JSON-like structures. From here, the repository layer can transform this however it needs for its persistence. (If you're curious about the overhead of this, it's nanoseconds. [See here](https://github.com/alechenninger/go-ddd-bench).)

### Tests

Tests here are true "unit tests." They can usually isolate a single method or struct. They tend to hit all branches because the units are so small. Tests are small, instant, and numerous. 

## Infrastructure

Infrastructure is where implementations of interfaces defined elsewhere go. These are not the essence of the application, but adapters to make the essential abstractions work in certain environments or dependencies. Implementations that require I/O in particular almost always go here. There is little business logic here. Some business rules are expressed insofar as they are requirements of their interfaces. As such, it is not the responsibility of infrastructure to define business logic, but it may have to adhere to it. For example, a repository has to understand the domain model enough in order to enforce constraints and follow query parameters.

### Tests

Tests in infrastructure are usually ["medium" or "large"](https://testing.googleblog.com/2010/12/test-sizes.html) in the sense that they typically, by design, require I/O or an external system. This is one of the reasons we isolate this code.

#### Contract Tests {#contract-tests}

Tests in infrastructure often benefit from being designed as "contract tests" which are reusable for other implementations. "Contract tests" are defined *where the interface is defined, not the implementation*. They speak only in terms of a factory (to get some implementation) and clean up (cleaning up external resources). Then, they exercise that interface to demonstrate expectations. The actual test runner exists in infrastructure for a specific implementation. It invokes the contract tests, providing only the necessary factory and clean up. [Example](https://github.com/alechenninger/falcon/blob/ae638df2a195b903a76e414db00d3aa32078a09a/internal/domain/storetest.go). This makes it easy to:

* document the expectations of an interface in terms of the business language and domain model  
* test different implementations adhere to it  
* …which is especially important for when an implementation necessitates I/O, and therefore you want a correct fake in-memory implementation to also pass tests such that it is a confident substitute for the real thing.

# Testing

Tests are done deterministically and [hermetically](https://testing.googleblog.com/2018/11/testing-on-toilet-exercise-service-call.html).

## No I/O

By default, there is NO external I/O in tests. This often includes syscalls (e.g. time, randomization). This means you often need to design main code to support testability:

* Instead of using time directly, inject a Clock abstraction, OR use [synctest](https://go.dev/blog/testing-time)  
* Instead of databases or queues, use in memory fakes  
* Instead of using the filesystem directly, inject a filesystem abstraction (either [io/fs](https://pkg.go.dev/io/fs) or something more full featured like [afero](https://github.com/spf13/afero))  
* Instead of using randomness directly, inject an abstract source of randomness and use a deterministic version for tests

Never use time.Sleep in a test. Use a clock abstraction to advance the time, or [deterministic concurrency](#deterministic-concurrency).

The only exception is when the code under test itself is necessarily coupled to external I/O. If you have a PostgresRepository, you obviously have to test it by connecting to a postgres instance. But if you aren't specifically testing the implementation of something dependent on I/O, avoiding it will improve your tests and your designs.

## Deterministic concurrency {#deterministic-concurrency}

Coordinating threads / goroutines is sometimes necessary in tests. To do this deterministically and cleanly, take advantage of the [Domain Oriented Observability](https://martinfowler.com/articles/domain-oriented-observability.html) pattern. The main code is coupled on to an interface with certain probe points. Then, an implementation of this injected at test time uses these probes to block, or signal waiting code.

For an example of how to do this, see [this](https://github.com/alechenninger/falcon/blob/ae638df2a195b903a76e414db00d3aa32078a09a/internal/domain/observer.go#L252).

## No Mocks

No "method verifying" mocks. [Not to be confused with *stubs*](https://martinfowler.com/bliki/TestDouble.html) (which can be perfectly fine).

Prefer simply using a real instance. If an object is not coupled to external I/O, there is no reason not to reuse it.

If it is, prefer using a Fake. In memory fakes are a useful feature of an application ("Kessel in a box"), so the investment pays for itself quickly. When implementing fakes (or any second implementation of an interface), it is useful to first define a set of "[contract tests](#contract-tests)" at the interface layer.

## Hermetic

When external dependencies are needed, leverage testcontainers to download and run them locally. This should only be for when this essential. For example, we can't test a PostgresStore without a Postgres. Writing a "fake" postgres is absurd 🙂. But, if you need to test business logic that involves a repository, using a real postgres is overkill. Just use the in memory fake.

# What about Kratos?

Kratos is the framework used by Inventory. Kratos has a [best practice project layout](https://go-kratos.dev/docs/intro/layout/). They are both inspired by DDD practices (also layered aka hexagonal architecture). So, these are largely compatible with a few tweaks:

* We separate transport / presentation layer concerns from application services. In Kratos, the "service" layer is overloaded as both presentation and application service. The decoupling costs very little (a little bit of code and nanoseconds of wall time), but gains in clarity and reusability. For example, application services could be invoked directly from CLI without having to convert to protos in the middle. So we recommend that here, instead.  
* "biz" \== the model  
* "data" \== infrastructure

Also, maybe we don't use Kratos in the long run. This architecture helps remove coupling to the framework by isolating it.