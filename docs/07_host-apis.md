# Host API Injection

Host APIs allow the OKRA runtime to expose system capabilities (e.g., state, logging, service calls) into WASM services in a controlled and sandboxed way.

---

## Injection Model

- Host APIs are registered and injected by the `WASMActor` at instantiation
- Each API is exposed as a named host function (or set of functions)
- Only explicitly declared APIs are injected per service, based on config

---

## Common Host APIs

- `okra.state` – Shared or persistent key-value storage
- `okra.log` – Structured logging
- `okra.call` – Used by service stubs to invoke other services
- (future) `okra.time`, `okra.metrics`, `okra.queue`, etc.

---

## ✅ Summary

- Host APIs are the bridge between the WASM sandbox and the OKRA runtime
- Services opt into APIs via configuration (or operators provide policies to allow/prevent certain Host APIs)
- Each API is versioned and registered at runtime
