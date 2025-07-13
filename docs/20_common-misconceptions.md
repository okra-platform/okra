# Common Misconceptions About OKRA

This document addresses common misunderstandings about OKRA's architecture and design decisions. Understanding these clarifications will help both developers and AI assistants work more effectively with the platform.

---

## 🎭 Misconception: "The Actor Model is Part of the API"

### ❌ Wrong Understanding:
"I need to create actors, send messages to actors, and manage actor lifecycles."

### ✅ Reality:
The actor model (GoAKT) is completely transparent to developers. It's an internal implementation detail used for:
- Clustering and distribution
- Fault tolerance and supervision
- Automatic scaling
- Service discovery

### What You Actually Do:
```graphql
# Define services with methods - no actors!
service OrderService @okra(namespace: "shop", version: "1.0") {
  createOrder(input: OrderInput): Order
}
```

```typescript
// Call services directly - no actor messaging!
const order = await orderService.createOrder({ ... });
```

---

## 🔄 Misconception: "Services Can Emit Any Event"

### ❌ Wrong Understanding:
"Service A can emit Service B's events to trigger behavior."

### ✅ Reality:
Services can ONLY emit events they declare with `@emits`. This maintains proper encapsulation:

```graphql
service OrderService {
  # ✅ Can emit this - declared here
  @emits
  orderCreated(event: OrderCreatedEvent): void
}

service InventoryService {
  # ❌ OrderService CANNOT emit this
  @emits
  stockLow(event: StockLowEvent): void
}
```

### Why This Matters:
- Clear ownership boundaries
- No hidden dependencies
- Events are part of the service's public contract

---

## 📞 Misconception: "Callbacks Work in WASM"

### ❌ Wrong Understanding:
"I can pass callback functions to Host APIs or scheduling."

### ✅ Reality:
WASM's single-threaded model doesn't support callbacks or closures:

```typescript
// ❌ NOT POSSIBLE - No callbacks
schedule.after(1000, () => doWork());
events.subscribe("topic", (msg) => handle(msg));

// ✅ CORRECT - Reference methods by name
@scheduleOnly
scheduledWork(): void

@handles(event: "topic")
handleEvent(event: Event): void
```

### Why This Is Good:
- Predictable execution
- All code paths are statically analyzable
- Better for AI code generation
- Natural scaling (each instance is independent)

---

## 🏭 Misconception: "I Need Middleware for Cross-Cutting Concerns"

### ❌ Wrong Understanding:
"I should write middleware for auth, retry, logging, etc."

### ✅ Reality:
OKRA uses decorators to handle all cross-cutting concerns declaratively:

```graphql
# No middleware needed!
@auth(roles: ["admin"])
@durable(retries: 3, timeout: "5s")
@audit
deleteUser(id: String): void
```

### Why Decorators Are Better:
- Build-time validation
- No hidden execution order
- Self-documenting
- AI can understand without reading middleware code

---

## 📅 Misconception: "Services Can Schedule Other Services"

### ❌ Wrong Understanding:
"Service A can schedule Service B's methods to run later."

### ✅ Reality:
Services can only schedule their own methods. For cross-service orchestration, use workflows:

```typescript
// ✅ Schedule own methods
await schedule.schedule({
  cronExpression: "0 * * * *",
  method: "hourlyCleanup"  // Must be this service's method
});

// ✅ Trigger workflows for cross-service needs
await schedule.triggerWorkflow({
  workflowId: "daily.reconciliation.v1",
  cronExpression: "0 0 * * *"
});

// ❌ CANNOT schedule another service's methods
```

---

## 🔧 Misconception: "Host APIs Are Optional Plugins"

### ❌ Wrong Understanding:
"Host APIs are like npm packages I can add/remove dynamically."

### ✅ Reality:
Host APIs are capabilities injected by the runtime based on service configuration. They're not:
- Dynamically loaded plugins
- NPM packages
- Optional imports

They're core capabilities like logging, state management, and HTTP access that the runtime provides.

---

## 🎯 Misconception: "Events Are Just Message Queues"

### ❌ Wrong Understanding:
"I need to manage subscriptions, acknowledgments, and queue configuration."

### ✅ Reality:
OKRA events are declarative and type-safe:

```graphql
# No subscription code needed!
@handles(event: "order.created")
onOrderCreated(event: OrderCreatedEvent): void

# No queue configuration!
@emits(durable: true, distribution: { mode: "fanout" })
orderShipped(event: OrderShippedEvent): void
```

The runtime handles all routing, delivery, and reliability based on decorators.

---

## 📝 Misconception: "Generated Code Is for All Methods"

### ❌ Wrong Understanding:
"The generated interface includes all methods from the service IDL."

### ✅ Reality:
Different interfaces are generated for different contexts:

**For Your Service Implementation:**
- Service methods to implement
- Event handlers to implement
- Event emitter for dependency injection

**For Other Services:**
- Only callable service methods (client stub)
- Only event type definitions
- No event emission methods (encapsulation!)

---

## 🚀 Key Takeaways

1. **Actors are invisible** - You write services, not actor code
2. **Events are owned** - Services emit only their own events
3. **No callbacks in WASM** - Use method references instead
4. **Decorators over middleware** - Declarative > imperative
5. **Service boundaries enforced** - Can't schedule other services
6. **Events aren't queues** - They're typed, declarative contracts
7. **Code generation is contextual** - Different views for different needs

Understanding these distinctions helps write better OKRA services and enables AI assistants to generate correct code without common pitfalls.