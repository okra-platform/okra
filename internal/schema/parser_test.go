package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSchema_BasicTypes(t *testing.T) {
	// Test plan:
	// - Parse basic object types
	// - Parse enum types
	// - Verify field types and required flags

	input := `
type User {
  id: ID!
  name: String!
  email: String
  age: Int
}

enum UserRole {
  ADMIN
  USER
  GUEST
}

type Post {
  id: ID!
  title: String!
  content: String
  author: User!
  tags: [String!]!
}`

	schema, err := ParseSchema(input)
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Test: Check types
	assert.Len(t, schema.Types, 2)
	assert.Len(t, schema.Enums, 1)
	assert.Len(t, schema.Services, 0)

	// Test: User type
	userType := schema.Types[0]
	assert.Equal(t, "User", userType.Name)
	assert.Len(t, userType.Fields, 4)

	// Check User fields
	assert.Equal(t, "id", userType.Fields[0].Name)
	assert.Equal(t, "ID", userType.Fields[0].Type)
	assert.True(t, userType.Fields[0].Required)

	assert.Equal(t, "name", userType.Fields[1].Name)
	assert.Equal(t, "String", userType.Fields[1].Type)
	assert.True(t, userType.Fields[1].Required)

	assert.Equal(t, "email", userType.Fields[2].Name)
	assert.Equal(t, "String", userType.Fields[2].Type)
	assert.False(t, userType.Fields[2].Required)

	// Test: UserRole enum
	roleEnum := schema.Enums[0]
	assert.Equal(t, "UserRole", roleEnum.Name)
	assert.Len(t, roleEnum.Values, 3)
	assert.Equal(t, "ADMIN", roleEnum.Values[0].Name)
	assert.Equal(t, "USER", roleEnum.Values[1].Name)
	assert.Equal(t, "GUEST", roleEnum.Values[2].Name)

	// Test: Post type with list field
	postType := schema.Types[1]
	assert.Equal(t, "Post", postType.Name)
	
	// Check tags field (list type)
	tagsField := postType.Fields[4]
	assert.Equal(t, "tags", tagsField.Name)
	assert.Equal(t, "[String]", tagsField.Type)
	assert.True(t, tagsField.Required)
}

func TestParseSchema_WithOkraDirective(t *testing.T) {
	// Test plan:
	// - Parse @okra directive
	// - Verify metadata extraction

	input := `@okra(namespace: "auth.users", version: "v1", service: "UserService")

type User {
  id: ID!
  name: String!
}`

	schema, err := ParseSchema(input)
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Test: Check metadata
	assert.Equal(t, "auth.users", schema.Meta.Namespace)
	assert.Equal(t, "v1", schema.Meta.Version)
	assert.Equal(t, "UserService", schema.Meta.Service)

	// Test: Ensure _Schema type is not included in types
	for _, typ := range schema.Types {
		assert.NotEqual(t, "_Schema", typ.Name)
	}
}

func TestParseSchema_Services(t *testing.T) {
	// Test plan:
	// - Parse service definitions
	// - Verify methods and their types
	// - Check directives on methods

	input := `
type CreateUserInput {
  name: String!
  email: String!
}

type CreateUserResponse {
  userId: ID!
  success: Boolean!
}

service UserService {
  createUser(input: CreateUserInput): CreateUserResponse
    @auth(cel: "auth.role == 'admin'")
    @durable
  
  getUser(input: GetUserInput): User
    @auth(cel: "true")
}`

	schema, err := ParseSchema(input)
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Test: Check service
	assert.Len(t, schema.Services, 1)
	
	service := schema.Services[0]
	assert.Equal(t, "UserService", service.Name)
	assert.Len(t, service.Methods, 2)

	// Test: createUser method
	createMethod := service.Methods[0]
	assert.Equal(t, "createUser", createMethod.Name)
	assert.Equal(t, "CreateUserInput", createMethod.InputType)
	assert.Equal(t, "CreateUserResponse", createMethod.OutputType)
	assert.Len(t, createMethod.Directives, 2)

	// Check directives
	authDirective := createMethod.Directives[0]
	assert.Equal(t, "auth", authDirective.Name)
	assert.Equal(t, "auth.role == 'admin'", authDirective.Args["cel"])

	durableDirective := createMethod.Directives[1]
	assert.Equal(t, "durable", durableDirective.Name)

	// Test: getUser method
	getMethod := service.Methods[1]
	assert.Equal(t, "getUser", getMethod.Name)
	assert.Equal(t, "GetUserInput", getMethod.InputType)
	assert.Equal(t, "User", getMethod.OutputType)
}

func TestParseSchema_FieldDirectives(t *testing.T) {
	// Test plan:
	// - Parse directives on fields
	// - Verify directive arguments

	input := `
type User {
  name: String! @validate(cel: "size(name) > 0", message: "Name cannot be empty")
  email: String! @validate(cel: "email.matches('.*@.*')")
  age: Int @min(value: "0") @max(value: "150")
}`

	schema, err := ParseSchema(input)
	require.NoError(t, err)
	require.NotNil(t, schema)

	userType := schema.Types[0]

	// Test: name field directives
	nameField := userType.Fields[0]
	assert.Len(t, nameField.Directives, 1)
	validateDir := nameField.Directives[0]
	assert.Equal(t, "validate", validateDir.Name)
	assert.Equal(t, "size(name) > 0", validateDir.Args["cel"])
	assert.Equal(t, "Name cannot be empty", validateDir.Args["message"])

	// Test: age field directives
	ageField := userType.Fields[2]
	assert.Len(t, ageField.Directives, 2)
	assert.Equal(t, "min", ageField.Directives[0].Name)
	assert.Equal(t, "0", ageField.Directives[0].Args["value"])
	assert.Equal(t, "max", ageField.Directives[1].Name)
	assert.Equal(t, "150", ageField.Directives[1].Args["value"])
}

func TestParseSchema_ComplexExample(t *testing.T) {
	// Test the example from the documentation

	input := `@okra(namespace: "auth.users", version: "v1")

type CreateUser {
  name: String! @validate(cel: "size(name) > 0")
  email: String! @validate(cel: "email.matches('.*@.*')")
  role: UserRole!
}

enum UserRole {
  ADMIN
  USER
  GUEST
}

type CreateUserResponse {
  userId: ID!
}

service UserService {
  createUser(input: CreateUser): CreateUserResponse
    @auth(cel: "auth.role == 'admin'")
    @durable
    @idempotent(level: "FULL")
}`

	schema, err := ParseSchema(input)
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Verify complete structure
	assert.Equal(t, "auth.users", schema.Meta.Namespace)
	assert.Equal(t, "v1", schema.Meta.Version)
	
	assert.Len(t, schema.Types, 2) // CreateUser, CreateUserResponse
	assert.Len(t, schema.Enums, 1) // UserRole
	assert.Len(t, schema.Services, 1) // UserService

	// Check CreateUser type with validations
	createUserType := schema.Types[0]
	assert.Equal(t, "CreateUser", createUserType.Name)
	assert.Len(t, createUserType.Fields, 3)
	
	// Verify field validations
	nameField := createUserType.Fields[0]
	assert.Equal(t, "name", nameField.Name)
	assert.True(t, nameField.Required)
	assert.Len(t, nameField.Directives, 1)
	assert.Equal(t, "size(name) > 0", nameField.Directives[0].Args["cel"])

	// Check service method
	service := schema.Services[0]
	method := service.Methods[0]
	assert.Equal(t, "createUser", method.Name)
	assert.Len(t, method.Directives, 3)
	
	// Verify idempotent directive
	idempotentDir := method.Directives[2]
	assert.Equal(t, "idempotent", idempotentDir.Name)
	assert.Equal(t, "FULL", idempotentDir.Args["level"])
}

func TestParseSchema_MultipleServices(t *testing.T) {
	// Test plan:
	// - Parse multiple services
	// - Verify each service is parsed correctly

	input := `
service UserService {
  getUser(input: GetUserInput): User
}

service OrderService {
  createOrder(input: CreateOrderInput): Order
  cancelOrder(input: CancelOrderInput): CancelOrderResponse
}

service ProductService {
  listProducts(input: ListProductsInput): ProductList
}`

	schema, err := ParseSchema(input)
	require.NoError(t, err)
	require.NotNil(t, schema)

	assert.Len(t, schema.Services, 3)
	
	// Verify service names and method counts
	assert.Equal(t, "UserService", schema.Services[0].Name)
	assert.Len(t, schema.Services[0].Methods, 1)
	
	assert.Equal(t, "OrderService", schema.Services[1].Name)
	assert.Len(t, schema.Services[1].Methods, 2)
	
	assert.Equal(t, "ProductService", schema.Services[2].Name)
	assert.Len(t, schema.Services[2].Methods, 1)
}

func TestParseSchema_Errors(t *testing.T) {
	// Test plan:
	// - Test invalid GraphQL syntax
	// - Test empty input

	tests := []struct {
		name  string
		input string
	}{
		{
			name: "invalid syntax",
			input: `type User {
  name: String!
  email String  // missing colon
}`,
		},
		{
			name: "unclosed type",
			input: `type User {
  name: String!`,
		},
		{
			name: "invalid directive",
			input: `type User {
  name: String! @validate(
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := ParseSchema(tt.input)
			assert.Error(t, err)
			assert.Nil(t, schema)
		})
	}
}

func TestParseSchema_IntegrationTestSchema(t *testing.T) {
	// Test the exact schema from the integration test that's failing
	input := `@okra(namespace: "test", version: "v1")

service Service {
  greet(input: GreetRequest): GreetResponse
}

type GreetRequest {
  name: String!
}

type GreetResponse {
  message: String!
  timestamp: String!
}`

	schema, err := ParseSchema(input)
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Test: Check metadata
	assert.Equal(t, "test", schema.Meta.Namespace)
	assert.Equal(t, "v1", schema.Meta.Version)

	// Test: Check services
	assert.Len(t, schema.Services, 1, "Expected 1 service")
	service := schema.Services[0]
	assert.Equal(t, "Service", service.Name)
	assert.Len(t, service.Methods, 1)

	// Test: Check method
	method := service.Methods[0]
	assert.Equal(t, "greet", method.Name)
	assert.Equal(t, "GreetRequest", method.InputType)
	assert.Equal(t, "GreetResponse", method.OutputType)

	// Test: Check types
	assert.Len(t, schema.Types, 2)
}

func TestParseSchema_EmptySchema(t *testing.T) {
	// Test parsing empty or minimal schemas

	tests := []struct {
		name     string
		input    string
		expected func(*Schema)
	}{
		{
			name:  "empty string",
			input: "",
			expected: func(s *Schema) {
				assert.Empty(t, s.Types)
				assert.Empty(t, s.Enums)
				assert.Empty(t, s.Services)
			},
		},
		{
			name:  "only comments",
			input: "# This is a comment\n# Another comment",
			expected: func(s *Schema) {
				assert.Empty(t, s.Types)
				assert.Empty(t, s.Enums)
				assert.Empty(t, s.Services)
			},
		},
		{
			name:  "only okra directive",
			input: `@okra(namespace: "test", version: "v1")`,
			expected: func(s *Schema) {
				assert.Equal(t, "test", s.Meta.Namespace)
				assert.Equal(t, "v1", s.Meta.Version)
				assert.Empty(t, s.Types)
				assert.Empty(t, s.Services)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := ParseSchema(tt.input)
			require.NoError(t, err)
			require.NotNil(t, schema)
			tt.expected(schema)
		})
	}
}