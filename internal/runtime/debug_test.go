package runtime

import (
	"testing"

	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/require"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/astvalidation"
)

// TestDebugGreetSchemaGeneration helps debug the greet schema validation issue
func TestDebugGreetSchemaGeneration(t *testing.T) {
	// Create the exact schema from integration tests
	testSchema := &schema.Schema{
		Meta: schema.Metadata{
			Namespace: "test",
			Version:   "v1",
		},
		Types: []schema.ObjectType{
			{
				Name: "GreetRequest",
				Fields: []schema.Field{
					{Name: "name", Type: "String", Required: true},
				},
			},
			{
				Name: "GreetResponse", 
				Fields: []schema.Field{
					{Name: "message", Type: "String", Required: true},
					{Name: "timestamp", Type: "String", Required: true},
				},
			},
		},
		Services: []schema.Service{
			{
				Name: "Service",
				Methods: []schema.Method{
					{
						Name:       "greet",
						InputType:  "GreetRequest",
						OutputType: "GreetResponse",
					},
				},
			},
		},
	}

	// Generate GraphQL schema
	schemaStr, err := generateGraphQLSchema("test", []*schema.Schema{testSchema})
	require.NoError(t, err)

	t.Logf("Generated GraphQL Schema:\n%s", schemaStr)

	// Check for expected elements in the schema
	require.Contains(t, schemaStr, "input GreetInput", "Should generate input type GreetInput")
	require.Contains(t, schemaStr, "type GreetResponse", "Should generate output type GreetResponse")
	require.Contains(t, schemaStr, "greet(input: GreetInput!): GreetResponse", "Should generate greet method")
	require.Contains(t, schemaStr, "type Mutation", "Should have Mutation type")
	
	// Test the specific method classification
	isQuery := isQueryMethod("greet")
	t.Logf("Is 'greet' classified as query? %v", isQuery)
	require.False(t, isQuery, "greet should be classified as a mutation, not a query")

	// Now test query validation against this schema
	testQuery := `mutation {
		greet(input: {name: "GraphQL Test"}) {
			message
			timestamp
		}
	}`
	
	// Parse both schema and query
	schemaParser := &defaultSchemaParser{}
	schemaDoc, schemaReport := schemaParser.ParseGraphqlDocumentString(schemaStr)
	require.False(t, schemaReport.HasErrors(), "Schema should parse without errors: %v", schemaReport.Error())
	
	queryDoc, queryReport := schemaParser.ParseGraphqlDocumentString(testQuery)
	require.False(t, queryReport.HasErrors(), "Query should parse without errors: %v", queryReport.Error())
	
	// Validate query against schema
	validator := &defaultSchemaValidator{validator: astvalidation.DefaultOperationValidator()}
	validator.Validate(&queryDoc, &schemaDoc, &queryReport)
	
	if queryReport.HasErrors() {
		t.Logf("Query validation errors: %s", queryReport.Error())
		t.Logf("Query being validated:\n%s", testQuery)
		t.Logf("Against schema:\n%s", schemaStr)
	}
	
	require.False(t, queryReport.HasErrors(), "Query should validate successfully against the generated schema")
}