package hostapi

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test Plan:
// 1. Test RunHostAPI with valid request
// 2. Test RunHostAPI with missing host API set in context
// 3. Test RunHostAPI with invalid request format
// 4. Test RunHostAPI with API execution error
// 5. Test NextIterator with valid request
// 6. Test NextIterator with invalid request format
// 7. Test error response formatting

// Test: RunHostAPI with valid request
func TestRunHostAPI_ValidRequest(t *testing.T) {
	// Create mock host API set
	mockSet := &mockHostAPISet{
		executeFunc: func(ctx context.Context, apiName, method string, parameters json.RawMessage) (json.RawMessage, error) {
			assert.Equal(t, "test.api", apiName)
			assert.Equal(t, "echo", method)
			return parameters, nil
		},
	}

	// Create context with host API set
	ctx := context.WithValue(context.Background(), hostAPISetKey{}, mockSet)

	// Create request
	req := HostAPIRequest{
		API:        "test.api",
		Method:     "echo",
		Parameters: json.RawMessage(`{"message":"hello"}`),
	}
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	// Call RunHostAPI
	response, err := RunHostAPI(ctx, string(reqJSON))
	require.NoError(t, err)

	// Verify response
	var resp HostAPIResponse
	require.NoError(t, json.Unmarshal([]byte(response), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, json.RawMessage(`{"message":"hello"}`), resp.Data)
	assert.Nil(t, resp.Error)
}

// Test: RunHostAPI with missing host API set
func TestRunHostAPI_MissingHostAPISet(t *testing.T) {
	ctx := context.Background() // No host API set in context

	req := HostAPIRequest{
		API:    "test.api",
		Method: "method",
	}
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	_, err = RunHostAPI(ctx, string(reqJSON))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host API set not found")
}

// Test: RunHostAPI with invalid request format
func TestRunHostAPI_InvalidRequestFormat(t *testing.T) {
	mockSet := &mockHostAPISet{}
	ctx := context.WithValue(context.Background(), hostAPISetKey{}, mockSet)

	_, err := RunHostAPI(ctx, "invalid json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request format")
}

// Test: RunHostAPI with API execution error (HostAPIError)
func TestRunHostAPI_APIExecutionError(t *testing.T) {
	// Create mock that returns HostAPIError
	mockSet := &mockHostAPISet{
		executeFunc: func(ctx context.Context, apiName, method string, parameters json.RawMessage) (json.RawMessage, error) {
			return nil, &HostAPIError{
				Code:    ErrorCodePolicyDenied,
				Message: "access denied",
				Details: "insufficient permissions",
			}
		},
	}

	ctx := context.WithValue(context.Background(), hostAPISetKey{}, mockSet)

	req := HostAPIRequest{
		API:    "test.api",
		Method: "restricted",
	}
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	// Call should succeed but return error in response
	response, err := RunHostAPI(ctx, string(reqJSON))
	require.NoError(t, err)

	// Verify error response
	var resp HostAPIResponse
	require.NoError(t, json.Unmarshal([]byte(response), &resp))
	assert.False(t, resp.Success)
	assert.Nil(t, resp.Data)
	require.NotNil(t, resp.Error)
	assert.Equal(t, ErrorCodePolicyDenied, resp.Error.Code)
	assert.Equal(t, "access denied", resp.Error.Message)
	assert.Equal(t, "insufficient permissions", resp.Error.Details)
}

// Test: RunHostAPI with generic error
func TestRunHostAPI_GenericError(t *testing.T) {
	// Create mock that returns generic error
	mockSet := &mockHostAPISet{
		executeFunc: func(ctx context.Context, apiName, method string, parameters json.RawMessage) (json.RawMessage, error) {
			return nil, errors.New("something went wrong")
		},
	}

	ctx := context.WithValue(context.Background(), hostAPISetKey{}, mockSet)

	req := HostAPIRequest{
		API:    "test.api",
		Method: "failing",
	}
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	response, err := RunHostAPI(ctx, string(reqJSON))
	require.NoError(t, err)

	// Verify error response
	var resp HostAPIResponse
	require.NoError(t, json.Unmarshal([]byte(response), &resp))
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, ErrorCodeInternalError, resp.Error.Code)
	assert.Equal(t, "something went wrong", resp.Error.Message)
}

// Test: NextIterator with valid request
func TestNextIterator_ValidRequest(t *testing.T) {
	// Create mock host API set
	mockSet := &mockHostAPISet{
		nextIteratorFunc: func(ctx context.Context, iteratorID string) (json.RawMessage, bool, error) {
			assert.Equal(t, "test-iter-123", iteratorID)
			return json.RawMessage(`{"item":1}`), true, nil
		},
	}

	ctx := context.WithValue(context.Background(), hostAPISetKey{}, mockSet)

	req := NextRequest{
		IteratorID: "test-iter-123",
	}
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	response, err := NextIterator(ctx, string(reqJSON))
	require.NoError(t, err)

	// Verify response
	var resp NextResponse
	require.NoError(t, json.Unmarshal([]byte(response), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, json.RawMessage(`{"item":1}`), resp.Data)
	assert.True(t, resp.HasMore)
	assert.Nil(t, resp.Error)
}

// Test: NextIterator with no more data
func TestNextIterator_NoMoreData(t *testing.T) {
	mockSet := &mockHostAPISet{
		nextIteratorFunc: func(ctx context.Context, iteratorID string) (json.RawMessage, bool, error) {
			return json.RawMessage(`{"item":3}`), false, nil
		},
	}

	ctx := context.WithValue(context.Background(), hostAPISetKey{}, mockSet)

	req := NextRequest{
		IteratorID: "test-iter-123",
	}
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	response, err := NextIterator(ctx, string(reqJSON))
	require.NoError(t, err)

	// Verify response
	var resp NextResponse
	require.NoError(t, json.Unmarshal([]byte(response), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, json.RawMessage(`{"item":3}`), resp.Data)
	assert.False(t, resp.HasMore)
}

// Test: NextIterator with invalid request format
func TestNextIterator_InvalidRequestFormat(t *testing.T) {
	mockSet := &mockHostAPISet{}
	ctx := context.WithValue(context.Background(), hostAPISetKey{}, mockSet)

	_, err := NextIterator(ctx, "invalid json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid next request format")
}

// Test: NextIterator with error
func TestNextIterator_Error(t *testing.T) {
	// Test with HostAPIError
	t.Run("HostAPIError", func(t *testing.T) {
		mockSet := &mockHostAPISet{
			nextIteratorFunc: func(ctx context.Context, iteratorID string) (json.RawMessage, bool, error) {
				return nil, false, &HostAPIError{
					Code:    ErrorCodeIteratorNotFound,
					Message: "iterator not found",
				}
			},
		}

		ctx := context.WithValue(context.Background(), hostAPISetKey{}, mockSet)

		req := NextRequest{
			IteratorID: "non-existent",
		}
		reqJSON, err := json.Marshal(req)
		require.NoError(t, err)

		response, err := NextIterator(ctx, string(reqJSON))
		require.NoError(t, err)

		var resp NextResponse
		require.NoError(t, json.Unmarshal([]byte(response), &resp))
		assert.False(t, resp.Success)
		require.NotNil(t, resp.Error)
		assert.Equal(t, ErrorCodeIteratorNotFound, resp.Error.Code)
	})

	// Test with generic error
	t.Run("generic error", func(t *testing.T) {
		mockSet := &mockHostAPISet{
			nextIteratorFunc: func(ctx context.Context, iteratorID string) (json.RawMessage, bool, error) {
				return nil, false, errors.New("iterator error")
			},
		}

		ctx := context.WithValue(context.Background(), hostAPISetKey{}, mockSet)

		req := NextRequest{
			IteratorID: "test-iter",
		}
		reqJSON, err := json.Marshal(req)
		require.NoError(t, err)

		response, err := NextIterator(ctx, string(reqJSON))
		require.NoError(t, err)

		var resp NextResponse
		require.NoError(t, json.Unmarshal([]byte(response), &resp))
		assert.False(t, resp.Success)
		require.NotNil(t, resp.Error)
		assert.Equal(t, ErrorCodeInternalError, resp.Error.Code)
		assert.Equal(t, "iterator error", resp.Error.Message)
	})
}

// mockHostAPISet implements HostAPISet for testing
type mockHostAPISet struct {
	executeFunc      func(ctx context.Context, apiName, method string, parameters json.RawMessage) (json.RawMessage, error)
	nextIteratorFunc func(ctx context.Context, iteratorID string) (json.RawMessage, bool, error)
}

func (m *mockHostAPISet) Get(name string) (HostAPI, bool) {
	return nil, false
}

func (m *mockHostAPISet) Execute(ctx context.Context, apiName, method string, parameters json.RawMessage) (json.RawMessage, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, apiName, method, parameters)
	}
	return nil, errors.New("not implemented")
}

func (m *mockHostAPISet) NextIterator(ctx context.Context, iteratorID string) (json.RawMessage, bool, error) {
	if m.nextIteratorFunc != nil {
		return m.nextIteratorFunc(ctx, iteratorID)
	}
	return nil, false, errors.New("not implemented")
}

func (m *mockHostAPISet) CloseIterator(ctx context.Context, iteratorID string) error {
	return nil
}

func (m *mockHostAPISet) CleanupStaleIterators() int {
	return 0
}

func (m *mockHostAPISet) Config() HostAPIConfig {
	return HostAPIConfig{}
}

func (m *mockHostAPISet) Close() error {
	return nil
}
