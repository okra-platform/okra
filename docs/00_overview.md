# OKRA Overview

OKRA is an open-source platform for building **secufe, scalable, type-safe backend services** using **WebAssembly (WASM)** and **Protobuf**.  Under the hood it uses an **Actor System** to support features like Stateful Services, Singleton Services, Service Discovery, Passivation (scale to zero).

It is designed deliver amazing DX while also providing automatic observabiilty and enteprise governance.  For example:
- Define services in `.proto` files
- Implement logic in plain Go, TypeScript, or other WASM-compatible languages
- Deploy those services as secure, composable backend components
- Scale safely with built-in concurrency, state, and isolation features
- Control the exact Host API surface area a sevice has accress to

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
- `build`: building service code and creating an okra package which includes a manifest, the WASM module (.wasm) and the compiled protobuf descrtiptors (.pb.bin) 
- `deplooy`: deploying the services to a cluster

### OKRA Runtime
The runtime service that is responsible for creating the ***Actor System***, deploying okra packages to it and exposing services to ConnectRPC/gRPC (for exposed services).

### Service
User code defined by a .proto definition and implemented in plain Go, Typescript, etc.
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
- service.pb.bin: The descriptor set for the .proto
- okra.service.json: manifest and configuration file that describes the service

## Related Docs
01_system-diagrams.md – High-level system and flow diagrams
02_actor-messaging.md – Message routing with GoAKT and mapping to Services
03_exposed-services.md – Details of how services are exposed to the outside via ConnectRPC / gRPC
04_service-to-service.md - Details of how service-to-service communication is built on top of GoAKT
05_wasm-actors.md – WASMActor and WASMSingletonActor details & worker pool
06_service-packages.md – The generated code and built artifacts that live in an okra package
07_host-apis.md - How Host APIs are injected into the services

100_coding-conventions.md - Coding conventions and best-practices for this repo 
101_testing-strategy.md - Testing philosophy, strategy and conventions
102_testing-best-practices.md - Best practices for approaching testing


