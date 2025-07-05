# 03 – Exposing Services via ConnectRPC

OKRA services are internally actor-based, but they can be exposed to the outside world via **ConnectRPC** or standard **gRPC**.

This allows external clients (e.g., browsers, mobile apps, backend systems) to call OKRA services using standard protocols, while the runtime handles routing to internal actors and WASM modules.

---

## Built-In Runtime Behavior

External exposure is **not handled by a separate gateway**.  
Instead, the **OKRA runtime itself**:

- Initializes the GoAKT Actor System
- Loads and registers configured services
- Starts a ConnectRPC/gRPC server to expose those services externally
- Routes incoming calls to the appropriate service actor

---

## ServiceRequest Flow

When a client makes an external call:

1. OKRA's ConnectRPC server receives a Protobuf request (e.g., `CreateUser`)
2. The runtime wraps the request into a `ServiceRequest`:
   - `method`: fully qualified method name (e.g., `UserService.CreateUser`)
   - `input`: serialized Protobuf message (`[]byte`)
3. Sends the request to the actor registered under the service name (`UserService`)
4. Waits for a response (`[]byte`)
5. Deserializes the response and returns it to the client

---

## Protocol Support

- **ConnectRPC** (default; supports gRPC, gRPC-web, and REST over HTTP/2/1.1)
- **gRPC** (native gRPC clients)
- Future: REST/GraphQL gateways could layer on top

---

## Security & Metadata

- The runtime can inject or validate metadata at the RPC boundary
- Auth info and headers are forwarded with the `ServiceRequest`
- Per-method or per-caller validation can be done in-actor or via policy modules

---

## Summary

- OKRA exposes services directly via ConnectRPC — no external gateway needed
- The OKRA runtime handles both actor system creation and RPC server wiring
- External clients call services as normal; internally they route to actors
- This keeps external protocols and internal actor-based execution fully decoupled
