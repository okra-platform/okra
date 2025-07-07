# Testing Strategy

This document outlines the testing philosophy and conventions for the OKRA platform and modules. Our goal is to ensure that OKRA remains fast to iterate on, deeply reliable, and easy for contributors to test at any layer — from generated service stubs to WASM runtime behavior.

We use a layered approach to testing that emphasizes **clear boundaries**, **fast feedback**, and **real-world confidence**.

---

## ✅ Testing Layers

### 1. Unit Tests

**Scope:**  
Test individual Go components in isolation.

**Guidelines:**
- Use [`testify`](https://github.com/stretchr/testify) for assertions and mocking.
- Prefer internal interfaces + explicit injection over reflection or monkeypatching.
- Follow TDD when designing reusable components.
- Favor clarity and coverage over DRY in test cases.

**Examples:**
- `pool_test.go` tests that workers are reused and max limits are respected.
- `registry_test.go` ensures descriptors load and resolve input/output types correctly.

---

### 2. Integration Tests

**Scope:**  
Test how parts of the system fit together — usually through the boundary of:
- Actor messaging
- WASM invocation
- Host API injection
- Schema resolution

**Guidelines:**
- Use real `.wasm` files compiled from minimal guest code (or include `.wat → .wasm` fixtures).
- Run real schema validation using `service.description.json` files.
- Prefer `black-box` validation: treat the actor as a unit and assert observable behavior.

**Examples:**
- Sending a `WASMInvokeRequest` message to a `WASMActor` and validating the output matches the expected `AddResult`.
- Injecting a `log()` host API and verifying that log output is recorded.

---

### 3. Fixture/Contract Tests (Guest Code)

**Scope:**  
Validate that generated guest-side stubs (Go, TS, etc.) correctly compile, expose expected signatures, and work with real host APIs.

**Guidelines:**
- Include example GraphQL schema files and generated guest stubs in `testdata/fixtures/`
- Compile guest WASM using a build matrix (TinyGo, AssemblyScript, etc.)
- Test against real host modules using simulated messages or the full runtime

**Examples:**
- A Go guest service with a `MathService.Add` implementation that calls back to another injected service
- A TS guest that logs a message using the `okra_host.log()` API

---

### 4. CLI Tests (Coming Soon)

**Scope:**  
Test the developer-facing CLI experience (`okra build`, `okra run`, `okra test`).

**Guidelines:**
- Use [Go’s `os/exec`](https://pkg.go.dev/os/exec) or `testscript` to simulate developer workflows
- Validate generated output (e.g., `.okra.pkg`, stubs, logs)
- Prefer real examples over mocks

---

## 🧪 Testing Goals

| Goal | Approach |
|------|----------|
| 🟢 Fast feedback on core components | Use mock-based unit tests |
| 🔒 Type-safe runtime schema validation | Use real `service.description.json` files |
| ⚙️ Confidence in WASM memory & ABI | Test with compiled guest modules |
| 🔄 Cross-service contract fidelity | Use generated stubs and simulate full request/response |
| 🧰 Developer confidence | Build fixtures that mimic real usage: compile, load, run, assert |

---

## File Structure Convention

```bash
internal/
  wasm/
    pool.go
    pool_test.go         # Unit tests
    pool_fixture_test.go # Integration tests using real WASM

testdata/
  fixtures/
    math/
      math.graphql
      math.wasm
      service.description.json
      main.go            # Guest service source (optional)
```

## Test Tools
- Unit & integration tests are build with testify
- There are tasks in the taskfile.yml for running tests via `gotestsum`

