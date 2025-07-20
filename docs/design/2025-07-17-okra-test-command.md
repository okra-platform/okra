# OKRA Test Command Design

## Overview

### Problem Statement
OKRA service developers need a streamlined way to test their services with proper isolation, mocked dependencies, and realistic WASM execution environments. Currently, there's no dedicated tooling for testing OKRA services, making it difficult to ensure service quality and catch issues early in development.

### Goals
1. **Simple Testing** - Make it easy to run tests with a single command
2. **Host API Mocking** - Provide mock implementations of all host APIs for unit testing
3. **Integration Testing** - Support testing with real WASM compilation and execution
4. **Test Discovery** - Automatically find and run all tests in a service
5. **Developer Experience** - Fast feedback loops with clear error messages

### Non-Goals
1. **End-to-End Testing** - Not replacing full system integration tests
2. **Load Testing** - Not focused on performance testing (separate tool)
3. **Browser Testing** - Not testing frontend/UI components
4. **Cluster Testing** - Not testing distributed actor behavior
5. **Language-Specific Tools** - Not replacing go test, jest, etc. for unit tests

### High-Level Solution
Introduce `okra test` command that provides a testing framework specifically designed for OKRA services, with built-in mocks for host APIs, support for both unit and integration testing, and seamless integration with existing test runners.

## Interfaces & APIs

### CLI Interface

```bash
# Run all tests in current service
okra test

# Run specific test files
okra test service/*_test.go
okra test src/**/*.test.ts

# Run with options
okra test --unit          # Unit tests only (with mocks)
okra test --integration   # Integration tests only (real WASM)
okra test --watch        # Re-run on file changes
okra test --coverage     # Generate coverage report
okra test --verbose      # Detailed output
okra test --timeout=30s  # Custom timeout

# Run specific test by name
okra test --run TestUserCreation
okra test --run "Test.*Creation"  # Regex pattern
```

### Test Harness Interface

```go
// TestHarness provides testing utilities for OKRA services
type TestHarness interface {
    // NewTestService creates a test instance of a service
    NewTestService(ctx context.Context, opts TestServiceOptions) (TestService, error)
    
    // MockHostAPI registers a mock for a specific host API
    MockHostAPI(name string, mock HostAPIMock) error
    
    // RunTest executes a test with proper setup/teardown
    RunTest(ctx context.Context, test TestFunc) error
    
    // Cleanup releases all test resources
    Cleanup() error
}

// TestService represents a service instance for testing
type TestService interface {
    // Invoke calls a service method
    Invoke(ctx context.Context, method string, input interface{}) (interface{}, error)
    
    // GetHostAPICalls returns all host API calls made
    GetHostAPICalls(apiName string) []HostAPICall
    
    // ResetMocks clears all mock call history
    ResetMocks()
    
    // Shutdown stops the test service
    Shutdown(ctx context.Context) error
}

// TestServiceOptions configures test service creation
type TestServiceOptions struct {
    // ServicePath to the service directory
    ServicePath string
    
    // CompileMode for test execution
    CompileMode CompileMode
    
    // HostAPIs to enable
    EnabledHostAPIs []string
    
    // InitialState for stateful services
    InitialState map[string]interface{}
}

// CompileMode determines how tests are run
type CompileMode string

const (
    // Mock mode - no WASM compilation, pure mocks
    CompileModeMock CompileMode = "mock"
    
    // WASM mode - compile to WASM and run
    CompileModeWASM CompileMode = "wasm"
    
    // Hybrid mode - unit tests use mocks, integration uses WASM
    CompileModeHybrid CompileMode = "hybrid"
)
```

### Host API Mocking

```go
// HostAPIMock provides mock implementations of host APIs
type HostAPIMock interface {
    // HandleCall processes a host API call
    HandleCall(ctx context.Context, function string, request []byte) ([]byte, error)
    
    // GetCalls returns all calls made to this mock
    GetCalls() []HostAPICall
    
    // Reset clears call history
    Reset()
}

// HostAPICall represents a call to a host API
type HostAPICall struct {
    Function  string
    Request   json.RawMessage
    Response  json.RawMessage
    Error     error
    Timestamp time.Time
    Duration  time.Duration
}

// Built-in mock implementations
type LogAPIMock struct {
    calls []HostAPICall
}

func (m *LogAPIMock) ExpectLog(level, message string) {
    // Set expectation for a log call
}

type StateAPIMock struct {
    store map[string][]byte
    calls []HostAPICall
}

func (m *StateAPIMock) SetState(key string, value interface{}) error {
    // Pre-populate state for testing
}

// Example: Mocking fetch API
type FetchAPIMock struct {
    responses map[string]FetchResponse
}

func (m *FetchAPIMock) WhenURL(url string) *FetchExpectation {
    // Fluent API for setting expectations
    return &FetchExpectation{mock: m, url: url}
}
```

### Test Utilities

```go
// Test assertion helpers
type TestAssertions struct {
    t testing.TB
}

func (a *TestAssertions) HostAPICalled(api, function string) {
    // Assert a host API was called
}

func (a *TestAssertions) HostAPICalledWith(api, function string, expected interface{}) {
    // Assert a host API was called with specific args
}

func (a *TestAssertions) HostAPINotCalled(api, function string) {
    // Assert a host API was not called
}

// Test data builders
type TestDataBuilder struct {
    // Helper methods for creating test data
}

func (b *TestDataBuilder) User(opts ...UserOption) *User {
    // Build test user data
}

func (b *TestDataBuilder) GraphQLRequest(query string, vars map[string]interface{}) *Request {
    // Build GraphQL request
}
```

## Component Interactions

### Test Execution Flow

```
okra test -> Test Discovery -> Test Compilation -> Test Execution -> Report
    |              |                 |                  |              |
    v              v                 v                  v              v
CLI Parse    Find *_test.*     Compile Service    Run Test Cases   Format Output
    |              |            (if needed)            |              |
    v              v                 |                  v              v
Options      Load Service          v              Setup Mocks    Coverage Data
             Configuration    WASM Module         Execute Test   Test Results
```

### Mock Integration Flow

```
Test Code -> Service Method -> Host API Call -> Mock Router -> Mock Implementation
                                      |                              |
                                      v                              v
                                Request Data                   Handle & Record
                                      |                              |
                                      v                              v
                              Serialize Request              Return Response
                                                                    |
                                                                    v
                                                              Track Call
```

## Implementation Approach

### Key Components

1. **Test Discovery Engine**
```go
// TestDiscovery finds all test files in a service
type TestDiscovery struct {
    servicePath string
    patterns    []string
}

func (td *TestDiscovery) FindTests() ([]TestFile, error) {
    // Walk directory tree
    // Match test patterns (*_test.go, *.test.ts, etc.)
    // Parse test functions
    // Return test metadata
}
```

2. **Mock Registry**
```go
// MockRegistry manages all host API mocks
type MockRegistry struct {
    mocks map[string]HostAPIMock
    mu    sync.RWMutex
}

func (r *MockRegistry) Register(name string, mock HostAPIMock) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.mocks[name] = mock
}

func (r *MockRegistry) GetMock(name string) (HostAPIMock, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    mock, ok := r.mocks[name]
    return mock, ok
}
```

3. **Test Runner**
```go
// TestRunner executes tests with proper isolation
type TestRunner struct {
    harness  TestHarness
    reporter TestReporter
}

func (tr *TestRunner) RunTests(ctx context.Context, tests []TestCase) error {
    for _, test := range tests {
        // Setup test environment
        // Execute test
        // Capture results
        // Cleanup
    }
    return tr.reporter.GenerateReport()
}
```

### Language-Specific Support

#### Go Services
```go
// Generated test helper in service package
package service_test

import (
    "testing"
    "github.com/myorg/myservice/service"
    "okra.io/testing"
)

func TestUserCreation(t *testing.T) {
    // Create test harness
    harness := okra.NewTestHarness(t)
    defer harness.Cleanup()
    
    // Mock host APIs
    stateMock := okra.NewStateMock()
    harness.MockHostAPI("okra.state", stateMock)
    
    // Create test service
    svc, err := harness.NewTestService(okra.TestServiceOptions{
        ServicePath: "..",
        CompileMode: okra.CompileModeMock,
    })
    require.NoError(t, err)
    
    // Test the service
    result, err := svc.Invoke(ctx, "CreateUser", &CreateUserInput{
        Name:  "Test User",
        Email: "test@example.com",
    })
    require.NoError(t, err)
    
    // Assert expectations
    assert.Equal(t, "Test User", result.(*User).Name)
    harness.Assert().HostAPICalled("okra.state", "set")
}
```

#### TypeScript Services
```typescript
// Generated test helper
import { TestHarness, mockHostAPIs } from '@okra/testing';
import { UserService } from '../src/service';

describe('UserService', () => {
  let harness: TestHarness;
  let service: UserService;
  
  beforeEach(async () => {
    harness = new TestHarness();
    
    // Mock host APIs
    const stateMock = mockHostAPIs.state();
    harness.mockHostAPI('okra.state', stateMock);
    
    // Create test service
    service = await harness.createTestService({
      servicePath: '..',
      compileMode: 'mock'
    });
  });
  
  afterEach(() => harness.cleanup());
  
  test('creates user successfully', async () => {
    // Arrange
    const input = { name: 'Test User', email: 'test@example.com' };
    
    // Act
    const result = await service.invoke('CreateUser', input);
    
    // Assert
    expect(result.name).toBe('Test User');
    expect(harness.hostAPICalls('okra.state')).toHaveLength(1);
  });
});
```

### Mock Implementations

```go
// Example: Complete state API mock
type StateAPIMock struct {
    mu         sync.RWMutex
    store      map[string]stateEntry
    calls      []HostAPICall
    assertions []assertion
}

type stateEntry struct {
    value   []byte
    version int64
    ttl     *time.Time
}

func (m *StateAPIMock) HandleCall(ctx context.Context, function string, request []byte) ([]byte, error) {
    call := HostAPICall{
        Function:  function,
        Request:   request,
        Timestamp: time.Now(),
    }
    
    var response []byte
    var err error
    
    switch function {
    case "get":
        response, err = m.handleGet(request)
    case "set":
        response, err = m.handleSet(request)
    case "delete":
        response, err = m.handleDelete(request)
    case "list":
        response, err = m.handleList(request)
    default:
        err = fmt.Errorf("unknown function: %s", function)
    }
    
    call.Response = response
    call.Error = err
    call.Duration = time.Since(call.Timestamp)
    
    m.mu.Lock()
    m.calls = append(m.calls, call)
    m.mu.Unlock()
    
    // Check assertions
    m.checkAssertions(call)
    
    return response, err
}
```

## Test Strategy

### Unit Test Cases
```
// Test: Service method with mocked dependencies
// Test: Error handling when host API fails
// Test: Validation of input parameters
// Test: Business logic with various inputs
// Test: State management operations
// Test: Concurrent access handling
```

### Integration Test Cases
```
// Test: Full WASM compilation and execution
// Test: Cross-service communication
// Test: Real host API integration
// Test: Memory management under load
// Test: Error propagation across boundaries
```

### Edge Cases
```
// Test: Host API timeout handling
// Test: Large data serialization
// Test: Unicode and special characters
// Test: Null/undefined handling
// Test: Resource cleanup on panic
```

## Error Handling

### Error Types
1. **Test Discovery Errors** - Can't find tests or parse service
2. **Compilation Errors** - Service fails to compile
3. **Runtime Errors** - Test execution failures
4. **Mock Errors** - Unexpected API calls or failed assertions
5. **Timeout Errors** - Tests taking too long

### Error Propagation
```go
// TestError provides detailed test failure information
type TestError struct {
    TestName string
    Phase    TestPhase // discovery, compilation, execution
    Cause    error
    Stack    []byte
    HostAPICalls []HostAPICall // For debugging
}

func (e *TestError) Error() string {
    return fmt.Sprintf("test %s failed during %s: %v", e.TestName, e.Phase, e.Cause)
}
```

## Performance Considerations

### Scalability
- Parallel test execution by default
- Reuse compiled WASM modules across tests
- Lazy loading of test dependencies

### Resource Usage
- Mock mode: Minimal overhead, no WASM compilation
- WASM mode: Higher memory usage, but isolated per test
- Configurable worker pool for parallel execution

### Optimization Opportunities
- Cache compiled WASM modules
- Share mock implementations across tests
- Incremental test runs based on file changes

## Security Considerations

### Attack Vectors
- Malicious test code trying to access host system
- Resource exhaustion through infinite loops
- Information leakage through error messages

### Mitigations
- Run tests in isolated WASM sandbox
- Enforce resource limits (memory, CPU time)
- Sanitize error output in CI environments
- No network access in unit test mode

## Open Questions

### Design Decisions
1. **Test File Naming** - Should we enforce `*_test.go` or allow `*.test.go`?
2. **Mock Behavior** - Should mocks be strict (fail on unexpected calls) or permissive?
3. **Coverage Integration** - Built-in coverage or integrate with existing tools?

### Trade-offs
1. **Compilation Speed vs Accuracy**
   - Pro: Mock mode is fast
   - Con: May miss WASM-specific issues
   
2. **API Design**
   - Fluent API: More readable but more complex
   - Simple API: Easy to understand but verbose

### Future Considerations
1. **Snapshot Testing** - Capture and compare service outputs
2. **Property-Based Testing** - Generate test inputs automatically
3. **Mutation Testing** - Verify test quality
4. **Visual Test Reports** - HTML reports with graphs

## Implementation Plan

### Week 1: Foundation
- [ ] CLI command structure and argument parsing
- [ ] Test discovery engine for Go and TypeScript
- [ ] Basic test runner without mocks
- [ ] Simple console reporter

### Week 2: Mock Framework  
- [ ] Host API mock interface design
- [ ] Implement mocks for core APIs (log, state, env)
- [ ] Mock registry and routing
- [ ] Assertion helpers

### Week 3: Language Support
- [ ] Go test integration and helpers
- [ ] TypeScript test integration
- [ ] Code generation for test stubs
- [ ] Example tests for templates

### Week 4: Polish
- [ ] Watch mode implementation
- [ ] Coverage reporting
- [ ] Performance optimizations
- [ ] Documentation and tutorials

## Testing the Test Framework

### Unit Tests
- Test discovery with various file structures
- Mock behavior verification
- Error handling in test runner
- Reporter output formatting

### Integration Tests
- Full test runs with example services
- WASM compilation and execution
- Cross-language service testing
- CI/CD integration

## Conclusion

The `okra test` command will provide a comprehensive testing solution specifically designed for OKRA services. By offering both fast unit tests with mocks and thorough integration tests with real WASM execution, developers can confidently build and maintain high-quality services. The familiar testing patterns and language-specific integrations ensure a smooth developer experience while the built-in host API mocks enable thorough testing without external dependencies.