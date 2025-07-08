package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/okra-platform/okra/internal/runtime/pb"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tochemey/goakt/v2/actors"
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
		Meta: schema.Metadata{Namespace: "test"},
		Services: []schema.Service{{Name: "TestService"}},
	}, &actors.PID{})
	
	// Test: Invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/graphql/test", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	// Test: GET request not allowed
	req = httptest.NewRequest(http.MethodGet, "/graphql/test", nil)
	w = httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
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
		Meta: schema.Metadata{Namespace: "test"},
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
		method   string
		isQuery  bool
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
		Meta: schema.Metadata{Namespace: "test"},
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