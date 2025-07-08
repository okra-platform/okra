package golang

import (
	"strings"
	"testing"

	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerator_EmptySchema(t *testing.T) {
	// Test: Empty schema generates minimal valid Go code
	g := NewGenerator("example")
	s := &schema.Schema{}
	
	code, err := g.Generate(s)
	require.NoError(t, err)
	
	result := string(code)
	assert.Contains(t, result, "package example")
	assert.NotContains(t, result, "import")
}

func TestGenerator_BasicTypes(t *testing.T) {
	// Test: Generate structs for basic types
	g := NewGenerator("models")
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
	assert.Contains(t, result, "package models")
	assert.Contains(t, result, "// User represents a system user")
	assert.Contains(t, result, "type User struct {")
	assert.Contains(t, result, "Id string `json:\"id\"`")
	assert.Contains(t, result, "Name string `json:\"name\"`")
	assert.Contains(t, result, "Email *string `json:\"email,omitempty\"`")
	assert.Contains(t, result, "Age int `json:\"age\"`")
	assert.Contains(t, result, "Active bool `json:\"active\"`")
}

func TestGenerator_Enums(t *testing.T) {
	// Test: Generate enum types with validation
	g := NewGenerator("types")
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
	assert.Contains(t, result, "type Status string")
	assert.Contains(t, result, "const (")
	assert.Contains(t, result, "StatusPending Status = \"Pending\"")
	assert.Contains(t, result, "StatusActive Status = \"Active\"")
	assert.Contains(t, result, "StatusCompleted Status = \"Completed\"")
	assert.Contains(t, result, "func (e Status) Valid() bool {")
	assert.Contains(t, result, "case StatusPending, StatusActive, StatusCompleted:")
}

func TestGenerator_Services(t *testing.T) {
	// Test: Generate service interfaces
	g := NewGenerator("services")
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
	assert.Contains(t, result, "// UserService manages user operations")
	assert.Contains(t, result, "type UserService interface {")
	assert.Contains(t, result, "// GetUser retrieves a user by ID")
	assert.Contains(t, result, "GetUser(input *GetUserRequest) (*User, error)")
	assert.Contains(t, result, "// CreateUser creates a new user")
	assert.Contains(t, result, "CreateUser(input *CreateUserRequest) (*CreateUserResponse, error)")
}

func TestGenerator_ArrayTypes(t *testing.T) {
	// Test: Handle array types properly
	g := NewGenerator("models")
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
	assert.Contains(t, result, "Members []string `json:\"members\"`")
	assert.Contains(t, result, "Tags *[]string `json:\"tags,omitempty\"`")
	assert.Contains(t, result, "Permissions []Permission `json:\"permissions\"`")
}

func TestGenerator_TypeMapping(t *testing.T) {
	// Test: Verify type mapping works correctly
	g := NewGenerator("types")
	
	tests := []struct {
		okraType string
		required bool
		expected string
	}{
		{"String", true, "string"},
		{"String", false, "*string"},
		{"Int", true, "int"},
		{"Int32", true, "int32"},
		{"Int64", true, "int64"},
		{"Float", true, "float32"},
		{"Float64", true, "float64"},
		{"Boolean", true, "bool"},
		{"Bool", true, "bool"},
		{"Bytes", true, "[]byte"},
		{"Time", true, "time.Time"},
		{"Any", true, "interface{}"},
		{"CustomType", true, "CustomType"},
		{"[String]", true, "[]string"},
		{"[CustomType]", true, "[]CustomType"},
	}
	
	for _, tt := range tests {
		result := g.mapToGoType(tt.okraType, tt.required)
		assert.Equal(t, tt.expected, result, "Failed for type %s (required=%v)", tt.okraType, tt.required)
	}
}

func TestGenerator_TimeImport(t *testing.T) {
	// Test: Time type triggers time import
	g := NewGenerator("models")
	s := &schema.Schema{
		Types: []schema.ObjectType{
			{
				Name: "Event",
				Fields: []schema.Field{
					{Name: "createdAt", Type: "Time", Required: true},
				},
			},
		},
	}
	
	code, err := g.Generate(s)
	require.NoError(t, err)
	
	result := string(code)
	assert.Contains(t, result, "import (")
	assert.Contains(t, result, "\"time\"")
	assert.Contains(t, result, "CreatedAt time.Time")
}

func TestGenerator_CompleteExample(t *testing.T) {
	// Test: Generate a complete example with all features
	g := NewGenerator("api")
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
	
	// Check structure
	assert.Contains(t, result, "package api")
	assert.Contains(t, result, "import (")
	assert.Contains(t, result, "\"time\"")
	
	// Check enum
	assert.Contains(t, result, "type Role string")
	assert.Contains(t, result, "RoleAdmin Role = \"Admin\"")
	
	// Check types
	assert.Contains(t, result, "type User struct")
	assert.Contains(t, result, "type GetUserRequest struct")
	
	// Check service
	assert.Contains(t, result, "type UserService interface")
	assert.Contains(t, result, "GetUser(input *GetUserRequest) (*User, error)")
}


func TestGenerator_FieldNameExport(t *testing.T) {
	// Test: Field names are properly exported
	g := NewGenerator("test")
	
	tests := []struct {
		input    string
		expected string
	}{
		{"id", "Id"},
		{"name", "Name"},
		{"userID", "UserID"},
		{"HTTPStatus", "HTTPStatus"},
		{"xmlData", "XmlData"},
		{"", ""},
	}
	
	for _, tt := range tests {
		result := g.exportedName(tt.input)
		assert.Equal(t, tt.expected, result, "Failed for input: %s", tt.input)
	}
}

func TestGenerator_EmptyDocComment(t *testing.T) {
	// Test: Empty doc comments don't generate comment lines
	g := NewGenerator("test")
	s := &schema.Schema{
		Types: []schema.ObjectType{
			{
				Name: "NoDoc",
				Doc:  "", // Empty doc
				Fields: []schema.Field{
					{Name: "field", Type: "String", Required: true, Doc: ""},
				},
			},
		},
	}
	
	code, err := g.Generate(s)
	require.NoError(t, err)
	
	result := string(code)
	lines := strings.Split(result, "\n")
	
	// Find the type declaration
	for i, line := range lines {
		if strings.Contains(line, "type NoDoc struct") {
			// Check that the previous line is not a comment
			if i > 0 {
				assert.NotContains(t, lines[i-1], "//")
			}
			break
		}
	}
}