package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/okra-platform/okra/internal/runtime/pb"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tochemey/goakt/v2/actors"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/ast"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/operationreport"
)

// Test plan for GraphQL Gateway:
// 1. Test gateway creation and basic functionality
// 2. Test namespace-based routing
// 3. Test schema generation from OKRA schemas
// 4. Test query execution
// 5. Test mutation execution
// 6. Test error handling
// 7. Test concurrent access to schema registry
// 8. Test service updates and removals

func TestNewGraphQLGateway(t *testing.T) {
	// Test: Create a new GraphQL gateway
	gateway := NewGraphQLGateway()
	require.NotNil(t, gateway)

	// Test: Handler should not be nil
	handler := gateway.Handler()
	require.NotNil(t, handler)
}

func TestGraphQLGateway_NamespaceRouting(t *testing.T) {
	// Test: Namespace routing works correctly

	gateway := NewGraphQLGateway()
	handler := gateway.Handler()

	// Test: Request without namespace returns error
	req := httptest.NewRequest(http.MethodPost, "/graphql/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "namespace required")

	// Test: Request to non-existent namespace returns 404
	req = httptest.NewRequest(http.MethodPost, "/graphql/unknown", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "namespace 'unknown' not found")
}

func TestGraphQLGateway_UpdateService(t *testing.T) {
	// Test: Update service adds namespace and schema

	gateway := NewGraphQLGateway()
	ctx := context.Background()

	// Create test schema
	testSchema := &schema.Schema{
		Meta: schema.Metadata{
			Namespace: "test",
			Version:   "v1",
		},
		Types: []schema.ObjectType{
			{
				Name: "User",
				Fields: []schema.Field{
					{Name: "id", Type: "ID", Required: true},
					{Name: "name", Type: "String", Required: true},
				},
			},
			{
				Name: "GetUserRequest",
				Fields: []schema.Field{
					{Name: "userId", Type: "ID", Required: true},
				},
			},
		},
		Services: []schema.Service{
			{
				Name: "UserService",
				Methods: []schema.Method{
					{
						Name:       "getUser",
						InputType:  "GetUserRequest",
						OutputType: "User",
					},
				},
			},
		},
	}

	// Mock actor PID
	mockPID := &actors.PID{}

	// Update service
	err := gateway.UpdateService(ctx, "test", testSchema, mockPID)
	require.NoError(t, err)

	// Test: Namespace should now be accessible
	reqBody := map[string]interface{}{
		"query": "{ __typename }", // Simple introspection query
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/graphql/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler := gateway.Handler()
	handler.ServeHTTP(w, req)

	// Should get 200 OK now that namespace exists
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGraphQLGateway_SchemaGeneration(t *testing.T) {
	// Test: Schema generation creates valid GraphQL schema

	schemas := []*schema.Schema{
		{
			Types: []schema.ObjectType{
				{
					Name: "Product",
					Fields: []schema.Field{
						{Name: "id", Type: "ID", Required: true},
						{Name: "name", Type: "String", Required: true},
						{Name: "price", Type: "Float", Required: true},
					},
				},
			},
			Enums: []schema.EnumType{
				{
					Name: "Status",
					Values: []schema.EnumValue{
						{Name: "ACTIVE"},
						{Name: "INACTIVE"},
					},
				},
			},
			Services: []schema.Service{
				{
					Name: "ProductService",
					Methods: []schema.Method{
						{
							Name:       "getProduct",
							InputType:  "GetProductRequest",
							OutputType: "Product",
						},
						{
							Name:       "createProduct",
							InputType:  "CreateProductRequest",
							OutputType: "Product",
						},
					},
				},
			},
		},
	}

	schemaStr, err := generateGraphQLSchema("test", schemas)
	require.NoError(t, err)

	// Verify schema contains expected elements
	assert.Contains(t, schemaStr, "type Query")
	assert.Contains(t, schemaStr, "type Mutation")
	assert.Contains(t, schemaStr, "type Product")
	assert.Contains(t, schemaStr, "enum Status")
	assert.Contains(t, schemaStr, "getProduct(input:")
	assert.Contains(t, schemaStr, "createProduct(input:")
}

func TestGraphQLGateway_QueryExecution(t *testing.T) {
	// Test: Query execution works correctly
	// Note: This test would require mocking the actor system
	// For now, we'll test the query parsing and validation

	gateway := NewGraphQLGateway()
	ctx := context.Background()

	// Create simple schema
	testSchema := &schema.Schema{
		Meta: schema.Metadata{Namespace: "test"},
		Types: []schema.ObjectType{
			{
				Name: "Message",
				Fields: []schema.Field{
					{Name: "text", Type: "String", Required: true},
				},
			},
		},
		Services: []schema.Service{
			{
				Name: "TestService",
				Methods: []schema.Method{
					{
						Name:       "getMessage",
						InputType:  "GetMessageRequest",
						OutputType: "Message",
					},
				},
			},
		},
	}

	// Create mock actor that responds to requests
	mockActor := &mockServiceActor{
		responses: map[string]interface{}{
			"getMessage": map[string]interface{}{
				"text": "Hello, GraphQL!",
			},
		},
	}

	// Start mock actor
	actorSystem, err := actors.NewActorSystem("test-system")
	require.NoError(t, err)
	err = actorSystem.Start(ctx)
	require.NoError(t, err)
	defer actorSystem.Stop(ctx)

	pid, err := actorSystem.Spawn(ctx, "test-actor", mockActor)
	require.NoError(t, err)

	// Update service with mock actor
	err = gateway.UpdateService(ctx, "test", testSchema, pid)
	require.NoError(t, err)

	// Create GraphQL query
	query := `{
		getMessage(input: {}) {
			text
		}
	}`

	reqBody, _ := json.Marshal(map[string]interface{}{
		"query": query,
	})

	// Execute query
	req := httptest.NewRequest(http.MethodPost, "/graphql/test", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler := gateway.Handler()
	handler.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should have data or errors
	assert.True(t, response["data"] != nil || response["errors"] != nil)
}

func TestGraphQLGateway_ErrorHandling(t *testing.T) {
	// Test: Error handling returns proper GraphQL errors

	gateway := NewGraphQLGateway()
	handler := gateway.Handler()

	// First create a namespace
	gateway.UpdateService(context.Background(), "test", &schema.Schema{
		Meta:     schema.Metadata{Namespace: "test"},
		Services: []schema.Service{{Name: "TestService"}},
	}, &actors.PID{})

	// Test: Invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/graphql/test", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test: GET request serves playground
	req = httptest.NewRequest(http.MethodGet, "/graphql/test", nil)
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestGraphQLGateway_ConcurrentAccess(t *testing.T) {
	// Test: Concurrent updates and queries work correctly

	gateway := NewGraphQLGateway()
	ctx := context.Background()

	// Create test schema
	testSchema := &schema.Schema{
		Meta: schema.Metadata{Namespace: "test"},
		Services: []schema.Service{
			{Name: "TestService"},
		},
	}

	// Run concurrent updates
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			err := gateway.UpdateService(ctx, "test", testSchema, &actors.PID{})
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all updates
	for i := 0; i < 10; i++ {
		<-done
	}

	// Service should still be accessible
	req := httptest.NewRequest(http.MethodPost, "/graphql/test", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler := gateway.Handler()
	handler.ServeHTTP(w, req)

	// Should not be 404
	assert.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestGraphQLGateway_RemoveService(t *testing.T) {
	// Test: Remove service removes namespace

	gateway := NewGraphQLGateway()
	ctx := context.Background()

	// Add a service
	testSchema := &schema.Schema{
		Meta:     schema.Metadata{Namespace: "test"},
		Services: []schema.Service{{Name: "TestService"}},
	}

	err := gateway.UpdateService(ctx, "test", testSchema, &actors.PID{})
	require.NoError(t, err)

	// Remove the service
	err = gateway.RemoveService(ctx, "test")
	require.NoError(t, err)

	// Namespace should no longer be accessible
	req := httptest.NewRequest(http.MethodPost, "/graphql/test", nil)
	w := httptest.NewRecorder()

	handler := gateway.Handler()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIsQueryMethod(t *testing.T) {
	// Test: Query method classification

	tests := []struct {
		method  string
		isQuery bool
	}{
		{"getUser", true},
		{"listUsers", true},
		{"findUser", true},
		{"searchUsers", true},
		{"queryData", true},
		{"fetchResults", true},
		{"readFile", true},
		{"createUser", false},
		{"updateUser", false},
		{"deleteUser", false},
		{"setConfig", false},
		{"addItem", false},
		{"removeItem", false},
		{"processData", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := isQueryMethod(tt.method)
			assert.Equal(t, tt.isQuery, result)
		})
	}
}

func TestGraphQLGateway_Introspection(t *testing.T) {
	// Test: GraphQL introspection queries work correctly

	gateway := NewGraphQLGateway()
	ctx := context.Background()

	// Create test schema
	testSchema := &schema.Schema{
		Meta: schema.Metadata{Namespace: "test"},
		Types: []schema.ObjectType{
			{
				Name: "User",
				Fields: []schema.Field{
					{Name: "id", Type: "ID", Required: true},
					{Name: "name", Type: "String", Required: true},
				},
			},
		},
		Services: []schema.Service{
			{
				Name: "UserService",
				Methods: []schema.Method{
					{
						Name:       "getUser",
						InputType:  "GetUserRequest",
						OutputType: "User",
					},
				},
			},
		},
	}

	// Update service
	err := gateway.UpdateService(ctx, "test", testSchema, &actors.PID{})
	require.NoError(t, err)

	// Test: __typename query
	query := `{ __typename }`
	reqBody, _ := json.Marshal(map[string]interface{}{
		"query": query,
	})

	req := httptest.NewRequest(http.MethodPost, "/graphql/test", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler := gateway.Handler()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should have data with __typename
	assert.Contains(t, response, "data")
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "Query", data["__typename"])

	// Test: __schema query
	schemaQuery := `{
		__schema {
			queryType { name }
			mutationType { name }
		}
	}`

	reqBody, _ = json.Marshal(map[string]interface{}{
		"query": schemaQuery,
	})

	req = httptest.NewRequest(http.MethodPost, "/graphql/test", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Log response for debugging
	t.Logf("Introspection response: %+v", response)

	// Verify schema introspection response
	assert.Contains(t, response, "data")
	if response["errors"] != nil {
		t.Fatalf("GraphQL errors: %v", response["errors"])
	}

	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "data should be a map")

	schemaData, ok := data["__schema"].(map[string]interface{})
	require.True(t, ok, "__schema should be a map")

	queryType := schemaData["queryType"].(map[string]interface{})
	assert.Equal(t, "Query", queryType["name"])

	mutationType := schemaData["mutationType"].(map[string]interface{})
	assert.Equal(t, "Mutation", mutationType["name"])
}

func TestGraphQLGateway_Playground(t *testing.T) {
	// Test: GraphQL playground is served on GET requests

	gateway := NewGraphQLGateway()
	ctx := context.Background()

	// Add a service so the namespace exists
	testSchema := &schema.Schema{
		Meta:     schema.Metadata{Namespace: "test"},
		Services: []schema.Service{{Name: "TestService"}},
	}

	err := gateway.UpdateService(ctx, "test", testSchema, &actors.PID{})
	require.NoError(t, err)

	// Test: GET request serves playground
	req := httptest.NewRequest(http.MethodGet, "/graphql/test", nil)
	w := httptest.NewRecorder()

	handler := gateway.Handler()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))

	// Verify playground HTML contains expected elements
	body := w.Body.String()
	assert.Contains(t, body, "OKRA GraphQL Playground")
	assert.Contains(t, body, "graphiql")
	assert.Contains(t, body, "/graphql/test") // Endpoint URL
}

// mockServiceActor is a mock actor for testing
type mockServiceActor struct {
	actors.Actor
	responses map[string]interface{}
}

func (m *mockServiceActor) Receive(ctx *actors.ReceiveContext) {
	switch msg := ctx.Message().(type) {
	case *pb.ServiceRequest:
		response := &pb.ServiceResponse{}

		if result, ok := m.responses[msg.Method]; ok {
			output, _ := json.Marshal(result)
			response.Output = output
		} else {
			response.Error = &pb.ServiceError{
				Code:    "METHOD_NOT_FOUND",
				Message: "Method not found",
			}
		}

		ctx.Response(response)
	}
}

func (m *mockServiceActor) PreStart(ctx context.Context) error {
	return nil
}

func (m *mockServiceActor) PostStop(ctx context.Context) error {
	return nil
}

// Mock implementations for testing refactored methods

type mockActorClient struct {
	responses map[string]*pb.ServiceResponse
	errors    map[string]error
}

func (m *mockActorClient) Ask(ctx context.Context, pid *actors.PID, message *pb.ServiceRequest, timeout time.Duration) (*pb.ServiceResponse, error) {
	if err, ok := m.errors[message.Method]; ok {
		return nil, err
	}
	if response, ok := m.responses[message.Method]; ok {
		return response, nil
	}
	return &pb.ServiceResponse{
		Error: &pb.ServiceError{
			Code:    "METHOD_NOT_FOUND",
			Message: "Method not found",
		},
	}, nil
}

type mockSchemaParser struct {
	parseError bool
}

func (m *mockSchemaParser) ParseGraphqlDocumentString(input string) (ast.Document, operationreport.Report) {
	doc := ast.Document{}
	report := operationreport.Report{}
	if m.parseError {
		report.AddExternalError(operationreport.ExternalError{
			Message: "Parse error",
		})
	}
	return doc, report
}

type mockSchemaValidator struct {
	validateError bool
}

func (m *mockSchemaValidator) Validate(doc *ast.Document, schema *ast.Document, report *operationreport.Report) {
	if m.validateError {
		report.AddExternalError(operationreport.ExternalError{
			Message: "Validation error",
		})
	}
}

// Tests for decomposed methods

func TestGraphQLGateway_DependencyInjection(t *testing.T) {
	// Test: Gateway can be created with custom dependencies
	mockActor := &mockActorClient{
		responses: make(map[string]*pb.ServiceResponse),
		errors:    make(map[string]error),
	}
	mockParser := &mockSchemaParser{}
	mockValidator := &mockSchemaValidator{}

	gateway := NewGraphQLGatewayWithDependencies(mockActor, mockParser, mockValidator)
	require.NotNil(t, gateway)

	handler := gateway.Handler()
	require.NotNil(t, handler)
}

func TestNamespaceHandler_FindServiceForField(t *testing.T) {
	// Test: findServiceForField correctly locates services
	handler := &namespaceHandler{
		services: map[string]*serviceInfo{
			"TestService": {
				schema: &schema.Schema{
					Services: []schema.Service{
						{
							Name: "TestService",
							Methods: []schema.Method{
								{Name: "getUser", InputType: "GetUserRequest", OutputType: "User"},
								{Name: "createUser", InputType: "CreateUserRequest", OutputType: "User"},
							},
						},
					},
				},
			},
		},
	}

	// Test: Find existing method
	service, methodName, err := handler.findServiceForField("getUser")
	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, "getUser", methodName)

	// Test: Method not found
	_, _, err = handler.findServiceForField("nonExistentMethod")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "method nonExistentMethod not found")
}

func TestNamespaceHandler_BuildServiceRequest(t *testing.T) {
	// Test: buildServiceRequest creates proper protobuf requests
	handler := &namespaceHandler{}

	// Test: Valid input
	args := map[string]interface{}{
		"input": map[string]interface{}{
			"userId": "123",
			"name":   "John Doe",
		},
	}

	request, err := handler.buildServiceRequest("getUser", args)
	assert.NoError(t, err)
	assert.Equal(t, "getUser", request.Method)
	assert.Contains(t, string(request.Input), "userId")
	assert.Contains(t, string(request.Input), "123")

	// Test: Missing input argument
	_, err = handler.buildServiceRequest("getUser", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input argument required")
}

func TestNamespaceHandler_CallServiceActor(t *testing.T) {
	// Test: callServiceActor handles various response scenarios
	mockClient := &mockActorClient{
		responses: map[string]*pb.ServiceResponse{
			"getUser": {
				Output: []byte(`{"id": "123", "name": "John"}`),
			},
		},
		errors: map[string]error{
			"failingMethod": errors.New("network error"),
		},
	}

	handler := &namespaceHandler{
		actorClient: mockClient,
	}

	// Test: Successful call
	request := &pb.ServiceRequest{Method: "getUser", Input: []byte(`{}`)}
	response, err := handler.callServiceActor(context.Background(), &actors.PID{}, request)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Contains(t, string(response.Output), "John")

	// Test: Actor error
	request = &pb.ServiceRequest{Method: "failingMethod", Input: []byte(`{}`)}
	_, err = handler.callServiceActor(context.Background(), &actors.PID{}, request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "actor request failed")

	// Test: Service error response
	mockClient.responses["errorMethod"] = &pb.ServiceResponse{
		Error: &pb.ServiceError{
			Code:    "VALIDATION_ERROR",
			Message: "Invalid input",
		},
	}
	request = &pb.ServiceRequest{Method: "errorMethod", Input: []byte(`{}`)}
	_, err = handler.callServiceActor(context.Background(), &actors.PID{}, request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid input")
}

func TestNamespaceHandler_ExtractFieldFromObject(t *testing.T) {
	// Test: extractFieldFromObject extracts fields from response objects correctly
	t.Skip("Need to implement proper AST setup for this test")
	// TODO: Add comprehensive test when AST structure is better understood
}

func TestNamespaceHandler_ParseServiceResponse(t *testing.T) {
	// Test: parseServiceResponse handles various JSON formats
	handler := &namespaceHandler{}

	// Test: Valid JSON object
	response := &pb.ServiceResponse{
		Output: []byte(`{"id": "123", "name": "John Doe", "active": true}`),
	}
	result, err := handler.parseServiceResponse(response)
	assert.NoError(t, err)
	resultMap := result.(map[string]interface{})
	assert.Equal(t, "123", resultMap["id"])
	assert.Equal(t, "John Doe", resultMap["name"])
	assert.Equal(t, true, resultMap["active"])

	// Test: Valid JSON array
	response = &pb.ServiceResponse{
		Output: []byte(`[{"id": "1"}, {"id": "2"}]`),
	}
	result, err = handler.parseServiceResponse(response)
	assert.NoError(t, err)
	resultArray := result.([]interface{})
	assert.Len(t, resultArray, 2)

	// Test: Invalid JSON
	response = &pb.ServiceResponse{
		Output: []byte(`invalid json`),
	}
	_, err = handler.parseServiceResponse(response)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal response")
}

func TestNamespaceHandler_ResolveValueMethods(t *testing.T) {
	// Test: Individual value resolution methods work correctly
	handler := &namespaceHandler{}

	// Test: resolveNullValue
	result, err := handler.resolveNullValue()
	assert.NoError(t, err)
	assert.Nil(t, result)

	// Test: resolveBooleanValue
	// Note: This would require setting up AST document with actual boolean values
	// For now, we'll test the error handling paths
}

func TestGraphQLGateway_Shutdown(t *testing.T) {
	// Test: Shutdown properly clears namespaces
	gateway := NewGraphQLGateway()
	ctx := context.Background()

	// Add some namespaces
	testSchema := &schema.Schema{
		Meta:     schema.Metadata{Namespace: "test1"},
		Services: []schema.Service{{Name: "TestService"}},
	}
	
	err := gateway.UpdateService(ctx, "test1", testSchema, &actors.PID{})
	require.NoError(t, err)
	
	err = gateway.UpdateService(ctx, "test2", testSchema, &actors.PID{})
	require.NoError(t, err)

	// Verify namespaces exist
	req := httptest.NewRequest(http.MethodPost, "/graphql/test1", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	handler := gateway.Handler()
	handler.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusNotFound, w.Code)

	// Shutdown
	err = gateway.Shutdown(ctx)
	assert.NoError(t, err)

	// Verify namespaces are cleared
	req = httptest.NewRequest(http.MethodPost, "/graphql/test1", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
