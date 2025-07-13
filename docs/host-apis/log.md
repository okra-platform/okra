# Host API: `log.write`

The `log.write` Host API provides a structured logging interface for guest services. Logs may be routed to stdout, file-based logs, remote log collectors, or observability platforms depending on host configuration.

---

## Interface

```ts
interface LogEntry {
  level: 'debug' | 'info' | 'warn' | 'error';
  message: string;
  context?: Record<string, string | number | boolean>;
  timestamp?: string; // ISO 8601, optional (host will add if missing)
}

interface LogHostAPI {
  write(entry: LogEntry): void;
}
```

---

## Host API Configuration

Host systems may control log routing, filtering, and enrichment.

```ts
interface LogHostAPIConfig {
  allowedLevels?: ('debug' | 'info' | 'warn' | 'error')[]; // e.g. ['info', 'warn', 'error']
  redactFields?: string[]; // e.g. ["context.password", "context.token"]
  enrichWithTrace?: boolean; // automatically attach trace ID if available
  maxEntrySizeKb?: number;
  forwardTo?: 'stdout' | 'file' | 'remote';
}
```

---

## Enforceable Okra Policies

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **Maximum message size** - Prevent memory exhaustion (default: 1MB)
2. **Valid log levels** - Only accept 'debug', 'info', 'warn', 'error'
3. **Log injection prevention** - Sanitize control characters in messages
4. **Maximum context depth** - Prevent deeply nested objects (default: 5 levels)
5. **Maximum context keys** - Limit number of context fields (default: 100)
6. **Valid timestamp format** - Ensure ISO 8601 compliance if provided

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### 1. **Level restriction**

```json
"policy.log.write.allowedLevels": ["info", "warn", "error"]
```

#### 2. **Conditional logging based on context**

```json
"policy.log.write.condition": "entry.level != 'debug' || request.auth.claims.role == 'admin'"
```

#### 3. **Environment-based filtering**

```json
"policy.log.write.condition": "env.DEPLOYMENT_ENV == 'production' ? entry.level != 'debug' : true"
```

#### 4. **Required context fields**

```json
"policy.log.write.requiredContextKeys": ["requestId", "component"]
```

#### 5. **Field redaction patterns**

```json
"policy.log.write.redactFields": ["context.password", "context.apiKey", "context.*.token"]
```

#### 6. **Rate limiting**

```json
"policy.log.write.rateLimit": "100/minute"
```

#### 7. **Size limits (within code-enforced maximum)**

```json
"policy.log.write.maxEntrySizeKb": 10
```

#### 8. **Sensitive data patterns**

```json
"policy.log.write.sensitivePatterns": ["\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b"]
```

---

## Notes for Codegen / Shim Targets

- Log entries should be structured JSON objects.
- Timestamps may be inferred by the host if omitted.
- Redaction and policy checks should occur before log emission.
- Guests may optionally batch logs in memory for performance.

---

This spec supports auto-generation of:

- Guest shims and logger interfaces
- Host-side logging pipelines
- Policy enforcement hooks
- Observability integration (e.g., OpenTelemetry, Loki, Datadog)

