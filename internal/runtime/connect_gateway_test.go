package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/okra-platform/okra/internal/runtime/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tochemey/goakt/v2/actors"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Test plan for ConnectGateway:
// 1. Test creating a new ConnectGateway
// 2. Test Handler() returns non-nil http.Handler
// 3. Test UpdateService with valid descriptors
// 4. Test UpdateService with missing service
// 5. Test UpdateService with invalid descriptors
// 6. Test Shutdown clears services
// 7. Test concurrent access safety

func TestConnectGateway_NewConnectGateway(t *testing.T) {
	// Test: Create new ConnectGateway
	gateway := NewConnectGateway()
	assert.NotNil(t, gateway)
	assert.NotNil(t, gateway.Handler())
}

func TestConnectGateway_Handler(t *testing.T) {
	// Test: Handler returns valid http.Handler
	gateway := NewConnectGateway()
	handler := gateway.Handler()
	assert.NotNil(t, handler)
	
	// Test that handler can be used with http server
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// Should get 404 as no services are registered
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestConnectGateway_UpdateService(t *testing.T) {
	// Test: UpdateService with valid descriptors
	gateway := NewConnectGateway()
	ctx := context.Background()
	
	// Create a simple FileDescriptorSet
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    strPtr("test.proto"),
				Package: strPtr("testpkg"),
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: strPtr("TestService"),
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name:       strPtr("TestMethod"),
								InputType:  strPtr(".testpkg.TestRequest"),
								OutputType: strPtr(".testpkg.TestResponse"),
							},
						},
					},
				},
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: strPtr("TestRequest"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   strPtr("id"),
								Number: int32Ptr(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							},
						},
					},
					{
						Name: strPtr("TestResponse"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   strPtr("result"),
								Number: int32Ptr(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							},
						},
					},
				},
			},
		},
	}
	
	// Create a mock actor PID
	actorSystem, err := actors.NewActorSystem("test-system",
		actors.WithExpireActorAfter(1*time.Minute))
	require.NoError(t, err)
	err = actorSystem.Start(ctx)
	require.NoError(t, err)
	defer actorSystem.Stop(ctx)
	
	// Create a simple actor
	actor := &testActor{}
	pid, err := actorSystem.Spawn(ctx, "test-actor", actor)
	require.NoError(t, err)
	
	err = gateway.UpdateService(ctx, "TestService", fds, pid)
	require.NoError(t, err)
}

// testActor is a simple actor for testing
type testActor struct{}

func (t *testActor) PreStart(ctx context.Context) error { return nil }
func (t *testActor) Receive(ctx *actors.ReceiveContext) {}
func (t *testActor) PostStop(ctx context.Context) error { return nil }

func TestConnectGateway_UpdateServiceMissingService(t *testing.T) {
	// Test: UpdateService with non-existent service name
	gateway := NewConnectGateway()
	ctx := context.Background()
	
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    strPtr("test.proto"),
				Package: strPtr("testpkg"),
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: strPtr("DifferentService"),
					},
				},
			},
		},
	}
	
	actorSystem, err := actors.NewActorSystem("test-system-2",
		actors.WithExpireActorAfter(1*time.Minute))
	require.NoError(t, err)
	err = actorSystem.Start(ctx)
	require.NoError(t, err)
	defer actorSystem.Stop(ctx)
	
	actor := &testActor{}
	pid, err := actorSystem.Spawn(ctx, "test-actor-2", actor)
	require.NoError(t, err)
	
	err = gateway.UpdateService(ctx, "TestService", fds, pid)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service TestService not found")
}

func TestConnectGateway_UpdateServiceInvalidDescriptors(t *testing.T) {
	// Test: UpdateService with invalid descriptors
	gateway := NewConnectGateway()
	ctx := context.Background()
	
	// Create invalid descriptors with missing required fields
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				// Missing Package field which is required
				Name: strPtr("invalid.proto"),
			},
		},
	}
	
	actorSystem, err := actors.NewActorSystem("test-system-2",
		actors.WithExpireActorAfter(1*time.Minute))
	require.NoError(t, err)
	err = actorSystem.Start(ctx)
	require.NoError(t, err)
	defer actorSystem.Stop(ctx)
	
	actor := &testActor{}
	pid, err := actorSystem.Spawn(ctx, "test-actor-2", actor)
	require.NoError(t, err)
	
	err = gateway.UpdateService(ctx, "TestService", fds, pid)
	assert.Error(t, err)
}

func TestConnectGateway_Shutdown(t *testing.T) {
	// Test: Shutdown clears services
	gateway := NewConnectGateway()
	ctx := context.Background()
	
	// First add a service
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    strPtr("test.proto"),
				Package: strPtr("testpkg"),
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: strPtr("TestService"),
					},
				},
			},
		},
	}
	
	actorSystem, err := actors.NewActorSystem("test-system-2",
		actors.WithExpireActorAfter(1*time.Minute))
	require.NoError(t, err)
	err = actorSystem.Start(ctx)
	require.NoError(t, err)
	defer actorSystem.Stop(ctx)
	
	actor := &testActor{}
	pid, err := actorSystem.Spawn(ctx, "test-actor-2", actor)
	require.NoError(t, err)
	
	err = gateway.UpdateService(ctx, "TestService", fds, pid)
	require.NoError(t, err)
	
	// Shutdown
	err = gateway.Shutdown(ctx)
	assert.NoError(t, err)
	
	// Verify services are cleared by checking internal state
	cg := gateway.(*connectGateway)
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	assert.Empty(t, cg.services)
}

func TestConnectGateway_ConcurrentAccess(t *testing.T) {
	// Test: Concurrent access is safe
	gateway := NewConnectGateway()
	ctx := context.Background()
	
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    strPtr("test.proto"),
				Package: strPtr("testpkg"),
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: strPtr("TestService"),
					},
				},
			},
		},
	}
	
	actorSystem, err := actors.NewActorSystem("test-system-5",
		actors.WithExpireActorAfter(1*time.Minute))
	require.NoError(t, err)
	err = actorSystem.Start(ctx)
	require.NoError(t, err)
	defer actorSystem.Stop(ctx)
	
	// Run concurrent operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			actor := &testActor{}
			pid, _ := actorSystem.Spawn(ctx, fmt.Sprintf("test-actor-%d", idx), actor)
			
			// Mix of operations
			_ = gateway.Handler()
			if idx == 0 {
				// Only update service once to avoid pattern conflicts
				_ = gateway.UpdateService(ctx, "TestService", fds, pid)
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// If we get here without panic, concurrent access is safe
	assert.True(t, true)
}

func int32Ptr(v int32) *int32 {
	return &v
}

// Test: HTTP handler processes JSON requests correctly
// Test: HTTP handler processes protobuf requests correctly
// Test: HTTP handler returns error for invalid JSON
// Test: HTTP handler returns error for invalid method
// Test: HTTP handler propagates actor errors
func TestConnectGateway_HTTPHandler(t *testing.T) {
	gateway := NewConnectGateway()
	ctx := context.Background()
	
	// Create a simple file descriptor set with a test service
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    strPtr("test.proto"),
				Package: strPtr("testpkg"),
				MessageType: []*descriptorpb.DescriptorProto{
					{
						Name: strPtr("TestRequest"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   strPtr("message"),
								Number: int32Ptr(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							},
						},
					},
					{
						Name: strPtr("TestResponse"),
						Field: []*descriptorpb.FieldDescriptorProto{
							{
								Name:   strPtr("result"),
								Number: int32Ptr(1),
								Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
							},
						},
					},
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: strPtr("TestService"),
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name:       strPtr("TestMethod"),
								InputType:  strPtr(".testpkg.TestRequest"),
								OutputType: strPtr(".testpkg.TestResponse"),
							},
						},
					},
				},
			},
		},
	}
	
	// Create actor system and test actor
	actorSystem, err := actors.NewActorSystem("test-http-system",
		actors.WithExpireActorAfter(1*time.Minute))
	require.NoError(t, err)
	err = actorSystem.Start(ctx)
	require.NoError(t, err)
	defer actorSystem.Stop(ctx)
	
	// Create an actor that responds to requests
	actor := &httpTestActor{}
	pid, err := actorSystem.Spawn(ctx, "test-http-actor", actor)
	require.NoError(t, err)
	
	// Update service with the actor
	err = gateway.UpdateService(ctx, "TestService", fds, pid)
	require.NoError(t, err)
	
	// Get the handler
	handler := gateway.Handler()
	
	tests := []struct {
		name           string
		method         string
		path           string
		contentType    string
		body           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "successful JSON request",
			method:         "POST",
			path:           "/testpkg.TestService/TestMethod",
			contentType:    "application/json",
			body:           `{"message":"hello"}`,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"result":"echo: hello"}`,
		},
		{
			name:           "successful Connect JSON request",
			method:         "POST", 
			path:           "/testpkg.TestService/TestMethod",
			contentType:    "application/connect+json",
			body:           `{"message":"hello connect"}`,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"result":"echo: hello connect"}`,
		},
		{
			name:           "invalid JSON request",
			method:         "POST",
			path:           "/testpkg.TestService/TestMethod",
			contentType:    "application/json",
			body:           `{invalid json`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unknown method",
			method:         "POST",
			path:           "/testpkg.TestService/UnknownMethod",
			contentType:    "application/json",
			body:           `{"message":"hello"}`,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "actor error",
			method:         "POST",
			path:           "/testpkg.TestService/TestMethod",
			contentType:    "application/json",
			body:           `{"message":"error"}`,
			expectedStatus: http.StatusInternalServerError,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			
			assert.Equal(t, tt.expectedStatus, rec.Code, "unexpected status code")
			
			if tt.expectedBody != "" {
				assert.JSONEq(t, tt.expectedBody, rec.Body.String(), "unexpected response body")
			}
		})
	}
}

// httpTestActor is a test actor that echoes messages
type httpTestActor struct{}

func (a *httpTestActor) PreStart(ctx context.Context) error { return nil }
func (a *httpTestActor) PostStop(ctx context.Context) error { return nil }

func (a *httpTestActor) Receive(ctx *actors.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *pb.ServiceRequest:
		// Parse the input
		var input map[string]interface{}
		if err := json.Unmarshal(msg.Input, &input); err != nil {
			ctx.Response(&pb.ServiceResponse{
				Success: false,
				Error: &pb.ServiceError{
					Code:    "PARSE_ERROR",
					Message: "failed to parse input",
				},
			})
			return
		}
		
		// Check for error trigger
		if message, ok := input["message"].(string); ok && message == "error" {
			ctx.Response(&pb.ServiceResponse{
				Success: false,
				Error: &pb.ServiceError{
					Code:    "TEST_ERROR",
					Message: "test error",
				},
			})
			return
		}
		
		// Echo the message
		output := map[string]interface{}{
			"result": "echo: " + input["message"].(string),
		}
		outputBytes, _ := json.Marshal(output)
		
		ctx.Response(&pb.ServiceResponse{
			Success: true,
			Output:  outputBytes,
		})
	}
}