# Core Host APIs Implementation Design

## Overview

This design document details the implementation of five foundational host APIs for OKRA: log, state, env, secrets, and fetch. These APIs follow the patterns established in the [Host API Interface Design](./2025-01-15-host-api-interface.md) and provide essential capabilities for WASM services.

**Note**: This document builds upon the host API infrastructure already implemented in the `internal/hostapi` package. All type definitions and interfaces referenced here are defined in that package.

## Key Changes from Original Design

Based on the implemented host API infrastructure, this design has been updated to:

1. **Use the unified host API interface** - All APIs go through `run_host_api` and `next` functions instead of individual host functions
2. **Follow the factory pattern** - Each API has a factory that creates service-specific instances
3. **Reference existing types** - No duplicate type definitions; all common types are in `internal/hostapi`
4. **Use proper error constants** - Error codes use the predefined constants like `ErrorCodeInternalError`
5. **Follow JSON naming conventions** - All JSON fields use camelCase per project conventions

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

All APIs use JSON for cross-boundary communication. The common types are already defined in the `internal/hostapi` package:

- `HostAPIRequest` - Unified request format with API name, method, and parameters
- `HostAPIResponse` - Unified response with success flag, data, and error
- `HostAPIError` - Structured error with code, message, and details
- `RequestMetadata` - Trace context and service information

For the complete type definitions, see `internal/hostapi/api.go` and `internal/hostapi/errors.go`.

### 1. Log API (`okra.log`)

#### Request/Response Types

```go
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

#### Factory Implementation

```go
// Factory implementation for log API
type logAPIFactory struct{}

func NewLogAPIFactory() hostapi.HostAPIFactory {
    return &logAPIFactory{}
}

func (f *logAPIFactory) Name() string { return "okra.log" }
func (f *logAPIFactory) Version() string { return "v1.0.0" }

func (f *logAPIFactory) Create(ctx context.Context, config hostapi.HostAPIConfig) (hostapi.HostAPI, error) {
    // Create service-specific logger
    logger := config.Logger.With(
        slog.String("api", "okra.log"),
        slog.String("service", config.ServiceName),
    )
    
    return &logAPI{
        logger:      logger,
        serviceName: config.ServiceName,
        config:      defaultLogConfig(),
    }, nil
}

func (f *logAPIFactory) Methods() []hostapi.MethodMetadata {
    return []hostapi.MethodMetadata{
        {
            Name:        "write",
            Description: "Write a structured log message",
            Parameters: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "level":     *spec.StringProperty().WithEnum("debug", "info", "warn", "error"),
                    "message":   *spec.StringProperty().WithDescription("Log message"),
                    "context":   *spec.ObjectProperty().WithDescription("Additional context"),
                    "timestamp": *spec.StringProperty().WithFormat("date-time"),
                }).
                WithRequired("level", "message"),
            Returns: spec.ObjectProperty(), // Empty object
            Errors: []hostapi.ErrorMetadata{
                {Code: "INVALID_LEVEL", Description: "Invalid log level"},
                {Code: "MESSAGE_TOO_LARGE", Description: "Message exceeds size limit"},
            },
        },
    }
}

// Instance implementation
type logAPI struct {
    logger      *slog.Logger
    serviceName string
    config      LogConfig
}

type LogConfig struct {
    MaxMessageSize  int      // Default: 1MB
    MaxContextKeys  int      // Default: 100
    MaxContextDepth int      // Default: 10
    AllowedLevels   []string // Default: all levels
}

func (l *logAPI) Name() string    { return "okra.log" }
func (l *logAPI) Version() string { return "v1.0.0" }

func (l *logAPI) Execute(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
    switch method {
    case "write":
        var req LogWriteRequest
        if err := json.Unmarshal(params, &req); err != nil {
            return nil, &hostapi.HostAPIError{
                Code:    "INVALID_PARAMETERS",
                Message: "Failed to parse log write request",
                Details: err.Error(),
            }
        }
        
        // Validate request
        if err := l.validateRequest(&req); err != nil {
            return nil, err
        }
        
        // Log the message
        level := parseLevel(req.Level)
        l.logger.LogAttrs(ctx, level, req.Message, 
            slog.Any("context", req.Context),
        )
        
        // Return empty success response
        return json.Marshal(LogWriteResponse{})
        
    default:
        return nil, &hostapi.HostAPIError{
            Code:    "METHOD_NOT_FOUND",
            Message: fmt.Sprintf("Unknown method: %s", method),
        }
    }
}

func (l *logAPI) validateRequest(req *LogWriteRequest) error {
    // Message size check
    if len(req.Message) > l.config.MaxMessageSize {
        return &hostapi.HostAPIError{
            Code:    "MESSAGE_TOO_LARGE",
            Message: fmt.Sprintf("Message size %d exceeds limit %d", len(req.Message), l.config.MaxMessageSize),
        }
    }
    
    // Level validation
    if !isValidLevel(req.Level, l.config.AllowedLevels) {
        return &hostapi.HostAPIError{
            Code:    "INVALID_LEVEL",
            Message: fmt.Sprintf("Invalid log level: %s", req.Level),
        }
    }
    
    // Context validation
    if len(req.Context) > l.config.MaxContextKeys {
        return &hostapi.HostAPIError{
            Code:    "CONTEXT_TOO_LARGE",
            Message: fmt.Sprintf("Context has %d keys, exceeds limit %d", len(req.Context), l.config.MaxContextKeys),
        }
    }
    
    return nil
}
```

### 2. State API (`okra.state`)

#### Request/Response Types

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
    NextCursor string   `json:"nextCursor,omitempty"` // camelCase per convention
    HasMore    bool     `json:"hasMore"`              // camelCase per convention
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

#### Factory Implementation

```go
// Factory implementation for state API
type stateAPIFactory struct{}

func NewStateAPIFactory() hostapi.HostAPIFactory {
    return &stateAPIFactory{}
}

func (f *stateAPIFactory) Name() string { return "okra.state" }
func (f *stateAPIFactory) Version() string { return "v1.0.0" }

func (f *stateAPIFactory) Create(ctx context.Context, config hostapi.HostAPIConfig) (hostapi.HostAPI, error) {
    // Create service-specific state store
    store, err := createStateStore(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create state store: %w", err)
    }
    
    return &stateAPI{
        store:       store,
        logger:      config.Logger.With("api", "okra.state", "service", config.ServiceName),
        serviceName: config.ServiceName,
        config:      defaultStateConfig(),
    }, nil
}

func (f *stateAPIFactory) Methods() []hostapi.MethodMetadata {
    return []hostapi.MethodMetadata{
        {
            Name:        "get",
            Description: "Retrieve a value from state storage",
            Parameters: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "key": *spec.StringProperty().WithDescription("The key to retrieve"),
                }).
                WithRequired("key"),
            Returns: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "value":   *spec.StringProperty().WithFormat("byte"),
                    "version": *spec.IntegerProperty(),
                    "exists":  *spec.BoolProperty(),
                }),
            Errors: []hostapi.ErrorMetadata{
                {Code: "KEY_TOO_LONG", Description: "Key exceeds maximum length"},
            },
        },
        {
            Name:        "set",
            Description: "Store a value in state storage",
            Parameters: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "key":     *spec.StringProperty(),
                    "value":   *spec.StringProperty().WithFormat("byte"),
                    "version": *spec.IntegerProperty().WithDescription("For optimistic concurrency"),
                    "ttl":     *spec.IntegerProperty().WithDescription("TTL in seconds"),
                }).
                WithRequired("key", "value"),
            Returns: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "version": *spec.IntegerProperty(),
                }),
            Errors: []hostapi.ErrorMetadata{
                {Code: "VERSION_CONFLICT", Description: "Version mismatch for optimistic concurrency"},
                {Code: "VALUE_TOO_LARGE", Description: "Value exceeds maximum size"},
            },
        },
        {
            Name:        "listKeys",
            Description: "List all keys matching a prefix",
            Streaming:   true, // This method returns an iterator
            Parameters: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "prefix": *spec.StringProperty(),
                    "limit":  *spec.IntegerProperty().WithMaximum(1000, false),
                }).
                WithRequired("prefix"),
            Returns: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "iteratorId": *spec.StringProperty(),
                    "hasData":    *spec.BoolProperty(),
                }),
        },
    }
}

// Instance implementation
type stateAPI struct {
    store       StateStore
    logger      *slog.Logger
    serviceName string
    config      StateConfig
}

type StateConfig struct {
    MaxKeyLength   int
    MaxValueSize   int  
    MaxTTL         time.Duration
    EnableVersions bool
    KeyPrefix      string // Service-specific prefix
}

// StateStore abstracts storage implementation
type StateStore interface {
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

func (s *stateAPI) Name() string    { return "okra.state" }
func (s *stateAPI) Version() string { return "v1.0.0" }

func (s *stateAPI) Execute(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
    // For streaming methods, we need to implement StreamingHostAPI
    // This is shown for completeness - actual implementation would use ExecuteStreaming
    switch method {
    case "get":
        return s.executeGet(ctx, params)
    case "set":
        return s.executeSet(ctx, params)
    case "delete":
        return s.executeDelete(ctx, params)
    case "increment":
        return s.executeIncrement(ctx, params)
    default:
        return nil, &hostapi.HostAPIError{
            Code:    hostapi.ErrorCodeInternalError,
            Message: fmt.Sprintf("Unknown method: %s", method),
        }
    }
}

// ExecuteStreaming implements StreamingHostAPI for listKeys
func (s *stateAPI) ExecuteStreaming(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, hostapi.Iterator, error) {
    switch method {
    case "listKeys":
        return s.executeListKeys(ctx, params)
    default:
        // Non-streaming methods fall back to Execute
        result, err := s.Execute(ctx, method, params)
        return result, nil, err
    }
}
```

### 3. Environment API (`okra.env`)

#### Request/Response Types

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

#### Factory Implementation

```go
// Factory implementation for env API
type envAPIFactory struct{}

func NewEnvAPIFactory() hostapi.HostAPIFactory {
    return &envAPIFactory{}
}

func (f *envAPIFactory) Name() string { return "okra.env" }
func (f *envAPIFactory) Version() string { return "v1.0.0" }

func (f *envAPIFactory) Create(ctx context.Context, config hostapi.HostAPIConfig) (hostapi.HostAPI, error) {
    api := &envAPI{
        serviceName: config.ServiceName,
        logger:      config.Logger.With("api", "okra.env"),
        config:      defaultEnvConfig(),
        values:      make(map[string]string),
    }
    
    // Cache allowed environment variables at startup
    api.loadEnvironment()
    
    return api, nil
}

func (f *envAPIFactory) Methods() []hostapi.MethodMetadata {
    return []hostapi.MethodMetadata{
        {
            Name:        "get",
            Description: "Get environment variable value",
            Parameters: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "key": *spec.StringProperty().WithDescription("Environment variable name"),
                }).
                WithRequired("key"),
            Returns: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "value":  *spec.StringProperty(),
                    "exists": *spec.BoolProperty(),
                }),
            Errors: []hostapi.ErrorMetadata{
                {Code: "KEY_NOT_ALLOWED", Description: "Key is not in allowed list"},
                {Code: "KEY_BLOCKED", Description: "Key is explicitly blocked"},
            },
        },
    }
}

// Instance implementation  
type envAPI struct {
    serviceName string
    logger      *slog.Logger
    config      EnvConfig
    values      map[string]string // Cached at initialization
}

type EnvConfig struct {
    MaxKeyLength  int
    AllowedKeys   []string // If set, only these keys allowed
    BlockedKeys   []string // Never allow these keys
    ServiceScoped bool     // Prefix keys with service name
}

func (e *envAPI) Execute(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
    switch method {
    case "get":
        var req EnvGetRequest
        if err := json.Unmarshal(params, &req); err != nil {
            return nil, &hostapi.HostAPIError{
                Code:    "INVALID_PARAMETERS",
                Message: "Failed to parse env get request",
                Details: err.Error(),
            }
        }
        
        // Validate key
        if err := e.validateKey(req.Key); err != nil {
            return nil, err
        }
        
        // Apply service scoping if configured
        key := req.Key
        if e.config.ServiceScoped {
            key = fmt.Sprintf("%s_%s", e.serviceName, key)
        }
        
        // Get value
        value, exists := e.values[key]
        
        return json.Marshal(EnvGetResponse{
            Value:  value,
            Exists: exists,
        })
        
    default:
        return nil, &hostapi.HostAPIError{
            Code:    "METHOD_NOT_FOUND",
            Message: fmt.Sprintf("Unknown method: %s", method),
        }
    }
}
```

### 4. Secrets API (`okra.secrets`)

#### Request/Response Types

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

#### Factory Implementation

```go
// Factory implementation for secrets API
type secretsAPIFactory struct{}

func NewSecretsAPIFactory() hostapi.HostAPIFactory {
    return &secretsAPIFactory{}
}

func (f *secretsAPIFactory) Name() string { return "okra.secrets" }
func (f *secretsAPIFactory) Version() string { return "v1.0.0" }

func (f *secretsAPIFactory) Create(ctx context.Context, config hostapi.HostAPIConfig) (hostapi.HostAPI, error) {
    // Create secrets backend based on environment
    backend, err := createSecretsBackend(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create secrets backend: %w", err)
    }
    
    return &secretsAPI{
        backend:     backend,
        serviceName: config.ServiceName,
        logger:      config.Logger.With("api", "okra.secrets"),
        config:      defaultSecretsConfig(),
    }, nil
}

func (f *secretsAPIFactory) Methods() []hostapi.MethodMetadata {
    return []hostapi.MethodMetadata{
        {
            Name:        "get",
            Description: "Retrieve a secret value",
            Parameters: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "key": *spec.StringProperty().WithDescription("Secret key"),
                }).
                WithRequired("key"),
            Returns: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "value":  *spec.StringProperty().WithFormat("byte"),
                    "exists": *spec.BoolProperty(),
                }),
            Errors: []hostapi.ErrorMetadata{
                {Code: "INVALID_KEY", Description: "Secret key is invalid"},
            },
        },
    }
}

// Instance implementation
type secretsAPI struct {
    backend     SecretsBackend
    serviceName string
    logger      *slog.Logger
    config      SecretsConfig
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

func (s *secretsAPI) Execute(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
    switch method {
    case "get":
        var req SecretsGetRequest
        if err := json.Unmarshal(params, &req); err != nil {
            return nil, &hostapi.HostAPIError{
                Code:    "INVALID_PARAMETERS",
                Message: "Failed to parse secrets get request",
            }
        }
        
        // Enhanced validation for secrets
        if err := s.validateSecretKey(req.Key); err != nil {
            // Log attempt but don't reveal details
            s.logger.Warn("invalid secret access attempt",
                slog.String("service", s.serviceName),
            )
            return nil, &hostapi.HostAPIError{
                Code:    "INVALID_KEY",
                Message: "Invalid secret key",
            }
        }
        
        // Apply scoping
        key := req.Key
        if s.config.ServiceScoped {
            key = fmt.Sprintf("%s/%s", s.serviceName, key)
        }
        
        // Get secret with timing attack prevention
        value, err := s.backend.Get(ctx, key)
        if err != nil {
            // Don't distinguish between not found and other errors
            return json.Marshal(SecretsGetResponse{
                Value:  "",
                Exists: false,
            })
        }
        
        // Audit if configured
        if s.config.AuditAllAccess {
            s.logger.Info("secret accessed",
                slog.String("service", s.serviceName),
                slog.String("key", key),
            )
        }
        
        return json.Marshal(SecretsGetResponse{
            Value:  base64.StdEncoding.EncodeToString(value),
            Exists: true,
        })
        
    default:
        return nil, &hostapi.HostAPIError{
            Code:    "METHOD_NOT_FOUND",
            Message: fmt.Sprintf("Unknown method: %s", method),
        }
    }
}
```

### 5. HTTP Fetch API (`okra.fetch`)

#### Request/Response Types

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

#### Factory Implementation

```go
// Factory implementation for fetch API
type fetchAPIFactory struct{}

func NewFetchAPIFactory() hostapi.HostAPIFactory {
    return &fetchAPIFactory{}
}

func (f *fetchAPIFactory) Name() string { return "okra.fetch" }
func (f *fetchAPIFactory) Version() string { return "v1.0.0" }

func (f *fetchAPIFactory) Create(ctx context.Context, config hostapi.HostAPIConfig) (hostapi.HostAPI, error) {
    fetchConfig := defaultFetchConfig()
    
    return &fetchAPI{
        serviceName: config.ServiceName,
        logger:      config.Logger.With("api", "okra.fetch"),
        config:      fetchConfig,
        httpClient: &http.Client{
            CheckRedirect: func(req *http.Request, via []*http.Request) error {
                if len(via) >= fetchConfig.MaxRedirects {
                    return fmt.Errorf("too many redirects")
                }
                return nil
            },
        },
    }, nil
}

func (f *fetchAPIFactory) Methods() []hostapi.MethodMetadata {
    return []hostapi.MethodMetadata{
        {
            Name:        "request",
            Description: "Make an HTTP request",
            Parameters: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "url":     *spec.StringProperty().WithFormat("uri"),
                    "method":  *spec.StringProperty().WithEnum("GET", "POST", "PUT", "DELETE", "PATCH"),
                    "headers": *spec.ObjectProperty().WithAdditionalProperties(&spec.Schema{Type: []string{"string"}}),
                    "body":    *spec.StringProperty().WithFormat("byte"),
                    "timeout": *spec.IntegerProperty().WithDescription("Timeout in milliseconds"),
                }).
                WithRequired("url"),
            Returns: spec.ObjectProperty().
                WithProperties(map[string]spec.Schema{
                    "status":     *spec.IntegerProperty(),
                    "statusText": *spec.StringProperty(),
                    "headers":    *spec.ObjectProperty().WithAdditionalProperties(&spec.Schema{Type: []string{"string"}}),
                    "body":       *spec.StringProperty().WithFormat("byte"),
                }),
            Errors: []hostapi.ErrorMetadata{
                {Code: "INVALID_URL", Description: "URL is malformed"},
                {Code: "HTTPS_REQUIRED", Description: "HTTPS is required but URL uses HTTP"},
                {Code: "REQUEST_FAILED", Description: "HTTP request failed"},
                {Code: "BODY_TOO_LARGE", Description: "Response body exceeds size limit"},
            },
        },
    }
}

// Instance implementation
type fetchAPI struct {
    serviceName string
    logger      *slog.Logger
    httpClient  *http.Client
    config      FetchConfig
}

type FetchConfig struct {
    MaxBodySize      int64
    MaxRedirects     int
    DefaultTimeout   time.Duration
    AllowedProtocols []string
    RequireHTTPS     bool
}

func (f *fetchAPI) Name() string    { return "okra.fetch" }
func (f *fetchAPI) Version() string { return "v1.0.0" }

func (f *fetchAPI) Execute(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
    switch method {
    case "request":
        var req FetchRequest
        if err := json.Unmarshal(params, &req); err != nil {
            return nil, &hostapi.HostAPIError{
                Code:    "INVALID_PARAMETERS",
                Message: "Failed to parse fetch request",
                Details: err.Error(),
            }
        }
        
        // Default method to GET
        if req.Method == "" {
            req.Method = "GET"
        }
        
        // Validate request
        if err := f.validateRequest(&req); err != nil {
            return nil, err
        }
        
        // Parse URL
        parsedURL, err := url.Parse(req.URL)
        if err != nil {
            return nil, &hostapi.HostAPIError{
                Code:    "INVALID_URL",
                Message: "Invalid URL format",
                Details: err.Error(),
            }
        }
        
        // Protocol validation
        if f.config.RequireHTTPS && parsedURL.Scheme != "https" {
            return nil, &hostapi.HostAPIError{
                Code:    "HTTPS_REQUIRED",
                Message: "HTTPS is required for fetch requests",
            }
        }
        
        // Prepare request body
        var body io.Reader
        if req.Body != "" {
            bodyBytes, err := base64.StdEncoding.DecodeString(req.Body)
            if err != nil {
                return nil, &hostapi.HostAPIError{
                    Code:    "INVALID_BODY",
                    Message: "Failed to decode request body",
                }
            }
            body = bytes.NewReader(bodyBytes)
        }
        
        // Create HTTP request
        httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
        if err != nil {
            return nil, &hostapi.HostAPIError{
                Code:    "REQUEST_FAILED",
                Message: "Failed to create HTTP request",
                Details: err.Error(),
            }
        }
        
        // Set headers (with injection protection)
        for k, v := range req.Headers {
            if !isProtectedHeader(k) {
                httpReq.Header.Set(k, v)
            }
        }
        
        // Apply timeout
        timeout := f.config.DefaultTimeout
        if req.Timeout > 0 {
            timeout = time.Duration(req.Timeout) * time.Millisecond
        }
        
        ctx, cancel := context.WithTimeout(ctx, timeout)
        defer cancel()
        
        // Make request
        resp, err := f.httpClient.Do(httpReq.WithContext(ctx))
        if err != nil {
            return nil, &hostapi.HostAPIError{
                Code:    "REQUEST_FAILED",
                Message: "HTTP request failed",
                Details: err.Error(),
            }
        }
        defer resp.Body.Close()
        
        // Read response with size limit
        bodyReader := io.LimitReader(resp.Body, f.config.MaxBodySize)
        respBody, err := io.ReadAll(bodyReader)
        if err != nil {
            return nil, &hostapi.HostAPIError{
                Code:    "REQUEST_FAILED",
                Message: "Failed to read response body",
                Details: err.Error(),
            }
        }
        
        // Build response
        headers := make(map[string]string)
        for k, v := range resp.Header {
            headers[k] = v[0] // Simplify multi-value headers
        }
        
        return json.Marshal(FetchResponse{
            Status:     resp.StatusCode,
            StatusText: resp.Status,
            Headers:    headers,
            Body:       base64.StdEncoding.EncodeToString(respBody),
        })
        
    default:
        return nil, &hostapi.HostAPIError{
            Code:    "METHOD_NOT_FOUND",
            Message: fmt.Sprintf("Unknown method: %s", method),
        }
    }
}
```

## Guest-Side Stub Generation

### Go Stubs

Guest-side stubs use the unified host API interface. Here's an example for the log API:

```go
// Generated package for okra.log
package log

import (
    "encoding/json"
    "fmt"
    "github.com/okra/sdk/go/hostapi" // Common types
)

// Client provides access to the okra.log host API
type Client struct{}

// NewClient creates a new log API client
func NewClient() *Client {
    return &Client{}
}

// Debug writes a debug log message
func (c *Client) Debug(message string, context map[string]interface{}) error {
    return c.write("debug", message, context)
}

// Info writes an info log message
func (c *Client) Info(message string, context map[string]interface{}) error {
    return c.write("info", message, context)
}

// Warn writes a warning log message
func (c *Client) Warn(message string, context map[string]interface{}) error {
    return c.write("warn", message, context)
}

// Error writes an error log message
func (c *Client) Error(message string, context map[string]interface{}) error {
    return c.write("error", message, context)
}

func (c *Client) write(level, message string, context map[string]interface{}) error {
    params := LogWriteRequest{
        Level:   level,
        Message: message,
        Context: context,
    }
    
    paramsJSON, err := json.Marshal(params)
    if err != nil {
        return fmt.Errorf("failed to marshal parameters: %w", err)
    }
    
    req := hostapi.HostAPIRequest{
        API:        "okra.log",
        Method:     "write",
        Parameters: paramsJSON,
        Metadata:   hostapi.RequestMetadata{}, // Populated by runtime
    }
    
    reqJSON, err := json.Marshal(req)
    if err != nil {
        return fmt.Errorf("failed to marshal request: %w", err)
    }
    
    respJSON, err := hostapi.CallHost(string(reqJSON))
    if err != nil {
        return err
    }
    
    var resp hostapi.HostAPIResponse
    if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
        return fmt.Errorf("invalid response format: %w", err)
    }
    
    if !resp.Success {
        return fmt.Errorf("%s: %s", resp.Error.Code, resp.Error.Message)
    }
    
    return nil
}

// Import the host functions from the "okra" module
//go:wasmimport okra run_host_api
func runHostAPI(requestPtr, requestLen uint32) (responsePtr uint32, responseLen uint32)

//go:wasmimport okra next
func next(requestPtr, requestLen uint32) (responsePtr uint32, responseLen uint32)

// hostapi.CallHost is implemented by the SDK to handle memory management
// It uses the allocate/deallocate functions exported by the main WASM module
```

### TypeScript Stubs

```typescript
// Generated package for okra.log
import { HostAPIRequest, HostAPIResponse, callHost } from '@okra/sdk';

export interface LogWriteRequest {
    level: 'debug' | 'info' | 'warn' | 'error';
    message: string;
    context?: Record<string, any>;
    timestamp?: string;
}

export class LogClient {
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
        const params: LogWriteRequest = {
            level: level as any,
            message,
            context
        };
        
        const request: HostAPIRequest = {
            api: 'okra.log',
            method: 'write',
            parameters: JSON.stringify(params),
            metadata: {} // Populated by runtime
        };
        
        const response = await callHost(JSON.stringify(request));
        const resp: HostAPIResponse = JSON.parse(response);
        
        if (!resp.success) {
            throw new Error(`${resp.error?.code}: ${resp.error?.message}`);
        }
    }
}

// The @okra/sdk package provides:
// - Common types (HostAPIRequest, HostAPIResponse, etc.)
// - callHost function that handles WASM memory management
// - TypeScript declarations for WASM imports
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

### Phase 1: Foundation ✅ COMPLETED
The host API infrastructure has been implemented in `internal/hostapi/`:
- ✅ Host API interfaces (`HostAPI`, `HostAPIFactory`, `StreamingHostAPI`)
- ✅ Host API registry for managing factories
- ✅ Host API set for service-specific instances
- ✅ Unified host function interface (`run_host_api`, `next`)
- ✅ JSON encoding/decoding with proper error handling
- ✅ Policy engine interface
- ✅ Iterator support for streaming APIs

### Phase 2: Core APIs (Current Phase)
1. Implement Log API factory and instance
2. Implement Env API factory and instance
3. Implement State API with in-memory backend
4. Write unit tests for each API

### Phase 3: Advanced APIs
1. Implement Secrets API with configurable backend
2. Implement Fetch API with HTTP client
3. Integration testing with real WASM modules
4. Performance benchmarking

### Phase 4: Guest Stubs & Polish
1. Create stub generator using existing codegen framework
2. Generate Go stubs for each API
3. Generate TypeScript stubs
4. Create SDK packages for common types
5. Documentation and examples

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