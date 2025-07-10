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

## Host Distribution

Hosts are distributed via the OKRA registry system:

- Published as `type: host` registry entries
- Downloaded automatically by `okra dev` or `okra serve` based on `okra.json` configuration
- Selected using `host` or `hostVersion` field in service's `okra.json`
- Include platform-specific binaries (darwin-arm64, linux-amd64, etc.)
- Contain a manifest describing available Host APIs and capabilities

### Creating Custom Hosts

To extend Host APIs:
1. Create a new OKRA host project with `okra init:host` (future)
2. Add custom Host API implementations
3. Build and publish to a registry
4. Reference in service `okra.json` via `host` or `hostVersion`

---

## ✅ Summary

- Host APIs are the bridge between the WASM sandbox and the OKRA runtime
- Services opt into APIs via configuration (or operators provide policies to allow/prevent certain Host APIs)
- Each API is versioned and registered at runtime
- Hosts are distributed via registries for easy deployment and versioning
