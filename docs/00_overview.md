# OKRA Overview

OKRA is an open-source platform for building **secure, scalable, type-safe backend services** using **WebAssembly (WASM)** and a **GraphQL-based IDL**.  Under the hood it uses an **Actor System** to support features like Stateful Services, Singleton Services, Service Discovery, Passivation (scale to zero).

It is designed to deliver amazing DX while also providing automatic observability and enterprise governance.  For example:
- Define services using GraphQL-style schema files with `type`, `enum`, and `service` declarations
- Implement logic in plain Go, TypeScript, or other WASM-compatible languages
- Deploy those services as secure, composable backend components
- Scale safely with built-in concurrency, state, and isolation features
- Control the exact Host API surface area a service has access to

OKRA is inspired by systems like Modus, Elixir, Temporal, and Caddy — with a focus on:
- **Simple defaults**
- **Strong type boundaries**
- **Control over Host APIs**
- **Fast startup and local dev flow**

---

## Core Concepts

### OKRA CLI
A CLI tool (`okra`) that allows users to do the following: 
- `init`: create new projects
- `dev`: develop okra services locally 
- `build`: building service code and creating an okra package which includes a manifest, the WASM module (.wasm) and the compiled service description (service.description.json) 
- `deplooy`: deploying the services to a cluster

### OKRA Runtime
The runtime service that is responsible for creating the ***Actor System***, deploying okra packages to it and exposing services to ConnectRPC/gRPC (for exposed services).

### Service
User code defined by a GraphQL schema and implemented in plain Go, Typescript, etc.
- Is compiled to WASM buy OKRA
- Can be exposed internally to other services by name
- Can be exposed via ConnectRPC/gRPC
- Can be stateful and/or singleton
- Is run in an isolated context
- Simple, built-in concurrency:
    - Each call to the service is intentionally single-threaded (WASM)
    - Each service is automatically executed in a worker pool - with multiple instances of the WASM module handling requests 
- Co-pilot friendly Host API is provided and is configurable per-service and extensible

### Actor System
GoAKT provides an underlying actor system that faciliates messaging, supervision, and passivation, etc.

### WASMActor
A runtime actor that:
- Loads a compiled `.wasm` module
- Creates one or more instances of the wasm module
- Receives messages (via GoAKT)
- Dispatches those messages to a method inside the WASM module

### WASMSingletonAtor
A singelton actor (GoAKT SingletonActor) that only exists once in the cluster and can access shared state.
- Performs the same behaviors as WASMActor

### OKRA Package (.okra.pkg)
Each deployed module contains:
- service.wasm: The compiled logic
- service.description.json: The JSON description of the parsed GraphQL IDL
- okra.service.json: manifest and configuration file that describes the service

## Related Docs

### Architecture & Implementation
- [01_system-diagrams.md](./01_system-diagrams.md) – High-level system and flow diagrams
- [02_actor-messaging.md](./02_actor-messaging.md) – Message routing with GoAKT and mapping to Services
- [03_exposed-services.md](./03_exposed-services.md) – Details of how services are exposed to the outside via ConnectRPC / gRPC
- [04_service-to-service.md](./04_service-to-service.md) – Details of how service-to-service communication is built on top of GoAKT
- [05_wasm_actors.md](./05_wasm_actors.md) – WASMActor and WASMSingletonActor details & worker pool

### Service Development
- [06_service-packages.md](./06_service-packages.md) – The generated code and built artifacts that live in an okra package
- [07_host-apis.md](./07_host-apis.md) – How Host APIs are injected into the services
- [08_service_IDL.md](./08_service_IDL.md) – Service Interface Definition Language (GraphQL schema) documentation
- [09_typescript_setup.md](./09_typescript_setup.md) – TypeScript service development setup and guidelines
- [10_development-debugging.md](./10_development-debugging.md) – Development and debugging guide for OKRA services

### Operations
- [11_okra-serve.md](./11_okra-serve.md) – Production runtime server documentation (okra serve command)

### Development Guidelines
- [100_coding-conventions.md](./100_coding-conventions.md) – Coding conventions and best-practices for this repo 
- [101_testing-strategy.md](./101_testing-strategy.md) – Testing philosophy, strategy and conventions
- [102_testing-best-practices.md](./102_testing-best-practices.md) – Best practices for approaching testing

---

## Implementation Details vs Developer Experience

### What Developers See:
- **Services with methods** - Define services using GraphQL IDL with decorators
- **Type-safe service stubs** - Call remote services with full type safety
- **Host APIs for capabilities** - Injected APIs for state, logging, HTTP, etc.
- **Events for decoupled communication** - Emit and handle typed events
- **Workflows for orchestration** - Declarative multi-service coordination

### What's Hidden (Implementation Details):
- **Actor model (GoAKT)** - Used internally for clustering, supervision, passivation
- **Message passing between actors** - All RPC calls are actor messages internally
- **WASM worker pool management** - Automatic scaling of WASM instances
- **Internal routing and load balancing** - Handled by the actor system
- **Service discovery** - Actor registry manages service locations

**Important**: Developers never interact with actors directly. They write services with methods, and the runtime handles all distribution, scaling, and messaging transparently.

---

## WASM Constraints & Design Decisions

Due to WebAssembly's single-threaded execution model, OKRA makes specific design choices:

### WASM Limitations:
- **No callbacks**: Methods must be explicitly exported by name
- **No closures**: Cannot pass functions to host APIs
- **Synchronous within instance**: Each WASM instance handles one request at a time
- **No shared memory**: Instances are isolated from each other

### How This Influences Our APIs:
- **Scheduling references method names**, not callbacks
  ```graphql
  # ✅ Correct: References method by name
  @scheduleOnly
  dailyCleanup(): void
  
  # ❌ Not possible: No callback functions
  schedule(() => cleanup())  
  ```

- **Events use declarative handlers**, not runtime subscriptions
  ```graphql
  # ✅ Correct: Declarative handler
  @handles(event: "order.created")
  onOrderCreated(event: OrderCreatedEvent): void
  
  # ❌ Not possible: Runtime subscription
  events.subscribe("order.created", callback)
  ```

- **No timer callbacks** - Use scheduling or workflows for time-based operations
  ```graphql
  # ✅ Correct: Schedule a method
  schedules:
    - cron: "0 * * * *"
      method: "hourlyTask"
  
  # ❌ Not possible: Timer with callback
  setTimeout(() => doWork(), 1000)
  ```

### Why These Constraints Are Good:
- **Predictable execution** - No callback hell or closure surprises
- **Better for AI** - Clear patterns without dynamic behavior
- **Easier debugging** - All code paths are statically analyzable
- **Natural scaling** - Each instance is independent


