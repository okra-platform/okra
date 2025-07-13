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

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **URL validation** - Must be valid URL with http/https scheme
2. **Maximum URL length** - Prevent memory exhaustion (default: 8KB)
3. **Maximum body size** - Prevent memory exhaustion (default: 10MB request, 100MB response)
4. **Header injection protection** - Sanitize header names/values, prevent CRLF injection
5. **Timeout bounds** - Minimum 100ms, maximum 5 minutes
6. **Redirect limit** - Maximum 10 redirects to prevent loops
7. **Protocol restrictions** - Only HTTP/HTTPS allowed (no file://, ftp://, etc.)
8. **Reserved header protection** - Cannot override Host, Content-Length, etc.

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### 1. **Restrict to allowed domains**

```json
"policy.http.fetch.allowedSites": ["https://api.example.com", "*.trusted.net"]
```

#### 2. **Method restrictions**

```json
"policy.http.fetch.allowedMethods": ["GET", "POST", "PUT", "DELETE"]
```

#### 3. **Conditional access based on context**

```json
"policy.http.fetch.condition": "request.url.host() in ['api.trusted.com'] || request.auth.claims.role == 'admin'"
```

#### 4. **Timeout limits**

```json
"policy.http.fetch.maxTimeoutMs": 5000,
"policy.http.fetch.defaultTimeoutMs": 3000
```

#### 5. **Required headers**

```json
"policy.http.fetch.requiredHeaders": ["x-service-id", "x-request-id"]
```

#### 6. **Blocked headers**

```json
"policy.http.fetch.blockedHeaders": ["x-internal-token", "x-admin-override"]
```

#### 7. **Rate limiting**

```json
"policy.http.fetch.rateLimit": "100/minute",
"policy.http.fetch.rateLimitPerDomain": "20/minute"
```

#### 8. **Environment-specific policies**

```json
"policy.http.fetch.condition": "env.DEPLOYMENT_ENV == 'production' ? request.url.startsWith('https://') : true"
```

#### 9. **Body size limits (within code maximum)**

```json
"policy.http.fetch.maxRequestBodyKb": 1024,
"policy.http.fetch.maxResponseBodyKb": 5120
```

#### 10. **Audit logging**

```json
"policy.http.fetch.audit": true,
"policy.http.fetch.auditLevel": "headers" // "none", "headers", "full"
```

#### 11. **TLS requirements**

```json
"policy.http.fetch.minTLSVersion": "1.2",
"policy.http.fetch.requireValidCert": true
```

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

