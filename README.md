# OKRA

OKRA is an open-source backend platform that makes it simple to build and scale services, agentic workflows, and distributed systems - without boilerplate or complexity.

Define your APIs with a GraphQL-like schema language, write plain code, and OKRA handles the rest: automatically generating gRPC and ConnectRPC handlers that expose your logic via both JSON and Protobuf formats.

With WASM isolation, actor concurrency, built-in governance, and powerful observability, OKRA brings joy back to backend development - whether you're a solo hacker or an enterprise team.

## Installation

```bash
go install github.com/okra-platform/okra
```

## Quick Start

1. **Initialize a new OKRA project:**
   ```bash
   okra init my-service
   cd my-service
   ```

2. **Define your service in `service.okra.gql`:**
   ```graphql
   @okra(namespace: "myapp", version: "v1")
   
   service GreeterService {
     greet(input: GreetRequest): GreetResponse
   }
   
   type GreetRequest {
     name: String!
   }
   
   type GreetResponse {
     message: String!
   }
   ```

3. **Run in development mode:**
   ```bash
   okra dev
   ```

OKRA automatically generates from your schema:
- 🌐 **ConnectRPC handlers** - HTTP endpoints accepting JSON
- 🔧 **gRPC services** - Binary Protobuf protocol
- 📦 **Type definitions** - For your chosen language (Go, TypeScript)

Your service is now accessible via multiple protocols without writing any boilerplate!

## Development

OKRA includes powerful development tools:

- **Hot Reload** - Automatic rebuilding on file changes
- **Error Reporting** - Clear error messages with troubleshooting tips
- **Debug Mode** - Set `OKRA_KEEP_BUILD_DIR=1` to preserve build artifacts for debugging

See [docs/10_development-debugging.md](docs/10_development-debugging.md) for detailed debugging guide.

## Documentation

- [System Overview](docs/00_overview.md)
- [Development & Debugging](docs/10_development-debugging.md)
- [Testing Strategy](docs/101_testing-strategy.md)
- [Coding Conventions](docs/100_coding-conventions.md)
