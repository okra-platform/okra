# Host API: `state.*`

The `state.*` Host API provides persistent key-value storage for services, enabling stateful actors and durable service state. This API is essential for implementing the actor model's state persistence and service-level data storage.

---

## Interface

```ts
interface StateValue {
  data: any;
  version: number;
  lastModified: string; // ISO 8601
}

interface StateOptions {
  ttl?: number; // Time to live in seconds
  ifVersion?: number; // Optimistic concurrency control
  ifAbsent?: boolean; // Only set if key doesn't exist
}

interface StateHostAPI {
  get(key: string): Promise<StateValue | null>;
  set(key: string, value: any, options?: StateOptions): Promise<StateValue>;
  delete(key: string, options?: { ifVersion?: number }): Promise<void>;
  list(prefix: string, options?: { limit?: number; cursor?: string }): Promise<{
    keys: string[];
    cursor?: string;
  }>;
  increment(key: string, delta: number): Promise<number>;
}
```

---

## Host API Configuration

State storage can be configured per deployment with different backends and isolation levels.

```ts
interface StateHostAPIConfig {
  backend: 'memory' | 'redis' | 'dynamodb' | 'postgres';
  keyPrefix?: string; // Global prefix for all keys
  defaultTTL?: number; // Default TTL in seconds
  maxKeyLength?: number;
  maxValueSizeKb?: number;
  enableVersioning?: boolean;
}
```

---

## Enforceable Okra Policies

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **Key validation** - Only alphanumeric, underscore, dash, slash, colon allowed
2. **Maximum key length** - Prevent excessive memory usage (default: 512 characters)
3. **Maximum value size** - Prevent memory exhaustion (default: 1MB)
4. **Key path traversal protection** - Prevent ../ or absolute paths
5. **Value serialization safety** - Ensure values can be safely serialized/deserialized
6. **Version number bounds** - Prevent integer overflow in version tracking
7. **TTL bounds** - Minimum 1 second, maximum 1 year

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### 1. **Key namespace restrictions**

```json
"policy.state.keyPrefix": "${service.namespace}:${service.name}:",
"policy.state.allowedPrefixes": ["user:", "session:", "cache:"]
```

#### 2. **Operation restrictions**

```json
"policy.state.allowedOperations": ["get", "set", "delete"],
"policy.state.readOnly": false
```

#### 3. **Conditional access based on key pattern**

```json
"policy.state.condition": "request.key.startsWith('public:') || request.auth.claims.role == 'admin'"
```

#### 4. **TTL requirements**

```json
"policy.state.minTTL": 60,
"policy.state.maxTTL": 86400,
"policy.state.requireTTL": true
```

#### 5. **Size limits (within code maximum)**

```json
"policy.state.maxValueSizeKb": 100,
"policy.state.maxKeysPerService": 10000
```

#### 6. **Rate limiting**

```json
"policy.state.rateLimit": "1000/minute",
"policy.state.burstLimit": 100
```

#### 7. **Audit logging**

```json
"policy.state.audit": true,
"policy.state.auditOperations": ["set", "delete"],
"policy.state.auditKeyPatterns": ["user:*", "admin:*"]
```

#### 8. **Environment-specific policies**

```json
"policy.state.condition": "env.DEPLOYMENT_ENV == 'production' ? !request.key.startsWith('debug:') : true"
```

#### 9. **Data classification**

```json
"policy.state.sensitiveKeyPatterns": ["session:*", "token:*"],
"policy.state.encryptionRequired": ["payment:*", "pii:*"]
```

#### 10. **Garbage collection**

```json
"policy.state.autoExpireAfter": "90d",
"policy.state.warnOnStaleData": "30d"
```

---

## Notes for Codegen / Shim Targets

- State operations should be atomic and consistent
- Implement optimistic concurrency control via version numbers
- Support both JSON and binary value serialization
- Provide helper methods for common patterns (counters, sets, maps)
- Cache frequently accessed values with proper invalidation
- Handle backend-specific errors gracefully
- Support batch operations for efficiency

---

This specification enables:

- Actor state persistence
- Session management
- Distributed locks and coordination
- Feature flag storage
- Configuration management
- Caching with TTL support