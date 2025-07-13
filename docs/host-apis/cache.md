# Host API: `cache.*`

The `cache.*` Host API provides distributed caching operations for performance optimization. This API enables services to store and retrieve frequently accessed data with configurable eviction policies and TTL support.

---

## Interface

```ts
interface CacheValue {
  data: any;
  ttl?: number; // Remaining TTL in seconds
  created: string; // ISO 8601
  accessed: string; // ISO 8601 - Last access time
  hits: number; // Access count
}

interface CacheOptions {
  ttl?: number; // Time to live in seconds
  tags?: string[]; // Cache tags for group invalidation
  priority?: 'low' | 'normal' | 'high'; // Eviction priority
}

interface CacheBatchOperation {
  key: string;
  value?: any;
  options?: CacheOptions;
}

interface CacheHostAPI {
  get(key: string): Promise<CacheValue | null>;
  set(key: string, value: any, options?: CacheOptions): Promise<void>;
  delete(key: string): Promise<void>;
  invalidate(pattern: string): Promise<number>; // Returns count of invalidated entries
  getMany(keys: string[]): Promise<Map<string, CacheValue>>;
  setMany(operations: CacheBatchOperation[]): Promise<void>;
}
```

---

## Host API Configuration

Cache backend and behavior can be configured per deployment for different performance characteristics.

```ts
interface CacheHostAPIConfig {
  backend: 'memory' | 'redis' | 'memcached' | 'hazelcast';
  maxSizeGb?: number; // Total cache size limit
  evictionPolicy?: 'lru' | 'lfu' | 'fifo' | 'ttl';
  defaultTTL?: number; // Default TTL in seconds
  maxKeyLength?: number;
  maxValueSizeKb?: number;
  compressionThreshold?: number; // Compress values larger than this (KB)
  warmupKeys?: string[]; // Keys to preload on startup
}
```

---

## Enforceable Okra Policies

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **Key validation** - Only alphanumeric, underscore, dash, colon, slash allowed
2. **Maximum key length** - Prevent excessive memory usage (default: 256 characters)
3. **Maximum value size** - Prevent memory exhaustion (default: 10MB)
4. **TTL bounds** - Minimum 1 second, maximum 30 days
5. **Tag validation** - Maximum 10 tags per entry, 64 characters each
6. **Pattern safety** - Invalidation patterns must be safe (no * or **)
7. **Batch operation limits** - Maximum 1000 keys per batch operation

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### 1. **Key namespace enforcement**

```json
"policy.cache.keyPrefix": "${service.namespace}:${service.name}:",
"policy.cache.allowedNamespaces": ["app:", "session:", "api:"]
```

#### 2. **Value size limits (within code maximum)**

```json
"policy.cache.maxValueSizeKb": 1024,
"policy.cache.compressionRequired": "value.size > 100 * 1024"
```

#### 3. **TTL requirements**

```json
"policy.cache.minTTL": 10,
"policy.cache.maxTTL": 86400,
"policy.cache.defaultTTL": 3600
```

#### 4. **Eviction policy overrides**

```json
"policy.cache.priorityKeys": ["session:*", "auth:*"],
"policy.cache.lowPriorityPatterns": ["temp:*", "preview:*"]
```

#### 5. **Hit ratio monitoring**

```json
"policy.cache.minHitRatio": 0.7,
"policy.cache.alertOnLowHitRatio": true,
"policy.cache.hitRatioWindow": "5m"
```

#### 6. **Invalidation restrictions**

```json
"policy.cache.allowedInvalidationPatterns": ["user:*", "product:*"],
"policy.cache.maxInvalidationScope": 1000,
"policy.cache.requireInvalidationAuth": true
```

#### 7. **Environment-specific rules**

```json
"policy.cache.condition": "env.DEPLOYMENT_ENV == 'production' ? request.options.ttl >= 60 : true",
"policy.cache.disableInEnvironments": ["test", "ci"]
```

#### 8. **Rate limiting**

```json
"policy.cache.rateLimit": "10000/minute",
"policy.cache.burstLimit": 1000,
"policy.cache.rateLimitByOperation": {
  "get": "50000/minute",
  "set": "5000/minute",
  "invalidate": "10/minute"
}
```

#### 9. **Memory pressure handling**

```json
"policy.cache.maxMemoryPercent": 80,
"policy.cache.evictOnMemoryPressure": true,
"policy.cache.emergencyEvictionThreshold": 90
```

#### 10. **Tag-based policies**

```json
"policy.cache.requiredTags": ["version", "tenant"],
"policy.cache.maxTagsPerEntry": 5,
"policy.cache.tagNamePattern": "^[a-z0-9-]+$"
```

#### 11. **Batch operation policies**

```json
"policy.cache.maxBatchSize": 100,
"policy.cache.batchTimeout": "5s",
"policy.cache.allowBatchOperations": ["getMany"]
```

#### 12. **Data classification**

```json
"policy.cache.sensitiveKeyPatterns": ["token:*", "key:*"],
"policy.cache.prohibitedData": ["pii:*", "credit_card:*"]
```

---

## Notes for Codegen / Shim Targets

- Cache operations should be non-blocking and fast
- Implement automatic retry with exponential backoff for transient failures
- Support both JSON and binary value serialization
- Provide cache-aside pattern helpers
- Handle cache stampede prevention (e.g., probabilistic early expiration)
- Support cache warming and preloading
- Implement metrics collection (hit/miss ratio, latency)
- Gracefully degrade when cache is unavailable

---

This specification enables:

- Response caching for API endpoints
- Session data caching
- Computed value memoization
- Database query result caching
- Rate limiting state storage
- Feature flag caching
- Static asset caching