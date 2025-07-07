package schema

import (
	"fmt"
	"strings"

	"github.com/wundergraph/graphql-go-tools/v2/pkg/ast"
	"github.com/wundergraph/graphql-go-tools/v2/pkg/astparser"
)

// ParseSchema parses a GraphQL schema (after preprocessing) into our Schema model
func ParseSchema(input string) (*Schema, error) {
	// First preprocess the input
	preprocessed := PreprocessGraphQL(input)

	// Parse the GraphQL document
	doc, report := astparser.ParseGraphqlDocumentString(preprocessed)
	if report.HasErrors() {
		return nil, fmt.Errorf("failed to parse GraphQL: %v", report)
	}

	// Create the schema
	schema := &Schema{
		Types:    []ObjectType{},
		Enums:    []EnumType{},
		Services: []Service{},
		Meta:     Metadata{},
	}

	// Walk through definitions
	for i := range doc.RootNodes {
		node := &doc.RootNodes[i]
		switch node.Kind {
		case ast.NodeKindObjectTypeDefinition:
			if err := parseObjectType(&doc, node.Ref, schema); err != nil {
				return nil, err
			}
		case ast.NodeKindEnumTypeDefinition:
			if err := parseEnumType(&doc, node.Ref, schema); err != nil {
				return nil, err
			}
		}
	}

	return schema, nil
}

func parseObjectType(doc *ast.Document, ref int, schema *Schema) error {
	typeDef := doc.ObjectTypeDefinitions[ref]
	typeName := doc.Input.ByteSliceString(typeDef.Name)

	// Check if this is the _Schema type (contains okra metadata)
	if typeName == "_Schema" {
		return parseOkraMetadata(doc, typeDef, schema)
	}

	// Check if this is a service (type Service_*)
	if strings.HasPrefix(typeName, "Service_") {
		serviceName := strings.TrimPrefix(typeName, "Service_")
		return parseService(doc, typeDef, serviceName, schema)
	}

	// Regular object type
	objType := ObjectType{
		Name:   typeName,
		Doc:    getDescription(doc, typeDef.Description),
		Fields: []Field{},
	}

	// Parse fields
	for _, fieldRef := range typeDef.FieldsDefinition.Refs {
		field := parseField(doc, fieldRef)
		objType.Fields = append(objType.Fields, field)
	}

	schema.Types = append(schema.Types, objType)
	return nil
}

func parseEnumType(doc *ast.Document, ref int, schema *Schema) error {
	enumDef := doc.EnumTypeDefinitions[ref]
	
	enumType := EnumType{
		Name:   doc.Input.ByteSliceString(enumDef.Name),
		Doc:    getDescription(doc, enumDef.Description),
		Values: []EnumValue{},
	}

	// Parse enum values
	for _, valueRef := range enumDef.EnumValuesDefinition.Refs {
		valueDef := doc.EnumValueDefinitions[valueRef]
		enumType.Values = append(enumType.Values, EnumValue{
			Name: doc.Input.ByteSliceString(valueDef.EnumValue),
			Doc:  getDescription(doc, valueDef.Description),
		})
	}

	schema.Enums = append(schema.Enums, enumType)
	return nil
}

func parseOkraMetadata(doc *ast.Document, typeDef ast.ObjectTypeDefinition, schema *Schema) error {
	// Find the field with @okra directive
	for _, fieldRef := range typeDef.FieldsDefinition.Refs {
		fieldDef := doc.FieldDefinitions[fieldRef]
		
		// Look for @okra directive on the field
		for _, directiveRef := range fieldDef.Directives.Refs {
			directive := doc.Directives[directiveRef]
			directiveName := doc.Input.ByteSliceString(directive.Name)
			
			if directiveName == "okra" {
				// Parse directive arguments
				args := parseDirectiveArgs(doc, directive)
				schema.Meta.Namespace = args["namespace"]
				schema.Meta.Version = args["version"]
				schema.Meta.Service = args["service"]
				return nil
			}
		}
	}
	
	return nil
}

func parseService(doc *ast.Document, typeDef ast.ObjectTypeDefinition, serviceName string, schema *Schema) error {
	service := Service{
		Name:    serviceName,
		Doc:     getDescription(doc, typeDef.Description),
		Methods: []Method{},
	}

	// Parse methods (fields in the service type)
	for _, fieldRef := range typeDef.FieldsDefinition.Refs {
		method := parseMethod(doc, fieldRef)
		service.Methods = append(service.Methods, method)
	}

	schema.Services = append(schema.Services, service)
	return nil
}

func parseField(doc *ast.Document, fieldRef int) Field {
	fieldDef := doc.FieldDefinitions[fieldRef]
	
	field := Field{
		Name:       doc.Input.ByteSliceString(fieldDef.Name),
		Doc:        getDescription(doc, fieldDef.Description),
		Directives: parseDirectives(doc, fieldDef.Directives),
	}

	// Parse type
	typeStr, required := parseType(doc, fieldDef.Type)
	field.Type = typeStr
	field.Required = required

	return field
}

func parseMethod(doc *ast.Document, fieldRef int) Method {
	fieldDef := doc.FieldDefinitions[fieldRef]
	
	method := Method{
		Name:       doc.Input.ByteSliceString(fieldDef.Name),
		Doc:        getDescription(doc, fieldDef.Description),
		Directives: parseDirectives(doc, fieldDef.Directives),
	}

	// Parse output type
	outputType, _ := parseType(doc, fieldDef.Type)
	method.OutputType = outputType

	// Parse input type from arguments
	if len(fieldDef.ArgumentsDefinition.Refs) > 0 {
		// Assume first argument is the input
		argRef := fieldDef.ArgumentsDefinition.Refs[0]
		argDef := doc.InputValueDefinitions[argRef]
		inputType, _ := parseType(doc, argDef.Type)
		method.InputType = inputType
	}

	return method
}

func parseType(doc *ast.Document, typeRef int) (string, bool) {
	required := false
	currentRef := typeRef

	// Handle NonNull wrapper
	if doc.Types[currentRef].TypeKind == ast.TypeKindNonNull {
		required = true
		currentRef = doc.Types[currentRef].OfType
	}

	// Handle List wrapper
	if doc.Types[currentRef].TypeKind == ast.TypeKindList {
		innerRef := doc.Types[currentRef].OfType
		innerType, _ := parseType(doc, innerRef)
		return "[" + innerType + "]", required
	}

	// Named type
	if doc.Types[currentRef].TypeKind == ast.TypeKindNamed {
		typeName := doc.Input.ByteSliceString(doc.Types[currentRef].Name)
		return typeName, required
	}

	return "Unknown", required
}

func parseDirectives(doc *ast.Document, directives ast.DirectiveList) []Directive {
	result := []Directive{}
	
	for _, directiveRef := range directives.Refs {
		directive := doc.Directives[directiveRef]
		
		result = append(result, Directive{
			Name: doc.Input.ByteSliceString(directive.Name),
			Args: parseDirectiveArgs(doc, directive),
		})
	}
	
	return result
}

func parseDirectiveArgs(doc *ast.Document, directive ast.Directive) map[string]string {
	args := make(map[string]string)
	
	for _, argRef := range directive.Arguments.Refs {
		arg := doc.Arguments[argRef]
		argName := doc.Input.ByteSliceString(arg.Name)
		
		// Get the value using ArgumentValue method
		value := doc.ArgumentValue(argRef)
		argValue := parseValue(doc, value)
		args[argName] = argValue
	}
	
	return args
}

func parseValue(doc *ast.Document, value ast.Value) string {
	switch value.Kind {
	case ast.ValueKindString:
		// For string values, use the document's string value methods
		return doc.StringValueContentString(value.Ref)
		
	case ast.ValueKindEnum:
		// For enum values, the Ref points to the EnumValue
		if value.Ref >= 0 && value.Ref < len(doc.EnumValues) {
			return doc.Input.ByteSliceString(doc.EnumValues[value.Ref].Name)
		}
		
	case ast.ValueKindBoolean:
		// Boolean values are stored in the BooleanValues array
		// The Ref is either 0 (false) or 1 (true)
		if value.Ref >= 0 && value.Ref < len(doc.BooleanValues) {
			if doc.BooleanValues[value.Ref] {
				return "true"
			}
			return "false"
		}
		
	case ast.ValueKindInteger:
		// For integer values, use the document's int value methods
		return fmt.Sprintf("%d", doc.IntValueAsInt(value.Ref))
		
	case ast.ValueKindFloat:
		// For float values, use the document's float value methods
		return fmt.Sprintf("%f", doc.FloatValueAsFloat32(value.Ref))
	}
	
	return ""
}

func getDescription(doc *ast.Document, desc ast.Description) string {
	if !desc.IsDefined {
		return ""
	}
	
	return doc.Input.ByteSliceString(desc.Content)
}