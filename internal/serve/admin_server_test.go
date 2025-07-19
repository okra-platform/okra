package serve

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	// Note: Without mocks, we can't test this properly
	// In production, we'd use dependency injection or generate mocks
	t.Skip("Skipping test - requires runtime and gateway mocks")
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
	// Note: Without mocks and dependency injection, we can't test the full flow
	t.Skip("Skipping test - requires mocks and dependency injection")
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
	// Note: Without mocks, we can't test the runtime interaction
	t.Skip("Skipping test - requires runtime mock")
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
	// Note: Without mocks, we can't test this properly
	t.Skip("Skipping test - requires runtime and gateway mocks")
}
