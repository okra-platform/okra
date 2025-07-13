# OKRA Events: Type-Safe Event-Driven Architecture

Events in OKRA are a first-class runtime feature that enable decoupled, event-driven communication between services while maintaining strong typing and schema validation.

---

## 🎯 Overview

OKRA events provide:
- **Type-safe event emission** through generated interfaces
- **Declarative event handling** via IDL decorators
- **Automatic routing** based on service declarations
- **Schema validation** for all event payloads
- **Policy enforcement** at the runtime level

Unlike traditional message queues, OKRA events are:
- Part of the service contract (defined in IDL)
- Strongly typed with compile-time safety
- Automatically bound through declarations
- Fire-and-forget by default (async delivery)

---

## 🔑 Key Principles

Understanding these principles is crucial for working with OKRA events:

### 1. **Event Ownership**
- Services can **ONLY** emit events they declare with `@emits`
- A service cannot emit another service's events
- Events are part of a service's public API contract

### 2. **Event Consumption**
- Services can handle **ANY** event via `@handles` decorator
- No runtime subscription needed - it's all declarative
- Handlers are just methods that receive typed event data

### 3. **No Cross-Service Emission**
- Service A **cannot** emit Service B's events
- This maintains proper encapsulation
- Need to trigger another service? Use service calls or workflows

### Example:
```graphql
service OrderService @okra(namespace: "shop", version: "1.0") {
  # ✅ Can emit this - it's declared here
  @emits(topic: "order.created")
  orderCreated(event: OrderCreatedEvent): void
  
  # ✅ Can handle this - any service can handle any event
  @handles(event: "inventory.stockLow")
  onStockLow(event: StockLowEvent): void
}

service InventoryService @okra(namespace: "shop", version: "1.0") {
  # ❌ OrderService CANNOT emit this event
  @emits(topic: "stock.low")
  stockLow(event: StockLowEvent): void
}
```

This design ensures:
- Clear ownership boundaries
- No hidden dependencies
- Predictable event flow
- Easy impact analysis

---

## 📝 Defining Events in IDL

Services declare events using `@emits` and `@handles` decorators with rich configuration options:

```graphql
service Orders @okra(namespace: "shop", version: "1.0") {
  # Regular service methods
  createOrder(input: OrderInput): Order
  cancelOrder(id: String): Order
  
  # Event emitters with configuration
  @emits(
    topic: "user.created",
    durable: true,
    ttl: "1h",
    distribution: {
      mode: "fanout",  # or: queue | partitioned | loadBalanced | direct
      strategy: "roundRobin",  # or: weighted | leastLoaded | hash
      keyField: "userId",  # for partitioned delivery
      sticky: true,  # affinity per key
      delay: "10s",  # delayed delivery
      priority: "high"  # delivery priority hint
    }
  )
  orderCreated(event: OrderCreatedEvent): void
  
  @emits(topic: "order.cancelled", durable: true)
  orderCancelled(event: OrderCancelledEvent): void
  
  # Event handlers with configuration
  @handles(
    event: "shop.inventory.v1/stockLow",
    idempotent: true,
    auth: { roles: ["inventory-manager"] },
    batch: { maxItems: 100, maxTime: "500ms" },
    filters: "currentStock < threshold"
  )
  onStockLow(events: [StockLowEvent]): void
  
  @handles(
    event: "shop.payments.v1/paymentFailed",
    durable: { retries: 3, timeout: "5s" }
  )
  onPaymentFailed(event: PaymentFailedEvent): void
}

# Event type definitions
type OrderCreatedEvent {
  orderId: String!
  customerId: String!
  items: [OrderItem!]!
  total: Float!
  timestamp: String!
}

type OrderCancelledEvent {
  orderId: String!
  reason: String!
  refundAmount: Float
}
```

---

## 🔧 Using Events in Service Code

### Emitting Events

The OKRA compiler generates a typed event emitter for each service:

```typescript
// Generated interface for Orders service
interface OrdersEventEmitter {
  emitOrderCreated(event: OrderCreatedEvent): Promise<void>;
  emitOrderCancelled(event: OrderCancelledEvent): Promise<void>;
}

// Service implementation with injected event emitter
class OrdersService implements OrdersServiceImpl {
  constructor(
    private events: OrdersEventEmitter,     // Injected by runtime
    private inventory: InventoryService,    // Remote service stub
  ) {}
  
  async createOrder(input: OrderInput): Promise<Order> {
    // Business logic...
    const order = await this.processOrder(input);
    
    // Emit event with full type safety
    await this.events.emitOrderCreated({
      orderId: order.id,
      customerId: order.customerId,
      items: order.items,
      total: order.total,
      timestamp: new Date().toISOString()
    });
    
    return order;
  }
}
```

### Handling Events

Event handlers are regular service methods decorated with `@handles`:

```typescript
// Implementation of event handler
class OrdersService implements OrdersServiceImpl {
  // Batch handler - receives array when batch is configured
  async onStockLow(events: StockLowEvent[]): Promise<void> {
    // Batch processing for efficiency
    const productIds = events.map(e => e.productId);
    await this.pauseOrdersForProducts(productIds);
  }
  
  // Single event handler with retry semantics
  async onPaymentFailed(event: PaymentFailedEvent): Promise<void> {
    // This handler will be retried 3 times with 5s timeout
    // as configured in the @handles decorator
    await this.cancelOrder(event.orderId);
  }
}
```

---

## 🚀 Event Flow

1. **Service A** calls `this.events.emitOrderCreated(payload)`
2. **Runtime** validates the payload against the schema
3. **Runtime** applies distribution strategy (fanout, queue, partitioned)
4. **Runtime** constructs the full event type: `shop.orders.v1/orderCreated`
5. **Runtime** finds all services with `@handles(event: "shop.orders.v1/orderCreated")`
6. **Runtime** applies filters, batching, and delivery configuration
7. **Handlers** process events according to their configuration

---

## 🎛️ Decorator Configuration Options

### `@emits` Configuration

| Option | Description | Default |
|--------|-------------|---------|
| `topic` | Event topic name (optional, defaults to method name) | Method name |
| `durable` | Persist events for reliable delivery | `false` |
| `ttl` | Time-to-live for undelivered events | `24h` |
| `distribution.mode` | How events are distributed | `fanout` |
| `distribution.strategy` | Load balancing strategy | `roundRobin` |
| `distribution.keyField` | Field for partitioned delivery | - |
| `distribution.sticky` | Maintain affinity per key | `false` |
| `distribution.delay` | Delay before delivery | - |
| `distribution.priority` | Delivery priority hint | `normal` |

#### Distribution Modes:
- **fanout**: All handlers receive the event
- **queue**: Only one handler receives each event
- **partitioned**: Events partitioned by key field
- **loadBalanced**: Distributed based on handler load
- **direct**: Point-to-point delivery

### `@handles` Configuration

| Option | Description | Default |
|--------|-------------|---------|
| `event` | Fully qualified event to handle | Required |
| `idempotent` | Safe to retry without side effects | `false` |
| `auth` | Authorization requirements | - |
| `batch` | Batch processing configuration | - |
| `filters` | CEL expression to filter events | - |
| `durable` | Retry configuration | - |

---

## 📦 Code Generation

The OKRA compiler generates:

### For Event Emitters (own service):
```typescript
// Generated event emitter implementation
class OrdersEventEmitterImpl implements OrdersEventEmitter {
  async emitOrderCreated(event: OrderCreatedEvent): Promise<void> {
    // Runtime handles validation, routing, and policy enforcement
    return __okraRuntime.emit('orderCreated', event, {
      service: 'shop.orders.v1',
      schema: OrderCreatedEventSchema
    });
  }
}
```

### For Event Types (from other services):
```typescript
// Only the type is generated for external events
export interface StockLowEvent {
  productId: string;
  currentStock: number;
  threshold: number;
  timestamp: string;
}
```

---

## 🛡️ Policy Enforcement

Event policies are enforced at the runtime level, not through a host API:

```json
{
  "policy.events.emit.rateLimit": "1000/minute",
  "policy.events.emit.allowedEvents": ["orderCreated", "orderUpdated"],
  "policy.events.emit.blockedEvents": ["debugEvent", "testEvent"],
  "policy.events.emit.condition": "env.DEPLOYMENT_ENV == 'production' ? !event.name.startsWith('debug') : true",
  
  "policy.events.handle.maxConcurrent": 100,
  "policy.events.handle.timeout": 30000,
  "policy.events.handle.retryPolicy": {
    "maxAttempts": 3,
    "backoffMs": 1000
  }
}
```

Policies can control:
- Which events a service can emit
- Rate limiting per event type
- Cross-namespace event restrictions
- Handler concurrency and timeouts
- Retry behavior for failed handlers
- Dead letter queue configuration

---

## 🔄 Event Delivery Guarantees

OKRA events support configurable delivery semantics:

### Durability Levels:
- **Non-durable** (default): Best-effort, fire-and-forget delivery
- **Durable**: Persistent storage with guaranteed delivery and retries

### Distribution Modes:
- **Fanout**: All handlers receive every event
- **Queue**: Each event delivered to exactly one handler
- **Partitioned**: Events distributed by key for ordering
- **LoadBalanced**: Dynamic distribution based on handler capacity
- **Direct**: Point-to-point delivery to specific handler

### Processing Guarantees:
- **At-least-once**: Default for durable events
- **At-most-once**: For non-durable events
- **Ordered delivery**: Per partition key or handler
- **Batch processing**: Configured via `@handles(batch: {...})`

### Idempotency:
Handlers marked with `idempotent: true` can be safely retried by the runtime without concerns about duplicate side effects.

---

## 🎯 Best Practices

1. **Event Naming**: Use past tense for events (orderCreated, not createOrder)
2. **Event Payload**: Include all necessary data - handlers shouldn't need to query
3. **Versioning**: Include version in event types when schemas change
4. **Idempotency**: Design handlers to be safely retryable
5. **Event Size**: Keep payloads small - reference IDs rather than full objects
6. **Error Handling**: Handlers should not throw for business logic errors

---

## 🚫 What Events Are NOT For

- **Request-Response**: Use service methods for synchronous operations
- **Large Data Transfer**: Events should be small - use blob storage for large payloads
- **Ordered Processing**: Use workflows for complex ordered operations
- **Transactions**: Events are eventually consistent, not transactional

---

## 🔍 Observability

The runtime provides comprehensive event observability:
- Event emission metrics
- Handler success/failure rates
- Event flow tracing
- Dead letter queue monitoring
- Event replay capabilities (with proper permissions)

---

## 📊 Comparison with Traditional Approaches

| Feature | OKRA Events | Message Queues | Pub/Sub |
|---------|-------------|----------------|---------|
| Type Safety | ✅ Compile-time | ❌ Runtime only | ❌ Runtime only |
| Schema Evolution | ✅ Versioned | ⚠️ Manual | ⚠️ Manual |
| Service Discovery | ✅ Automatic | ❌ Manual | ❌ Manual |
| Configuration | ✅ IDL-based | ❌ External | ❌ External |
| IDE Support | ✅ Full | ❌ Limited | ❌ Limited |

---

This event system enables building truly decoupled, event-driven architectures while maintaining the type safety and developer experience that OKRA provides throughout the platform.