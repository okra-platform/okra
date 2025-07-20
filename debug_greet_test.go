package main

import (
	"context"
	"testing"

	"github.com/okra-platform/okra/internal/runtime"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/require"
	"github.com/tochemey/goakt/v2/actors"
)

func TestGreetSchemaValidation(t *testing.T) {
	// Reproduce the exact schema from integration tests
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

	// Create gateway
	gateway := runtime.NewGraphQLGateway()
	ctx := context.Background()
	mockPID := &actors.PID{}

	// Update service - this should generate the schema internally
	err := gateway.UpdateService(ctx, "test", testSchema, mockPID)
	require.NoError(t, err, "Failed to update service")

	t.Logf("Schema structure: %+v", testSchema)
	t.Logf("Service methods: %+v", testSchema.Services[0].Methods)
}