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
	"github.com/wundergraph/graphql-go-tools/v2/pkg/operationreport"
)

// Interfaces for better testability through dependency injection

// ActorClient provides an interface for actor communication
type ActorClient interface {
	Ask(ctx context.Context, pid *actors.PID, message *pb.ServiceRequest, timeout time.Duration) (*pb.ServiceResponse, error)
}

// SchemaParser provides an interface for GraphQL schema parsing
type SchemaParser interface {
	ParseGraphqlDocumentString(input string) (ast.Document, operationreport.Report)
}

// SchemaValidator provides an interface for GraphQL schema validation
type SchemaValidator interface {
	Validate(doc *ast.Document, schema *ast.Document, report *operationreport.Report)
}

// Default implementations using the actual libraries

type defaultActorClient struct{}

func (c *defaultActorClient) Ask(ctx context.Context, pid *actors.PID, message *pb.ServiceRequest, timeout time.Duration) (*pb.ServiceResponse, error) {
	reply, err := actors.Ask(ctx, pid, message, timeout)
	if err != nil {
		return nil, err
	}
	response, ok := reply.(*pb.ServiceResponse)
	if !ok {
		return nil, fmt.Errorf("invalid response type from actor")
	}
	return response, nil
}

type defaultSchemaParser struct{}

func (p *defaultSchemaParser) ParseGraphqlDocumentString(input string) (ast.Document, operationreport.Report) {
	return astparser.ParseGraphqlDocumentString(input)
}

type defaultSchemaValidator struct {
	validator *astvalidation.OperationValidator
}

func (v *defaultSchemaValidator) Validate(doc *ast.Document, schema *ast.Document, report *operationreport.Report) {
	v.validator.Validate(doc, schema, report)
}

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

// NewGraphQLGateway creates a new GraphQL gateway with default dependencies
func NewGraphQLGateway() GraphQLGateway {
	return NewGraphQLGatewayWithDependencies(
		&defaultActorClient{},
		&defaultSchemaParser{},
		&defaultSchemaValidator{validator: astvalidation.DefaultOperationValidator()},
	)
}

// NewGraphQLGatewayWithDependencies creates a new GraphQL gateway with custom dependencies for testing
func NewGraphQLGatewayWithDependencies(actorClient ActorClient, schemaParser SchemaParser, schemaValidator SchemaValidator) GraphQLGateway {
	return &graphqlGateway{
		namespaces:      make(map[string]*namespaceHandler),
		actorClient:     actorClient,
		schemaParser:    schemaParser,
		schemaValidator: schemaValidator,
	}
}

type graphqlGateway struct {
	mu              sync.RWMutex
	namespaces      map[string]*namespaceHandler
	actorClient     ActorClient
	schemaParser    SchemaParser
	schemaValidator SchemaValidator
}

// namespaceHandler handles GraphQL requests for a specific namespace
type namespaceHandler struct {
	namespace       string
	schema          *atomic.Value // holds *compiledSchema
	services        map[string]*serviceInfo
	servicesMu      sync.RWMutex
	actorClient     ActorClient
	schemaParser    SchemaParser
	schemaValidator SchemaValidator
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

// Value objects for better parameter management

// QueryContext holds all information needed to execute a GraphQL query
type QueryContext struct {
	Document      *ast.Document
	Variables     map[string]interface{}
	Operation     *ast.OperationDefinition
	OperationName string
}

// FieldResolutionContext holds information needed to resolve a GraphQL field
type FieldResolutionContext struct {
	FieldName    string
	Arguments    map[string]interface{}
	SelectionSet int32
	HasSelection bool
}

// ValueResolutionContext holds information needed to resolve GraphQL values
type ValueResolutionContext struct {
	Document  *ast.Document
	Value     ast.Value
	Variables map[string]interface{}
}

// NewQueryContext creates a new QueryContext from request data
func NewQueryContext(document *ast.Document, variables map[string]interface{}, operationName string) *QueryContext {
	return &QueryContext{
		Document:      document,
		Variables:     variables,
		OperationName: operationName,
	}
}

// FindOperation finds the operation to execute based on the operation name
func (qc *QueryContext) FindOperation() *ast.OperationDefinition {
	for i := range qc.Document.OperationDefinitions {
		op := &qc.Document.OperationDefinitions[i]
		opName := qc.Document.OperationDefinitionNameString(i)
		if qc.OperationName == "" || opName == qc.OperationName {
			qc.Operation = op
			return op
		}
	}
	return nil
}

// NewFieldResolutionContext creates a new FieldResolutionContext
func NewFieldResolutionContext(fieldName string, args map[string]interface{}) *FieldResolutionContext {
	return &FieldResolutionContext{
		FieldName: fieldName,
		Arguments: args,
	}
}

// NewValueResolutionContext creates a new ValueResolutionContext
func NewValueResolutionContext(doc *ast.Document, value ast.Value, variables map[string]interface{}) *ValueResolutionContext {
	return &ValueResolutionContext{
		Document:  doc,
		Value:     value,
		Variables: variables,
	}
}

// GraphQL request/response structures
type graphqlRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

type graphqlResponse struct {
	Data   interface{}    `json:"data,omitempty"`
	Errors []graphqlError `json:"errors,omitempty"`
}

type graphqlError struct {
	Message    string                 `json:"message"`
	Path       []interface{}          `json:"path,omitempty"`
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
			namespace:       namespace,
			schema:          &atomic.Value{},
			services:        make(map[string]*serviceInfo),
			actorClient:     g.actorClient,
			schemaParser:    g.schemaParser,
			schemaValidator: g.schemaValidator,
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

	// Clear all namespaces
	for namespace := range g.namespaces {
		delete(g.namespaces, namespace)
	}

	return nil
}

// ServeHTTP handles GraphQL requests for a namespace
func (h *namespaceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle GET requests by serving the playground
	if r.Method == http.MethodGet {
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
	query, report := h.schemaParser.ParseGraphqlDocumentString(req.Query)
	if report.HasErrors() {
		h.sendErrorResponse(w, "Query parse error", fmt.Errorf("%s", report.Error()))
		return
	}

	// Validate the query against schema
	h.schemaValidator.Validate(&query, compiled.schemaDocument, &report)
	if report.HasErrors() {
		h.sendErrorResponse(w, "Query validation error", fmt.Errorf("%s", report.Error()))
		return
	}

	// Create query context and execute the query
	queryCtx := NewQueryContext(&query, req.Variables, req.OperationName)
	result, err := h.executeQuery(r.Context(), queryCtx)
	if err != nil {
		h.sendErrorResponse(w, "Query execution error", err)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *namespaceHandler) executeQuery(ctx context.Context, queryCtx *QueryContext) (*graphqlResponse, error) {
	// Find the operation to execute
	operation := queryCtx.FindOperation()
	if operation == nil {
		return nil, fmt.Errorf("operation not found")
	}

	// Execute based on operation type
	switch operation.OperationType {
	case ast.OperationTypeQuery:
		return h.executeQueryOperation(ctx, queryCtx.Document, operation, queryCtx.Variables)
	case ast.OperationTypeMutation:
		return h.executeMutationOperation(ctx, queryCtx.Document, operation, queryCtx.Variables)
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
			// If we have sub-selections, we need to extract fields from the response object
			if fieldResult != nil {
				if fieldMap, ok := fieldResult.(map[string]interface{}); ok {
					// Extract only the requested sub-fields from the response object
					subResult := make(map[string]interface{})
					for _, subSelection := range doc.SelectionSets[field.SelectionSet].SelectionRefs {
						if err := h.extractFieldFromObject(doc, subSelection, fieldMap, subResult); err != nil {
							return err
						}
					}
					result[fieldName] = subResult
				} else {
					result[fieldName] = fieldResult
				}
			} else {
				result[fieldName] = nil
			}
		} else {
			result[fieldName] = fieldResult
		}

		return nil

	default:
		return fmt.Errorf("unsupported selection kind")
	}
}

// extractFieldFromObject extracts a field from a response object (not a service method call)
func (h *namespaceHandler) extractFieldFromObject(doc *ast.Document, selectionRef int, sourceObject map[string]interface{}, result map[string]interface{}) error {
	selection := doc.Selections[selectionRef]
	
	switch selection.Kind {
	case ast.SelectionKindField:
		fieldName := doc.FieldNameString(selection.Ref)
		
		// Extract the field value from the source object
		if value, exists := sourceObject[fieldName]; exists {
			result[fieldName] = value
		} else {
			result[fieldName] = nil
		}
		
		return nil
		
	default:
		return fmt.Errorf("unsupported selection kind in field extraction")
	}
}

func (h *namespaceHandler) resolveField(ctx context.Context, fieldName string, args map[string]interface{}) (interface{}, error) {
	// Find the service for this field
	targetService, methodName, err := h.findServiceForField(fieldName)
	if err != nil {
		return nil, err
	}

	// Build the service request
	serviceRequest, err := h.buildServiceRequest(methodName, args)
	if err != nil {
		return nil, err
	}

	// Call the service actor
	serviceResponse, err := h.callServiceActor(ctx, targetService.actorPID, serviceRequest)
	if err != nil {
		return nil, err
	}

	// Parse the response
	return h.parseServiceResponse(serviceResponse)
}

// findServiceForField finds the service and method for a given GraphQL field
func (h *namespaceHandler) findServiceForField(fieldName string) (*serviceInfo, string, error) {
	h.servicesMu.RLock()
	defer h.servicesMu.RUnlock()

	// Look for the method in all services
	for _, service := range h.services {
		for _, svc := range service.schema.Services {
			for _, method := range svc.Methods {
				if method.Name == fieldName {
					return service, method.Name, nil
				}
			}
		}
	}

	return nil, "", fmt.Errorf("method %s not found", fieldName)
}

// buildServiceRequest creates a protobuf service request from GraphQL arguments
func (h *namespaceHandler) buildServiceRequest(methodName string, args map[string]interface{}) (*pb.ServiceRequest, error) {
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

	return &pb.ServiceRequest{
		Method: methodName,
		Input:  inputJSON,
	}, nil
}

// callServiceActor sends a request to the service actor and returns the response
func (h *namespaceHandler) callServiceActor(ctx context.Context, pid *actors.PID, request *pb.ServiceRequest) (*pb.ServiceResponse, error) {
	serviceResponse, err := h.actorClient.Ask(ctx, pid, request, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("actor request failed: %w", err)
	}

	// Check for errors
	if serviceResponse.Error != nil {
		return nil, fmt.Errorf("%s", serviceResponse.Error.Message)
	}

	return serviceResponse, nil
}

// parseServiceResponse parses the JSON output from a service response
func (h *namespaceHandler) parseServiceResponse(response *pb.ServiceResponse) (interface{}, error) {
	var result interface{}
	if err := json.Unmarshal(response.Output, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return result, nil
}

func (h *namespaceHandler) resolveValue(doc *ast.Document, value ast.Value, variables map[string]interface{}) (interface{}, error) {
	ctx := NewValueResolutionContext(doc, value, variables)
	return h.resolveValueWithContext(ctx)
}

func (h *namespaceHandler) resolveValueWithContext(ctx *ValueResolutionContext) (interface{}, error) {
	switch ctx.Value.Kind {
	case ast.ValueKindString:
		return h.resolveStringValue(ctx.Document, ctx.Value)
	case ast.ValueKindInteger:
		return h.resolveIntegerValue(ctx.Document, ctx.Value)
	case ast.ValueKindFloat:
		return h.resolveFloatValue(ctx.Document, ctx.Value)
	case ast.ValueKindBoolean:
		return h.resolveBooleanValue(ctx.Document, ctx.Value)
	case ast.ValueKindNull:
		return h.resolveNullValue()
	case ast.ValueKindObject:
		return h.resolveObjectValueWithContext(ctx)
	case ast.ValueKindList:
		return h.resolveListValueWithContext(ctx)
	case ast.ValueKindVariable:
		return h.resolveVariableValueWithContext(ctx)
	default:
		return nil, fmt.Errorf("unsupported value kind: %v", ctx.Value.Kind)
	}
}

// resolveStringValue resolves a GraphQL string value
func (h *namespaceHandler) resolveStringValue(doc *ast.Document, value ast.Value) (interface{}, error) {
	return doc.StringValueContentString(value.Ref), nil
}

// resolveIntegerValue resolves a GraphQL integer value
func (h *namespaceHandler) resolveIntegerValue(doc *ast.Document, value ast.Value) (interface{}, error) {
	intStr := doc.IntValueRaw(value.Ref)
	intVal, err := strconv.ParseInt(string(intStr), 10, 64)
	if err != nil {
		return nil, err
	}
	return intVal, nil
}

// resolveFloatValue resolves a GraphQL float value
func (h *namespaceHandler) resolveFloatValue(doc *ast.Document, value ast.Value) (interface{}, error) {
	floatStr := doc.FloatValueRaw(value.Ref)
	floatVal, err := strconv.ParseFloat(string(floatStr), 64)
	if err != nil {
		return nil, err
	}
	return floatVal, nil
}

// resolveBooleanValue resolves a GraphQL boolean value
func (h *namespaceHandler) resolveBooleanValue(doc *ast.Document, value ast.Value) (interface{}, error) {
	return doc.BooleanValue(value.Ref), nil
}

// resolveNullValue resolves a GraphQL null value
func (h *namespaceHandler) resolveNullValue() (interface{}, error) {
	return nil, nil
}

// resolveObjectValue resolves a GraphQL object value
func (h *namespaceHandler) resolveObjectValue(doc *ast.Document, value ast.Value, variables map[string]interface{}) (interface{}, error) {
	ctx := NewValueResolutionContext(doc, value, variables)
	return h.resolveObjectValueWithContext(ctx)
}

// resolveObjectValueWithContext resolves a GraphQL object value using context
func (h *namespaceHandler) resolveObjectValueWithContext(ctx *ValueResolutionContext) (interface{}, error) {
	obj := make(map[string]interface{})
	for _, fieldRef := range ctx.Document.ObjectValues[ctx.Value.Ref].Refs {
		field := ctx.Document.ObjectFields[fieldRef]
		fieldName := ctx.Document.ObjectFieldNameString(fieldRef)
		fieldValue, err := h.resolveValue(ctx.Document, field.Value, ctx.Variables)
		if err != nil {
			return nil, err
		}
		obj[fieldName] = fieldValue
	}
	return obj, nil
}

// resolveListValue resolves a GraphQL list value
func (h *namespaceHandler) resolveListValue(doc *ast.Document, value ast.Value, variables map[string]interface{}) (interface{}, error) {
	ctx := NewValueResolutionContext(doc, value, variables)
	return h.resolveListValueWithContext(ctx)
}

// resolveListValueWithContext resolves a GraphQL list value using context
func (h *namespaceHandler) resolveListValueWithContext(ctx *ValueResolutionContext) (interface{}, error) {
	list := make([]interface{}, 0)
	for _, valueRef := range ctx.Document.ListValues[ctx.Value.Ref].Refs {
		itemValue, err := h.resolveValue(ctx.Document, ctx.Document.Values[valueRef], ctx.Variables)
		if err != nil {
			return nil, err
		}
		list = append(list, itemValue)
	}
	return list, nil
}

// resolveVariableValue resolves a GraphQL variable value
func (h *namespaceHandler) resolveVariableValue(doc *ast.Document, value ast.Value, variables map[string]interface{}) (interface{}, error) {
	ctx := NewValueResolutionContext(doc, value, variables)
	return h.resolveVariableValueWithContext(ctx)
}

// resolveVariableValueWithContext resolves a GraphQL variable value using context
func (h *namespaceHandler) resolveVariableValueWithContext(ctx *ValueResolutionContext) (interface{}, error) {
	varName := ctx.Document.VariableValueNameString(ctx.Value.Ref)
	if val, ok := ctx.Variables[varName]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("variable %s not found", varName)
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
	doc, report := h.schemaParser.ParseGraphqlDocumentString(schemaStr)
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
		"types":      h.getSchemaTypes(),
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
			"kind":        "SCALAR",
			"name":        "String",
			"description": "The String scalar type",
		},
		map[string]interface{}{
			"kind":        "SCALAR",
			"name":        "Int",
			"description": "The Int scalar type",
		},
		map[string]interface{}{
			"kind":        "SCALAR",
			"name":        "Float",
			"description": "The Float scalar type",
		},
		map[string]interface{}{
			"kind":        "SCALAR",
			"name":        "Boolean",
			"description": "The Boolean scalar type",
		},
		map[string]interface{}{
			"kind":        "SCALAR",
			"name":        "ID",
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
					"name":        field.Name,
					"type":        h.buildTypeRef(fieldType, field.Required),
					"description": field.Doc,
				})
			}

			kind := "OBJECT"
			if isInput {
				kind = "INPUT_OBJECT"
			}

			types = append(types, map[string]interface{}{
				"kind":        kind,
				"name":        typeName,
				"description": typ.Doc,
				"fields":      fields,
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
					"name":        val.Name,
					"description": val.Doc,
				})
			}

			types = append(types, map[string]interface{}{
				"kind":        "ENUM",
				"name":        enum.Name,
				"description": enum.Doc,
				"enumValues":  values,
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
		"kind":   "OBJECT",
		"name":   "Query",
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
		"kind":   "OBJECT",
		"name":   "Mutation",
		"fields": fields,
	}
}

// buildTypeRef builds a type reference for introspection
func (h *namespaceHandler) buildTypeRef(typeName string, required bool) map[string]interface{} {
	// Handle array types
	if strings.HasSuffix(typeName, "[]") {
		baseType := strings.TrimSuffix(typeName, "[]")
		listType := map[string]interface{}{
			"kind":   "LIST",
			"ofType": h.buildTypeRef(baseType, false),
		}
		if required {
			return map[string]interface{}{
				"kind":   "NON_NULL",
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
			"kind":   "NON_NULL",
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
					"kind":   "OBJECT",
					"name":   typ.Name,
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
					"kind":       "ENUM",
					"name":       enum.Name,
					"enumValues": values,
				}
			}
		}
	}

	return nil
}
