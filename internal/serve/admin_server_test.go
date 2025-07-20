package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/okra-platform/okra/internal/runtime"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Test plan for AdminServer:
// 1. Test NewAdminServer creates server correctly
// 2. Test health endpoint returns healthy status
// 3. Test deploy endpoint with valid request
// 4. Test deploy endpoint with invalid request
// 5. Test list services endpoint
// 6. Test undeploy endpoint with existing service
// 7. Test undeploy endpoint with non-existent service

func TestNewAdminServer(t *testing.T) {
	// Test: NewAdminServer creates server with dependencies
	mockRT := new(mockRuntime)
	mockConnectGW := new(mockConnectGateway)
	mockGraphQLGW := new(mockGraphQLGateway)

	server := NewAdminServer(mockRT, mockConnectGW, mockGraphQLGW)

	assert.NotNil(t, server)
	
	// Verify it's the correct type
	as, ok := server.(*adminServer)
	assert.True(t, ok)
	assert.Equal(t, mockRT, as.runtime)
	assert.Equal(t, mockConnectGW, as.connectGateway)
	assert.Equal(t, mockGraphQLGW, as.graphqlGateway)
	assert.NotNil(t, as.deployedServices)
	assert.NotNil(t, as.packageLoader)
}

func TestNewAdminServerWithPackageLoader(t *testing.T) {
	// Test: NewAdminServerWithPackageLoader creates server with custom package loader
	mockRT := new(mockRuntime)
	mockConnectGW := new(mockConnectGateway)
	mockGraphQLGW := new(mockGraphQLGateway)
	
	mockLoader := func(ctx context.Context, source string) (*runtime.ServicePackage, error) {
		return nil, nil
	}

	server := NewAdminServerWithPackageLoader(mockRT, mockConnectGW, mockGraphQLGW, mockLoader)

	assert.NotNil(t, server)
	
	// Verify it's the correct type
	as, ok := server.(*adminServer)
	assert.True(t, ok)
	assert.Equal(t, mockRT, as.runtime)
	assert.Equal(t, mockConnectGW, as.connectGateway)
	assert.Equal(t, mockGraphQLGW, as.graphqlGateway)
	assert.NotNil(t, as.deployedServices)
	assert.NotNil(t, as.packageLoader)
}

func TestAdminServer_HandleHealth(t *testing.T) {
	// Test: Health endpoint returns healthy status

	server := &adminServer{
		deployedServices: make(map[string]*DeployedService),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.NotEmpty(t, response["time"])
}

func TestAdminServer_HandleDeploy_Success(t *testing.T) {
	// Test: Deploy endpoint successfully deploys a package
	mockRT := new(mockRuntime)
	mockConnectGW := new(mockConnectGateway)
	mockGraphQLGW := new(mockGraphQLGateway)

	// Mock package for testing
	testPkg := &runtime.ServicePackage{
		ServiceName: "TestService",
		Schema: &schema.Schema{
			Meta: schema.Metadata{
				Namespace: "test",
			},
			Services: []schema.Service{
				{
					Name: "TestService",
					Methods: []schema.Method{
						{
							Name:       "TestMethod",
							InputType:  "TestInput",
							OutputType: "TestOutput",
						},
					},
				},
			},
		},
		Methods: map[string]*schema.Method{
			"TestMethod": {
				Name:       "TestMethod",
				InputType:  "TestInput", 
				OutputType: "TestOutput",
			},
		},
		FileDescriptors: &descriptorpb.FileDescriptorSet{},
	}

	// Mock package loader
	mockPackageLoader := func(ctx context.Context, source string) (*runtime.ServicePackage, error) {
		if source == "file:///test/package.okra.pkg" {
			return testPkg, nil
		}
		return nil, fmt.Errorf("package not found")
	}

	server := &adminServer{
		runtime:          mockRT,
		connectGateway:   mockConnectGW,
		graphqlGateway:   mockGraphQLGW,
		packageLoader:    mockPackageLoader,
		deployedServices: make(map[string]*DeployedService),
	}

	// Set up expectations
	mockRT.On("Deploy", mock.Anything, testPkg).Return("test-actor-123", nil)

	reqBody := `{"source": "file:///test/package.okra.pkg", "override": false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/packages/deploy", 
		bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleDeploy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response DeployResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "test-actor-123", response.ServiceID)
	assert.Equal(t, "deployed", response.Status)

	// Verify service was tracked
	assert.Len(t, server.deployedServices, 1)
	assert.Contains(t, server.deployedServices, "test-actor-123")

	mockRT.AssertExpectations(t)
}

func TestAdminServer_HandleDeploy_InvalidRequest(t *testing.T) {
	// Test: Deploy endpoint rejects invalid requests

	server := &adminServer{
		deployedServices: make(map[string]*DeployedService),
	}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid JSON",
			body:       `{"invalid json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing source",
			body:       `{"override": true}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/packages/deploy",
				bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.handleDeploy(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestAdminServer_HandleListServices(t *testing.T) {
	// Test: List services returns all deployed services

	server := &adminServer{
		deployedServices: map[string]*DeployedService{
			"test.Service1.v1": {
				ID:         "test.Service1.v1",
				Source:     "file:///path/to/service1.pkg",
				DeployedAt: time.Now().Add(-1 * time.Hour),
			},
			"test.Service2.v1": {
				ID:         "test.Service2.v1",
				Source:     "s3://bucket/service2.pkg",
				DeployedAt: time.Now().Add(-30 * time.Minute),
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil)
	w := httptest.NewRecorder()

	server.handleListServices(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ListServicesResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Len(t, response.Services, 2)
}

func TestAdminServer_HandleUndeploy_Success(t *testing.T) {
	// Test: Undeploy successfully removes an existing service
	mockRT := new(mockRuntime)

	server := &adminServer{
		runtime: mockRT,
		deployedServices: map[string]*DeployedService{
			"test-service-123": {
				ID:         "test-service-123",
				Source:     "file:///test/service.pkg",
				DeployedAt: time.Now(),
			},
		},
	}

	// Set up expectations
	mockRT.On("Undeploy", mock.Anything, "test-service-123").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/packages/test-service-123", nil)
	w := httptest.NewRecorder()

	server.handleUndeploy(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify service was removed from tracking
	assert.Len(t, server.deployedServices, 0)
	assert.NotContains(t, server.deployedServices, "test-service-123")

	mockRT.AssertExpectations(t)
}

func TestAdminServer_HandleUndeploy_NotFound(t *testing.T) {
	// Test: Undeploy returns 404 for non-existent service

	server := &adminServer{
		deployedServices: make(map[string]*DeployedService),
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/packages/non.existent.service", nil)
	w := httptest.NewRecorder()

	server.handleUndeploy(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "service not found", response.Error)
}

func TestAdminServer_Start(t *testing.T) {
	// Test: Start method starts the HTTP server
	mockRT := new(mockRuntime)
	mockConnectGW := new(mockConnectGateway)
	mockGraphQLGW := new(mockGraphQLGateway)

	server := &adminServer{
		runtime:          mockRT,
		connectGateway:   mockConnectGW,
		graphqlGateway:   mockGraphQLGW,
		deployedServices: make(map[string]*DeployedService),
	}

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start the server on a random port
	err := server.Start(ctx, 0)
	assert.NoError(t, err) // Context cancellation is not an error

	// Verify server was set
	assert.NotNil(t, server.server)
}

func TestAdminServer_HandleHealth_WrongMethod(t *testing.T) {
	// Test: Health endpoint rejects non-GET methods
	server := &adminServer{
		deployedServices: make(map[string]*DeployedService),
	}

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/health", nil)
			w := httptest.NewRecorder()
			
			server.handleHealth(w, req)
			
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			
			var response ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, "method not allowed", response.Error)
		})
	}
}

func TestAdminServer_HandleListServices_WrongMethod(t *testing.T) {
	// Test: List services endpoint rejects non-GET methods
	server := &adminServer{
		deployedServices: make(map[string]*DeployedService),
	}

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/packages", nil)
			w := httptest.NewRecorder()
			
			server.handleListServices(w, req)
			
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			
			var response ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, "method not allowed", response.Error)
		})
	}
}

func TestAdminServer_HandleUndeploy_WrongMethod(t *testing.T) {
	// Test: Undeploy endpoint rejects non-DELETE methods
	server := &adminServer{
		deployedServices: make(map[string]*DeployedService),
	}

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch}
	
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/packages/test-service", nil)
			w := httptest.NewRecorder()
			
			server.handleUndeploy(w, req)
			
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			
			var response ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, "method not allowed", response.Error)
		})
	}
}

func TestAdminServer_HandleDeploy_WrongMethod(t *testing.T) {
	// Test: Deploy endpoint rejects non-POST methods
	server := &adminServer{
		deployedServices: make(map[string]*DeployedService),
	}

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/packages/deploy", nil)
			w := httptest.NewRecorder()
			
			server.handleDeploy(w, req)
			
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			
			var response ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, "method not allowed", response.Error)
		})
	}
}

func TestAdminServer_HandleUndeploy_EmptyServiceID(t *testing.T) {
	// Test: Undeploy rejects empty service ID
	server := &adminServer{
		deployedServices: make(map[string]*DeployedService),
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/packages/", nil)
	w := httptest.NewRecorder()

	server.handleUndeploy(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	var response ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "service ID required", response.Error)
}

func TestAdminServer_HandleUndeploy_RuntimeError(t *testing.T) {
	// Test: Undeploy handles runtime errors properly
	mockRT := new(mockRuntime)
	
	server := &adminServer{
		runtime: mockRT,
		deployedServices: map[string]*DeployedService{
			"failing-service": {
				ID:         "failing-service",
				Source:     "file:///test/failing.pkg",
				DeployedAt: time.Now(),
			},
		},
	}

	// Set up expectation for failure
	mockRT.On("Undeploy", mock.Anything, "failing-service").Return(assert.AnError)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/packages/failing-service", nil)
	w := httptest.NewRecorder()

	server.handleUndeploy(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	
	var response ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Contains(t, response.Error, "assert.AnError")

	// Verify service was NOT removed from tracking due to error
	assert.Len(t, server.deployedServices, 1)
	assert.Contains(t, server.deployedServices, "failing-service")

	mockRT.AssertExpectations(t)
}

func TestAdminServer_ConcurrentOperations(t *testing.T) {
	// Test: Admin server handles concurrent read/write operations safely
	server := &adminServer{
		deployedServices: make(map[string]*DeployedService),
	}

	// Pre-populate with some services
	for i := 0; i < 5; i++ {
		serviceID := fmt.Sprintf("service-%d", i)
		server.deployedServices[serviceID] = &DeployedService{
			ID:         serviceID,
			Source:     fmt.Sprintf("file:///test/service%d.pkg", i),
			DeployedAt: time.Now(),
		}
	}

	// Run concurrent operations
	done := make(chan bool)
	
	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/packages", nil)
			w := httptest.NewRecorder()
			server.handleListServices(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}()
	}

	// Concurrent health checks
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			w := httptest.NewRecorder()
			server.handleHealth(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestAdminServer_HandleDeploy_LoadPackageError(t *testing.T) {
	// Test: Deploy endpoint handles package loading errors
	server := &adminServer{
		packageLoader: func(ctx context.Context, source string) (*runtime.ServicePackage, error) {
			return nil, fmt.Errorf("failed to load package: file not found")
		},
		deployedServices: make(map[string]*DeployedService),
	}

	reqBody := `{"source": "file:///nonexistent/package.okra.pkg", "override": false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/packages/deploy",
		bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleDeploy(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Contains(t, response.Error, "failed to load package")
}

func TestAdminServer_HandleDeploy_RuntimeDeployError(t *testing.T) {
	// Test: Deploy endpoint handles runtime deployment errors
	mockRT := new(mockRuntime)

	testPkg := &runtime.ServicePackage{
		ServiceName: "TestService",
	}

	server := &adminServer{
		runtime: mockRT,
		packageLoader: func(ctx context.Context, source string) (*runtime.ServicePackage, error) {
			return testPkg, nil
		},
		deployedServices: make(map[string]*DeployedService),
	}

	// Set up expectation for deployment failure
	mockRT.On("Deploy", mock.Anything, testPkg).Return("", fmt.Errorf("deployment failed"))

	reqBody := `{"source": "file:///test/package.okra.pkg", "override": false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/packages/deploy",
		bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleDeploy(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Contains(t, response.Error, "failed to deploy to runtime")

	// Verify no service was tracked due to failure
	assert.Len(t, server.deployedServices, 0)

	mockRT.AssertExpectations(t)
}
