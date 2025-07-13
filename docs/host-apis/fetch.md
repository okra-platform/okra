# Host API: `http.fetch`

The `http.fetch` Host API provides guest services the ability to make outbound HTTP requests. It closely mirrors the standard [Fetch API](https://developer.mozilla.org/en-US/docs/Web/API/fetch) available in browsers, with minimal additions or modifications to accommodate policy enforcement, request metadata injection, and host environment constraints.

---

## Interface

```ts
interface FetchRequest {
  url: string;
  method?: string; // Default: 'GET'
  headers?: Record<string, string>;
  body?: string | Uint8Array;
  timeoutMs?: number; // Optional timeout in milliseconds
}

interface FetchResponse {
  status: number;
  statusText: string;
  headers: Record<string, string>;
  body: Uint8Array;
}

interface HttpHostAPI {
  fetch(request: FetchRequest): Promise<FetchResponse>;
}
```

---

## Host API Configuration

The `http.fetch` Host API may be configured per environment, per service, or globally using a host-provided configuration file or `okra.json`. Configuration settings include:

```ts
interface HttpHostAPIConfig {
  allowedSites?: string[]; // e.g. ["https://api.example.com", "*.trusted.net"]
  injectHeaders?: Record<string, string>; // headers to add to every outbound request
  timeoutMsDefault?: number; // default timeout to use if not provided in request
  maxBodySizeKb?: number; // upper limit on size of request or response body
  enableTracing?: boolean; // auto-injects trace headers if available
}
```

---

## Enforceable Okra Policies

The `http.fetch` Host API supports a variety of enforcement policies. These can be declared globally or per service via CEL (Common Expression Language) expressions referencing request and context metadata.

### Examples:

#### 1. **Restrict to allowed domains**

```json
"policy.http.fetch.allowedSites": ["https://api.example.com", "*.trusted.net"]
```

#### 2. **Enforce read-only methods**

```json
"policy.http.fetch.allowMethods": ["GET", "HEAD"]
```

#### 3. **CEL-based policy**

```json
"policy.http.fetch.condition": "request.method in ['GET', 'POST'] && request.url.startsWith('https://api.') && request.auth.claims.role == 'internal'"
```

#### 4. **Limit timeout**

```json
"policy.http.fetch.maxTimeoutMs": 5000
```

#### 5. **Require specific header**

```json
"policy.http.fetch.requiredHeaders": ["x-service-id"]
```

#### 6. **Audit logging policy**

```json
"policy.http.fetch.audit": true
```

Enables structured audit logging of all outbound requests.

---

## Notes for Codegen / Shim Targets

- The guest-language shims should conform to the above `HttpHostAPI` interface.
- The host implementation should handle header injection, tracing, and policy enforcement before making the outbound request.
- Any disallowed request (due to policy) should return an appropriate structured error.
- A `fetch` polyfill may be used in supported guest environments (e.g., JS or WASM) to simplify integration.

---

This document serves as the canonical definition for the `http.fetch` Host API. It will be used to generate:

- Interface bindings per supported guest language
- Policy enforcement stubs in the host runtime
- Developer documentation
- Auto-generated policy schema for validation and tooling

---

Next APIs to define: `sql.query`, `log.write`, `event.emit`, `secrets.get`.

