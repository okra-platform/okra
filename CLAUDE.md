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

### Key Testing Tools
- **testify** for assertions and mocking
- **gotestsum** via taskfile.yml tasks
- Real WASM compilation for integration tests
- Black-box validation approach for actor behavior

## Project Structure
This is a Go-based platform for building WebAssembly services with Protobuf definitions, using an actor system (GoAKT) for concurrency and state management.