# Code Review Command

Perform a comprehensive code review of a feature implementation to ensure quality, security, performance, and maintainability.

## Usage
```
code-review <feature-name>
```

## Prerequisites
1. **Read the design document**: `docs/design/*-<feature-name>.md`
2. **Understand project conventions**: 
   - CLAUDE.md - Project-specific requirements
   - docs/100_coding-conventions.md - Coding standards
   - docs/101_testing-strategy.md - Testing requirements
   - docs/102_testing-best-practices.md - Testing patterns

## Review Process

### Phase 1: Automated Checks
Before manual review, ensure these pass:
```bash
# Run comprehensive tests with appropriate timeout
.claude/scripts/test-runner.sh 10m race

# Generate comprehensive code metrics
.claude/scripts/code-metrics.sh

# Run linting and formatting checks
.claude/scripts/lint-check.sh

# Check for security issues
.claude/scripts/security-check.sh
```

### Phase 2: Architecture & Design Review

#### 1. Implementation Alignment
- [ ] Implementation matches the design document
- [ ] No significant deviations without documentation
- [ ] All design requirements are fulfilled
- [ ] Edge cases from design are handled

#### 2. Package Structure
- [ ] Clear separation of concerns
- [ ] No circular dependencies
- [ ] Internal packages used appropriately
- [ ] Public API surface is minimal and intentional

#### 3. Interface Design
```go
// GOOD: Interface defined by consumer
type Repository interface {
    Get(ctx context.Context, id string) (*Entity, error)
}

// BAD: Interface defined by implementation
type DatabaseRepository interface {
    Get(id string) *Entity
    Connect() error
    Disconnect() error
}
```

### Phase 3: Code Quality Review

#### 1. Readability & Maintainability
- **Naming**: Are names descriptive and consistent?
  - Functions: verbNoun (e.g., `validateInput`, `parseConfig`)
  - Interfaces: noun + "er" suffix (e.g., `Reader`, `Validator`)
  - Packages: singular, lowercase (e.g., `hostapi`, not `hostapis`)
  
- **Function Length**: No function > 50 lines (consider refactoring)
- **Cognitive Complexity**: Can you understand the function's purpose in < 30 seconds?
- **DRY Principle**: No duplicated logic (extract common patterns)

#### 2. Error Handling
```go
// GOOD: Descriptive errors with context
if err != nil {
    return fmt.Errorf("failed to parse config at %s: %w", path, err)
}

// GOOD: Custom error types for API boundaries
type ValidationError struct {
    Field string
    Code  string
}

// BAD: Generic errors
return errors.New("error")
```

#### 3. Resource Management
- [ ] All resources have proper cleanup (defer Close())
- [ ] No resource leaks in error paths
- [ ] Context cancellation is respected
- [ ] Goroutines are properly managed

```go
// GOOD: Resource cleanup pattern
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()
    
    // Always check Close() errors for writers
    if err := writer.Close(); err != nil {
        return fmt.Errorf("failed to close writer: %w", err)
    }
}
```

### Phase 4: Security Review

#### 1. Input Validation
- [ ] All external inputs are validated
- [ ] Size limits enforced (see constants)
- [ ] No SQL/command injection vulnerabilities
- [ ] Path traversal attacks prevented

#### 2. Authentication & Authorization
- [ ] Proper access control checks
- [ ] No hardcoded credentials
- [ ] Secrets handled appropriately
- [ ] Policy enforcement at boundaries

#### 3. Data Handling
- [ ] Sensitive data not logged
- [ ] Proper encryption for data at rest/transit
- [ ] No PII in error messages
- [ ] Safe serialization/deserialization

### Phase 5: Performance Review

#### 1. Algorithmic Complexity
- [ ] No unnecessary O(n²) operations
- [ ] Appropriate data structures used
- [ ] Batch operations where applicable

#### 2. Memory Management
- [ ] No unnecessary allocations in hot paths
- [ ] Proper use of sync.Pool for temporary objects
- [ ] Bounded growth (e.g., limited cache sizes)

```go
// GOOD: Reuse allocations
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

// BAD: Allocation in hot path
for _, item := range items {
    buf := new(bytes.Buffer) // Allocates every iteration
}
```

#### 3. Concurrency
- [ ] No data races (run with -race flag)
- [ ] Proper synchronization primitives
- [ ] No goroutine leaks
- [ ] Channels closed by sender

```go
// GOOD: Proper goroutine lifecycle
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func() {
    select {
    case <-ctx.Done():
        return
    case result := <-ch:
        process(result)
    }
}()
```

### Phase 6: Testing Review

#### 1. Test Coverage
- [ ] Critical paths have > 80% coverage
- [ ] Error paths are tested
- [ ] Edge cases are covered
- [ ] Benchmarks for performance-critical code

#### 2. Test Quality
```go
// GOOD: Descriptive test with clear phases
func TestHostAPISet_Execute(t *testing.T) {
    // Arrange
    api := createMockAPI()
    set := createHostAPISet(api)
    
    // Act
    result, err := set.Execute(ctx, "test.api", "method", params)
    
    // Assert
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}

// GOOD: Table-driven tests for multiple scenarios
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid input", "test", false},
        {"empty input", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validate(tt.input)
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### Phase 7: Documentation Review

#### 1. Code Documentation
- [ ] Package has a doc comment explaining its purpose
- [ ] Public types/functions have godoc comments
- [ ] Complex algorithms have explanatory comments
- [ ] TODOs include context and ownership

```go
// Package hostapi provides the foundational infrastructure for exposing
// system capabilities to WASM services through controlled, policy-enforced
// host functions with built-in observability.
package hostapi

// Execute routes a request to the appropriate host API with cross-cutting concerns
// including telemetry, policy enforcement, and error handling.
//
// The method will return an error if:
//   - The requested API is not found
//   - Policy denies the request
//   - The underlying API call fails
func (s *defaultHostAPISet) Execute(ctx context.Context, apiName, method string, parameters json.RawMessage) (json.RawMessage, error) {
```

### Phase 8: OKRA-Specific Review

#### 1. WASM Boundary Safety
- [ ] Proper memory management across boundaries
- [ ] No protobuf across WASM boundary (use JSON)
- [ ] Size limits enforced
- [ ] Proper pointer/length handling

#### 2. Actor Model Compliance
- [ ] No shared mutable state
- [ ] Message passing only
- [ ] Proper supervision trees
- [ ] Idempotent message handling

#### 3. Host API Patterns
- [ ] Follows factory pattern correctly
- [ ] Proper resource lifecycle
- [ ] Policy enforcement implemented
- [ ] Telemetry instrumentation complete

## Review Output Template

```markdown
## Code Review Summary

**Feature**: <feature-name>
**Reviewer**: <your-name>
**Date**: <date>
**Status**: ✅ Approved | ⚠️ Approved with comments | ❌ Changes required

### Critical Issues
[Required fixes before merge]
1. **[Security/Performance/Correctness]**: Description
   - File: `path/to/file.go:42`
   - Why: Impact explanation
   - Fix: Specific solution
   ```go
   // Example code showing the fix
   ```

### Important Suggestions
[Should be addressed but not blocking]
1. **[Category]**: Description
   - Consider: Alternative approach
   - Benefits: Why this improves the code

### Minor Issues
[Nice to have improvements]
- [ ] Typo in comment at `file.go:15`
- [ ] Consider extracting magic number to constant

### Positive Highlights
- Excellent test coverage on critical paths
- Clean separation of concerns in package structure
- Good error handling with contextual information

### Metrics
- Test Coverage: 85% (Total)
  - Package breakdown: api: 92%, runtime: 78%, wasm: 88%
- Code Statistics:
  - Total Go files: 142
  - Total lines: 15,432
  - Production code: 9,821 lines (63.6%)
  - Test code: 5,611 lines (36.4%)
  - Test/Prod ratio: 0.57 (good)
- Cyclomatic Complexity: Max 7 (good)
- Code Duplication: < 2%
- New Dependencies: 2 (justified in design)

### Security Checklist
- [x] Input validation complete
- [x] No sensitive data exposure
- [x] Resource limits enforced
- [x] Policy checks implemented

### Performance Notes
- Memory allocation pattern is efficient
- No obvious bottlenecks identified
- Consider adding benchmark for `Execute` method
```

## Common Anti-Patterns in OKRA

1. **Storing instances in registry instead of factories**
   ```go
   // BAD
   registry.Register(NewStateAPI())
   
   // GOOD
   registry.Register(NewStateAPIFactory())
   ```

2. **Missing nil checks after type assertions**
   ```go
   // BAD
   serviceInfo := ctx.Value(serviceInfoKey{}).(ServiceInfo)
   
   // GOOD
   serviceInfo, ok := ctx.Value(serviceInfoKey{}).(ServiceInfo)
   if !ok {
       return fmt.Errorf("service info not found in context")
   }
   ```

3. **Synchronous operations in actor message handlers**
   ```go
   // BAD: Blocks the actor
   func (a *MyActor) Receive(msg interface{}) {
       time.Sleep(5 * time.Second) // Never sleep in actors
   }
   
   // GOOD: Use scheduling or separate goroutine
   func (a *MyActor) Receive(msg interface{}) {
       a.scheduler.Schedule(5*time.Second, func() {
           a.Send(DelayedMessage{})
       })
   }
   ```

4. **Protobuf serialization across WASM boundary**
   ```go
   // BAD: TinyGo doesn't support reflection
   data, _ := proto.Marshal(msg)
   
   // GOOD: Use JSON
   data, _ := json.Marshal(msg)
   ```

## Review Priorities

When time is limited, prioritize in this order:
1. **Security vulnerabilities** - Must fix
2. **Data corruption risks** - Must fix
3. **Resource leaks** - Must fix
4. **API breaking changes** - Must discuss
5. **Performance regressions** - Should fix
6. **Code maintainability** - Should improve
7. **Style/formatting** - Nice to have

## Tools & Commands

```bash
# Quick review setup
alias review='go test ./... -race && golangci-lint run && go mod tidy'

# Run all review scripts
.claude/scripts/code-metrics.sh      # Comprehensive metrics with 10min timeout
.claude/scripts/complexity-check.sh  # Cyclomatic complexity analysis
.claude/scripts/security-check.sh    # Security vulnerability scan
.claude/scripts/lint-check.sh        # Linting and formatting
.claude/scripts/pr-metrics.sh        # PR-specific metrics

# Individual checks
.claude/scripts/test-runner.sh 10m coverage  # Run with coverage
.claude/scripts/find-issues.sh               # Quick issue scan

# Visualize dependencies
go mod graph | grep -v '@' | sort | uniq
```

Remember: The goal of code review is to improve code quality while maintaining team velocity. Be constructive, specific, and teach through your reviews.