# GraphQL Support in OKRA

## Overview

OKRA services can be exposed via GraphQL in addition to ConnectRPC/gRPC. This provides a flexible, query-based API that allows clients to request exactly the data they need. The GraphQL implementation is dynamic, automatically generating schemas and resolvers based on the OKRA service definitions.

## Architecture

```
┌─────────────────────┐
│   HTTP Gateway      │
│   (Port 8080)       │
└──────────┬──────────┘
           │
    ┌──────┴──────┐
    │             │
    ▼             ▼
┌─────────┐  ┌─────────┐
│/connect/│  │/graphql/│
└────┬────┘  └────┬────┘
     │            │
     ▼            ▼
┌──────────┐ ┌──────────┐
│ Connect  │ │ GraphQL  │
│ Gateway  │ │ Gateway  │
└────┬─────┘ └────┬─────┘
     │            │
     └──────┬─────┘
            │
            ▼
    ┌─────────────┐
    │OKRA Runtime │
    │  (Actors)   │
    └─────────────┘
```

## URL Structure

To avoid namespace collisions and provide clear separation between protocols, OKRA uses path-based routing:

- **ConnectRPC**: `/connect/{namespace}.{ServiceName}/{methodName}`
- **GraphQL**: `/graphql/{namespace}`

### Examples:
- ConnectRPC: `POST /connect/auth.UserService/createUser`
- GraphQL: `POST /graphql/auth`

## GraphQL Schema Generation

Each OKRA namespace gets its own GraphQL endpoint to avoid type name collisions. The schema is dynamically generated from the OKRA service definitions:

### Type Mapping

| OKRA Type | GraphQL Type |
|-----------|--------------|
| String | String |
| Int | Int |
| Long | Int |
| Float | Float |
| Double | Float |
| Boolean | Boolean |
| ID | ID |
| Time/DateTime/Timestamp | String (ISO 8601) |
| Custom Types | Object Types |
| Enums | Enum Types |
| Arrays | List Types |

### Service Method Mapping

OKRA service methods are mapped to GraphQL fields based on their semantics:

1. **Query Methods** (read operations):
   - Methods starting with `get`, `list`, `find`, `search`, `query`
   - Methods that don't modify state
   
2. **Mutation Methods** (write operations):
   - Methods starting with `create`, `update`, `delete`, `set`, `add`, `remove`
   - Any method not classified as a query

### Example Schema Generation

Given this OKRA service definition:

```graphql
@okra(namespace: "users", version: "v1")

type User {
  id: ID!
  name: String!
  email: String!
  createdAt: DateTime!
}

type GetUserRequest {
  userId: ID!
}

type CreateUserRequest {
  name: String!
  email: String!
}

service UserService {
  getUser(input: GetUserRequest): User
  createUser(input: CreateUserRequest): User
}
```

The generated GraphQL schema would be:

```graphql
type User {
  id: ID!
  name: String!
  email: String!
  createdAt: String!
}

input GetUserInput {
  userId: ID!
}

input CreateUserInput {
  name: String!
  email: String!
}

type Query {
  getUser(input: GetUserInput!): User
}

type Mutation {
  createUser(input: CreateUserInput!): User
}
```

## Schema Registry Design

The GraphQL Gateway uses a thread-safe schema registry that supports dynamic updates for service redeployment. 

### Design Decision: Immutable vs Mutable Registry

After careful consideration, we've chosen an **immutable registry pattern** with atomic updates for the following reasons:

1. **Thread Safety**: No locks needed for reads, which are the most common operation
2. **Consistency**: All requests see a consistent view of the schema
3. **Rollback Capability**: Easy to revert to previous schema versions
4. **Debugging**: Clear audit trail of schema changes

The registry works as follows:
- Each namespace has its own schema version
- Updates create a new immutable schema instance
- References are atomically swapped
- Old schemas are garbage collected after grace period

## Implementation Details

### GraphQL Gateway Interface

```go
type GraphQLGateway interface {
    // Handler returns the HTTP handler for GraphQL requests
    Handler() http.Handler
    
    // UpdateService updates or adds a service to the GraphQL schema
    UpdateService(ctx context.Context, namespace string, schema *schema.Schema, actorPID *actors.PID) error
    
    // RemoveService removes a service from the GraphQL schema
    RemoveService(ctx context.Context, namespace string) error
    
    // Shutdown gracefully shuts down the gateway
    Shutdown(ctx context.Context) error
}
```

### Request Flow

1. **Request arrives** at `/graphql/{namespace}`
2. **Gateway looks up** the schema for the namespace
3. **GraphQL query is parsed** and validated against schema
4. **Resolvers execute** by sending messages to actors
5. **Responses are formatted** according to GraphQL spec

### Resolver Implementation

Resolvers act as a bridge between GraphQL and the actor system:

```go
func (r *resolver) ResolveField(ctx context.Context, field graphql.CollectedField) (interface{}, error) {
    // 1. Extract method name and input from GraphQL request
    // 2. Convert GraphQL input to JSON for actor message
    // 3. Send ServiceRequest to actor
    // 4. Convert actor response back to GraphQL types
    // 5. Handle errors according to GraphQL spec
}
```

## GraphQL Playground and Introspection

### GraphQL Playground (Dev Mode)

In development mode (`okra dev`), a GraphQL playground is automatically available for each namespace:

```
http://localhost:{port}/graphql/{namespace}
```

The playground provides:
- Interactive query editor with syntax highlighting
- Schema documentation browser
- Query history
- Auto-completion based on schema
- Real-time query execution

Example output when running `okra dev`:
```
🚀 Service TestService deployed and exposed via:
   - ConnectRPC: /connect/test.TestService/*
   - GraphQL: /graphql/test
   - GraphQL Playground: http://localhost:59848/graphql/test (open in browser)
```

### Introspection Support

OKRA's GraphQL implementation includes full introspection support, enabling:
- Schema discovery via `__schema` queries
- Type information via `__type` queries
- Tool integration (GraphQL clients, code generators)
- IDE auto-completion and validation

Example introspection queries:

```graphql
# Get all types in the schema
{
  __schema {
    types {
      name
      kind
      description
    }
  }
}

# Get details about a specific type
{
  __type(name: "User") {
    name
    fields {
      name
      type {
        name
        kind
      }
    }
  }
}
```

## Usage Examples

### Query Example

```graphql
# Request to /graphql/users
query GetUserInfo {
  getUser(input: { userId: "123" }) {
    id
    name
    email
  }
}
```

### Mutation Example

```graphql
# Request to /graphql/users
mutation CreateNewUser {
  createUser(input: { 
    name: "John Doe", 
    email: "john@example.com" 
  }) {
    id
    name
    email
    createdAt
  }
}
```

### Multiple Operations

```graphql
# Request to /graphql/users
query GetMultipleUsers {
  user1: getUser(input: { userId: "123" }) {
    name
  }
  user2: getUser(input: { userId: "456" }) {
    name
  }
}
```

## Error Handling

GraphQL errors are returned in the standard format:

```json
{
  "errors": [
    {
      "message": "User not found",
      "path": ["getUser"],
      "extensions": {
        "code": "NOT_FOUND"
      }
    }
  ],
  "data": {
    "getUser": null
  }
}
```

## Performance Considerations

1. **Schema Caching**: Compiled schemas are cached per namespace
2. **Query Complexity**: Consider implementing query depth/complexity limits
3. **DataLoader Pattern**: Future enhancement for N+1 query optimization
4. **Concurrent Resolvers**: Field resolvers can execute in parallel

## Security Considerations

1. **Query Depth Limiting**: Prevent deeply nested queries
2. **Query Complexity Analysis**: Prevent expensive queries
3. **Field-Level Authorization**: Directives can control field access
4. **Rate Limiting**: Applied at the namespace level

## Migration Guide

For existing ConnectRPC clients migrating to GraphQL:

1. Update endpoint from `/` to `/connect/` for ConnectRPC
2. GraphQL endpoints are available at `/graphql/{namespace}`
3. Method names remain the same in GraphQL queries/mutations
4. Input/output types are automatically converted

## Future Enhancements

1. **Subscriptions**: Real-time updates via WebSocket
2. **Federation**: Compose multiple OKRA services into unified graph
3. **Schema Stitching**: Combine schemas from different namespaces
4. **Introspection Control**: Fine-grained control over schema visibility
5. **Custom Directives**: Support for OKRA-specific GraphQL directives