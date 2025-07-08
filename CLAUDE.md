# CLAUDE.md - Development Context for OKRA

## Testing Philosophy & Approach

OKRA follows a comprehensive testing strategy with emphasis on **clear boundaries**, **fast feedback**, and **real-world confidence**. See `docs/101_testing-strategy.md` and `docs/102_testing-best-practices.md` for full details.

### Testing Layers
1. **Unit Tests** - Individual Go components in isolation using testify
2. **Integration Tests** - System boundaries (Actor messaging, WASM invocation, Host API injection)
3. **Fixture/Contract Tests** - Generated guest-side stubs with real WASM compilation
4. **CLI Tests** - Developer-facing CLI experience (planned)

### Test-Driven Development Workflow
When implementing new features or fixing bugs:

1. **Write test plan as comments** at top of test file
2. **Plan test cases** with single-line comments describing each scenario
3. **Think of general solution** - implement robust, general-purpose logic (not just test-passing code)
4. **Review & iterate** on test cases to ensure completeness
5. **Implement test cases** one-by-one, ensuring each passes
6. **Review code and test** for correctness, design quality, readability, and consistency

### Testing Conventions
- File naming: `*_test.go` (unit), `*_integration_test.go` (integration)
- Function naming: `Test<Component>_<Behavior>`
- Use `require` for setup/preconditions, `assert` for validations
- Prefer real dependencies over mocks when possible
- Use test fixtures with compiled `.wasm` files in `testdata/fixtures/`
- When implementing test-cases put the test-case comment lines ("// Test: ..") above the test-case they correspond to

### Key Testing Tools
- **testify** for assertions and mocking
- **gotestsum** via taskfile.yml tasks
- Real WASM compilation for integration tests
- Black-box validation approach for actor behavior

## Coding Conventions

OKRA follows a **public interface, internal implementation** pattern for all major components. See `docs/100_coding-conventions.md` for full details.

### Core Pattern
- **Public interfaces** define the contract
- **Unexported struct implementations** hide details
- **Public constructors** (`NewX(...)`) return interface types

### Benefits
- **Encapsulation** - Safe refactoring of internals
- **Testability** - Interface-driven development and mocking
- **Clarity** - Clean usage surface
- **Stability** - Internals can evolve without breaking consumers
- **Consistency** - Follows idiomatic Go patterns

### Example Structure
```go
// Public interface
type WASMWorkerPool interface {
    Invoke(ctx context.Context, method string, input []byte) ([]byte, error)
    Shutdown(ctx context.Context) error
}

// Public constructor
func NewWASMWorkerPool(...) WASMWorkerPool {
    return &wasmWorkerPool{...}
}

// Unexported implementation
type wasmWorkerPool struct {
    // internal fields
}
```

## Project Structure
This is a Go-based platform for building WebAssembly services with Protobuf definitions, using an actor system (GoAKT) for concurrency and state management.

## Development Tools

### Task Runner
- This project uses Task (https://taskfile.dev) as the task runner
- Project level tasks are available in taskfile.yml
- The task command is called `task`
- To see a list of available commands run `task` with no arguments

## WebAssembly Development

### WASM Compilation and Runtime
- We are using TinyGo to build Go WASI and Wazero to load and run .wasm files - as such we should make sure we follow TinyGo WASI approach
- When you need info about how to create a go file that can be built by TinyGo as WASI, please consult this example:
https://raw.githubusercontent.com/tetratelabs/wazero/refs/heads/main/examples/allocation/tinygo/testdata/greet.go

And these build instructions for TinyGo WASI:
https://raw.githubusercontent.com/tetratelabs/wazero/refs/heads/main/examples/allocation/tinygo/README.md

## Git & Collaboration Guidelines

### Commit Best Practices
IMPORTANT: - Never add claude user as a co-author to commits
