# Host API: `metrics.*`

The `metrics.*` Host API provides observability operations for monitoring service health, performance, and business metrics. Metrics can be exported to various backends including Prometheus, StatsD, DataDog, or cloud-native monitoring solutions.

---

## Interface

```ts
interface MetricTags {
  [key: string]: string | number | boolean;
}

interface MetricOptions {
  tags?: MetricTags;
  timestamp?: string; // ISO 8601, optional (host will add if missing)
  unit?: 'seconds' | 'milliseconds' | 'bytes' | 'requests' | 'errors' | 'percent' | 'custom';
}

interface HistogramOptions extends MetricOptions {
  buckets?: number[]; // Custom bucket boundaries
}

interface TimerContext {
  stop(): number; // Returns elapsed time and records metric
  cancel(): void; // Cancel without recording
}

interface MetricsHostAPI {
  counter(name: string, value: number, options?: MetricOptions): void;
  gauge(name: string, value: number, options?: MetricOptions): void;
  histogram(name: string, value: number, options?: HistogramOptions): void;
  timer(name: string, options?: MetricOptions): TimerContext;
  summary(name: string, value: number, options?: MetricOptions): void;
}
```

---

## Host API Configuration

Metrics collection and export can be configured per deployment with different backends and aggregation rules.

```ts
interface MetricsHostAPIConfig {
  backend: 'prometheus' | 'statsd' | 'datadog' | 'otlp' | 'cloudwatch';
  endpoint?: string;
  pushInterval?: number; // Seconds between metric pushes
  metricPrefix?: string; // Global prefix for all metrics
  defaultTags?: MetricTags; // Tags applied to all metrics
  aggregationWindow?: number; // Seconds for local aggregation
  enableDistributions?: boolean; // Enable percentile calculations
}
```

---

## Enforceable Okra Policies

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **Metric name validation** - Only alphanumeric, underscore, dot allowed (Prometheus compatible)
2. **Maximum metric name length** - Prevent excessive memory usage (default: 256 characters)
3. **Tag cardinality protection** - Limit unique tag combinations (default: 10,000 per metric)
4. **Tag key/value validation** - Sanitize tag names and values
5. **Value bounds checking** - Prevent NaN, Infinity, or extreme values
6. **Maximum tags per metric** - Limit number of tags (default: 20)
7. **Histogram bucket validation** - Ensure monotonic increasing bucket boundaries
8. **Rate limiting per metric** - Prevent metric bombing (default: 1000/second per metric)

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### 1. **Metric namespace restrictions**

```json
"policy.metrics.allowedPrefixes": ["app.", "business.", "system."],
"policy.metrics.requiredPrefix": "${service.namespace}.${service.name}."
```

#### 2. **Tag requirements and restrictions**

```json
"policy.metrics.requiredTags": ["environment", "service"],
"policy.metrics.forbiddenTags": ["password", "api_key", "token"],
"policy.metrics.maxTagLength": 128
```

#### 3. **Conditional metric collection**

```json
"policy.metrics.condition": "env.DEPLOYMENT_ENV != 'development' || metric.name.startsWith('debug.')"
```

#### 4. **Value range validation**

```json
"policy.metrics.counter.minValue": 0,
"policy.metrics.counter.maxValue": 1000000,
"policy.metrics.gauge.allowNegative": true,
"policy.metrics.histogram.maxValue": 3600
```

#### 5. **Cardinality limits (within code maximum)**

```json
"policy.metrics.maxUniqueTimeseries": 5000,
"policy.metrics.maxTagCardinality": {
  "user_id": 10000,
  "endpoint": 100,
  "status_code": 10
}
```

#### 6. **Rate limiting and sampling**

```json
"policy.metrics.rateLimit": "10000/minute",
"policy.metrics.sampling": {
  "debug.*": 0.1,
  "trace.*": 0.01,
  "error.*": 1.0
}
```

#### 7. **Aggregation rules**

```json
"policy.metrics.aggregation": {
  "request.duration": ["p50", "p95", "p99", "mean"],
  "memory.usage": ["max", "mean"],
  "error.count": ["sum"]
}
```

#### 8. **Environment-specific policies**

```json
"policy.metrics.condition": "env.DEPLOYMENT_ENV == 'production' ? !metric.name.startsWith('debug.') : true",
"policy.metrics.enabledEnvironments": ["staging", "production"]
```

#### 9. **Business metric validation**

```json
"policy.metrics.businessMetrics": {
  "revenue.*": {
    "requiredTags": ["currency", "product"],
    "minValue": 0,
    "maxValue": 1000000
  },
  "user.signup": {
    "requiredTags": ["source", "plan"],
    "allowedValues": [0, 1]
  }
}
```

#### 10. **Cost control**

```json
"policy.metrics.maxMetricsPerMinute": 100000,
"policy.metrics.maxUniqueMetricNames": 1000,
"policy.metrics.dropOnQuotaExceeded": false,
"policy.metrics.alertOnHighCardinality": true
```

#### 11. **Histogram bucket policies**

```json
"policy.metrics.histogram.defaultBuckets": [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10],
"policy.metrics.histogram.maxBuckets": 20,
"policy.metrics.histogram.requireExplicitBuckets": false
```

#### 12. **Metric naming conventions**

```json
"policy.metrics.namingPattern": "^[a-z][a-z0-9_]*(\\.[a-z][a-z0-9_]*)*$",
"policy.metrics.unitSuffixes": {
  "seconds": "_seconds",
  "bytes": "_bytes",
  "percent": "_ratio"
}
```

---

## Notes for Codegen / Shim Targets

- Metrics should be fire-and-forget with minimal latency impact
- Support local aggregation before export to reduce network overhead
- Implement automatic retry with exponential backoff for failed exports
- Provide convenience methods for common patterns (HTTP metrics, DB metrics)
- Support batch metric submission for efficiency
- Handle backend-specific formats (Prometheus exposition, StatsD protocols)
- Implement proper metric type semantics (counters only increase, gauges can vary)
- Timer contexts should be exception-safe with automatic cleanup

---

This specification enables:

- Application performance monitoring (APM)
- Business KPI tracking
- SLA/SLO measurement
- Capacity planning metrics
- Error rate tracking
- Custom dashboards and alerting
- Distributed tracing integration
- Cost attribution and monitoring