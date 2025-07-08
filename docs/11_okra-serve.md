# OKRA Serve - Production Runtime Server

## Overview

The `okra serve` command runs a production-ready OKRA runtime server that hosts WebAssembly services with automatic HTTP/gRPC gateway exposure via both ConnectRPC and GraphQL protocols. It provides a REST admin API for dynamic service deployment and management.

## Architecture

```
┌─────────────────────────────────────────────────┐
│            Service Gateway (Port 8080)          │
│                                                 │
│  ┌─────────────────┐     ┌──────────────────┐  │
│  │   /connect/*    │     │   /graphql/*     │  │     ┌─────────────────────┐
│  │                 │     │                  │  │     │     Admin API       │
│  │  ConnectRPC     │     │    GraphQL       │  │     │    (Port 8081)      │
│  │   Gateway       │     │    Gateway       │  │     └──────────┬──────────┘
│  │                 │     │                  │  │                │
│  └────────┬────────┘     └────────┬─────────┘  │                │
│           │                       │             │                │
└───────────┼───────────────────────┼─────────────┘                │
            │                       │                              │
            │                       │                              │
            │      ┌────────────────▼──────────────────────────────▼──┐
            └─────►│              OKRA Runtime                        │
                   │         (Actor System + WASM)                    │
                   │                                                  │
                   │  ┌────────────┐  ┌────────────┐  ┌────────────┐ │
                   │  │  Service    │  │  Service    │  │  Service    │ │
                   │  │  Actor 1    │  │  Actor 2    │  │  Actor N    │ │
                   │  └────────────┘  └────────────┘  └────────────┘ │
                   └──────────────────────────────────────────────────┘
```

### Components

1. **Service Gateway (Port 8080)**: Exposes deployed services via multiple protocols
   - **ConnectRPC Gateway** (`/connect/*`): 
     - Supports both JSON and Protobuf encoding
     - Dynamic route registration based on service descriptors
     - Full gRPC-Web compatibility
   - **GraphQL Gateway** (`/graphql/*`):
     - Namespace-based routing (`/graphql/{namespace}`)
     - Full introspection support
     - Automatic query/mutation classification
     - Type-safe schema generation from OKRA schemas

2. **Admin API (Port 8081)**: REST API for service management
   - Deploy services from packages
   - List deployed services
   - Undeploy services
   - Health checks

3. **OKRA Runtime**: Core actor-based execution environment
   - WASM module isolation
   - Actor supervision and lifecycle management
   - Message-based communication

4. **Package Loader**: Validates and extracts service packages
   - Supports file:// and S3:// URLs
   - WASM validation
   - Schema and configuration parsing

## Usage

### Starting the Server

```bash
# Start with default ports
okra serve

# Start with custom ports
okra serve --service-port 9090 --admin-port 9091
```

### Command-Line Options

- `--service-port`: Port for the service gateway (default: 8080)
- `--admin-port`: Port for the admin API (default: 8081)

## Admin API Reference

### Health Check

Check server health status.

```bash
GET /api/v1/health
```

Response:
```json
{
  "status": "healthy",
  "time": "2024-01-01T12:00:00Z"
}
```

### Deploy Service

Deploy a service from a package file.

```bash
POST /api/v1/packages/deploy
Content-Type: application/json

{
  "source": "file:///path/to/service.okra.pkg",
  "override": false
}
```

Request body:
- `source`: Package location (file:// or s3:// URL)
- `override`: Allow redeploying existing service (optional, default: false)

Response:
```json
{
  "service_id": "namespace.ServiceName.v1",
  "status": "deployed",
  "endpoints": [
    "/connect/namespace.ServiceName/methodName"
  ]
}
```

Note: Services are also automatically exposed via GraphQL at `/graphql/{namespace}`

### List Services

Get all deployed services.

```bash
GET /api/v1/packages
```

Response:
```json
{
  "services": [
    {
      "id": "namespace.ServiceName.v1",
      "source": "file:///path/to/service.okra.pkg",
      "deployed_at": "2024-01-01T12:00:00Z"
    }
  ]
}
```

### Undeploy Service

Remove a deployed service.

```bash
DELETE /api/v1/packages/{service_id}
```

Response: HTTP 204 No Content

## Service Package Format

OKRA services are deployed as `.okra.pkg` files (tar.gz archives) containing:

```
service.okra.pkg/
├── service.wasm           # Compiled WebAssembly module
├── okra.json             # Service configuration
├── service.okra.gql      # GraphQL schema
├── service.description.json  # Service metadata
└── service.pb.desc       # Protobuf descriptors
```

### Package Validation

The server validates packages before deployment:
- WASM magic number verification (0x00 0x61 0x73 0x6D)
- Required files presence check
- Schema parsing validation
- Configuration structure validation

## Service Invocation

Once deployed, services are accessible via the service gateway using either ConnectRPC or GraphQL protocols.

### ConnectRPC (JSON) Request Example

```bash
curl -X POST http://localhost:8080/connect/namespace.ServiceName/methodName \
  -H "Content-Type: application/json" \
  -d '{"field": "value"}'
```

### GraphQL Request Example

```bash
curl -X POST http://localhost:8080/graphql/namespace \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { methodName(input: { field: \"value\" }) { result } }"
  }'
```

### Endpoint Formats

**ConnectRPC endpoints**:
```
/connect/{namespace}.{ServiceName}/{methodName}
```

**GraphQL endpoints**:
```
/graphql/{namespace}
```

Where:
- `namespace`: From the `@okra(namespace: "...")` directive in the schema
- `ServiceName`: The service name from the schema
- `methodName`: The RPC method name

### GraphQL Introspection

The GraphQL gateway supports full introspection:

```bash
# Query available types
curl -X POST http://localhost:8080/graphql/namespace \
  -H "Content-Type: application/json" \
  -d '{
    "query": "{ __schema { types { name kind } } }"
  }'
```

## Implementation Details

### Dynamic Protobuf Handling

The server uses dynamic protobuf descriptor loading to:
1. Parse FileDescriptorSet from packages
2. Register services with protoregistry
3. Create dynamic message types at runtime
4. Handle JSON/Protobuf conversion automatically

### Actor Integration

Each deployed service runs as an actor in the runtime:
- Actor ID format: `{namespace}.{ServiceName}.{version}`
- Actors handle ServiceRequest messages
- Responses are wrapped in ServiceResponse messages
- Errors are propagated through the error field

### Graceful Shutdown

The server handles shutdown signals (SIGINT, SIGTERM) gracefully:
1. Stops accepting new requests
2. Waits for in-flight requests to complete
3. Shuts down actors and runtime
4. Closes all connections

## Production Considerations

### Resource Limits

- Each service runs in isolated WASM sandbox
- Memory usage controlled by WASM runtime
- CPU usage limited by actor system scheduling

### Monitoring

- Health endpoint for liveness checks
- Service deployment status tracking
- Error propagation through response codes

### Security

- WASM sandboxing for code isolation
- No direct file system access from services
- Network access controlled by host functions

## Example Deployment Flow

1. Build your service package:
   ```bash
   okra build
   ```

2. Deploy to running server:
   ```bash
   curl -X POST http://localhost:8081/api/v1/packages/deploy \
     -H "Content-Type: application/json" \
     -d '{
       "source": "file:///path/to/dist/MyService-1.0.0.okra.pkg"
     }'
   ```

3. Call your service:
   
   **Via ConnectRPC**:
   ```bash
   curl -X POST http://localhost:8080/connect/myapp.MyService/greet \
     -H "Content-Type: application/json" \
     -d '{"name": "World"}'
   ```
   
   **Via GraphQL**:
   ```bash
   curl -X POST http://localhost:8080/graphql/myapp \
     -H "Content-Type: application/json" \
     -d '{
       "query": "mutation { greet(input: { name: \"World\" }) { message } }"
     }'
   ```

4. Check deployment status:
   ```bash
   curl http://localhost:8081/api/v1/packages
   ```

## Comparison with okra dev

| Feature | okra dev | okra serve |
|---------|----------|------------|
| Purpose | Development | Production |
| Hot reload | Yes | No |
| Build on start | Yes | No |
| Package deployment | No | Yes |
| Admin API | No | Yes |
| GraphQL playground | Yes | No |
| GraphQL introspection | Yes | Yes |
| File watching | Yes | No |
| Multiple services | No | Yes |
| ConnectRPC support | Yes | Yes |
| GraphQL support | Yes | Yes |

## Troubleshooting

### Service Returns 404

- Check the namespace in your schema `@okra(namespace: "...")`
- Verify the service was deployed successfully
- For ConnectRPC: Ensure the endpoint URL matches `/connect/{namespace}.{ServiceName}/{methodName}`
- For GraphQL: Ensure the endpoint URL matches `/graphql/{namespace}`
- Note that the path prefixes (`/connect/` and `/graphql/`) are required

### Deployment Fails

- Verify the package file exists and is readable
- Ensure all required files are in the package
- Check server logs for detailed error messages

### Connection Refused

- Verify the server is running on the expected ports
- Check firewall rules allow connections
- Ensure no other process is using the ports
