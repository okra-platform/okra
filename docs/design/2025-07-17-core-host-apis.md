# Core Host APIs Implementation Design

## Overview

This design document details the implementation of five foundational host APIs for OKRA: log, state, env, secrets, and fetch. These APIs follow the patterns established in the Host API Interface Design and provide essential capabilities for WASM services.

### Problem Statement

WASM services need secure, performant access to:
- Logging for debugging and observability
- Key-value state storage for persistence
- Environment configuration values
- Secure secret management
- HTTP client capabilities for external API calls

### Goals

1. **Implement the five core host APIs** following the established interface pattern
2. **Provide type-safe guest stubs** for Go and TypeScript
3. **Enforce hybrid policy model** with code-level and CEL-based policies
4. **Enable comprehensive observability** through OpenTelemetry
5. **Ensure production readiness** with proper error handling and resource limits

### Non-Goals

1. Advanced storage backends (initially use in-memory/local implementations)
2. Secret rotation automation (manual rotation only)
3. HTTP/2 or WebSocket support in fetch API
4. Distributed state synchronization
5. Log aggregation or centralized logging

## Detailed API Specifications

### Common Types

All APIs use JSON for cross-boundary communication with these common types:

```go
// HostAPIRequest is the common request wrapper
type HostAPIRequest struct {
    API      string          `json:"api"`      // e.g., "okra.log"
    Method   string          `json:"method"`   // e.g., "write"
    Payload  json.RawMessage `json:"payload"`  // Method-specific payload
}

// HostAPIResponse is the common response wrapper
type HostAPIResponse struct {
    Success bool            `json:"success"`
    Data    json.RawMessage `json:"data,omitempty"`
    Error   *HostAPIError   `json:"error,omitempty"`
}

// HostAPIError represents errors returned to guest code
type HostAPIError struct {
    Code    string                 `json:"code"`    
    Message string                 `json:"message"` 
    Details map[string]interface{} `json:"details,omitempty"`
}
```

### 1. Log API (`okra.log`)

#### Interface

```go
// LogAPI implements structured logging
type LogAPI interface {
    HostAPI
}

// Log method payloads
type LogWriteRequest struct {
    Level     string                 `json:"level"`     // debug, info, warn, error
    Message   string                 `json:"message"`   
    Context   map[string]interface{} `json:"context,omitempty"`
    Timestamp *time.Time            `json:"timestamp,omitempty"`
}

type LogWriteResponse struct {
    // Empty response on success
}
```

#### Implementation

```go
type logAPI struct {
    base   *BaseHostAPI
    logger *slog.Logger
    config LogConfig
}

type LogConfig struct {
    MaxMessageSize int    // Default: 1MB
    MaxContextKeys int    // Default: 100
    MaxContextDepth int   // Default: 10
    AllowedLevels  []string
}

func NewLogAPI(config LogConfig) HostAPI {
    return &logAPI{
        config: config,
    }
}

func (l *logAPI) Name() string { return "okra.log" }
func (l *logAPI) Version() string { return "v1.0.0" }

func (l *logAPI) Functions() []HostFunction {
    return []HostFunction{
        l.base.WrapFunction(&logWriteFunction{api: l}),
    }
}

type logWriteFunction struct {
    api *logAPI
}

func (f *logWriteFunction) Name() string { return "write" }

func (f *logWriteFunction) Execute(ctx context.Context, params []uint64) ([]uint64, error) {
    // Extract request from WASM memory
    reqPtr, reqLen := uint32(params[0]), uint32(params[1])
    reqData, err := f.api.base.memory.ReadBytes(reqPtr, reqLen)
    if err != nil {
        return nil, fmt.Errorf("failed to read request: %w", err)
    }
    
    var req LogWriteRequest
    if err := json.Unmarshal(reqData, &req); err != nil {
        return nil, fmt.Errorf("failed to unmarshal request: %w", err)
    }
    
    // Code-level policy checks
    if err := f.validateRequest(&req); err != nil {
        return f.returnError(err)
    }
    
    // Log the message
    level := parseLevel(req.Level)
    f.api.logger.LogAttrs(ctx, level, req.Message, 
        slog.Any("context", req.Context),
        slog.String("service", f.api.base.serviceName),
    )
    
    // Return success
    return f.returnSuccess(LogWriteResponse{})
}

func (f *logWriteFunction) validateRequest(req *LogWriteRequest) error {
    // Message size check
    if len(req.Message) > f.api.config.MaxMessageSize {
        return fmt.Errorf("message exceeds size limit")
    }
    
    // Level validation
    if !isValidLevel(req.Level, f.api.config.AllowedLevels) {
        return fmt.Errorf("invalid log level: %s", req.Level)
    }
    
    // Context validation
    if err := validateContext(req.Context, f.api.config); err != nil {
        return fmt.Errorf("invalid context: %w", err)
    }
    
    return nil
}
```

### 2. State API (`okra.state`)

#### Interface

```go
// State method payloads
type StateGetRequest struct {
    Key string `json:"key"`
}

type StateGetResponse struct {
    Value   json.RawMessage `json:"value"`
    Version int64          `json:"version"`
    Exists  bool           `json:"exists"`
}

type StateSetRequest struct {
    Key     string          `json:"key"`
    Value   json.RawMessage `json:"value"`
    Version *int64          `json:"version,omitempty"` // For optimistic concurrency
    TTL     *int64          `json:"ttl,omitempty"`     // Seconds
}

type StateSetResponse struct {
    Version int64 `json:"version"`
}

type StateDeleteRequest struct {
    Key     string `json:"key"`
    Version *int64 `json:"version,omitempty"`
}

type StateListRequest struct {
    Prefix string `json:"prefix"`
    Cursor string `json:"cursor,omitempty"`
    Limit  int    `json:"limit,omitempty"` // Default: 100, Max: 1000
}

type StateListResponse struct {
    Keys       []string `json:"keys"`
    NextCursor string   `json:"next_cursor,omitempty"`
    HasMore    bool     `json:"has_more"`
}

type StateIncrementRequest struct {
    Key   string `json:"key"`
    Delta int64  `json:"delta"`
    TTL   *int64 `json:"ttl,omitempty"`
}

type StateIncrementResponse struct {
    Value   int64 `json:"value"`
    Version int64 `json:"version"`
}
```

#### Implementation

```go
type stateAPI struct {
    base    *BaseHostAPI
    backend StateBackend
    config  StateConfig
}

type StateConfig struct {
    MaxKeyLength   int
    MaxValueSize   int  
    MaxTTL         time.Duration
    EnableVersions bool
    KeyPrefix      string // Service-specific prefix
}

// StateBackend abstracts storage implementation
type StateBackend interface {
    Get(ctx context.Context, key string) (*StateEntry, error)
    Set(ctx context.Context, key string, entry *StateEntry) error
    Delete(ctx context.Context, key string, version *int64) error
    List(ctx context.Context, prefix string, cursor string, limit int) (*ListResult, error)
    Increment(ctx context.Context, key string, delta int64, ttl *time.Duration) (*StateEntry, error)
}

type StateEntry struct {
    Value   []byte
    Version int64
    TTL     *time.Time
}

// In-memory implementation for development
type memoryStateBackend struct {
    mu      sync.RWMutex
    data    map[string]*StateEntry
    version int64
}

func (s *stateAPI) Functions() []HostFunction {
    return []HostFunction{
        s.base.WrapFunction(&stateGetFunction{api: s}),
        s.base.WrapFunction(&stateSetFunction{api: s}),
        s.base.WrapFunction(&stateDeleteFunction{api: s}),
        s.base.WrapFunction(&stateListFunction{api: s}),
        s.base.WrapFunction(&stateIncrementFunction{api: s}),
    }
}
```

### 3. Environment API (`okra.env`)

#### Interface

```go
// Env method payloads
type EnvGetRequest struct {
    Key string `json:"key"`
}

type EnvGetResponse struct {
    Value  string `json:"value"`
    Exists bool   `json:"exists"`
}
```

#### Implementation

```go
type envAPI struct {
    base   *BaseHostAPI
    config EnvConfig
    values map[string]string // Cached at initialization
}

type EnvConfig struct {
    MaxKeyLength   int
    AllowedKeys    []string // If set, only these keys allowed
    BlockedKeys    []string // Never allow these keys
    ServiceScoped  bool     // Prefix keys with service name
}

func NewEnvAPI(config EnvConfig) HostAPI {
    api := &envAPI{
        config: config,
        values: make(map[string]string),
    }
    
    // Cache allowed environment variables at startup
    api.loadEnvironment()
    
    return api
}

func (e *envAPI) loadEnvironment() {
    // Load from actual environment or config
    for _, key := range os.Environ() {
        parts := strings.SplitN(key, "=", 2)
        if len(parts) == 2 && e.isAllowedKey(parts[0]) {
            e.values[parts[0]] = parts[1]
        }
    }
}

type envGetFunction struct {
    api *envAPI
}

func (f *envGetFunction) Execute(ctx context.Context, params []uint64) ([]uint64, error) {
    // ... extract and unmarshal request ...
    
    // Code-level validation
    if err := f.validateKey(req.Key); err != nil {
        return f.returnError(err)
    }
    
    // Apply service scoping if configured
    key := req.Key
    if f.api.config.ServiceScoped {
        key = fmt.Sprintf("%s_%s", f.api.base.serviceName, key)
    }
    
    // Get value
    value, exists := f.api.values[key]
    
    return f.returnSuccess(EnvGetResponse{
        Value:  value,
        Exists: exists,
    })
}
```

### 4. Secrets API (`okra.secrets`)

#### Interface

```go
// Secrets method payloads
type SecretsGetRequest struct {
    Key string `json:"key"`
}

type SecretsGetResponse struct {
    Value  string `json:"value"` // Base64 encoded for binary safety
    Exists bool   `json:"exists"`
}
```

#### Implementation

```go
type secretsAPI struct {
    base     *BaseHostAPI
    backend  SecretsBackend
    config   SecretsConfig
}

type SecretsConfig struct {
    MaxKeyLength      int
    RequireEncryption bool
    AuditAllAccess    bool
    ServiceScoped     bool
}

// SecretsBackend abstracts secret storage
type SecretsBackend interface {
    Get(ctx context.Context, key string) ([]byte, error)
}

// Development backend using environment variables
type envSecretsBackend struct {
    prefix string
}

func (s *secretsAPI) Functions() []HostFunction {
    return []HostFunction{
        s.base.WrapFunction(&secretsGetFunction{api: s}),
    }
}

type secretsGetFunction struct {
    api *secretsAPI
}

func (f *secretsGetFunction) Execute(ctx context.Context, params []uint64) ([]uint64, error) {
    // ... extract request ...
    
    // Enhanced validation for secrets
    if err := f.validateSecretKey(req.Key); err != nil {
        // Log attempt but don't reveal details
        f.api.base.logger.Warn("invalid secret access attempt",
            slog.String("service", f.api.base.serviceName),
        )
        return f.returnError(fmt.Errorf("invalid key"))
    }
    
    // Apply scoping
    key := req.Key
    if f.api.config.ServiceScoped {
        key = fmt.Sprintf("%s/%s", f.api.base.serviceName, key)
    }
    
    // Get secret with timing attack prevention
    value, err := f.api.backend.Get(ctx, key)
    if err != nil {
        // Don't distinguish between not found and other errors
        return f.returnSuccess(SecretsGetResponse{
            Value:  "",
            Exists: false,
        })
    }
    
    // Audit if configured
    if f.api.config.AuditAllAccess {
        f.api.base.logger.Info("secret accessed",
            slog.String("service", f.api.base.serviceName),
            slog.String("key", key),
        )
    }
    
    return f.returnSuccess(SecretsGetResponse{
        Value:  base64.StdEncoding.EncodeToString(value),
        Exists: true,
    })
}
```

### 5. HTTP Fetch API (`okra.fetch`)

#### Interface

```go
// Fetch method payloads
type FetchRequest struct {
    URL     string              `json:"url"`
    Method  string              `json:"method,omitempty"`  // Default: GET
    Headers map[string]string   `json:"headers,omitempty"`
    Body    string              `json:"body,omitempty"`    // Base64 for binary
    Timeout int64               `json:"timeout,omitempty"` // Milliseconds
}

type FetchResponse struct {
    Status     int               `json:"status"`
    StatusText string            `json:"statusText"`
    Headers    map[string]string `json:"headers"`
    Body       string            `json:"body"` // Base64 encoded
}
```

#### Implementation

```go
type fetchAPI struct {
    base       *BaseHostAPI
    httpClient *http.Client
    config     FetchConfig
}

type FetchConfig struct {
    MaxBodySize      int64
    MaxRedirects     int
    DefaultTimeout   time.Duration
    AllowedProtocols []string
    RequireHTTPS     bool
}

func NewFetchAPI(config FetchConfig) HostAPI {
    return &fetchAPI{
        config: config,
        httpClient: &http.Client{
            CheckRedirect: func(req *http.Request, via []*http.Request) error {
                if len(via) >= config.MaxRedirects {
                    return fmt.Errorf("too many redirects")
                }
                return nil
            },
        },
    }
}

type fetchRequestFunction struct {
    api *fetchAPI
}

func (f *fetchRequestFunction) Execute(ctx context.Context, params []uint64) ([]uint64, error) {
    // ... extract request ...
    
    // Validate request
    if err := f.validateRequest(&req); err != nil {
        return f.returnError(err)
    }
    
    // Parse URL
    parsedURL, err := url.Parse(req.URL)
    if err != nil {
        return f.returnError(fmt.Errorf("invalid URL"))
    }
    
    // Protocol validation
    if f.api.config.RequireHTTPS && parsedURL.Scheme != "https" {
        return f.returnError(fmt.Errorf("HTTPS required"))
    }
    
    // Create HTTP request
    body := []byte(nil)
    if req.Body != "" {
        body, err = base64.StdEncoding.DecodeString(req.Body)
        if err != nil {
            return f.returnError(fmt.Errorf("invalid body encoding"))
        }
    }
    
    httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewReader(body))
    if err != nil {
        return f.returnError(err)
    }
    
    // Set headers (with injection protection)
    for k, v := range req.Headers {
        if isProtectedHeader(k) {
            continue
        }
        httpReq.Header.Set(k, v)
    }
    
    // Apply timeout
    timeout := f.api.config.DefaultTimeout
    if req.Timeout > 0 {
        timeout = time.Duration(req.Timeout) * time.Millisecond
    }
    
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    
    // Make request
    resp, err := f.api.httpClient.Do(httpReq.WithContext(ctx))
    if err != nil {
        return f.returnError(fmt.Errorf("request failed: %w", err))
    }
    defer resp.Body.Close()
    
    // Read response with size limit
    bodyReader := io.LimitReader(resp.Body, f.api.config.MaxBodySize)
    respBody, err := io.ReadAll(bodyReader)
    if err != nil {
        return f.returnError(fmt.Errorf("failed to read response"))
    }
    
    // Build response
    headers := make(map[string]string)
    for k, v := range resp.Header {
        headers[k] = v[0] // Simplify multi-value headers
    }
    
    return f.returnSuccess(FetchResponse{
        Status:     resp.StatusCode,
        StatusText: resp.Status,
        Headers:    headers,
        Body:       base64.StdEncoding.EncodeToString(respBody),
    })
}
```

## Guest-Side Stub Generation

### Go Stubs

```go
// Generated okra package for guest code
package okra

import (
    "encoding/json"
    "fmt"
)

// Log provides structured logging
type Log struct{}

func (l *Log) Debug(message string, context map[string]interface{}) error {
    return l.write("debug", message, context)
}

func (l *Log) Info(message string, context map[string]interface{}) error {
    return l.write("info", message, context)
}

func (l *Log) Warn(message string, context map[string]interface{}) error {
    return l.write("warn", message, context)
}

func (l *Log) Error(message string, context map[string]interface{}) error {
    return l.write("error", message, context)
}

func (l *Log) write(level, message string, context map[string]interface{}) error {
    req := LogWriteRequest{
        Level:   level,
        Message: message,
        Context: context,
    }
    
    resp, err := callHostAPI("okra.log", "write", req)
    if err != nil {
        return err
    }
    
    if !resp.Success {
        return fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
    }
    
    return nil
}

// State provides key-value storage
type State struct{}

func (s *State) Get(key string) ([]byte, int64, error) {
    req := StateGetRequest{Key: key}
    resp, err := callHostAPI("okra.state", "get", req)
    if err != nil {
        return nil, 0, err
    }
    
    if !resp.Success {
        return nil, 0, fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
    }
    
    var result StateGetResponse
    if err := json.Unmarshal(resp.Data, &result); err != nil {
        return nil, 0, err
    }
    
    if !result.Exists {
        return nil, 0, ErrNotFound
    }
    
    return result.Value, result.Version, nil
}

// ... similar implementations for Set, Delete, List, Increment ...

// Helper function to call host APIs
func callHostAPI(api, method string, payload interface{}) (*HostAPIResponse, error) {
    // Serialize request
    payloadData, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }
    
    req := HostAPIRequest{
        API:     api,
        Method:  method,
        Payload: payloadData,
    }
    
    reqData, err := json.Marshal(req)
    if err != nil {
        return nil, err
    }
    
    // Call WASM host function
    respData := hostCall(reqData)
    
    // Parse response
    var resp HostAPIResponse
    if err := json.Unmarshal(respData, &resp); err != nil {
        return nil, err
    }
    
    return &resp, nil
}

// Low-level WASM imports
//go:wasm-module okra
//export host_call
func hostCallImport(ptr, len uint32) uint32

func hostCall(data []byte) []byte {
    // Allocate memory and copy data
    ptr := allocate(uint32(len(data)))
    copy(memorySlice(ptr, uint32(len(data))), data)
    
    // Call host
    respPtr := hostCallImport(ptr, uint32(len(data)))
    
    // Read response length (first 4 bytes)
    respLen := *(*uint32)(unsafe.Pointer(uintptr(respPtr)))
    
    // Read response data
    respData := make([]byte, respLen)
    copy(respData, memorySlice(respPtr+4, respLen))
    
    // Free memory
    deallocate(ptr)
    deallocate(respPtr)
    
    return respData
}
```

### TypeScript Stubs

```typescript
// Generated okra package for TypeScript
export namespace okra {
    // Log API
    export class Log {
        async debug(message: string, context?: Record<string, any>): Promise<void> {
            return this.write('debug', message, context);
        }
        
        async info(message: string, context?: Record<string, any>): Promise<void> {
            return this.write('info', message, context);
        }
        
        async warn(message: string, context?: Record<string, any>): Promise<void> {
            return this.write('warn', message, context);
        }
        
        async error(message: string, context?: Record<string, any>): Promise<void> {
            return this.write('error', message, context);
        }
        
        private async write(level: string, message: string, context?: Record<string, any>): Promise<void> {
            const response = await callHostAPI('okra.log', 'write', {
                level,
                message,
                context: context || {}
            });
            
            if (!response.success) {
                throw new Error(`${response.error.code}: ${response.error.message}`);
            }
        }
    }
    
    // State API
    export class State {
        async get<T = any>(key: string): Promise<{ value: T; version: number } | null> {
            const response = await callHostAPI('okra.state', 'get', { key });
            
            if (!response.success) {
                throw new Error(`${response.error.code}: ${response.error.message}`);
            }
            
            const result = response.data as StateGetResponse;
            if (!result.exists) {
                return null;
            }
            
            return {
                value: JSON.parse(atob(result.value)) as T,
                version: result.version
            };
        }
        
        async set<T = any>(key: string, value: T, options?: { version?: number; ttl?: number }): Promise<number> {
            const response = await callHostAPI('okra.state', 'set', {
                key,
                value: btoa(JSON.stringify(value)),
                ...options
            });
            
            if (!response.success) {
                throw new Error(`${response.error.code}: ${response.error.message}`);
            }
            
            return (response.data as StateSetResponse).version;
        }
        
        // ... other methods ...
    }
    
    // Helper to call host APIs
    async function callHostAPI(api: string, method: string, payload: any): Promise<HostAPIResponse> {
        const request: HostAPIRequest = {
            api,
            method,
            payload: JSON.stringify(payload)
        };
        
        // Call WASM host function
        const response = await hostCall(JSON.stringify(request));
        return JSON.parse(response) as HostAPIResponse;
    }
    
    // WASM imports
    declare function hostCall(request: string): Promise<string>;
}
```

## Testing Strategy

### Unit Tests

1. **Individual Host APIs**
   ```go
   func TestLogAPI_Write(t *testing.T) {
       // Test successful logging
       // Test invalid log levels
       // Test message size limits
       // Test context validation
   }
   
   func TestStateAPI_OptimisticConcurrency(t *testing.T) {
       // Test version conflicts
       // Test successful updates
   }
   ```

2. **Policy Enforcement**
   ```go
   func TestHostAPI_PolicyEnforcement(t *testing.T) {
       // Test CEL policy evaluation
       // Test code-level policy checks
       // Test policy caching
   }
   ```

3. **Memory Management**
   ```go
   func TestMemoryAccessor_Boundaries(t *testing.T) {
       // Test reading past memory bounds
       // Test large allocations
       // Test memory cleanup
   }
   ```

### Integration Tests

1. **End-to-End WASM Tests**
   - Compile test WASM modules that use each API
   - Verify correct behavior across WASM boundary
   - Test error propagation

2. **Performance Tests**
   - Measure call overhead for each API
   - Test under concurrent load
   - Memory usage profiling

3. **Policy Integration**
   - Test complex CEL expressions
   - Verify audit logging
   - Test rate limiting

## Implementation Plan

### Phase 1: Foundation (Week 1)
1. Implement base host API infrastructure
2. Create memory accessor utilities
3. Set up host API registry
4. Implement JSON encoding/decoding

### Phase 2: Core APIs (Week 2)
1. Implement Log API
2. Implement Env API
3. Implement State API (in-memory backend)
4. Write unit tests

### Phase 3: Advanced APIs (Week 3)
1. Implement Secrets API
2. Implement Fetch API
3. Add policy enforcement
4. Integration testing

### Phase 4: Guest Stubs & Polish (Week 4)
1. Generate Go stubs
2. Generate TypeScript stubs
3. Performance optimization
4. Documentation

## Security Considerations

### Input Validation
- All inputs validated before processing
- Size limits strictly enforced
- Injection attacks prevented

### Resource Limits
- Request/response size limits
- Rate limiting per service
- Timeout enforcement

### Audit Trail
- All host API calls logged
- Security-sensitive operations audited
- Metrics for monitoring

## Performance Targets

- **Host API call overhead**: < 100μs
- **JSON serialization**: < 50μs for typical payloads
- **Memory allocation**: Pooled to minimize GC pressure
- **Concurrent calls**: Support 10k+ calls/second per worker

## Future Enhancements

1. **Backend Implementations**
   - Redis for distributed state
   - Vault for secrets management
   - Structured logging backends

2. **Advanced Features**
   - Batch operations for efficiency
   - Streaming for large data
   - WebSocket support in fetch

3. **Developer Experience**
   - IDE integrations
   - Debugging tools
   - Performance profiling