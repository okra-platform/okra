# OKRA Decorator Reference

Decorators in OKRA provide a declarative way to express cross-cutting concerns like durability, authentication, event handling, and retry logic. This eliminates boilerplate code and enables static validation, making the system both developer-friendly and AI-native.

---

## 🎯 Overview

OKRA decorators:
- **Eliminate boilerplate** - No custom routers or switch statements
- **Enable static analysis** - Build-time validation of system consistency
- **Support AI generation** - Predictable patterns reduce hallucination
- **Provide governance** - Policy enforcement at compile and runtime

---

## 📋 Core Decorators

### `@okra` - Service Configuration
Defines namespace and version for a service.

```graphql
service Orders @okra(namespace: "shop", version: "1.0") {
  # Service methods...
}
```

### `@emits` - Event Publishing
Declares that a method publishes an event with configuration options.

```graphql
@emits(
  topic: "order.created",      # Optional, defaults to method name
  durable: true,               # Persist for guaranteed delivery
  ttl: "1h",                   # Time-to-live for undelivered events
  distribution: {
    mode: "fanout",            # fanout | queue | partitioned | loadBalanced | direct
    strategy: "roundRobin",    # roundRobin | weighted | leastLoaded | hash
    keyField: "orderId",       # For partitioned delivery
    sticky: true,              # Maintain key affinity
    delay: "10s",              # Delayed delivery
    priority: "high"           # Delivery priority hint
  }
)
orderCreated(event: OrderCreatedEvent): void
```

### `@handles` - Event Handling
Connects a handler method to an event topic with filtering and processing options.

```graphql
@handles(
  event: "shop.inventory.v1/stockLow",
  idempotent: true,                      # Safe to retry
  auth: { roles: ["inventory-manager"] }, # Authorization requirements
  batch: {                               # Batch processing config
    maxItems: 100,
    maxTime: "500ms"
  },
  filters: "currentStock < threshold",    # CEL expression filter
  durable: {                             # Retry configuration
    retries: 3,
    timeout: "5s",
    backoff: "exponential"
  }
)
onStockLow(events: [StockLowEvent]): void
```

### `@durable` - Method Durability
Controls retry behavior, timeouts, and side-effect safety for service methods.

```graphql
@durable(
  retries: 3,              # Maximum retry attempts
  timeout: "5s",           # Per-attempt timeout
  backoff: "exponential",  # Retry backoff strategy
  sideEffect: false        # If true, method won't be auto-retried
)
chargeCustomer(request: ChargeRequest): ChargeResult
```

### `@idempotent` - Idempotency Marking
Indicates a method or handler can be safely retried without duplicate effects.

```graphql
@idempotent
processPayment(request: PaymentRequest): PaymentResult

@handles(event: "order.created", idempotent: true)
createShipment(event: OrderCreatedEvent): void
```

### `@auth` - Authorization Rules
Applies authorization requirements to methods and handlers.

```graphql
# Role-based access
@auth(roles: ["admin", "manager"])
deleteOrder(id: String): void

# Claim-based access
@auth(claims: { department: "finance", level: { min: 3 } })
viewFinancialReports(): [Report]

# Policy-based access (CEL expression)
@auth(policy: "request.user.id == resource.ownerId || 'admin' in request.user.roles")
updateProfile(id: String, data: ProfileData): Profile
```

### `@filters` - Runtime Filtering
Provides CEL-based filtering for events and method calls.

```graphql
# Event filtering
@handles(
  event: "user.activity",
  filters: "event.type in ['login', 'logout'] && event.timestamp > now() - duration('24h')"
)
trackActivity(event: UserActivityEvent): void

# Method filtering
@filters("input.amount > 100 && input.currency == 'USD'")
requireApproval(input: TransactionInput): ApprovalRequest
```

### `@batch` - Batch Processing
Configures batch consumption for event handlers.

```graphql
@handles(
  event: "metrics.recorded",
  batch: {
    maxItems: 1000,        # Maximum batch size
    maxTime: "1s",         # Maximum wait time
    maxBytes: "1MB"        # Maximum batch size in bytes
  }
)
processMetrics(events: [MetricEvent]): void
```

### `@scheduleOnly` - Schedule-Only Methods
Marks methods that can only be invoked via scheduling, not direct RPC.

```graphql
@scheduleOnly
dailyCleanup(): void

@scheduleOnly
generateMonthlyReports(): ReportSummary
```

### `@deprecated` - Deprecation Marking
Indicates a method or type is deprecated with migration guidance.

```graphql
@deprecated(
  reason: "Use createOrderV2 instead",
  removeBy: "2025-01-01"
)
createOrder(input: OrderInput): Order
```

---

## 🔧 Workflow-Specific Decorators

### `@guards` - Conditional Workflow Paths
Used in workflows to gate conditional execution based on CEL expressions.

```yaml
steps:
  - name: checkInventory
    @guards(condition: "result.available > 0")
    next: processOrder
    else: backorder
```

---

## 🎨 Decorator Composition

Decorators can be combined to express complex behaviors:

```graphql
@auth(roles: ["payment-processor"])
@durable(retries: 3, timeout: "10s")
@idempotent
@filters("request.amount <= 10000")
processPayment(request: PaymentRequest): PaymentResult

@handles(
  event: "order.created",
  auth: { roles: ["fulfillment"] },
  batch: { maxItems: 50, maxTime: "2s" },
  durable: { retries: 5, backoff: "exponential" }
)
@idempotent
fulfillOrders(events: [OrderCreatedEvent]): void
```

---

## ✅ Static Validation Rules

The OKRA compiler enforces consistency rules at build time:

1. **Durability Consistency**: Durable event emitters can only be consumed by idempotent handlers
2. **Batch Type Safety**: Batch handlers must accept array types
3. **Auth Completeness**: Referenced roles/claims must be defined in the service config
4. **Filter Validation**: CEL expressions are validated against the method/event schema
5. **Schedule Constraints**: @scheduleOnly methods cannot be exposed as RPC endpoints

### Example Validation Errors:

```
❌ Error: Non-idempotent handler 'sendEmail' cannot consume durable event 'user.created'
   Fix: Add @idempotent to the handler or remove durable from the emitter

❌ Error: Batch handler 'processOrders' must accept array type, got 'OrderEvent'
   Fix: Change parameter type to '[OrderEvent]'

❌ Error: Filter expression references undefined field 'priority'
   Fix: Ensure 'priority' exists in StockLowEvent type
```

---

## 🤖 AI-Native Benefits

Decorators make OKRA exceptionally AI-friendly:

1. **Predictable Patterns**: No custom routing logic to hallucinate
2. **Self-Documenting**: Intent is explicit in decorators
3. **Validated Generation**: AI output can be statically verified
4. **Single Source of Truth**: One way to express each concern

Example: An AI asked to "make this payment processing method retry-safe with admin-only access" knows exactly which decorators to apply:

```graphql
@auth(roles: ["admin"])
@durable(retries: 3, timeout: "5s")
@idempotent
processPayment(request: PaymentRequest): PaymentResult
```

---

## 🚀 Best Practices

1. **Start Simple**: Add decorators as needed, don't over-decorate
2. **Idempotency First**: Mark handlers idempotent when possible
3. **Explicit > Implicit**: Use decorators rather than runtime checks
4. **Validate Early**: Run `okra validate` frequently during development
5. **Document Intent**: Decorators are documentation - use them clearly

---

## 🔄 Why Decorators, Not Middleware?

OKRA's decorator approach fundamentally differs from traditional middleware patterns:

### Traditional Middleware Chains:
```javascript
// Hidden execution order, runtime configuration
app.use(authMiddleware);        // When does this run?
app.use(retryMiddleware);        // Before or after auth?
app.use(loggingMiddleware);      // What if it fails?
app.use(rateLimitMiddleware);    // Can this be bypassed?

// Actual handler is buried
app.post('/order', async (req, res) => {
  // What middleware ran before this?
  // What policies are enforced?
  // What happens on error?
});
```

Problems with middleware:
- **Hidden execution order** - Must read all middleware code
- **Runtime surprises** - Middleware can be added/removed dynamically
- **Debugging nightmare** - Stack traces through multiple layers
- **AI confusion** - Which middleware to use? What order?

### OKRA Decorators:
```graphql
@auth(roles: ["customer"])
@durable(retries: 3, timeout: "10s")
@rateLimit("100/minute")
@audit
processOrder(input: OrderInput): Order
```

Benefits of decorators:
- **Execution order is explicit** - Decorators are composed predictably
- **Build-time validation** - Invalid combinations caught early
- **Self-documenting** - Everything is visible at the method
- **AI-friendly** - One way to express each concern
- **No middleware magic** - What you see is what you get

### Execution Flow Comparison:

**Middleware:** Request → Auth MW → Retry MW → RateLimit MW → Logger MW → Handler → Response

**Decorators:** Request → Runtime (applies all policies atomically) → Handler → Response

### Key Insight:
Decorators aren't just syntactic sugar - they fundamentally change how cross-cutting concerns are expressed, validated, and executed. There's no middleware chain to debug, no ordering confusion, and no hidden behavior.

---

This decorator system eliminates entire categories of bugs while making code generation and validation straightforward for both humans and AI.