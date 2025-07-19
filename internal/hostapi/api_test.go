package hostapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Test Plan:
// 1. Test HostAPIError formatting and error interface
// 2. Test mock host API implementation
// 3. Test streaming host API with iterator
// 4. Test host API factory pattern
// 5. Test method metadata and schema validation

// Test: HostAPIError implements error interface correctly
func TestHostAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *HostAPIError
		expected string
	}{
		{
			name: "error with details",
			err: &HostAPIError{
				Code:    ErrorCodePolicyDenied,
				Message: "access denied",
				Details: "insufficient permissions",
			},
			expected: "POLICY_DENIED: access denied - insufficient permissions",
		},
		{
			name: "error without details",
			err: &HostAPIError{
				Code:    ErrorCodeAPINotFound,
				Message: "API not found",
			},
			expected: "API_NOT_FOUND: API not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// mockHostAPI implements HostAPI for testing
type mockHostAPI struct {
	name    string
	version string
	methods map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, error)
}

func (m *mockHostAPI) Name() string    { return m.name }
func (m *mockHostAPI) Version() string { return m.version }

func (m *mockHostAPI) Execute(ctx context.Context, method string, parameters json.RawMessage) (json.RawMessage, error) {
	handler, ok := m.methods[method]
	if !ok {
		return nil, &HostAPIError{
			Code:    "METHOD_NOT_FOUND",
			Message: fmt.Sprintf("method %s not found", method),
		}
	}
	return handler(ctx, parameters)
}

// Test: Mock host API executes methods correctly
func TestMockHostAPI_Execute(t *testing.T) {
	// Create a mock API
	api := &mockHostAPI{
		name:    "test.api",
		version: "v1.0.0",
		methods: map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, error){
			"echo": func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
				var req struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(params, &req); err != nil {
					return nil, err
				}
				resp := struct {
					Echo string `json:"echo"`
				}{
					Echo: req.Message,
				}
				return json.Marshal(resp)
			},
			"error": func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
				return nil, &HostAPIError{
					Code:    "TEST_ERROR",
					Message: "test error occurred",
				}
			},
		},
	}

	// Test successful method call
	t.Run("successful echo", func(t *testing.T) {
		params := json.RawMessage(`{"message":"hello"}`)
		result, err := api.Execute(context.Background(), "echo", params)
		require.NoError(t, err)

		var resp struct {
			Echo string `json:"echo"`
		}
		require.NoError(t, json.Unmarshal(result, &resp))
		assert.Equal(t, "hello", resp.Echo)
	})

	// Test error method
	t.Run("error method", func(t *testing.T) {
		params := json.RawMessage(`{}`)
		_, err := api.Execute(context.Background(), "error", params)
		require.Error(t, err)

		var apiErr *HostAPIError
		require.True(t, errors.As(err, &apiErr))
		assert.Equal(t, "TEST_ERROR", apiErr.Code)
	})

	// Test unknown method
	t.Run("unknown method", func(t *testing.T) {
		params := json.RawMessage(`{}`)
		_, err := api.Execute(context.Background(), "unknown", params)
		require.Error(t, err)

		var apiErr *HostAPIError
		require.True(t, errors.As(err, &apiErr))
		assert.Equal(t, "METHOD_NOT_FOUND", apiErr.Code)
	})
}

// mockStreamingAPI implements StreamingHostAPI for testing
type mockStreamingAPI struct {
	mockHostAPI
	streamingMethods map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, Iterator, error)
}

func (m *mockStreamingAPI) ExecuteStreaming(ctx context.Context, method string, parameters json.RawMessage) (json.RawMessage, Iterator, error) {
	handler, ok := m.streamingMethods[method]
	if !ok {
		// Fall back to regular execution
		result, err := m.Execute(ctx, method, parameters)
		return result, nil, err
	}
	return handler(ctx, parameters)
}

// mockIterator implements Iterator for testing
type mockIterator struct {
	data      []json.RawMessage
	index     int
	closeFunc func() error
}

func (i *mockIterator) Next(ctx context.Context) (json.RawMessage, bool, error) {
	if i.index >= len(i.data) {
		return nil, false, nil
	}
	data := i.data[i.index]
	i.index++
	hasMore := i.index < len(i.data)
	return data, hasMore, nil
}

func (i *mockIterator) Close() error {
	if i.closeFunc != nil {
		return i.closeFunc()
	}
	return nil
}

// Test: Streaming host API with iterator
func TestStreamingHostAPI_Iterator(t *testing.T) {
	api := &mockStreamingAPI{
		mockHostAPI: mockHostAPI{
			name:    "test.streaming",
			version: "v1.0.0",
			methods: make(map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, error)),
		},
		streamingMethods: map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, Iterator, error){
			"list": func(ctx context.Context, params json.RawMessage) (json.RawMessage, Iterator, error) {
				// Create test data
				data := []json.RawMessage{
					json.RawMessage(`{"item":1}`),
					json.RawMessage(`{"item":2}`),
					json.RawMessage(`{"item":3}`),
				}

				iterator := &mockIterator{
					data:  data,
					index: 0,
				}

				// Return initial response with iterator ID
				resp := StreamingResponse{
					IteratorID: "test-iterator-123",
					HasData:    true,
				}
				respJSON, _ := json.Marshal(resp)

				return respJSON, iterator, nil
			},
		},
	}

	// Test streaming method
	ctx := context.Background()
	result, iterator, err := api.ExecuteStreaming(ctx, "list", json.RawMessage(`{}`))
	require.NoError(t, err)
	require.NotNil(t, iterator)

	// Verify response
	var resp StreamingResponse
	require.NoError(t, json.Unmarshal(result, &resp))
	assert.Equal(t, "test-iterator-123", resp.IteratorID)
	assert.True(t, resp.HasData)

	// Test iterator
	var items []int
	for {
		data, hasMore, err := iterator.Next(ctx)
		require.NoError(t, err)

		if data == nil {
			break
		}

		var item struct {
			Item int `json:"item"`
		}
		require.NoError(t, json.Unmarshal(data, &item))
		items = append(items, item.Item)

		if !hasMore {
			break
		}
	}

	assert.Equal(t, []int{1, 2, 3}, items)

	// Test close
	require.NoError(t, iterator.Close())
}

// mockHostAPIFactory implements HostAPIFactory for testing
type mockHostAPIFactory struct {
	name    string
	version string
	methods []MethodMetadata
}

func (f *mockHostAPIFactory) Name() string              { return f.name }
func (f *mockHostAPIFactory) Version() string           { return f.version }
func (f *mockHostAPIFactory) Methods() []MethodMetadata { return f.methods }

func (f *mockHostAPIFactory) Create(ctx context.Context, config HostAPIConfig) (HostAPI, error) {
	return &mockHostAPI{
		name:    f.name,
		version: f.version,
		methods: make(map[string]func(ctx context.Context, params json.RawMessage) (json.RawMessage, error)),
	}, nil
}

// Test: Host API factory pattern
func TestHostAPIFactory(t *testing.T) {
	// Create factory with method metadata
	factory := &mockHostAPIFactory{
		name:    "test.factory",
		version: "v1.0.0",
		methods: []MethodMetadata{
			{
				Name:        "get",
				Description: "Get a value",
				Parameters: &spec.Schema{
					SchemaProps: spec.SchemaProps{
						Type: []string{"object"},
						Properties: map[string]spec.Schema{
							"key": {
								SchemaProps: spec.SchemaProps{
									Type:        []string{"string"},
									Description: "The key",
								},
							},
						},
						Required: []string{"key"},
					},
				},
				Returns: &spec.Schema{
					SchemaProps: spec.SchemaProps{
						Type: []string{"object"},
						Properties: map[string]spec.Schema{
							"value": {
								SchemaProps: spec.SchemaProps{
									Type: []string{"string"},
								},
							},
						},
					},
				},
				Errors: []ErrorMetadata{
					{Code: "NOT_FOUND", Description: "Key not found"},
				},
				Streaming: false,
			},
			{
				Name:        "list",
				Description: "List keys",
				Parameters: &spec.Schema{
					SchemaProps: spec.SchemaProps{
						Type: []string{"object"},
					},
				},
				Returns: &spec.Schema{
					SchemaProps: spec.SchemaProps{
						Type: []string{"object"},
						Properties: map[string]spec.Schema{
							"iteratorId": {
								SchemaProps: spec.SchemaProps{
									Type: []string{"string"},
								},
							},
						},
					},
				},
				Streaming: true,
			},
		},
	}

	// Test factory properties
	assert.Equal(t, "test.factory", factory.Name())
	assert.Equal(t, "v1.0.0", factory.Version())
	assert.Len(t, factory.Methods(), 2)

	// Test method metadata
	getMethod := factory.Methods()[0]
	assert.Equal(t, "get", getMethod.Name)
	assert.False(t, getMethod.Streaming)
	assert.NotNil(t, getMethod.Parameters)
	assert.Contains(t, getMethod.Parameters.Properties, "key")
	assert.Contains(t, getMethod.Parameters.Required, "key")

	listMethod := factory.Methods()[1]
	assert.Equal(t, "list", listMethod.Name)
	assert.True(t, listMethod.Streaming)

	// Test creating instance
	config := HostAPIConfig{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Logger:         slog.Default(),
		Tracer:         tracenoop.NewTracerProvider().Tracer("test"),
		Meter:          metricnoop.NewMeterProvider().Meter("test"),
	}

	api, err := factory.Create(context.Background(), config)
	require.NoError(t, err)
	assert.Equal(t, "test.factory", api.Name())
	assert.Equal(t, "v1.0.0", api.Version())
}

// mockPolicyEngine implements PolicyEngine for testing
type mockPolicyEngine struct {
	decisions map[string]PolicyDecision
	err       error
}

func (m *mockPolicyEngine) Evaluate(ctx context.Context, check PolicyCheck) (PolicyDecision, error) {
	if m.err != nil {
		return PolicyDecision{}, m.err
	}

	key := fmt.Sprintf("%s.%s", check.Request.API, check.Request.Method)
	if decision, ok := m.decisions[key]; ok {
		return decision, nil
	}

	// Default to allow
	return PolicyDecision{Allowed: true}, nil
}

// Test: Policy engine integration
func TestPolicyEngine(t *testing.T) {
	engine := &mockPolicyEngine{
		decisions: map[string]PolicyDecision{
			"test.api.restricted": {
				Allowed: false,
				Reason:  "access denied to restricted method",
			},
			"test.api.allowed": {
				Allowed:  true,
				Metadata: map[string]interface{}{"rate_limit": 100},
			},
		},
	}

	// Test denied request
	t.Run("denied request", func(t *testing.T) {
		check := PolicyCheck{
			Service: "test-service",
			Request: HostAPIRequest{
				API:    "test.api",
				Method: "restricted",
			},
		}

		decision, err := engine.Evaluate(context.Background(), check)
		require.NoError(t, err)
		assert.False(t, decision.Allowed)
		assert.Equal(t, "access denied to restricted method", decision.Reason)
	})

	// Test allowed request
	t.Run("allowed request", func(t *testing.T) {
		check := PolicyCheck{
			Service: "test-service",
			Request: HostAPIRequest{
				API:    "test.api",
				Method: "allowed",
			},
		}

		decision, err := engine.Evaluate(context.Background(), check)
		require.NoError(t, err)
		assert.True(t, decision.Allowed)
		assert.Equal(t, 100, decision.Metadata["rate_limit"])
	})

	// Test default allow
	t.Run("default allow", func(t *testing.T) {
		check := PolicyCheck{
			Service: "test-service",
			Request: HostAPIRequest{
				API:    "test.api",
				Method: "unknown",
			},
		}

		decision, err := engine.Evaluate(context.Background(), check)
		require.NoError(t, err)
		assert.True(t, decision.Allowed)
	})
}

// Test: Resource limit constants
func TestResourceLimitConstants(t *testing.T) {
	// Verify default values are reasonable
	assert.Equal(t, 100, DefaultMaxIteratorsPerService)
	assert.Equal(t, 5*time.Minute, DefaultIteratorTimeout)
	assert.Equal(t, 10*1024*1024, DefaultMaxRequestSize)
	assert.Equal(t, 10*1024*1024, DefaultMaxResponseSize)

	// Verify error codes are unique
	errorCodes := []string{
		ErrorCodeResponseTooLarge,
		ErrorCodeHostAPISetClosed,
		ErrorCodeAPINotFound,
		ErrorCodePolicyError,
		ErrorCodePolicyDenied,
		ErrorCodeInternalError,
		ErrorCodeIteratorNotFound,
		ErrorCodeIteratorLimitExceeded,
	}

	seen := make(map[string]bool)
	for _, code := range errorCodes {
		assert.False(t, seen[code], "duplicate error code: %s", code)
		seen[code] = true
	}
}
