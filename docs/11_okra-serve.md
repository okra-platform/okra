# OKRA Serve - Production Runtime Server

## Overview

The `okra serve` command runs a production-ready OKRA runtime server that hosts WebAssembly services with automatic HTTP/gRPC gateway exposure via ConnectRPC. It provides a REST admin API for dynamic service deployment and management.

## Architecture

```
┌─────────────────────┐     ┌─────────────────────┐
│   Service Gateway   │     │     Admin API       │
│   (Port 8080)       │     │    (Port 8081)      │
└──────────┬──────────┘     └──────────┬──────────┘
           │                           │
           │                           │
      ┌────▼──────────┐                │
      │  ConnectRPC   │                │
      │   Gateway     │                │
      │  (Dynamic     │                │
      │   Routing)    │                │
      └────┬──────────┘                │
           │                           │
           │      ┌────────────────────▼────┐
           └─────►│     OKRA Runtime        │
                  │  (Actor System + WASM)  │
                  └─────────────────────────┘
```

### Components

1. **Service Gateway (Port 8080)**: Exposes deployed services via ConnectRPC protocol
   - Supports both JSON and Protobuf encoding
   - Dynamic route registration based on service descriptors
   - Automatic protocol translation between HTTP and actor messages

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
    "/namespace.ServiceName/methodName"
  ]
}
```

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

Once deployed, services are accessible via the service gateway using ConnectRPC protocol.

### JSON Request Example

```bash
curl -X POST http://localhost:8080/namespace.ServiceName/methodName \
  -H "Content-Type: application/json" \
  -d '{"field": "value"}'
```

### Endpoint Format

Service endpoints follow the pattern:
```
/{namespace}.{ServiceName}/{methodName}
```

Where:
- `namespace`: From the `@okra(namespace: "...")` directive in the schema
- `ServiceName`: The service name from the schema
- `methodName`: The RPC method name

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
   ```bash
   curl -X POST http://localhost:8080/myapp.MyService/greet \
     -H "Content-Type: application/json" \
     -d '{"name": "World"}'
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
| File watching | Yes | No |
| Multiple services | No | Yes |

## Troubleshooting

### Service Returns 404

- Check the namespace in your schema `@okra(namespace: "...")`
- Verify the service was deployed successfully
- Ensure the endpoint URL matches the pattern `/{namespace}.{ServiceName}/{methodName}`

### Deployment Fails

- Verify the package file exists and is readable
- Ensure all required files are in the package
- Check server logs for detailed error messages

### Connection Refused

- Verify the server is running on the expected ports
- Check firewall rules allow connections
- Ensure no other process is using the ports
