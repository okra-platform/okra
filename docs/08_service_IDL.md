# Service IDL
Services are defined using an IDL that is "GraphQL-ish".

It reuses the `type` and `enum` declaration and adds a `service` declaration with support for comprehensive decorators that express cross-cutting concerns like authentication, durability, event handling, and more.

At build time `okra` cli parses the IDL. While running `okra dev` it uses the IDL info to generate the interface / struct file. `okra build` generates a `service.discovery.json` file with a serialized description of the service.

## Core Directives

### `@okra` - Service Configuration
Top-level directive that specifies namespace and version:

```graphql
service UserService @okra(namespace: "auth.users", version: "v1") {
  # Service methods...
}
```

### Method Decorators

Services can use decorators to declare behavior without writing boilerplate code:

```graphql
service OrderService @okra(namespace: "shop", version: "1.0") {
  # Durable method with retry semantics
  @durable(retries: 3, timeout: "5s", backoff: "exponential")
  @idempotent
  processPayment(request: PaymentRequest): PaymentResult
  
  # Method with authentication
  @auth(roles: ["admin", "manager"])
  cancelOrder(id: String): Order
  
  # Event emission
  @emits(
    topic: "order.created",
    durable: true,
    distribution: { mode: "fanout" }
  )
  orderCreated(event: OrderCreatedEvent): void
  
  # Event handling with batching
  @handles(
    event: "inventory.stockLow",
    batch: { maxItems: 100, maxTime: "500ms" },
    idempotent: true
  )
  onStockLow(events: [StockLowEvent]): void
  
  # Schedule-only method
  @scheduleOnly
  dailyReport(): ReportSummary
}
```

See the [Decorator Reference](./18_decorators.md) for complete documentation of all available decorators.

## Supported Scalar Types
| Scalar     | Description                     | Format / Standard                | Go Type   | JSON Type            |
| ---------- | ------------------------------- | -------------------------------- | --------- | -------------------- |
| `Int`      | 32-bit signed integer           | —                                | `int32`   | `number`             |
| `Float`    | 64-bit floating point number    | —                                | `float64` | `number`             |
| `String`   | UTF-8 encoded string            | —                                | `string`  | `string`             |
| `Boolean`  | Boolean true/false              | —                                | `bool`    | `boolean`            |
| `ID`       | Unique identifier               | —                                | `string`  | `string`             |
| `Date`     | Calendar date                   | ISO 8601 (`YYYY-MM-DD`)          | `string`  | `string`             |
| `Time`     | Time of day                     | ISO 8601 (`HH:MM[:SS]`)          | `string`  | `string`             |
| `DateTime` | Full timestamp with timezone    | RFC 3339 / ISO 8601              | `string`  | `string`             |
| `Duration` | Elapsed time duration           | ISO 8601 Duration (`PnDTnHnMnS`) | `string`  | `string`             |
| `UUID`     | Universally unique identifier   | UUID v4                          | `string`  | `string`             |
| `URL`      | Web or network resource locator | URI (RFC 3986)                   | `string`  | `string`             |
| `BigInt`   | Arbitrary-precision integer     | —                                | `string`  | `string` or `number` |
| `Decimal`  | Arbitrary-precision decimal     | —                                | `string`  | `string`             |
| `Bytes`    | Binary blob                     | Base64-encoded                   | `[]byte`  | `string`             |
| `Currency` | ISO currency code               | ISO 4217 (e.g., `USD`)           | `string`  | `string`             |


# OKRA IDL Preprocessing Rules

Before parsing `.okra.gql` files with a standard GraphQL parser, the OKRA CLI applies a set of preprocessing transforms to support custom syntax extensions. These transforms rewrite non-standard constructs into valid GraphQL types, allowing us to leverage existing parsing tools while preserving semantic intent.

see `schema/preprocess.go`

---

## 🔧 Supported Preprocessor Transforms

### 1. `@okra(...)` Directive → `_Schema` Wrapper

Top-level `@okra(...)` directives are transformed into a synthetic GraphQL `type` called `_Schema`, which wraps the directive on a dummy field.

#### Original:

```graphql
@okra(namespace: "auth.users", version: "v1")
```

#### Transformed:

```graphql
type _Schema {
  OkraDirective @okra(namespace: "auth.users", version: "v1")
}
```

This allows the directive and its arguments to be parsed and preserved in the AST, while clearly identifying it as schema-level metadata.

---

### 2. `service` Blocks → `type Service_*` Replacement

Custom `service ServiceName { ... }` blocks are rewritten as standard GraphQL `type` declarations, with the name prefixed by `Service_`.

#### Original:

```graphql
service UserService {
  createUser(input: CreateUser): CreateUserResponse
    @auth(cel: "auth.role == 'admin'")
}
```

#### Transformed:

```graphql
type Service_UserService {
  createUser(input: CreateUser): CreateUserResponse
    @auth(cel: "auth.role == 'admin'")
}
```

This allows the standard GraphQL parser to process the block as a regular type, while the OKRA toolchain can later extract service definitions by looking for types with the `Service_` prefix.

---

## Benefits of This Approach

* Allows custom syntax without modifying the GraphQL parser
* Leverages existing AST tooling and directive parsing
* Keeps the IDL readable and concise
* Ensures round-trip compatibility for tooling

---

## Static Validation

The OKRA compiler performs comprehensive validation at build time:

```bash
okra validate service.graphql

# Validates:
✅ Schema syntax and types
✅ Decorator consistency (e.g., durable events → idempotent handlers)
✅ Event flow integrity
✅ Authorization completeness
✅ Method signature compatibility
```

Common validation errors:
```
❌ Non-idempotent handler 'sendEmail' cannot consume durable event 'user.created'
❌ Batch handler 'processOrders' must accept array type '[OrderEvent]'
❌ Unknown event type 'inventory.restock' - no @emits declaration found
❌ Method 'internalProcess' marked @scheduleOnly cannot be exposed as RPC
```

---

## AI-Native Design Benefits

The declarative decorator approach makes OKRA exceptionally AI-friendly:

1. **Single Source of Truth**: Every concern has one way to express it
2. **No Boilerplate**: AI doesn't generate switch statements or routers
3. **Validated Output**: Generated code can be statically checked
4. **Self-Documenting**: Decorators clearly express intent

Example: An AI assistant asked to "add retry logic with admin access" knows exactly what to generate:

```graphql
@auth(roles: ["admin"])
@durable(retries: 3, timeout: "5s")
@idempotent
processOrder(input: OrderInput): Order
```

See [AI-Native Design](./19_ai-native-design.md) for more details.

---

## Code Generation Clarity

Understanding what code OKRA generates from your IDL is crucial. Here's exactly what gets generated:

### From a Service's Own IDL:

Given this service definition:
```graphql
service OrderService @okra(namespace: "shop", version: "1.0") {
  # Service methods
  createOrder(input: OrderInput): Order
  getOrder(id: String): Order
  
  # Event emitters
  @emits(topic: "order.created")
  orderCreated(event: OrderCreatedEvent): void
  
  # Event handlers
  @handles(event: "inventory.stockLow")
  onStockLow(event: StockLowEvent): void
}
```

OKRA generates:

#### 1. Service Implementation Interface
```typescript
// What YOU implement
interface OrderServiceImpl {
  // Service methods
  createOrder(input: OrderInput): Promise<Order>;
  getOrder(id: String): Promise<Order>;
  
  // Event handlers (from @handles)
  onStockLow(event: StockLowEvent): Promise<void>;
  
  // Note: NO event emission methods here!
}
```

#### 2. Event Emitter for Dependency Injection
```typescript
// Injected into your service
interface OrderServiceEventEmitter {
  // Generated from @emits decorators
  emitOrderCreated(event: OrderCreatedEvent): Promise<void>;
}

// Your service implementation
class OrderService implements OrderServiceImpl {
  constructor(
    private events: OrderServiceEventEmitter, // Injected!
    private inventory: InventoryService      // Remote service stub
  ) {}
  
  async createOrder(input: OrderInput): Promise<Order> {
    const order = // ... create order logic
    
    // Emit using injected emitter
    await this.events.emitOrderCreated({
      orderId: order.id,
      customerId: input.customerId
    });
    
    return order;
  }
}
```

### For Other Services Using This Service:

Other services get:

#### 1. Service Client Stub (Remote Calls Only)
```typescript
// Generated client stub
interface OrderService {
  createOrder(input: OrderInput): Promise<Order>;
  getOrder(id: String): Promise<Order>;
  
  // NO event emission methods - encapsulation!
  // NO event handler methods - internal only!
}
```

#### 2. Event Type Definitions Only
```typescript
// Only the types are exported for events
export interface OrderCreatedEvent {
  orderId: string;
  customerId: string;
  items: OrderItem[];
  total: number;
}
```

### Key Points:

1. **You never emit other services' events** - No methods available
2. **Event handlers are internal** - Not exposed in client stubs  
3. **Event types are shared** - For type-safe handling
4. **Emitters are injected** - Not implemented by you

This separation ensures proper encapsulation while maintaining full type safety.

---

These transforms are applied automatically as part of the OKRA CLI's parsing pipeline. They are invisible to most users but critical for bridging OKRA's extended semantics with standard GraphQL tooling.
