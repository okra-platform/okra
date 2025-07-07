# Service-to-Service Communication

Service-to-service communication in OKRA is built on top of GoAKT, but this is **fully transparent to the service code**.

---

## High-Level Behavior

- Services declare their dependencies in the **service config file**
- At runtime, the corresponding service stubs are **injected into the constructor**
- The constructor expects typed interfaces for each dependent service

---

## Service Stub Behavior

The generated stub for each dependency:

1. **Serializes** the input to JSON as `[]byte`
2. **Calls a host function** injected by the actor
3. **Receives raw response bytes**
4. **Deserializes** the JSON response into the expected type

From the perspective of the service, it’s a normal method call.

---

## Role of WASMActor

The `WASMActor` injects the host function used by all service stubs.

When the stub calls the host function, it:

1. Validates that the caller is authorized to invoke the target service
2. Constructs a `ServiceRequest` message with method name and payload
3. Sends it to the target actor via GoAKT (using the service name as the actor ID)
4. Waits for the reply and returns the response bytes back to the stub

---

## Summary

- GoAKT is the transport, but services only see typed interfaces
- Dependencies are declared via config and passed to constructors
- All inter-service communication is mediated by the WASM runtime and actor host
