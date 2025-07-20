package runtime

import (
	"testing"

	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/require"
)

// TestDebugSchemaParsingFlow tests the full schema parsing flow
func TestDebugSchemaParsingFlow(t *testing.T) {
	// Use the exact schema file from integration tests
	schemaInput := `@okra(namespace: "test", version: "v1")

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

	// Parse the schema using the actual parser
	parsedSchema, err := schema.ParseSchema(schemaInput)
	require.NoError(t, err, "Schema parsing should succeed")

	t.Logf("Parsed schema metadata: %+v", parsedSchema.Meta)
	t.Logf("Parsed services: %+v", parsedSchema.Services)
	t.Logf("Parsed types: %+v", parsedSchema.Types)

	// Verify the metadata
	require.Equal(t, "test", parsedSchema.Meta.Namespace)
	require.Equal(t, "v1", parsedSchema.Meta.Version)

	// Verify the service
	require.Len(t, parsedSchema.Services, 1)
	service := parsedSchema.Services[0]
	require.Equal(t, "Service", service.Name)

	// Verify the methods
	require.Len(t, service.Methods, 1)
	method := service.Methods[0]
	require.Equal(t, "greet", method.Name)
	t.Logf("Method input type: %s, output type: %s", method.InputType, method.OutputType)

	// Verify the types
	require.Len(t, parsedSchema.Types, 2)
	
	var greetRequest, greetResponse *schema.ObjectType
	for i := range parsedSchema.Types {
		if parsedSchema.Types[i].Name == "GreetRequest" {
			greetRequest = &parsedSchema.Types[i]
		} else if parsedSchema.Types[i].Name == "GreetResponse" {
			greetResponse = &parsedSchema.Types[i]
		}
	}
	
	require.NotNil(t, greetRequest, "GreetRequest type should exist")
	require.NotNil(t, greetResponse, "GreetResponse type should exist")

	// Now test GraphQL schema generation with the parsed schema
	schemaStr, err := generateGraphQLSchema("test", []*schema.Schema{parsedSchema})
	require.NoError(t, err)

	t.Logf("Generated GraphQL Schema from parsed schema:\n%s", schemaStr)

	// The rest should be the same as before
	require.Contains(t, schemaStr, "input GreetInput", "Should generate input type GreetInput")
	require.Contains(t, schemaStr, "type GreetResponse", "Should generate output type GreetResponse")
	require.Contains(t, schemaStr, "greet(input: GreetInput!): GreetResponse", "Should generate greet method")
}