# OKRA

OKRA is an open-source backend platform that makes it simple to build and scale services, agentic workflows, and distributed systems - without boilerplate or complexity.

Define your APIs with a GraphQL-like schema language, write plain code, and OKRA handles the rest: automatically generating gRPC, ConnectRPC, and GraphQL handlers that expose your logic via GraphQL, JSON and Protobuf formats.

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
- 📊 **GraphQL endpoints** - Query-based API with introspection support
- 📦 **Type definitions** - For your chosen language (Go, TypeScript)

Your service is now accessible via multiple protocols without writing any boilerplate!

### GraphQL Support

OKRA automatically generates GraphQL endpoints from your service definitions:

- **Namespace-based routing** - Each namespace gets its own GraphQL endpoint at `/graphql/{namespace}`
- **Automatic schema generation** - Types and resolvers are created from your OKRA schema
- **Query/Mutation classification** - Methods are automatically classified based on naming conventions
- **Full introspection support** - Compatible with GraphQL tools and IDEs
- **GraphQL Playground** - Interactive query editor available in development mode

Example:
```bash
# ConnectRPC endpoint
curl -X POST http://localhost:8080/connect/myapp.GreeterService/greet \
  -H "Content-Type: application/json" \
  -d '{"name": "World"}'

# GraphQL endpoint (same service)
curl -X POST http://localhost:8080/graphql/myapp \
  -H "Content-Type: application/json" \
  -d '{"query": "mutation { createUser(input: { name: \"John\" }) { id, name } }"}'
```

## Development

OKRA includes powerful development tools:

- **Hot Reload** - Automatic rebuilding on file changes
- **Error Reporting** - Clear error messages with troubleshooting tips
- **Debug Mode** - Set `OKRA_KEEP_BUILD_DIR=1` to preserve build artifacts for debugging

See [docs/10_development-debugging.md](docs/10_development-debugging.md) for detailed debugging guide.

## Documentation

- [System Overview](docs/00_overview.md)
- [Development & Debugging](docs/10_development-debugging.md)
- [OKRA Serve - Production Runtime](docs/11_okra-serve.md)
- [GraphQL Support](docs/12_graphql-support.md)
- [Testing Strategy](docs/101_testing-strategy.md)
- [Coding Conventions](docs/100_coding-conventions.md)
