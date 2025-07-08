package protobuf

import (
	"strings"
	"testing"

	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test plan for protobuf generator:
// 1. Test basic protobuf generation with header
// 2. Test enum generation with proper ordering
// 3. Test message generation with various field types
// 4. Test service generation
// 5. Test optional and repeated fields
// 6. Test timestamp import when needed
// 7. Test type mapping

func TestGenerator_Generate(t *testing.T) {
	// Test: Basic protobuf generation
	gen := NewGenerator("testpkg")
	
	schema := &schema.Schema{
		Services: []schema.Service{
			{
				Name: "TestService",
				Doc:  "A test service",
				Methods: []schema.Method{
					{
						Name:       "GetUser",
						Doc:        "Gets a user by ID",
						InputType:  "GetUserRequest",
						OutputType: "GetUserResponse",
					},
				},
			},
		},
		Types: []schema.ObjectType{
			{
				Name: "GetUserRequest",
				Fields: []schema.Field{
					{Name: "id", Type: "String", Required: true},
				},
			},
			{
				Name: "GetUserResponse",
				Fields: []schema.Field{
					{Name: "user", Type: "User", Required: true},
				},
			},
			{
				Name: "User",
				Fields: []schema.Field{
					{Name: "id", Type: "String", Required: true},
					{Name: "name", Type: "String", Required: true},
				},
			},
		},
	}
	
	proto, err := gen.Generate(schema)
	require.NoError(t, err)
	
	// Check header
	assert.Contains(t, proto, "syntax = \"proto3\";")
	assert.Contains(t, proto, "package testpkg;")
	assert.Contains(t, proto, "option go_package")
	
	// Check service
	assert.Contains(t, proto, "service TestService {")
	assert.Contains(t, proto, "rpc GetUser(GetUserRequest) returns (GetUserResponse);")
	
	// Check messages
	assert.Contains(t, proto, "message GetUserRequest {")
	assert.Contains(t, proto, "string id = 1;")
	assert.Contains(t, proto, "message User {")
}

func TestGenerator_GenerateEnum(t *testing.T) {
	// Test: Enum generation with UNSPECIFIED value
	gen := NewGenerator("testpkg")
	
	schema := &schema.Schema{
		Enums: []schema.EnumType{
			{
				Name: "Status",
				Doc:  "User status",
				Values: []schema.EnumValue{
					{Name: "ACTIVE", Doc: "User is active"},
					{Name: "INACTIVE"},
					{Name: "BANNED"},
				},
			},
		},
	}
	
	proto, err := gen.Generate(schema)
	require.NoError(t, err)
	
	// Check enum with proper values
	assert.Contains(t, proto, "// User status")
	assert.Contains(t, proto, "enum Status {")
	assert.Contains(t, proto, "STATUS_UNSPECIFIED = 0;")
	assert.Contains(t, proto, "// User is active")
	assert.Contains(t, proto, "ACTIVE = 1;")
	assert.Contains(t, proto, "INACTIVE = 2;")
	assert.Contains(t, proto, "BANNED = 3;")
}

func TestGenerator_FieldTypes(t *testing.T) {
	// Test: Various field types and modifiers
	gen := NewGenerator("testpkg")
	
	schema := &schema.Schema{
		Types: []schema.ObjectType{
			{
				Name: "TestMessage",
				Fields: []schema.Field{
					{Name: "string_field", Type: "String", Required: true},
					{Name: "int_field", Type: "Int", Required: true},
					{Name: "long_field", Type: "Long", Required: true},
					{Name: "float_field", Type: "Float", Required: true},
					{Name: "double_field", Type: "Double", Required: true},
					{Name: "bool_field", Type: "Boolean", Required: true},
					{Name: "bytes_field", Type: "Bytes", Required: true},
					{Name: "optional_field", Type: "String", Required: false},
					{Name: "repeated_field", Type: "String[]", Required: true},
					{Name: "custom_field", Type: "CustomType", Required: true},
				},
			},
		},
	}
	
	proto, err := gen.Generate(schema)
	require.NoError(t, err)
	
	// Check type mappings
	assert.Contains(t, proto, "string string_field = 1;")
	assert.Contains(t, proto, "int32 int_field = 2;")
	assert.Contains(t, proto, "int64 long_field = 3;")
	assert.Contains(t, proto, "float float_field = 4;")
	assert.Contains(t, proto, "double double_field = 5;")
	assert.Contains(t, proto, "bool bool_field = 6;")
	assert.Contains(t, proto, "bytes bytes_field = 7;")
	assert.Contains(t, proto, "optional string optional_field = 8;")
	assert.Contains(t, proto, "repeated string repeated_field = 9;")
	assert.Contains(t, proto, "CustomType custom_field = 10;")
}

func TestGenerator_TimestampImport(t *testing.T) {
	// Test: Timestamp import is added when needed
	gen := NewGenerator("testpkg")
	
	schema := &schema.Schema{
		Types: []schema.ObjectType{
			{
				Name: "Event",
				Fields: []schema.Field{
					{Name: "id", Type: "String", Required: true},
					{Name: "created_at", Type: "Timestamp", Required: true},
					{Name: "updated_at", Type: "DateTime", Required: true},
				},
			},
		},
	}
	
	proto, err := gen.Generate(schema)
	require.NoError(t, err)
	
	// Check timestamp import
	assert.Contains(t, proto, "import \"google/protobuf/timestamp.proto\";")
	assert.Contains(t, proto, "google.protobuf.Timestamp created_at = 2;")
	assert.Contains(t, proto, "google.protobuf.Timestamp updated_at = 3;")
}

func TestGenerator_ComplexSchema(t *testing.T) {
	// Test: Complex schema with multiple services, types, and enums
	gen := NewGenerator("complex")
	
	schema := &schema.Schema{
		Services: []schema.Service{
			{
				Name: "UserService",
				Methods: []schema.Method{
					{Name: "CreateUser", InputType: "CreateUserRequest", OutputType: "User"},
					{Name: "UpdateUser", InputType: "UpdateUserRequest", OutputType: "User"},
				},
			},
			{
				Name: "OrderService",
				Methods: []schema.Method{
					{Name: "PlaceOrder", InputType: "PlaceOrderRequest", OutputType: "Order"},
				},
			},
		},
		Types: []schema.ObjectType{
			{
				Name: "User",
				Fields: []schema.Field{
					{Name: "id", Type: "ID", Required: true},
					{Name: "email", Type: "String", Required: true},
					{Name: "orders", Type: "Order[]", Required: true},
				},
			},
			{
				Name: "Order",
				Fields: []schema.Field{
					{Name: "id", Type: "ID", Required: true},
					{Name: "items", Type: "OrderItem[]", Required: true},
					{Name: "status", Type: "OrderStatus", Required: true},
				},
			},
		},
		Enums: []schema.EnumType{
			{
				Name: "OrderStatus",
				Values: []schema.EnumValue{
					{Name: "PENDING"},
					{Name: "PAID"},
					{Name: "SHIPPED"},
					{Name: "DELIVERED"},
				},
			},
		},
	}
	
	proto, err := gen.Generate(schema)
	require.NoError(t, err)
	
	// Count occurrences
	serviceCount := strings.Count(proto, "service ")
	messageCount := strings.Count(proto, "message ")
	enumCount := strings.Count(proto, "enum ")
	
	assert.Equal(t, 2, serviceCount)
	assert.Equal(t, 2, messageCount)
	assert.Equal(t, 1, enumCount)
}