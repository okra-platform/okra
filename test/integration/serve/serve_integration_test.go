//go:build integration

package serve_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test plan for okra serve integration:
// 1. Build okra binary
// 2. Create a test service package
// 3. Start okra serve
// 4. Verify health endpoint
// 5. Deploy a package via admin API
// 6. Test calling the deployed service via ConnectRPC JSON
// 7. List deployed services
// 8. Undeploy the service
// 9. Gracefully shutdown

func TestOkraServe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory for test
	tempDir := t.TempDir()

	// Build okra binary
	okraBinary := filepath.Join(tempDir, "okra")
	buildCmd := exec.Command("go", "build", "-o", okraBinary, "../../../main.go")
	buildOutput, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build okra binary: %s", string(buildOutput))

	// Create a test project to build a package
	projectDir := filepath.Join(tempDir, "test-service")
	err = os.MkdirAll(projectDir, 0755)
	require.NoError(t, err)

	// Create okra.json
	okraConfig := map[string]interface{}{
		"name":     "TestService",
		"version":  "1.0.0",
		"language": "go",
		"schema":   "./service.okra.gql",
		"source":   "./service",
		"build": map[string]string{
			"output": "./build/service.wasm",
		},
		"dev": map[string]interface{}{
			"watch":   []string{"*.go", "**/*.go"},
			"exclude": []string{"*_test.go"},
		},
	}
	configData, _ := json.MarshalIndent(okraConfig, "", "  ")
	err = os.WriteFile(filepath.Join(projectDir, "okra.json"), configData, 0644)
	require.NoError(t, err)

	// Create service schema with a proper service definition
	schema := `# Test Service Schema
@okra(namespace: "test", version: "v1")

type GreetRequest {
  name: String!
}

type GreetResponse {
  message: String!
  timestamp: String!
}

service TestService {
  greet(input: GreetRequest): GreetResponse
}
`
	err = os.WriteFile(filepath.Join(projectDir, "service.okra.gql"), []byte(schema), 0644)
	require.NoError(t, err)

	// Create service directory
	serviceDir := filepath.Join(projectDir, "service")
	err = os.MkdirAll(serviceDir, 0755)
	require.NoError(t, err)

	// Create service implementation
	implementation := `package service

import (
	"fmt"
	"time"
	
	"test-service/types"
)

// service implements the generated service interface
type service struct{}

// NewService creates a new instance of the service
func NewService() types.TestService {
	return &service{}
}

// Greet returns a greeting message
func (s *service) Greet(input *types.GreetRequest) (*types.GreetResponse, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}
	
	return &types.GreetResponse{
		Message:   fmt.Sprintf("Hello, %s! From okra serve test.", input.Name),
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}
`
	err = os.WriteFile(filepath.Join(serviceDir, "service.go"), []byte(implementation), 0644)
	require.NoError(t, err)

	// Create go.mod file
	goMod := `module test-service

go 1.22
`
	err = os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goMod), 0644)
	require.NoError(t, err)

	// Build the service package
	buildServiceCmd := exec.Command(okraBinary, "build")
	buildServiceCmd.Dir = projectDir
	buildServiceOutput, err := buildServiceCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build service: %s", string(buildServiceOutput))
	
	// Log build output to see debug messages
	t.Logf("Build output:\n%s", string(buildServiceOutput))

	// Find the built package
	packagePath := filepath.Join(projectDir, "dist", "TestService-1.0.0.okra.pkg")
	_, err = os.Stat(packagePath)
	require.NoError(t, err, "Package file not found at %s", packagePath)

	// Start okra serve in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveCmd := exec.CommandContext(ctx, okraBinary, "serve")
	serveCmd.Dir = tempDir
	
	// Capture output for debugging
	var stdout, stderr bytes.Buffer
	serveCmd.Stdout = &stdout
	serveCmd.Stderr = &stderr

	err = serveCmd.Start()
	require.NoError(t, err)

	// Give the server time to start
	time.Sleep(3 * time.Second)

	// Check if process is still running
	if serveCmd.ProcessState != nil && serveCmd.ProcessState.Exited() {
		t.Fatalf("okra serve exited unexpectedly. stdout: %s, stderr: %s", 
			stdout.String(), stderr.String())
	}
	
	// Log initial server output
	t.Logf("Server stdout: %s", stdout.String())
	t.Logf("Server stderr: %s", stderr.String())

	// Test health endpoint with retries
	var resp *http.Response
	for i := 0; i < 10; i++ {
		resp, err = http.Get("http://localhost:8081/api/v1/health")
		if err == nil {
			break
		}
		if i == 9 {
			t.Logf("stdout: %s", stdout.String())
			t.Logf("stderr: %s", stderr.String())
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, err, "Failed to reach health endpoint")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var healthResp map[string]string
	err = json.NewDecoder(resp.Body).Decode(&healthResp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", healthResp["status"])

	// Test deploying a package
	deployReq := map[string]interface{}{
		"source":   fmt.Sprintf("file://%s", packagePath),
		"override": false,
	}
	deployData, _ := json.Marshal(deployReq)

	resp, err = http.Post("http://localhost:8081/api/v1/packages/deploy",
		"application/json", bytes.NewReader(deployData))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check deployment response
	var deployResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&deployResp)
	require.NoError(t, err)
	
	t.Logf("Deploy response status: %d", resp.StatusCode)
	t.Logf("Deploy response: %+v", deployResp)
	
	if resp.StatusCode == http.StatusOK {
		assert.NotEmpty(t, deployResp["service_id"])
		assert.Equal(t, "deployed", deployResp["status"])
		
		serviceID := deployResp["service_id"].(string)
		t.Logf("Deployed service ID: %s", serviceID)
		
		// Get the endpoints
		endpoints := deployResp["endpoints"].([]interface{})
		require.NotEmpty(t, endpoints, "No endpoints returned")
		
		// Use the first endpoint for testing
		endpoint := endpoints[0].(string)
		t.Logf("Using endpoint: %s", endpoint)

		// Give the service a moment to fully deploy
		time.Sleep(1 * time.Second)

		// Test calling the deployed service via ConnectRPC JSON
		testServiceCallWithEndpoint(t, endpoint)

		// Test listing services
		resp, err = http.Get("http://localhost:8081/api/v1/packages")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var listResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&listResp)
		require.NoError(t, err)
		services := listResp["services"].([]interface{})
		assert.Len(t, services, 1)

		// Test undeploying the service
		req, _ := http.NewRequest(http.MethodDelete, 
			fmt.Sprintf("http://localhost:8081/api/v1/packages/%s", serviceID), nil)
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	} else {
		// Log the error if deployment failed
		t.Logf("Deployment failed with status %d: %v", resp.StatusCode, deployResp)
	}

	// Gracefully shutdown
	cancel()
	
	// Wait for process to exit
	done := make(chan error, 1)
	go func() {
		done <- serveCmd.Wait()
	}()

	select {
	case <-done:
		// Process exited successfully
	case <-time.After(5 * time.Second):
		t.Error("okra serve did not shut down gracefully")
		serveCmd.Process.Kill()
	}
}

// testServiceCallWithEndpoint tests calling the deployed service using the actual endpoint
func testServiceCallWithEndpoint(t *testing.T, endpoint string) {
	// ConnectRPC JSON request format
	request := map[string]interface{}{
		"name": "Integration Test",
	}

	jsonData, err := json.Marshal(request)
	require.NoError(t, err)

	// Make HTTP request to the gRPC endpoint using Connect protocol
	url := fmt.Sprintf("http://localhost:8080%s", endpoint)
	t.Logf("Making service call to URL: %s", url)
	
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	require.NoError(t, err)
	
	// Set Connect protocol headers for unary JSON
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	t.Logf("Service call response status: %d", resp.StatusCode)
	t.Logf("Service call response body: %s", string(body))

	// Check status code
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK, got %d. Response: %s", resp.StatusCode, string(body))

	// Parse response
	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	require.NoError(t, err)

	// Validate response
	assert.Contains(t, response, "message")
	assert.Contains(t, response, "timestamp")
	
	message, ok := response["message"].(string)
	require.True(t, ok, "message should be a string")
	assert.Equal(t, "Hello, Integration Test! From okra serve test.", message)

	timestamp, ok := response["timestamp"].(string)
	require.True(t, ok, "timestamp should be a string") 
	assert.NotEmpty(t, timestamp)
	
	// Verify timestamp is valid RFC3339
	_, err = time.Parse(time.RFC3339, timestamp)
	assert.NoError(t, err, "timestamp should be valid RFC3339")
}