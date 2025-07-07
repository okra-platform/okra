# TypeScript Setup Guide

This guide explains how to set up and develop OKRA services using TypeScript.

## Prerequisites

Before you can build TypeScript OKRA services, you need to install:

1. **Node.js and npm** (version 18 or higher)
   - Download from https://nodejs.org/
   - Verify installation: `npm --version`

2. **Javy** - JavaScript to WebAssembly compiler
   ```bash
   npm install -g @shopify/javy
   ```
   - Verify installation: `javy --version`

## Creating a TypeScript Service

1. **Initialize a new TypeScript project:**
   ```bash
   okra init
   # Choose "TypeScript" when prompted for the template
   ```

2. **Install dependencies:**
   ```bash
   cd your-project-name
   npm install
   ```

3. **Project structure:**
   ```
   your-project/
   ├── okra.json              # Project configuration
   ├── service.okra.graphql   # Service schema definition
   ├── package.json           # Node.js dependencies
   ├── tsconfig.json          # TypeScript configuration
   └── src/
       └── index.ts           # Service implementation
   ```

## Development Workflow

1. **Define your service schema** in `service.okra.graphql`:
   ```graphql
   @okra(namespace: "myapp.users", version: "v1")
   
   service UserService {
     getUser(input: GetUserRequest): User
     createUser(input: CreateUserRequest): User
   }
   
   type GetUserRequest {
     id: String!
   }
   
   type CreateUserRequest {
     name: String!
     email: String!
   }
   
   type User {
     id: String!
     name: String!
     email: String!
   }
   ```

2. **Start the development server:**
   ```bash
   okra dev
   ```
   
   This will:
   - Generate `src/service.interface.ts` with TypeScript types from your schema
   - Watch for changes to your schema and source files
   - Automatically rebuild when files change

3. **Implement your service** in `src/index.ts`:
   ```typescript
   import type { GetUserRequest, CreateUserRequest, User } from './service.interface';
   
   export function getUser(input: GetUserRequest): User {
     // Your implementation here
     return {
       id: input.id,
       name: "John Doe",
       email: "john@example.com"
     };
   }
   
   export function createUser(input: CreateUserRequest): User {
     // Your implementation here
     const newUser: User = {
       id: Math.random().toString(36),
       name: input.name,
       email: input.email
     };
     
     return newUser;
   }
   ```

## Build Process

The TypeScript build process consists of two stages:

1. **ESBuild** bundles your TypeScript code into a single JavaScript file
2. **Javy** compiles the JavaScript bundle into WebAssembly

When you run `okra dev` or `okra build`, this happens automatically:

```
TypeScript Source → ESBuild → JavaScript Bundle → Javy → WASM Module
```

## Configuration

### okra.json

```json
{
  "name": "my-service",
  "version": "1.0.0",
  "language": "typescript",
  "schema": "./service.okra.graphql",
  "source": "./src",
  "build": {
    "output": "./build/service.wasm"
  },
  "dev": {
    "watch": ["*.ts", "**/*.ts", "*.okra.graphql", "**/*.okra.graphql"],
    "exclude": ["*.test.ts", "build/", "node_modules/", ".git/", "service.interface.ts"]
  }
}
```

### tsconfig.json

The default TypeScript configuration is optimized for OKRA services:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "lib": ["ES2020"],
    "moduleResolution": "node",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true
  }
}
```

## Important Notes

1. **Generated Files**: The `service.interface.ts` file is automatically generated from your schema. Don't edit it manually - it will be overwritten.

2. **Bundle Size**: Javy produces relatively large WASM modules (~870KB minimum) because it includes a JavaScript runtime. This is normal and expected.

3. **Async Functions**: Currently, OKRA services must be synchronous. Async/Promise support is planned for future releases.

4. **Hidden Build Files**: The build process uses a temporary directory for intermediate files. These are automatically cleaned up and won't clutter your project.

## Troubleshooting

### "npm not found"
- Install Node.js from https://nodejs.org/

### "javy not found"
- Install Javy: `npm install -g @shopify/javy`

### "node_modules not found"
- Run `npm install` in your project directory

### Build errors
- Check that your TypeScript code compiles: `npx tsc --noEmit`
- Ensure all exported functions match the schema methods
- Verify that function parameters and return types match the schema

## Example Services

See the TypeScript template for a complete example:
- Simple greeting service with type-safe implementation
- Demonstrates schema-driven development
- Shows proper project structure

## Next Steps

- Learn about [Host APIs](./07_host-apis.md) for accessing platform features
- Explore [Service-to-Service Communication](./04_service-to-service.md)
- Read about [Testing Strategies](./101_testing-strategy.md)