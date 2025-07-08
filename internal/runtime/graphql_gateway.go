package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/okra-platform/okra/internal/runtime/pb"
	"github.com/okra-platform/okra/internal/schema"
	"github.com/tochemey/goakt/v2/actors"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/ast"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/astparser"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/astvalidation"
)

// GraphQLGateway provides HTTP connectivity to OKRA services via GraphQL
type GraphQLGateway interface {
	// Handler returns the HTTP handler for GraphQL requests
	Handler() http.Handler

	// UpdateService updates or adds a service to the GraphQL schema
	UpdateService(ctx context.Context, namespace string, serviceSchema *schema.Schema, actorPID *actors.PID) error

	// RemoveService removes a service from the GraphQL schema
	RemoveService(ctx context.Context, namespace string) error

	// Shutdown gracefully shuts down the gateway
	Shutdown(ctx context.Context) error
}

// NewGraphQLGateway creates a new GraphQL gateway
func NewGraphQLGateway() GraphQLGateway {
	return &graphqlGateway{
		namespaces: make(map[string]*namespaceHandler),
	}
}

type graphqlGateway struct {
	mu         sync.RWMutex
	namespaces map[string]*namespaceHandler
}

// namespaceHandler handles GraphQL requests for a specific namespace
type namespaceHandler struct {
	namespace    string
	schema       *atomic.Value // holds *compiledSchema
	services     map[string]*serviceInfo
	servicesMu   sync.RWMutex
}

type serviceInfo struct {
	schema   *schema.Schema
	actorPID *actors.PID
}

// compiledSchema holds the compiled GraphQL schema
type compiledSchema struct {
	schemaDocument *ast.Document
	schemaString   string
}

// GraphQL request/response structures
type graphqlRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

type graphqlResponse struct {
	Data   interface{}     `json:"data,omitempty"`
	Errors []graphqlError `json:"errors,omitempty"`
}

type graphqlError struct {
	Message    string                 `json:"message"`
	Path       []interface{}         `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

func (g *graphqlGateway) Handler() http.Handler {
	mux := http.NewServeMux()
	
	// Handle /graphql/{namespace} pattern
	mux.HandleFunc("/graphql/", func(w http.ResponseWriter, r *http.Request) {
		// Extract namespace from path
		path := strings.TrimPrefix(r.URL.Path, "/graphql/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, "namespace required", http.StatusBadRequest)
			return
		}
		
		namespace := parts[0]
		
		// Get namespace handler
		g.mu.RLock()
		handler, exists := g.namespaces[namespace]
		g.mu.RUnlock()
		
		if !exists {
			http.Error(w, fmt.Sprintf("namespace '%s' not found", namespace), http.StatusNotFound)
			return
		}
		
		// Handle GraphQL request
		handler.ServeHTTP(w, r)
	})
	
	return mux
}

func (g *graphqlGateway) UpdateService(ctx context.Context, namespace string, serviceSchema *schema.Schema, actorPID *actors.PID) error {
	if namespace == "" {
		namespace = "default"
	}
	
	g.mu.Lock()
	defer g.mu.Unlock()
	
	// Get or create namespace handler
	handler, exists := g.namespaces[namespace]
	if !exists {
		handler = &namespaceHandler{
			namespace: namespace,
			schema:    &atomic.Value{},
			services:  make(map[string]*serviceInfo),
		}
		g.namespaces[namespace] = handler
	}
	
	// Update services
	handler.servicesMu.Lock()
	for _, service := range serviceSchema.Services {
		handler.services[service.Name] = &serviceInfo{
			schema:   serviceSchema,
			actorPID: actorPID,
		}
	}
	handler.servicesMu.Unlock()
	
	// Regenerate schema
	return handler.regenerateSchema()
}

func (g *graphqlGateway) RemoveService(ctx context.Context, namespace string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	delete(g.namespaces, namespace)
	return nil
}

func (g *graphqlGateway) Shutdown(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	g.namespaces = make(map[string]*namespaceHandler)
	return nil
}

// ServeHTTP handles GraphQL requests for a namespace
func (h *namespaceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle GET requests for GraphQL playground in dev mode
	if r.Method == http.MethodGet {
		// Check if we're in dev mode (could be passed via context or config)
		h.servePlayground(w, r)
		return
	}
	
	// Only accept POST requests for GraphQL queries
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse request
	var req graphqlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Get current schema
	schemaVal := h.schema.Load()
	if schemaVal == nil {
		http.Error(w, "Schema not initialized", http.StatusServiceUnavailable)
		return
	}
	
	compiled := schemaVal.(*compiledSchema)
	
	// Parse the query
	query, report := astparser.ParseGraphqlDocumentString(req.Query)
	if report.HasErrors() {
		h.sendErrorResponse(w, "Query parse error", fmt.Errorf("%s", report.Error()))
		return
	}
	
	// Validate the query against schema
	validator := astvalidation.DefaultOperationValidator()
	validator.Validate(&query, compiled.schemaDocument, &report)
	if report.HasErrors() {
		h.sendErrorResponse(w, "Query validation error", fmt.Errorf("%s", report.Error()))
		return
	}
	
	// Execute the query
	result, err := h.executeQuery(r.Context(), &query, req.Variables, req.OperationName)
	if err != nil {
		h.sendErrorResponse(w, "Query execution error", err)
		return
	}
	
	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *namespaceHandler) executeQuery(ctx context.Context, query *ast.Document, variables map[string]interface{}, operationName string) (*graphqlResponse, error) {
	// Find the operation to execute
	var operation *ast.OperationDefinition
	for i := range query.OperationDefinitions {
		op := &query.OperationDefinitions[i]
		opName := query.OperationDefinitionNameString(i)
		if operationName == "" || opName == operationName {
			operation = op
			break
		}
	}
	
	if operation == nil {
		return nil, fmt.Errorf("operation not found")
	}
	
	// Execute based on operation type
	switch operation.OperationType {
	case ast.OperationTypeQuery:
		return h.executeQueryOperation(ctx, query, operation, variables)
	case ast.OperationTypeMutation:
		return h.executeMutationOperation(ctx, query, operation, variables)
	default:
		return nil, fmt.Errorf("unsupported operation type")
	}
}

func (h *namespaceHandler) executeQueryOperation(ctx context.Context, doc *ast.Document, op *ast.OperationDefinition, variables map[string]interface{}) (*graphqlResponse, error) {
	result := &graphqlResponse{
		Data: make(map[string]interface{}),
	}
	
	// Execute each selection in the query
	if op.HasSelections {
		for _, selection := range doc.SelectionSets[op.SelectionSet].SelectionRefs {
			if err := h.executeSelection(ctx, doc, selection, result.Data.(map[string]interface{}), variables); err != nil {
			result.Errors = append(result.Errors, graphqlError{
				Message: err.Error(),
			})
			}
		}
	}
	
	return result, nil
}

func (h *namespaceHandler) executeMutationOperation(ctx context.Context, doc *ast.Document, op *ast.OperationDefinition, variables map[string]interface{}) (*graphqlResponse, error) {
	// Mutations are executed serially
	return h.executeQueryOperation(ctx, doc, op, variables)
}

func (h *namespaceHandler) executeSelection(ctx context.Context, doc *ast.Document, selectionRef int, result map[string]interface{}, variables map[string]interface{}) error {
	selection := doc.Selections[selectionRef]
	
	switch selection.Kind {
	case ast.SelectionKindField:
		field := doc.Fields[selection.Ref]
		fieldName := doc.FieldNameString(selection.Ref)
		
		// Handle introspection fields
		if strings.HasPrefix(fieldName, "__") {
			return h.handleIntrospection(ctx, doc, &field, fieldName, result)
		}
		
		// Handle empty fields
		if fieldName == "_empty" {
			result[fieldName] = nil
			return nil
		}
		
		// Extract arguments
		args := make(map[string]interface{})
		for _, argRef := range field.Arguments.Refs {
			arg := doc.Arguments[argRef]
			argName := doc.ArgumentNameString(argRef)
			value, err := h.resolveValue(doc, arg.Value, variables)
			if err != nil {
				return err
			}
			args[argName] = value
		}
		
		// Execute field resolver
		fieldResult, err := h.resolveField(ctx, fieldName, args)
		if err != nil {
			return err
		}
		
		// Process sub-selections if any
		if field.HasSelections {
			subResult := make(map[string]interface{})
			for _, subSelection := range doc.SelectionSets[field.SelectionSet].SelectionRefs {
				if err := h.executeSelection(ctx, doc, subSelection, subResult, variables); err != nil {
					return err
				}
			}
			// Merge field result with sub-selections
			if fieldResult != nil {
				if fieldMap, ok := fieldResult.(map[string]interface{}); ok {
					for k, v := range subResult {
						fieldMap[k] = v
					}
					result[fieldName] = fieldMap
				} else {
					result[fieldName] = fieldResult
				}
			} else {
				result[fieldName] = subResult
			}
		} else {
			result[fieldName] = fieldResult
		}
		
		return nil
		
	default:
		return fmt.Errorf("unsupported selection kind")
	}
}

func (h *namespaceHandler) resolveField(ctx context.Context, fieldName string, args map[string]interface{}) (interface{}, error) {
	// Find the service and method for this field
	h.servicesMu.RLock()
	defer h.servicesMu.RUnlock()
	
	var targetService *serviceInfo
	var methodName string
	
	// Look for the method in all services
	for _, service := range h.services {
		for _, svc := range service.schema.Services {
			for _, method := range svc.Methods {
				if method.Name == fieldName {
					targetService = service
					methodName = method.Name
					goto found
				}
			}
		}
	}
	
found:
	if targetService == nil {
		return nil, fmt.Errorf("method %s not found", fieldName)
	}
	
	// Get input from arguments
	input, ok := args["input"]
	if !ok {
		return nil, fmt.Errorf("input argument required")
	}
	
	// Convert input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}
	
	// Create service request
	serviceRequest := &pb.ServiceRequest{
		Method: methodName,
		Input:  inputJSON,
	}
	
	// Send request to actor
	reply, err := actors.Ask(ctx, targetService.actorPID, serviceRequest, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("actor request failed: %w", err)
	}
	
	// Cast response
	serviceResponse, ok := reply.(*pb.ServiceResponse)
	if !ok {
		return nil, fmt.Errorf("invalid response type from actor")
	}
	
	// Check for errors
	if serviceResponse.Error != nil {
		return nil, fmt.Errorf("%s", serviceResponse.Error.Message)
	}
	
	// Parse response
	var result interface{}
	if err := json.Unmarshal(serviceResponse.Output, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	return result, nil
}

func (h *namespaceHandler) resolveValue(doc *ast.Document, value ast.Value, variables map[string]interface{}) (interface{}, error) {
	switch value.Kind {
	case ast.ValueKindString:
		return doc.StringValueContentString(value.Ref), nil
	case ast.ValueKindInteger:
		intStr := doc.IntValueRaw(value.Ref)
		intVal, err := strconv.ParseInt(string(intStr), 10, 64)
		if err != nil {
			return nil, err
		}
		return intVal, nil
	case ast.ValueKindFloat:
		floatStr := doc.FloatValueRaw(value.Ref)
		floatVal, err := strconv.ParseFloat(string(floatStr), 64)
		if err != nil {
			return nil, err
		}
		return floatVal, nil
	case ast.ValueKindBoolean:
		return doc.BooleanValue(value.Ref), nil
	case ast.ValueKindNull:
		return nil, nil
	case ast.ValueKindObject:
		obj := make(map[string]interface{})
		for _, fieldRef := range doc.ObjectValues[value.Ref].Refs {
			field := doc.ObjectFields[fieldRef]
			fieldName := doc.ObjectFieldNameString(fieldRef)
			fieldValue, err := h.resolveValue(doc, field.Value, variables)
			if err != nil {
				return nil, err
			}
			obj[fieldName] = fieldValue
		}
		return obj, nil
	case ast.ValueKindList:
		list := make([]interface{}, 0)
		for _, valueRef := range doc.ListValues[value.Ref].Refs {
			itemValue, err := h.resolveValue(doc, doc.Values[valueRef], variables)
			if err != nil {
				return nil, err
			}
			list = append(list, itemValue)
		}
		return list, nil
	case ast.ValueKindVariable:
		varName := doc.VariableValueNameString(value.Ref)
		if val, ok := variables[varName]; ok {
			return val, nil
		}
		return nil, fmt.Errorf("variable %s not found", varName)
	default:
		return nil, fmt.Errorf("unsupported value kind: %v", value.Kind)
	}
}

func (h *namespaceHandler) handleIntrospection(ctx context.Context, doc *ast.Document, field *ast.Field, fieldName string, result map[string]interface{}) error {
	switch fieldName {
	case "__schema":
		return h.handleSchemaIntrospection(ctx, doc, field, result)
	case "__type":
		return h.handleTypeIntrospection(ctx, doc, field, result)
	case "__typename":
		result[fieldName] = "Query"
		return nil
	default:
		result[fieldName] = nil
		return nil
	}
}

func (h *namespaceHandler) sendErrorResponse(w http.ResponseWriter, message string, err error) {
	response := &graphqlResponse{
		Errors: []graphqlError{{
			Message: fmt.Sprintf("%s: %v", message, err),
		}},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // GraphQL errors are still 200 OK
	json.NewEncoder(w).Encode(response)
}

func (h *namespaceHandler) regenerateSchema() error {
	// Collect all schemas
	h.servicesMu.RLock()
	schemas := make([]*schema.Schema, 0, len(h.services))
	for _, service := range h.services {
		schemas = append(schemas, service.schema)
	}
	h.servicesMu.RUnlock()
	
	// Generate GraphQL schema
	schemaStr, err := generateGraphQLSchema(h.namespace, schemas)
	if err != nil {
		return fmt.Errorf("failed to generate GraphQL schema: %w", err)
	}
	
	// Parse the schema
	doc, report := astparser.ParseGraphqlDocumentString(schemaStr)
	if report.HasErrors() {
		return fmt.Errorf("failed to parse GraphQL schema: %v", report)
	}
	
	// Store compiled schema
	compiled := &compiledSchema{
		schemaDocument: &doc,
		schemaString:   schemaStr,
	}
	h.schema.Store(compiled)
	
	return nil
}

// generateGraphQLSchema generates a GraphQL schema from OKRA schemas
func generateGraphQLSchema(namespace string, schemas []*schema.Schema) (string, error) {
	var sb strings.Builder
	
	// Add scalar types first
	sb.WriteString("# Built-in scalar types\n")
	sb.WriteString("scalar String\n")
	sb.WriteString("scalar Int\n")
	sb.WriteString("scalar Float\n")
	sb.WriteString("scalar Boolean\n")
	sb.WriteString("scalar ID\n\n")
	
	// Start with schema definition
	sb.WriteString("schema {\n")
	sb.WriteString("  query: Query\n")
	sb.WriteString("  mutation: Mutation\n")
	sb.WriteString("}\n\n")
	
	// Generate type definitions
	typeMap := make(map[string]bool)
	var queryFields []string
	var mutationFields []string
	
	for _, s := range schemas {
		// Generate enum types
		for _, enum := range s.Enums {
			if !typeMap[enum.Name] {
				generateEnumType(&sb, &enum)
				typeMap[enum.Name] = true
			}
		}
		
		// Generate object types
		for _, typ := range s.Types {
			if !typeMap[typ.Name] {
				generateObjectType(&sb, &typ)
				typeMap[typ.Name] = true
			}
		}
		
		// Generate service methods as fields
		for _, service := range s.Services {
			for _, method := range service.Methods {
				field := generateMethodField(&method)
				
				if isQueryMethod(method.Name) {
					queryFields = append(queryFields, field)
				} else {
					mutationFields = append(mutationFields, field)
				}
			}
		}
	}
	
	// Generate Query type
	sb.WriteString("type Query {\n")
	
	// Add introspection fields
	sb.WriteString("  __schema: __Schema!\n")
	sb.WriteString("  __type(name: String!): __Type\n")
	sb.WriteString("  __typename: String!\n")
	
	if len(queryFields) == 0 {
		sb.WriteString("  _empty: String\n")
	} else {
		for _, field := range queryFields {
			sb.WriteString("  ")
			sb.WriteString(field)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("}\n\n")
	
	// Generate Mutation type
	sb.WriteString("type Mutation {\n")
	if len(mutationFields) == 0 {
		sb.WriteString("  _empty: String\n")
	} else {
		for _, field := range mutationFields {
			sb.WriteString("  ")
			sb.WriteString(field)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("}\n\n")
	
	// Add introspection types
	sb.WriteString(`# Introspection types
type __Schema {
  types: [__Type!]!
  queryType: __Type!
  mutationType: __Type
  subscriptionType: __Type
  directives: [__Directive!]!
}

type __Type {
  kind: __TypeKind!
  name: String
  description: String
  fields(includeDeprecated: Boolean = false): [__Field!]
  interfaces: [__Type!]
  possibleTypes: [__Type!]
  enumValues(includeDeprecated: Boolean = false): [__EnumValue!]
  inputFields: [__InputValue!]
  ofType: __Type
}

type __Field {
  name: String!
  description: String
  args: [__InputValue!]!
  type: __Type!
  isDeprecated: Boolean!
  deprecationReason: String
}

type __InputValue {
  name: String!
  description: String
  type: __Type!
  defaultValue: String
}

type __EnumValue {
  name: String!
  description: String
  isDeprecated: Boolean!
  deprecationReason: String
}

enum __TypeKind {
  SCALAR
  OBJECT
  INTERFACE
  UNION
  ENUM
  INPUT_OBJECT
  LIST
  NON_NULL
}

type __Directive {
  name: String!
  description: String
  locations: [__DirectiveLocation!]!
  args: [__InputValue!]!
}

enum __DirectiveLocation {
  QUERY
  MUTATION
  SUBSCRIPTION
  FIELD
  FRAGMENT_DEFINITION
  FRAGMENT_SPREAD
  INLINE_FRAGMENT
  SCHEMA
  SCALAR
  OBJECT
  FIELD_DEFINITION
  ARGUMENT_DEFINITION
  INTERFACE
  UNION
  ENUM
  ENUM_VALUE
  INPUT_OBJECT
  INPUT_FIELD_DEFINITION
}
`)
	
	return sb.String(), nil
}

// generateEnumType generates GraphQL enum type definition
func generateEnumType(sb *strings.Builder, enum *schema.EnumType) {
	if enum.Doc != "" {
		sb.WriteString(fmt.Sprintf("\"\"\"%s\"\"\"\n", enum.Doc))
	}
	sb.WriteString(fmt.Sprintf("enum %s {\n", enum.Name))
	for _, value := range enum.Values {
		if value.Doc != "" {
			sb.WriteString(fmt.Sprintf("  \"\"\"%s\"\"\"\n", value.Doc))
		}
		sb.WriteString(fmt.Sprintf("  %s\n", value.Name))
	}
	sb.WriteString("}\n\n")
}

// generateObjectType generates GraphQL object type definition
func generateObjectType(sb *strings.Builder, typ *schema.ObjectType) {
	if typ.Doc != "" {
		sb.WriteString(fmt.Sprintf("\"\"\"%s\"\"\"\n", typ.Doc))
	}
	
	// Determine if this should be an input type
	isInput := strings.HasSuffix(typ.Name, "Request") || strings.HasSuffix(typ.Name, "Input")
	
	if isInput {
		sb.WriteString(fmt.Sprintf("input %sInput {\n", strings.TrimSuffix(strings.TrimSuffix(typ.Name, "Request"), "Input")))
	} else {
		sb.WriteString(fmt.Sprintf("type %s {\n", typ.Name))
	}
	
	for _, field := range typ.Fields {
		if field.Doc != "" {
			sb.WriteString(fmt.Sprintf("  \"\"\"%s\"\"\"\n", field.Doc))
		}
		
		fieldType := mapToGraphQLType(field.Type)
		if field.Required {
			fieldType += "!"
		}
		
		sb.WriteString(fmt.Sprintf("  %s: %s\n", field.Name, fieldType))
	}
	sb.WriteString("}\n\n")
}

// generateMethodField generates a GraphQL field for a service method
func generateMethodField(method *schema.Method) string {
	inputType := method.InputType
	if strings.HasSuffix(inputType, "Request") {
		inputType = strings.TrimSuffix(inputType, "Request") + "Input"
	}
	
	outputType := mapToGraphQLType(method.OutputType)
	
	return fmt.Sprintf("%s(input: %s!): %s", method.Name, inputType, outputType)
}

// mapToGraphQLType maps OKRA types to GraphQL types
func mapToGraphQLType(okraType string) string {
	// Handle array types
	if strings.HasSuffix(okraType, "[]") {
		baseType := strings.TrimSuffix(okraType, "[]")
		return fmt.Sprintf("[%s]", mapToGraphQLType(baseType))
	}
	
	switch okraType {
	case "String":
		return "String"
	case "Int":
		return "Int"
	case "Long":
		return "Int"
	case "Float":
		return "Float"
	case "Double":
		return "Float"
	case "Boolean":
		return "Boolean"
	case "ID":
		return "ID"
	case "Time", "DateTime", "Timestamp":
		return "String"
	default:
		// Custom types remain as-is
		return okraType
	}
}

// isQueryMethod determines if a method should be a Query or Mutation
func isQueryMethod(methodName string) bool {
	queryPrefixes := []string{"get", "list", "find", "search", "query", "fetch", "read"}
	methodLower := strings.ToLower(methodName)
	
	for _, prefix := range queryPrefixes {
		if strings.HasPrefix(methodLower, prefix) {
			return true
		}
	}
	
	return false
}

// servePlayground serves the GraphQL playground UI
func (h *namespaceHandler) servePlayground(w http.ResponseWriter, r *http.Request) {
	// Get the endpoint URL from the request
	endpoint := r.URL.Path
	
	playgroundHTML := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <title>OKRA GraphQL Playground - %s</title>
  <link rel="stylesheet" href="https://unpkg.com/graphiql@3/graphiql.min.css" />
  <style>
    body {
      height: 100%%;
      margin: 0;
      width: 100%%;
      overflow: hidden;
    }
    #graphiql {
      height: 100vh;
    }
  </style>
</head>
<body>
  <div id="graphiql">Loading...</div>
  <script crossorigin src="https://unpkg.com/react@18/umd/react.production.min.js"></script>
  <script crossorigin src="https://unpkg.com/react-dom@18/umd/react-dom.production.min.js"></script>
  <script crossorigin src="https://unpkg.com/graphiql@3/graphiql.min.js"></script>
  <script>
    const fetcher = GraphiQL.createFetcher({
      url: '%s',
    });
    
    const root = ReactDOM.createRoot(document.getElementById('graphiql'));
    root.render(
      React.createElement(GraphiQL, {
        fetcher: fetcher,
        defaultQuery: '# Welcome to OKRA GraphQL Playground\\n# Namespace: %s\\n\\n# Example query:\\n{\\n  __schema {\\n    types {\\n      name\\n    }\\n  }\\n}',
      })
    );
  </script>
</body>
</html>
`, h.namespace, endpoint, h.namespace)
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(playgroundHTML))
}

// handleSchemaIntrospection handles __schema introspection queries
func (h *namespaceHandler) handleSchemaIntrospection(ctx context.Context, doc *ast.Document, field *ast.Field, result map[string]interface{}) error {
	// Get current schema
	schemaVal := h.schema.Load()
	if schemaVal == nil {
		return fmt.Errorf("schema not initialized")
	}
	
	_ = schemaVal.(*compiledSchema) // Ensure schema is compiled
	
	// Return introspection schema structure
	schemaInfo := map[string]interface{}{
		"queryType": map[string]interface{}{
			"name": "Query",
		},
		"mutationType": map[string]interface{}{
			"name": "Mutation",
		},
		"types": h.getSchemaTypes(),
		"directives": []interface{}{},
	}
	
	result["__schema"] = schemaInfo
	return nil
}

// handleTypeIntrospection handles __type introspection queries
func (h *namespaceHandler) handleTypeIntrospection(ctx context.Context, doc *ast.Document, field *ast.Field, result map[string]interface{}) error {
	// Extract the type name from arguments
	var typeName string
	for _, argRef := range field.Arguments.Refs {
		arg := doc.Arguments[argRef]
		if doc.ArgumentNameString(argRef) == "name" {
			if arg.Value.Kind == ast.ValueKindString {
				typeName = doc.StringValueContentString(arg.Value.Ref)
				break
			}
		}
	}
	
	if typeName == "" {
		result["__type"] = nil
		return nil
	}
	
	// Find the type in our schema
	typeInfo := h.findType(typeName)
	result["__type"] = typeInfo
	return nil
}

// getSchemaTypes returns all types in the schema for introspection
func (h *namespaceHandler) getSchemaTypes() []interface{} {
	h.servicesMu.RLock()
	defer h.servicesMu.RUnlock()
	
	types := []interface{}{
		// Built-in types
		map[string]interface{}{
			"kind": "SCALAR",
			"name": "String",
			"description": "The String scalar type",
		},
		map[string]interface{}{
			"kind": "SCALAR",
			"name": "Int",
			"description": "The Int scalar type",
		},
		map[string]interface{}{
			"kind": "SCALAR",
			"name": "Float",
			"description": "The Float scalar type",
		},
		map[string]interface{}{
			"kind": "SCALAR",
			"name": "Boolean",
			"description": "The Boolean scalar type",
		},
		map[string]interface{}{
			"kind": "SCALAR",
			"name": "ID",
			"description": "The ID scalar type",
		},
	}
	
	// Add custom types from all services
	typeMap := make(map[string]bool)
	for _, service := range h.services {
		// Add object types
		for _, typ := range service.schema.Types {
			if typeMap[typ.Name] {
				continue
			}
			typeMap[typ.Name] = true
			
			isInput := strings.HasSuffix(typ.Name, "Request") || strings.HasSuffix(typ.Name, "Input")
			typeName := typ.Name
			if isInput {
				typeName = strings.TrimSuffix(strings.TrimSuffix(typ.Name, "Request"), "Input") + "Input"
			}
			
			fields := make([]interface{}, 0, len(typ.Fields))
			for _, field := range typ.Fields {
				fieldType := mapToGraphQLType(field.Type)
				fields = append(fields, map[string]interface{}{
					"name": field.Name,
					"type": h.buildTypeRef(fieldType, field.Required),
					"description": field.Doc,
				})
			}
			
			kind := "OBJECT"
			if isInput {
				kind = "INPUT_OBJECT"
			}
			
			types = append(types, map[string]interface{}{
				"kind": kind,
				"name": typeName,
				"description": typ.Doc,
				"fields": fields,
				"inputFields": fields, // For INPUT_OBJECT types
			})
		}
		
		// Add enum types
		for _, enum := range service.schema.Enums {
			if typeMap[enum.Name] {
				continue
			}
			typeMap[enum.Name] = true
			
			values := make([]interface{}, 0, len(enum.Values))
			for _, val := range enum.Values {
				values = append(values, map[string]interface{}{
					"name": val.Name,
					"description": val.Doc,
				})
			}
			
			types = append(types, map[string]interface{}{
				"kind": "ENUM",
				"name": enum.Name,
				"description": enum.Doc,
				"enumValues": values,
			})
		}
	}
	
	// Add Query and Mutation types
	types = append(types, h.buildQueryType(), h.buildMutationType())
	
	return types
}

// buildQueryType builds the Query type for introspection
func (h *namespaceHandler) buildQueryType() map[string]interface{} {
	fields := []interface{}{
		// Introspection fields
		map[string]interface{}{
			"name": "__schema",
			"type": map[string]interface{}{
				"kind": "NON_NULL",
				"ofType": map[string]interface{}{
					"kind": "OBJECT",
					"name": "__Schema",
				},
			},
		},
		map[string]interface{}{
			"name": "__type",
			"type": map[string]interface{}{
				"kind": "OBJECT",
				"name": "__Type",
			},
			"args": []interface{}{
				map[string]interface{}{
					"name": "name",
					"type": map[string]interface{}{
						"kind": "NON_NULL",
						"ofType": map[string]interface{}{
							"kind": "SCALAR",
							"name": "String",
						},
					},
				},
			},
		},
	}
	
	// Add query methods
	h.servicesMu.RLock()
	for _, service := range h.services {
		for _, svc := range service.schema.Services {
			for _, method := range svc.Methods {
				if isQueryMethod(method.Name) {
					inputType := method.InputType
					if strings.HasSuffix(inputType, "Request") {
						inputType = strings.TrimSuffix(inputType, "Request") + "Input"
					}
					
					fields = append(fields, map[string]interface{}{
						"name": method.Name,
						"type": h.buildTypeRef(method.OutputType, false),
						"args": []interface{}{
							map[string]interface{}{
								"name": "input",
								"type": map[string]interface{}{
									"kind": "NON_NULL",
									"ofType": map[string]interface{}{
										"kind": "INPUT_OBJECT",
										"name": inputType,
									},
								},
							},
						},
					})
				}
			}
		}
	}
	h.servicesMu.RUnlock()
	
	return map[string]interface{}{
		"kind": "OBJECT",
		"name": "Query",
		"fields": fields,
	}
}

// buildMutationType builds the Mutation type for introspection
func (h *namespaceHandler) buildMutationType() map[string]interface{} {
	fields := []interface{}{}
	
	// Add mutation methods
	h.servicesMu.RLock()
	for _, service := range h.services {
		for _, svc := range service.schema.Services {
			for _, method := range svc.Methods {
				if !isQueryMethod(method.Name) {
					inputType := method.InputType
					if strings.HasSuffix(inputType, "Request") {
						inputType = strings.TrimSuffix(inputType, "Request") + "Input"
					}
					
					fields = append(fields, map[string]interface{}{
						"name": method.Name,
						"type": h.buildTypeRef(method.OutputType, false),
						"args": []interface{}{
							map[string]interface{}{
								"name": "input",
								"type": map[string]interface{}{
									"kind": "NON_NULL",
									"ofType": map[string]interface{}{
										"kind": "INPUT_OBJECT",
										"name": inputType,
									},
								},
							},
						},
					})
				}
			}
		}
	}
	h.servicesMu.RUnlock()
	
	if len(fields) == 0 {
		fields = append(fields, map[string]interface{}{
			"name": "_empty",
			"type": map[string]interface{}{
				"kind": "SCALAR",
				"name": "String",
			},
		})
	}
	
	return map[string]interface{}{
		"kind": "OBJECT",
		"name": "Mutation",
		"fields": fields,
	}
}

// buildTypeRef builds a type reference for introspection
func (h *namespaceHandler) buildTypeRef(typeName string, required bool) map[string]interface{} {
	// Handle array types
	if strings.HasSuffix(typeName, "[]") {
		baseType := strings.TrimSuffix(typeName, "[]")
		listType := map[string]interface{}{
			"kind": "LIST",
			"ofType": h.buildTypeRef(baseType, false),
		}
		if required {
			return map[string]interface{}{
				"kind": "NON_NULL",
				"ofType": listType,
			}
		}
		return listType
	}
	
	// Map to GraphQL type
	gqlType := mapToGraphQLType(typeName)
	
	typeRef := map[string]interface{}{
		"kind": "SCALAR",
		"name": gqlType,
	}
	
	// Check if it's a custom type
	if gqlType == typeName {
		// It's not a built-in scalar, so it's likely an object type
		typeRef["kind"] = "OBJECT"
	}
	
	if required {
		return map[string]interface{}{
			"kind": "NON_NULL",
			"ofType": typeRef,
		}
	}
	
	return typeRef
}

// findType finds a specific type by name for introspection
func (h *namespaceHandler) findType(typeName string) map[string]interface{} {
	// Check built-in types
	switch typeName {
	case "String", "Int", "Float", "Boolean", "ID":
		return map[string]interface{}{
			"kind": "SCALAR",
			"name": typeName,
		}
	case "Query":
		return h.buildQueryType()
	case "Mutation":
		return h.buildMutationType()
	}
	
	// Check custom types
	h.servicesMu.RLock()
	defer h.servicesMu.RUnlock()
	
	for _, service := range h.services {
		// Check object types
		for _, typ := range service.schema.Types {
			if typ.Name == typeName {
				fields := make([]interface{}, 0, len(typ.Fields))
				for _, field := range typ.Fields {
					fields = append(fields, map[string]interface{}{
						"name": field.Name,
						"type": h.buildTypeRef(field.Type, field.Required),
					})
				}
				
				return map[string]interface{}{
					"kind": "OBJECT",
					"name": typ.Name,
					"fields": fields,
				}
			}
		}
		
		// Check enum types
		for _, enum := range service.schema.Enums {
			if enum.Name == typeName {
				values := make([]interface{}, 0, len(enum.Values))
				for _, val := range enum.Values {
					values = append(values, map[string]interface{}{
						"name": val.Name,
					})
				}
				
				return map[string]interface{}{
					"kind": "ENUM",
					"name": enum.Name,
					"enumValues": values,
				}
			}
		}
	}
	
	return nil
}