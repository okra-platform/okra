# Testing Best Practices

This document outlines **how** we write tests in the OKRA codebase — whether by hand or using AI agents like Claude Code. The focus here is not on coverage, but on clarity, flow, and confidence. This complements [`101_testing-strategy.md`](./101_testing-strategy.md), which explains *what* layers to test and *why*.

Our core belief:  
> ✅ Tests are part of the design, not just a safety net.

---

## ✅ Recommended Testing Workflow

We use a consistent test-driven development (TDD) loop when building or modifying any component.

### 1. Write a test plan as comments

At the top of each test file, describe what the component must do — like a checklist:

```go
// Test Plan for WASMWorkerPool:
// - Can initialize with min workers
// - Invoke returns output from an idle worker
// - Creates new workers up to max
// - Blocks if all workers are busy
// - Releases workers back to pool
// - Respects context cancellation
// - Shutdown closes all workers
```

This helps think through importnat test scenarios up front.  Each test-case comment can be removed after each test is implemented.

### 2. Start with the happy path
Your first test should validate the most common, successful flow.
Run the test. Ensure it fails with a clear message. Then implement just enough code to make it pass.

### 3. Add edge cases incrementally
Once the happy path is working:

* Add tests for failure modes (timeouts, invalid inputs, etc.)
* Add concurrency tests (t.Parallel(), race detectors)
* Add shutdown and cleanup behavior
* This keeps complexity low and confidence high.

### 4. Review code and test
After tests are complete, make one final pass on the code, refactoring if necessary with this focus:
* Correctness - is the code doing what it should do, not just passing the tests
* Readabiilty & Meaning - if another human or agent reads this code will be immediately clear to them how it works and what it does
* Consistency - does this code follow existing code styles and naming conventions in this repo

If any refactors are necessary, esnure the tests pass after each one.

## Test Structure & Naming Conventions
* File naming: *_test.go (unit) and *_integration_test.go (integration)
* Function naming: Test<Component>_<Behavior>
* e.g., TestWASMWorkerPool_InvokeBlocksOnMaxWorkers
* Use subtests (t.Run(...)) for variations or permutations
* Always add t.Parallel() when safe

## Tools & Patterns

### Assertions
Use testify for:
* require – stop the test if this fails (setup, preconditions)
* assert – continue test, better for multi-field validation

### Mocks
Use mocks only when:
* The real dependency is slow (e.g., real WASM compile)
* The real behavior is hard to simulate (e.g., network error)
* You want to assert internal interactions (e.g., method calls)

Otherwise, prefer:
* Real dependency configured for the test
* Fake structs with real behavior
* Fixture/test data or dependencies, inlcuding compiled .wasm test fixtures
