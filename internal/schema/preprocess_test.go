package schema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreprocessGraphQL_OkraDirective(t *testing.T) {
	// Test plan:
	// - Test basic @okra directive transformation
	// - Test with multiple parameters
	// - Test with different whitespace
	// - Test multiple @okra directives
	// - Test @okra with complex values

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "basic @okra directive",
			input: `@okra(namespace: "auth.users", version: "v1")

type User {
  id: ID!
}`,
			expected: `type _Schema {
  _: String @okra(namespace: "auth.users", version: "v1")
}

type User {
  id: ID!
}`,
		},
		{
			name:  "okra directive with minimal spacing",
			input: `@okra(namespace:"auth",version:"v1")`,
			expected: `type _Schema {
  _: String @okra(namespace:"auth",version:"v1")
}`,
		},
		{
			name: "okra directive with extra whitespace",
			input: `@okra  (  namespace: "auth.users" ,  version: "v1"  )

type User {
  id: ID!
}`,
			expected: `type _Schema {
  _: String @okra(  namespace: "auth.users" ,  version: "v1"  )
}

type User {
  id: ID!
}`,
		},
		{
			name: "multiple okra directives - only first should match",
			input: `@okra(namespace: "auth", version: "v1")
type User {
  id: ID! @okra(internal: true)
}`,
			expected: `type _Schema {
  _: String @okra(namespace: "auth", version: "v1")
}
type User {
  id: ID! @okra(internal: true)
}`,
		},
		{
			name: "okra directive with nested parentheses",
			input: `@okra(namespace: "auth", metadata: { key: "value (nested)" })`,
			expected: `type _Schema {
  _: String @okra(namespace: "auth", metadata: { key: "value (nested)" })
}`,
		},
		{
			name:     "no okra directive",
			input:    `type User { id: ID! }`,
			expected: `type User { id: ID! }`,
		},
		{
			name: "okra directive not at start of line",
			input: `# Comment
  @okra(namespace: "auth", version: "v1")`,
			expected: `# Comment
  @okra(namespace: "auth", version: "v1")`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreprocessGraphQL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPreprocessGraphQL_ServiceBlocks(t *testing.T) {
	// Test plan:
	// - Test basic service block transformation
	// - Test multiple service blocks
	// - Test service with methods and directives
	// - Test different service name formats
	// - Test nested braces within service

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "basic service block",
			input: `service UserService {
  createUser(input: CreateUser): CreateUserResponse
}`,
			expected: `type Service_UserService {
  createUser(input: CreateUser): CreateUserResponse
}`,
		},
		{
			name: "service with directives",
			input: `service AuthService {
  login(input: LoginInput): LoginResponse
    @auth(cel: "true")
    @rateLimit(max: 10)
}`,
			expected: `type Service_AuthService {
  login(input: LoginInput): LoginResponse
    @auth(cel: "true")
    @rateLimit(max: 10)
}`,
		},
		{
			name: "multiple services",
			input: `service UserService {
  getUser(id: ID!): User
}

service OrderService {
  createOrder(input: OrderInput): Order
}`,
			expected: `type Service_UserService {
  getUser(id: ID!): User
}

type Service_OrderService {
  createOrder(input: OrderInput): Order
}`,
		},
		{
			name:  "service with no spacing",
			input: `service MyService{method():Response}`,
			expected: `type Service_MyService {method():Response}`,
		},
		{
			name: "service with underscore in name",
			input: `service User_Management_Service {
  createUser(input: CreateUser): User
}`,
			expected: `type Service_User_Management_Service {
  createUser(input: CreateUser): User
}`,
		},
		{
			name: "service with numbers in name",
			input: `service UserServiceV2 {
  createUser(input: CreateUser): User
}`,
			expected: `type Service_UserServiceV2 {
  createUser(input: CreateUser): User
}`,
		},
		{
			name:     "no service blocks",
			input:    `type User { id: ID! }`,
			expected: `type User { id: ID! }`,
		},
		{
			name: "service not at start of line",
			input: `  service UserService {
  createUser(input: CreateUser): User
}`,
			expected: `  service UserService {
  createUser(input: CreateUser): User
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreprocessGraphQL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPreprocessGraphQL_Combined(t *testing.T) {
	// Test plan:
	// - Test both @okra and service transformations together
	// - Test complex real-world schemas
	// - Test ordering and interactions

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "full schema with okra and service",
			input: `@okra(namespace: "auth.users", version: "v1")

type CreateUser {
  name: String!
  email: String!
}

type User {
  id: ID!
  name: String!
  email: String!
}

service UserService {
  createUser(input: CreateUser): User
    @auth(cel: "auth.role == 'admin'")
}`,
			expected: `type _Schema {
  _: String @okra(namespace: "auth.users", version: "v1")
}

type CreateUser {
  name: String!
  email: String!
}

type User {
  id: ID!
  name: String!
  email: String!
}

type Service_UserService {
  createUser(input: CreateUser): User
    @auth(cel: "auth.role == 'admin'")
}`,
		},
		{
			name: "multiple services with okra directive",
			input: `@okra(namespace: "ecommerce", version: "v2")

service ProductService {
  getProduct(id: ID!): Product
  listProducts(limit: Int): [Product]
}

service OrderService {
  createOrder(input: OrderInput): Order
  getOrder(id: ID!): Order
}`,
			expected: `type _Schema {
  _: String @okra(namespace: "ecommerce", version: "v2")
}

type Service_ProductService {
  getProduct(id: ID!): Product
  listProducts(limit: Int): [Product]
}

type Service_OrderService {
  createOrder(input: OrderInput): Order
  getOrder(id: ID!): Order
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreprocessGraphQL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPreprocessGraphQL_EdgeCases(t *testing.T) {
	// Test plan:
	// - Test empty input
	// - Test comments and special characters
	// - Test malformed input
	// - Test service/okra in strings or comments

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \n\t  \n   ",
			expected: "   \n\t  \n   ",
		},
		{
			name: "service keyword in comments",
			input: `# This is a service definition
type User {
  id: ID!
}`,
			expected: `# This is a service definition
type User {
  id: ID!
}`,
		},
		{
			name: "service keyword in string",
			input: `type Config {
  description: String @default(value: "This is a service config")
}`,
			expected: `type Config {
  description: String @default(value: "This is a service config")
}`,
		},
		{
			name: "okra in type name",
			input: `type OkraConfig {
  setting: String
}`,
			expected: `type OkraConfig {
  setting: String
}`,
		},
		{
			name: "incomplete service declaration",
			input: `service UserService
type User { id: ID! }`,
			expected: `service UserService
type User { id: ID! }`,
		},
		{
			name: "service with special regex characters in name",
			input: `service User$Service {
  method(): Response
}`,
			expected: `service User$Service {
  method(): Response
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PreprocessGraphQL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPreprocessGraphQL_Idempotency(t *testing.T) {
	// Test that preprocessing is idempotent - running it twice should give same result as once

	input := `@okra(namespace: "test", version: "v1")

service TestService {
  test(): String
}`

	firstPass := PreprocessGraphQL(input)
	secondPass := PreprocessGraphQL(firstPass)

	assert.Equal(t, firstPass, secondPass, "preprocessing should be idempotent")
}

func TestPreprocessGraphQL_Performance(t *testing.T) {
	// Test with a large schema to ensure reasonable performance

	var sb strings.Builder
	sb.WriteString(`@okra(namespace: "large", version: "v1")` + "\n\n")

	// Generate a large schema with many services
	for i := range 100 {
		sb.WriteString(`service Service` + string(rune('A'+i%26)) + `Service {` + "\n")
		for j := range 10 {
			sb.WriteString(`  method` + string(rune('0'+j)) + `(input: Input): Output` + "\n")
		}
		sb.WriteString("}\n\n")
	}

	input := sb.String()
	result := PreprocessGraphQL(input)

	// Verify the transformation worked
	assert.Contains(t, result, "type _Schema")
	assert.Contains(t, result, "type Service_ServiceAService")
	assert.NotContains(t, result, "service Service")
}

func TestPreprocessGraphQL_RealWorldExample(t *testing.T) {
	// Test with the example from the documentation

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

	result := PreprocessGraphQL(input)

	// Verify key transformations
	assert.Contains(t, result, "type _Schema")
	assert.Contains(t, result, "@okra(namespace: \"auth.users\", version: \"v1\")")
	assert.Contains(t, result, "type Service_UserService")
	assert.Contains(t, result, "@auth(cel: \"auth.role == 'admin'\")")
	assert.Contains(t, result, "@durable")
	assert.Contains(t, result, "@idempotent(level: \"FULL\")")

	// Verify other types remain unchanged
	assert.Contains(t, result, "type CreateUser {")
	assert.Contains(t, result, "enum UserRole {")
	assert.Contains(t, result, "type CreateUserResponse {")
}