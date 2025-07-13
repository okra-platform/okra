# Host API: `time.*`

The `time.*` Host API provides deterministic time operations for guest services. This API enables services to work with time in a controlled, testable manner for time-based logic and delays.

---

## Interface

```ts
interface TimeInfo {
  timestamp: number; // Unix timestamp in milliseconds
  timezone: string; // IANA timezone identifier (e.g., "America/New_York")
  iso8601: string; // ISO 8601 formatted string
}

interface TimeHostAPI {
  now(): TimeInfo;
  sleep(milliseconds: number): Promise<void>;
}
```

---

## Host API Configuration

Time operations can be configured to support deterministic testing and environment-specific behaviors.

```ts
interface TimeHostAPIConfig {
  maxSleepDurationMs?: number; // Maximum sleep duration allowed
  deterministicTime?: string; // Fixed time for testing (ISO 8601)
  timeDriftTolerance?: number; // Maximum allowed drift in milliseconds
  defaultTimezone?: string; // Default timezone if not specified
}
```

---

## Enforceable Okra Policies

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **Sleep duration bounds** - Minimum 1ms, maximum 24 hours
2. **Timezone validation** - Must be valid IANA timezone identifier
3. **Time precision** - Millisecond granularity for timestamps

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### 1. **Sleep duration limits**

```json
"policy.time.maxSleepMs": 30000,
"policy.time.minSleepMs": 10,
"policy.time.condition": "request.milliseconds <= 60000 || request.auth.claims.role == 'admin'"
```

#### 2. **Environment-specific policies**

```json
"policy.time.condition": "env.DEPLOYMENT_ENV == 'production' ? request.milliseconds <= 10000 : true"
```

#### 3. **Rate limiting**

```json
"policy.time.now.rateLimit": "1000/minute",
"policy.time.sleep.rateLimit": "100/minute"
```

#### 4. **Deterministic time for testing**

```json
"policy.time.deterministicMode": true,
"policy.time.fixedTime": "2024-01-01T00:00:00Z",
"policy.time.timeAdvanceRateMs": 1000
```

#### 5. **Timezone restrictions**

```json
"policy.time.allowedTimezones": ["UTC", "America/New_York", "Europe/London"],
"policy.time.defaultTimezone": "UTC"
```

#### 6. **Audit and monitoring**

```json
"policy.time.audit": true,
"policy.time.auditOperations": ["sleep"],
"policy.time.warnOnLongSleep": 5000
```

---

## Notes for Codegen / Shim Targets

- Time operations should be deterministic when configured for testing
- Sleep operations should be interruptible on service shutdown
- Support both UTC and timezone-aware operations
- Provide helper functions for common time calculations (date math, formatting)
- Handle clock drift and time synchronization gracefully
- Consider providing utilities for common patterns (exponential backoff, jitter)

---

This specification enables:

- Deterministic testing with controlled time
- Time-based delays and pauses
- Rate limiting and throttling implementation
- Time-sensitive business logic
- Retry logic with backoff
- Time-based access controls
- Synchronous waiting between operations