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
// - Can initialize with min and max workers
// - Invoke returns output from an idle worker
// - Creates new workers up to max
// - Blocks if all workers are busy
// - Releases workers back to pool
// - Respects context cancellation
// - Shutdown closes all workers
```

This helps think through important test scenarios up front.  

### 2. Plan test cases
Looking at the test plan comments, think deeply about what test cases will prove correctness of the behavior, including edge cases.  While you want correctness you also want to have each test case be meaningful - the goal is proving correctness with reasonable number of tests cases. Use a single-line comment to describe each test case.

``` go
// Test: Initializing with min workers greater than max workers should return an error
```

### 3. Think of a general solution
Please write a high quality, general purpose solution. Implement a solution that works correctly for all valid inputs, not just the test cases. Do not hard-code values or create solutions that only work for specific test inputs. Instead, implement the actual logic that solves the problem generally.

Focus on understanding the problem requirements and implementing the correct algorithm. Tests are there to verify correctness, not to define the solution. Provide a principled implementation that follows best practices and software design principles.

If the task is unreasonable or infeasible, or if any of the tests are incorrect, please tell me. The solution should be robust, maintainable, and extendable.

### 4. Review & iterate on test cases
Review the code that has been written and the test cases (written or in comments) to identify any missing test cases.  Add any missing test cases as comments to the test file.

### 5. Implement test cases
Referring to the test comments in the test-file, implement each test-case one-by-one, ensuring that each one passes before moving on to the next test case.
Some tips:
* Add tests for failure modes (timeouts, invalid inputs, etc.)
* Add concurrency tests (t.Parallel(), race detectors)
* Add shutdown and cleanup behavior
* This keeps complexity low and confidence high.

### 6. Review code and test
After tests are complete, make one final pass on the code, refactoring if necessary with this focus:
* Correctness - is the code doing what it should do, not just passing the tests
* Good Design - the code solves things high-quality, general purpose way - not just a way that passes the tests
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
