# WASM Actors and Worker Pool

This doc outlines the architecture and design of `WASMActor`, `WASMSingletonActor`, and the supporting `WASMWorkerPool`. These components enable efficient, scalable execution of GraphQL-defined service methods compiled to WebAssembly (WASM).

---

## WASMActor

- Regular GoAKT actor
- Addressed via the fully qualified service name
- Initialized with:
  - `[]byte` WASM binary
  - min/max instance count
  - optional host APIs (e.g., state)
- Dispatches requests to pooled WASM instances via `WASMWorkerPool`

---

## WASMSingletonActor

- GoAKT `SingletonActor`
- Used when the service requires:
  - Shared mutable state across requests
  - Background polling or coordination
- Otherwise behaves like `WASMActor`, using the same worker pool logic

---

## WASMWorkerPool

- Manages a pool of `WASMWorker` instances created from a compiled module
- Creates a new worker if none are available 
- Blocks if max workers are active and none are idle
- Uses `acquire(ctx)` and `release(worker)` internally
- Warms `min` workers on startup


### Key methods:
```go
type WASMWorkerPool interface {
    Invoke(ctx, method string, input []byte) ([]byte, error)
    WorkerCount() int
    Shutdown(ctx context.Context) error
}
```

## WASMWorker
* Wraps a Wazero module instance
* Exposes:
    ```go
    func (w *WASMWorker) Invoke(ctx, method string, input []byte) ([]byte, error)
    ```
* May be injected with optional host APIs (e.g., shared state, logging)

## 🔁 Message Flow

The typical request lifecycle for a WASM-backed service:

1. **Client** sends a `ServiceRequest` to an actor with:
   - Target actor ID = service name
   - Method name (as string)
   - Input payload (JSON as raw `bytes`)

2. **Actor** receives the message and:
   - Validates the method and payload using the service description
   - Delegates execution to the `WASMWorkerPool`

3. **WASMWorkerPool**:
   - Acquires an idle or new `WASMWorker` (up to max)
   - Calls `worker.Invoke(ctx, method, input)`
   - Returns the result or error

4. **WASMWorker**:
   - Instantiates (or reuses) a Wazero module
   - Writes `input` into WASM memory
   - Calls the exported function (e.g., `handle_request`)
   - Reads the result from memory
   - Returns `[]byte` output

5. **Actor**:
   - Wraps the output into a `ServiceResponse` (TBD)
   - Sends the response back to the caller


```mermaid 
sequenceDiagram
    participant Client
    participant Actor (WASMActor)
    participant Pool (WASMWorkerPool)
    participant Worker (WASMWorker)
    participant Module (Wazero Module)

    Client->>Actor (WASMActor): ServiceRequest(method, input: bytes)
    Actor->>Actor: Validate method + input using service description
    Actor->>Pool: Invoke(ctx, method, input)
    Pool->>Worker: Acquire or instantiate worker
    Worker->>Module: Write input to memory
    Worker->>Module: Call exported function (e.g., handle_request)
    Module-->>Worker: Return raw output bytes
    Worker-->>Pool: Release worker
    Pool-->>Actor: Return output bytes
    Actor-->>Client: ServiceResponse(output)
```

