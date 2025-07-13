# Host API: `queue.*`

The `queue.*` Host API provides message queue operations for async processing and event-driven architectures. This API enables services to publish messages, subscribe to topics, and implement reliable message processing patterns with dead letter queue support.

---

## Interface

```ts
interface QueueMessage {
  id: string;
  topic: string;
  payload: any;
  headers?: Record<string, string>;
  timestamp: string; // ISO 8601
  attempts?: number;
  maxRetries?: number;
}

interface PublishOptions {
  delay?: number; // Delay in seconds before message is available
  priority?: 'low' | 'normal' | 'high';
  ttl?: number; // Time to live in seconds
  deduplicationId?: string; // For exactly-once delivery
  headers?: Record<string, string>;
}

interface SubscribeOptions {
  maxConcurrency?: number; // Max messages processed concurrently
  visibilityTimeout?: number; // Seconds before message reappears if not acked
  maxRetries?: number; // Before sending to DLQ
  deadLetterTopic?: string; // DLQ topic name
}

interface QueueHostAPI {
  publish(topic: string, message: any, options?: PublishOptions): Promise<string>; // Returns message ID
  subscribe(topic: string, handler: (message: QueueMessage) => Promise<void>, options?: SubscribeOptions): Promise<() => void>; // Returns unsubscribe function
  ack(messageId: string): Promise<void>;
  nack(messageId: string, options?: { requeue?: boolean; delay?: number }): Promise<void>;
  listTopics(prefix?: string): Promise<string[]>;
  getQueueDepth(topic: string): Promise<number>;
}
```

---

## Host API Configuration

Queue systems can be configured with different backends and delivery guarantees.

```ts
interface QueueHostAPIConfig {
  backend: 'memory' | 'redis' | 'rabbitmq' | 'sqs' | 'kafka';
  topicPrefix?: string; // Global prefix for all topics
  defaultVisibilityTimeout?: number; // Default timeout in seconds
  defaultMaxRetries?: number;
  enableDeduplication?: boolean;
  deliveryGuarantee?: 'at-most-once' | 'at-least-once' | 'exactly-once';
}
```

---

## Enforceable Okra Policies

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **Topic name validation** - Only alphanumeric, underscore, dash, dot allowed
2. **Maximum topic length** - Prevent excessive memory usage (default: 256 characters)
3. **Maximum message size** - Prevent memory exhaustion (default: 256KB)
4. **Maximum header count** - Limit number of headers (default: 50)
5. **Maximum header size** - Each header value limited (default: 1KB)
6. **Valid delay bounds** - Minimum 0, maximum 15 minutes
7. **Valid TTL bounds** - Minimum 1 second, maximum 14 days
8. **Maximum retry attempts** - Prevent infinite loops (default: 10)

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### 1. **Topic namespace restrictions**

```json
"policy.queue.topicPrefix": "${service.namespace}:${service.name}:",
"policy.queue.allowedTopics": ["orders.*", "notifications.*", "events.*"]
```

#### 2. **Operation restrictions**

```json
"policy.queue.allowedOperations": ["publish", "subscribe"],
"policy.queue.publishOnly": false,
"policy.queue.subscribeOnly": false
```

#### 3. **Conditional access based on topic pattern**

```json
"policy.queue.condition": "request.topic.startsWith('public.') || request.auth.claims.role in ['admin', 'service']"
```

#### 4. **Message size limits (within code maximum)**

```json
"policy.queue.maxMessageSizeKb": 128,
"policy.queue.maxBatchSize": 100
```

#### 5. **Rate limiting**

```json
"policy.queue.publishRateLimit": "1000/minute",
"policy.queue.subscribeRateLimit": "10/minute",
"policy.queue.burstLimit": 100
```

#### 6. **Dead letter queue policies**

```json
"policy.queue.requireDLQ": true,
"policy.queue.dlqPrefix": "dlq.",
"policy.queue.maxRetriesBeforeDLQ": 3
```

#### 7. **Audit logging**

```json
"policy.queue.audit": true,
"policy.queue.auditOperations": ["publish", "nack"],
"policy.queue.auditTopics": ["payments.*", "admin.*"]
```

#### 8. **Environment-specific rules**

```json
"policy.queue.condition": "env.DEPLOYMENT_ENV == 'production' ? !request.topic.startsWith('test.') : true"
```

#### 9. **Priority restrictions**

```json
"policy.queue.allowedPriorities": ["normal"],
"policy.queue.highPriorityCondition": "request.auth.claims.role == 'premium'"
```

#### 10. **Deduplication requirements**

```json
"policy.queue.requireDeduplicationId": ["payments.*", "orders.*"],
"policy.queue.deduplicationWindow": 300
```

#### 11. **Subscription limits**

```json
"policy.queue.maxSubscriptionsPerTopic": 10,
"policy.queue.maxConcurrentMessages": 50,
"policy.queue.maxTopicsPerService": 20
```

#### 12. **Message retention**

```json
"policy.queue.defaultTTL": 86400,
"policy.queue.maxTTL": 604800,
"policy.queue.minVisibilityTimeout": 30
```

---

## Notes for Codegen / Shim Targets

- Messages should be serialized as JSON by default
- Support both synchronous ack/nack and automatic acknowledgment modes
- Implement connection pooling and circuit breakers for reliability
- Provide typed message schemas with validation
- Handle backend-specific errors and retry logic
- Support message batching for improved throughput
- Implement proper cleanup and graceful shutdown
- Provide metrics on queue depth, processing rates, and errors

---

This specification enables:

- Event-driven microservice communication
- Background job processing
- Workflow orchestration
- Real-time notifications
- Distributed transaction sagas
- Load leveling and buffering
- Fan-out messaging patterns
- Priority-based task scheduling