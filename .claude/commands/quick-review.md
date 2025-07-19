# Quick Review Command

A rapid code review checklist for efficient PR reviews and pair programming sessions.

## Usage
```
quick-review                    # Interactive checklist
quick-review <pr-number>        # Review specific PR
quick-review <feature-name>     # Review feature implementation
```

## ⏱️ Time-Boxed Review Levels

### 5-Minute Review (Small PRs, < 100 lines)
```bash
# Quick metrics and checks
.claude/scripts/quick-metrics.sh  # Instant metrics (cached or quick test)
.claude/scripts/quick-check.sh    # Format, vet, and quick race test
```

**Checklist:**
- [ ] CI/CD status green
- [ ] No obvious security issues (hardcoded secrets, SQL injection)
- [ ] Basic error handling exists (no `_, err` ignoring)
- [ ] Tests exist for new functionality
- [ ] No panic() in library code

### 15-Minute Review (Medium PRs, < 500 lines)
All of the above, plus:
- [ ] Architecture aligns with design doc (if exists)
- [ ] Resource cleanup handled (defer Close())
- [ ] Public APIs have godoc comments
- [ ] No performance red flags (O(n²), unbounded growth)
- [ ] Error messages provide context

### 30-Minute Review (Large PRs, > 500 lines)
All of the above, plus:
- [ ] Line-by-line logic review
- [ ] Edge cases considered
- [ ] Integration tests present
- [ ] Benchmarks for performance-critical code
- [ ] Refactoring opportunities identified

---

## 🚨 CRITICAL CHECKLIST (Stop & Fix)

### Security
```go
// ❌ BAD: SQL injection risk
query := fmt.Sprintf("SELECT * FROM users WHERE id = %s", userInput)

// ❌ BAD: Path traversal risk
file := filepath.Join(baseDir, userInput)

// ❌ BAD: Hardcoded secrets
apiKey := "sk_live_abcd1234"
```
- [ ] All external inputs validated
- [ ] No injection vulnerabilities
- [ ] No hardcoded credentials
- [ ] No sensitive data in logs

### Resource Management
```go
// ❌ BAD: Resource leak
func process() error {
    f, err := os.Open(file)
    if err != nil {
        return err
    }
    // Missing: defer f.Close()
}

// ❌ BAD: Goroutine leak
go func() {
    for {
        // No exit condition
    }
}()
```
- [ ] All resources have cleanup
- [ ] Context cancellation respected
- [ ] Goroutines can terminate
- [ ] No infinite loops/growth

### Error Handling
```go
// ❌ BAD: Ignored error
result, _ := dangerousOperation()

// ❌ BAD: No context
return errors.New("failed")

// ✅ GOOD: Wrapped with context
return fmt.Errorf("failed to process user %s: %w", userID, err)
```
- [ ] No ignored errors
- [ ] Errors wrapped with context
- [ ] Custom errors for API boundaries
- [ ] Error paths tested

---

## ⚡ PERFORMANCE CHECKLIST

### Algorithm Complexity
```go
// 🚩 RED FLAG: O(n²)
for _, user := range users {
    for _, order := range orders {
        if user.ID == order.UserID {
            // Process
        }
    }
}

// ✅ BETTER: O(n) with map
ordersByUser := make(map[string][]*Order)
for _, order := range orders {
    ordersByUser[order.UserID] = append(ordersByUser[order.UserID], order)
}
```
- [ ] No unnecessary nested loops
- [ ] Appropriate data structures
- [ ] Early returns/continues used
- [ ] Batch operations where possible

### Memory Management
```go
// 🚩 RED FLAG: Unbounded cache
cache[key] = value // No eviction policy

// 🚩 RED FLAG: Large allocations in loop
for _, item := range millionItems {
    buffer := make([]byte, 1<<20) // 1MB per iteration
}
```
- [ ] Bounded growth (caches, slices)
- [ ] sync.Pool for temporary objects
- [ ] No allocations in hot paths
- [ ] Appropriate buffer sizes

---

## 🏗️ ARCHITECTURE CHECKLIST

### Package Design
- [ ] Single responsibility per package
- [ ] No circular dependencies
- [ ] Public interface, private implementation
- [ ] Interfaces defined by consumer, not provider

### OKRA-Specific Patterns
```go
// ❌ BAD: Storing instances
registry.Register(NewStateAPI())

// ✅ GOOD: Storing factories
registry.Register(NewStateAPIFactory())

// ❌ BAD: Protobuf across WASM
proto.Marshal(msg) // TinyGo doesn't support

// ✅ GOOD: JSON for WASM boundary
json.Marshal(msg)
```
- [ ] Factory pattern for registries
- [ ] JSON (not protobuf) for WASM
- [ ] Policy enforcement at boundaries
- [ ] Telemetry instrumentation added

---

## 🧪 TESTING CHECKLIST

### Test Quality
```go
// ❌ BAD: Test without assertions
func TestSomething(t *testing.T) {
    result := DoSomething()
    fmt.Println(result) // No assertions!
}

// ✅ GOOD: Clear test structure
func TestSomething(t *testing.T) {
    // Arrange
    input := CreateTestInput()
    expected := "expected result"
    
    // Act
    result := DoSomething(input)
    
    // Assert
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```
- [ ] Tests have clear arrange/act/assert
- [ ] Table-driven tests for variants
- [ ] Error cases covered
- [ ] No time-dependent tests
- [ ] Parallel tests where appropriate

---

## 📋 PR REVIEW TEMPLATE

Copy and paste this into your PR review:

```markdown
## Quick Review Results

**Review Type**: ⬜ 5-min / ⬜ 15-min / ⬜ 30-min
**Review Status**: ✅ Approved / 💭 Comments / ❌ Changes Required

### Code Metrics
```bash
# Generate metrics for this review
.claude/scripts/quick-metrics.sh
.claude/scripts/pr-metrics.sh
```
- **Test Coverage**: ___% (target: >80%)
- **Project Size**: ___ total lines
  - Production: ___ lines
  - Tests: ___ lines  
  - Test/Prod Ratio: ___% (target: >40%)
- **Files Changed**: ___ files
- **Lines Changed**: +___ / -___

### Critical Issues
- [ ] Security: Input validation complete
- [ ] Resources: Proper cleanup (no leaks)
- [ ] Errors: Handled with context
- [ ] Tests: Meaningful coverage

### Code Quality
- [ ] Follows OKRA conventions
- [ ] No obvious performance issues
- [ ] Documentation adequate
- [ ] No code smells (see flags above)

### Automated Checks
- [ ] CI/CD green
- [ ] `go fmt` clean
- [ ] `go vet` passes
- [ ] Tests pass with `-race`

**Notes**: 
<!-- Add specific comments here -->

**Risk Level**: 🟢 Low / 🟡 Medium / 🔴 High
```

---

## 🚩 Quick Reference: Code Smells

### Stop and Fix (Red Flags)
```go
panic("error")                    // Never panic in libraries
_, _ = fmt.Println(sensitive)     // Double ignored errors
var GlobalMutable = &Config{}     // Global mutable state
go func() { for {} }()           // Goroutine leak
```

### Discuss (Yellow Flags)
```go
func veryLongFunction() {        // >50 lines
if a { if b { if c { } } }     // Deep nesting
time.Sleep(5 * time.Second)     // Magic numbers
// TODO: fix this               // TODO without owner
```

### Nice to Have (Green Flags)
```go
// Concurrent-safe cache with expiration
// Well-documented error types
// Comprehensive table-driven tests
// Benchmarks for critical paths
```

---

## 🎯 Focus Areas by Component

### API Endpoints
- [ ] Rate limiting implemented
- [ ] Request validation complete
- [ ] Response size bounded
- [ ] Proper HTTP status codes

### Database Layer
- [ ] Prepared statements used
- [ ] Connection pooling configured
- [ ] Transactions handled correctly
- [ ] Migrations versioned

### WASM Services
- [ ] Memory limits enforced
- [ ] Proper allocation/deallocation
- [ ] JSON serialization (not protobuf)
- [ ] Host API policies defined

### Actor System
- [ ] No shared mutable state
- [ ] Message handling idempotent
- [ ] Supervision strategy defined
- [ ] Mailbox size bounded

---

## 🛠️ Quick Commands

```bash
# Quick quality check
alias qcheck='.claude/scripts/quick-check.sh'

# Get instant code metrics
alias metrics='.claude/scripts/quick-metrics.sh'

# Get PR-specific metrics
alias pr_metrics='.claude/scripts/pr-metrics.sh'

# Find common issues
.claude/scripts/find-issues.sh

# Check complexity
.claude/scripts/complexity-check.sh

# Memory leak check (run specific test)
go test -run TestName -memprofile mem.prof -count=10
go tool pprof -top mem.prof
```

---

## 📊 Review Metrics

Track these metrics over time:
- **Review Speed**: PRs reviewed within 4 hours
- **Defect Escape Rate**: Bugs found post-merge
- **Review Iterations**: Number of back-and-forth cycles
- **Code Coverage**: Maintained or improved
- **Technical Debt**: TODOs added vs resolved

---

Remember: The goal is to ship quality code quickly. Use this checklist to catch common issues early while maintaining development velocity.