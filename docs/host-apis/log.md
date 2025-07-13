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

Policies allow fine-grained control over what logs can be emitted.

### Examples:

#### 1. **Level restriction**

```json
"policy.log.write.allowedLevels": ["info", "warn", "error"]
```

#### 2. **CEL-based context filtering**

```json
"policy.log.write.condition": "entry.level != 'debug' || request.auth.claims.role == 'admin'"
```

#### 3. **Max size per message**

```json
"policy.log.write.maxEntrySizeKb": 10
```

#### 4. **Audit required fields**

```json
"policy.log.write.requiredContextKeys": ["requestId", "component"]
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

