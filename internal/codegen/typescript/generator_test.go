package typescript

import (
	"strings"
	"testing"

	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerator_EmptySchema(t *testing.T) {
	// Test: Empty schema generates minimal valid TypeScript code
	g := NewGenerator("")
	s := &schema.Schema{}

	code, err := g.Generate(s)
	require.NoError(t, err)

	result := string(code)
	assert.Equal(t, "", strings.TrimSpace(result))
}

func TestGenerator_BasicTypes(t *testing.T) {
	// Test: Generate interfaces for basic types
	g := NewGenerator("")
	s := &schema.Schema{
		Types: []schema.ObjectType{
			{
				Name: "User",
				Doc:  "User represents a system user",
				Fields: []schema.Field{
					{Name: "id", Type: "String", Required: true},
					{Name: "name", Type: "String", Required: true},
					{Name: "email", Type: "String", Required: false},
					{Name: "age", Type: "Int", Required: true},
					{Name: "active", Type: "Boolean", Required: true},
				},
			},
		},
	}

	code, err := g.Generate(s)
	require.NoError(t, err)

	result := string(code)
	assert.Contains(t, result, "/** User represents a system user */")
	assert.Contains(t, result, "export interface User {")
	assert.Contains(t, result, "id: string;")
	assert.Contains(t, result, "name: string;")
	assert.Contains(t, result, "email?: string;")
	assert.Contains(t, result, "age: number;")
	assert.Contains(t, result, "active: boolean;")
}

func TestGenerator_WithClasses(t *testing.T) {
	// Test: Generate classes instead of interfaces
	g := NewGenerator("").WithClasses(true)
	s := &schema.Schema{
		Types: []schema.ObjectType{
			{
				Name: "User",
				Fields: []schema.Field{
					{Name: "id", Type: "String", Required: true},
				},
			},
		},
	}

	code, err := g.Generate(s)
	require.NoError(t, err)

	result := string(code)
	assert.Contains(t, result, "export class User {")
	assert.NotContains(t, result, "export interface User {")
}

func TestGenerator_Enums(t *testing.T) {
	// Test: Generate enum types with type guards
	g := NewGenerator("")
	s := &schema.Schema{
		Enums: []schema.EnumType{
			{
				Name: "Status",
				Doc:  "Status represents the status of an operation",
				Values: []schema.EnumValue{
					{Name: "Pending", Doc: "Operation is pending"},
					{Name: "Active", Doc: "Operation is active"},
					{Name: "Completed", Doc: "Operation is completed"},
				},
			},
		},
	}

	code, err := g.Generate(s)
	require.NoError(t, err)

	result := string(code)
	assert.Contains(t, result, "/** Status represents the status of an operation */")
	assert.Contains(t, result, "export enum Status {")
	assert.Contains(t, result, "/** Operation is pending */")
	assert.Contains(t, result, "Pending = \"Pending\",")
	assert.Contains(t, result, "Active = \"Active\",")
	assert.Contains(t, result, "Completed = \"Completed\",")
	assert.Contains(t, result, "export function isStatus(value: any): value is Status {")
	assert.Contains(t, result, "return Object.values(Status).includes(value);")
}

func TestGenerator_Services(t *testing.T) {
	// Test: Generate service interfaces and abstract classes
	g := NewGenerator("")
	s := &schema.Schema{
		Services: []schema.Service{
			{
				Name:      "UserService",
				Doc:       "UserService manages user operations",
				Namespace: "com.example.user",
				Version:   "v1",
				Methods: []schema.Method{
					{
						Name:       "getUser",
						Doc:        "GetUser retrieves a user by ID",
						InputType:  "GetUserRequest",
						OutputType: "User",
					},
					{
						Name:       "createUser",
						Doc:        "CreateUser creates a new user",
						InputType:  "CreateUserRequest",
						OutputType: "CreateUserResponse",
					},
				},
			},
		},
	}

	code, err := g.Generate(s)
	require.NoError(t, err)

	result := string(code)
	assert.Contains(t, result, "/** UserService manages user operations */")
	assert.Contains(t, result, "export interface UserService {")
	assert.Contains(t, result, "/** GetUser retrieves a user by ID */")
	assert.Contains(t, result, "getUser(input: GetUserRequest): Promise<User>;")
	assert.Contains(t, result, "createUser(input: CreateUserRequest): Promise<CreateUserResponse>;")
	assert.Contains(t, result, "export abstract class UserServiceClient implements UserService {")
	assert.Contains(t, result, "abstract getUser(input: GetUserRequest): Promise<User>;")
}

func TestGenerator_ArrayTypes(t *testing.T) {
	// Test: Handle array types properly
	g := NewGenerator("")
	s := &schema.Schema{
		Types: []schema.ObjectType{
			{
				Name: "Group",
				Fields: []schema.Field{
					{Name: "members", Type: "[String]", Required: true},
					{Name: "tags", Type: "[String]", Required: false},
					{Name: "permissions", Type: "[Permission]", Required: true},
				},
			},
		},
	}

	code, err := g.Generate(s)
	require.NoError(t, err)

	result := string(code)
	assert.Contains(t, result, "members: string[];")
	assert.Contains(t, result, "tags?: string[];")
	assert.Contains(t, result, "permissions: Permission[];")
}

func TestGenerator_TypeMapping(t *testing.T) {
	// Test: Verify type mapping works correctly
	g := NewGenerator("")

	tests := []struct {
		okraType string
		expected string
	}{
		{"String", "string"},
		{"Int", "number"},
		{"Int32", "number"},
		{"Int64", "number"},
		{"Float", "number"},
		{"Float64", "number"},
		{"Boolean", "boolean"},
		{"Bool", "boolean"},
		{"Bytes", "Uint8Array"},
		{"Time", "Date"},
		{"Any", "any"},
		{"CustomType", "CustomType"},
		{"[String]", "string[]"},
		{"[CustomType]", "CustomType[]"},
	}

	for _, tt := range tests {
		result := g.mapToTSType(tt.okraType)
		assert.Equal(t, tt.expected, result, "Failed for type %s", tt.okraType)
	}
}

func TestGenerator_WithModule(t *testing.T) {
	// Test: Generate code within a module
	g := NewGenerator("MyAPI")
	s := &schema.Schema{
		Types: []schema.ObjectType{
			{
				Name: "User",
				Fields: []schema.Field{
					{Name: "id", Type: "String", Required: true},
				},
			},
		},
	}

	code, err := g.Generate(s)
	require.NoError(t, err)

	result := string(code)
	assert.Contains(t, result, "export module MyAPI {")
	assert.Contains(t, result, "export interface User {")
	assert.Contains(t, result, "}")

	// Check proper indentation within module
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.Contains(line, "export interface User") {
			assert.True(t, strings.HasPrefix(line, "  "), "Interface should be indented within module")
		}
	}
}

func TestGenerator_JSDocFormatting(t *testing.T) {
	// Test: JSDoc formatting for single and multi-line docs
	g := NewGenerator("")
	s := &schema.Schema{
		Types: []schema.ObjectType{
			{
				Name: "Test",
				Doc:  "Single line doc",
				Fields: []schema.Field{
					{
						Name: "field1",
						Type: "String",
						Doc: `Multi line
documentation that
spans multiple lines`,
						Required: true,
					},
				},
			},
		},
	}

	code, err := g.Generate(s)
	require.NoError(t, err)

	result := string(code)
	// Single line JSDoc
	assert.Contains(t, result, "/** Single line doc */")
	// Multi-line JSDoc
	assert.Contains(t, result, "/**")
	assert.Contains(t, result, " * Multi line")
	assert.Contains(t, result, " * documentation that")
	assert.Contains(t, result, " * spans multiple lines")
	assert.Contains(t, result, " */")
}

func TestGenerator_CompleteExample(t *testing.T) {
	// Test: Generate a complete example with all features
	g := NewGenerator("")
	s := &schema.Schema{
		Meta: schema.Metadata{
			Namespace: "com.example.api",
			Version:   "v1",
			Service:   "example",
		},
		Enums: []schema.EnumType{
			{
				Name: "Role",
				Values: []schema.EnumValue{
					{Name: "Admin"},
					{Name: "User"},
					{Name: "Guest"},
				},
			},
		},
		Types: []schema.ObjectType{
			{
				Name: "User",
				Doc:  "User represents a system user",
				Fields: []schema.Field{
					{Name: "id", Type: "String", Required: true},
					{Name: "email", Type: "String", Required: true},
					{Name: "role", Type: "Role", Required: true},
					{Name: "createdAt", Type: "Time", Required: true},
					{Name: "metadata", Type: "Any", Required: false},
				},
			},
			{
				Name: "GetUserRequest",
				Fields: []schema.Field{
					{Name: "userId", Type: "String", Required: true},
				},
			},
			{
				Name: "CreateUserRequest",
				Fields: []schema.Field{
					{Name: "email", Type: "String", Required: true},
					{Name: "role", Type: "Role", Required: true},
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
					{
						Name:       "createUser",
						InputType:  "CreateUserRequest",
						OutputType: "User",
					},
				},
			},
		},
	}

	code, err := g.Generate(s)
	require.NoError(t, err)

	result := string(code)

	// Check enum
	assert.Contains(t, result, "export enum Role")
	assert.Contains(t, result, "Admin = \"Admin\"")

	// Check types
	assert.Contains(t, result, "export interface User")
	assert.Contains(t, result, "createdAt: Date;")
	assert.Contains(t, result, "metadata?: any;")

	// Check service
	assert.Contains(t, result, "export interface UserService")
	assert.Contains(t, result, "getUser(input: GetUserRequest): Promise<User>;")
	assert.Contains(t, result, "export abstract class UserServiceClient")
}

func TestGenerator_CamelCaseConversion(t *testing.T) {
	// Test: Method names are converted to camelCase
	g := NewGenerator("")

	tests := []struct {
		input    string
		expected string
	}{
		{"GetUser", "getUser"},
		{"CreateUser", "createUser"},
		{"ID", "iD"},
		{"HTTPStatus", "hTTPStatus"},
		{"xmlData", "xmlData"},
		{"", ""},
	}

	for _, tt := range tests {
		result := g.toCamelCase(tt.input)
		assert.Equal(t, tt.expected, result, "Failed for input: %s", tt.input)
	}
}
