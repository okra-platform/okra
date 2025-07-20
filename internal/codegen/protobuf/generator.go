package protobuf

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/okra-platform/okra/internal/schema"
)

// Generator generates protobuf definitions from OKRA schemas
type Generator struct {
	packageName string
}

// NewGenerator creates a new protobuf generator
func NewGenerator(packageName string) *Generator {
	return &Generator{
		packageName: packageName,
	}
}

// Generate creates protobuf definitions from the schema
func (g *Generator) Generate(s *schema.Schema) (string, error) {
	var buf bytes.Buffer

	// Write header
	buf.WriteString("syntax = \"proto3\";\n\n")
	buf.WriteString(fmt.Sprintf("package %s;\n\n", g.packageName))
	buf.WriteString("option go_package = \"github.com/okra-platform/okra/generated/pb\";\n\n")

	// Write import for common types if needed
	hasTimestamp := g.hasTimestampType(s)
	hasEmpty := g.hasEmptyInputType(s)
	
	if hasTimestamp || hasEmpty {
		if hasTimestamp {
			buf.WriteString("import \"google/protobuf/timestamp.proto\";\n")
		}
		if hasEmpty {
			buf.WriteString("import \"google/protobuf/empty.proto\";\n")
		}
		buf.WriteString("\n")
	}

	// Generate enums
	for _, enum := range s.Enums {
		g.generateEnum(&buf, &enum)
	}

	// Generate message types
	for _, typ := range s.Types {
		g.generateMessage(&buf, &typ)
	}

	// Generate response messages for scalar return types
	g.generateScalarResponseMessages(&buf, s)

	// Generate service definitions
	for _, service := range s.Services {
		g.generateService(&buf, &service)
	}

	return buf.String(), nil
}

// generateEnum generates a protobuf enum definition
func (g *Generator) generateEnum(buf *bytes.Buffer, enum *schema.EnumType) {
	if enum.Doc != "" {
		buf.WriteString(fmt.Sprintf("// %s\n", enum.Doc))
	}
	buf.WriteString(fmt.Sprintf("enum %s {\n", enum.Name))

	// Protobuf requires first enum value to be 0
	buf.WriteString(fmt.Sprintf("  %s_UNSPECIFIED = 0;\n", strings.ToUpper(enum.Name)))

	for i, value := range enum.Values {
		if value.Doc != "" {
			buf.WriteString(fmt.Sprintf("  // %s\n", value.Doc))
		}
		buf.WriteString(fmt.Sprintf("  %s = %d;\n", value.Name, i+1))
	}
	buf.WriteString("}\n\n")
}

// generateMessage generates a protobuf message definition
func (g *Generator) generateMessage(buf *bytes.Buffer, typ *schema.ObjectType) {
	if typ.Doc != "" {
		buf.WriteString(fmt.Sprintf("// %s\n", typ.Doc))
	}
	buf.WriteString(fmt.Sprintf("message %s {\n", typ.Name))

	for i, field := range typ.Fields {
		if field.Doc != "" {
			buf.WriteString(fmt.Sprintf("  // %s\n", field.Doc))
		}

		protoType := g.mapToProtoType(field.Type)
		fieldNum := i + 1

		// Check if field type is a list (ends with [])
		isList := strings.HasSuffix(field.Type, "[]")
		if isList {
			baseType := strings.TrimSuffix(field.Type, "[]")
			protoType = g.mapToProtoType(baseType)
			buf.WriteString(fmt.Sprintf("  repeated %s %s = %d;\n", protoType, field.Name, fieldNum))
		} else if !field.Required {
			buf.WriteString(fmt.Sprintf("  optional %s %s = %d;\n", protoType, field.Name, fieldNum))
		} else {
			buf.WriteString(fmt.Sprintf("  %s %s = %d;\n", protoType, field.Name, fieldNum))
		}
	}
	buf.WriteString("}\n\n")
}

// generateService generates a protobuf service definition
func (g *Generator) generateService(buf *bytes.Buffer, service *schema.Service) {
	if service.Doc != "" {
		buf.WriteString(fmt.Sprintf("// %s\n", service.Doc))
	}
	buf.WriteString(fmt.Sprintf("service %s {\n", service.Name))

	for _, method := range service.Methods {
		if method.Doc != "" {
			buf.WriteString(fmt.Sprintf("  // %s\n", method.Doc))
		}
		
		// Handle empty input type - use google.protobuf.Empty
		inputType := method.InputType
		if inputType == "" {
			inputType = "google.protobuf.Empty"
		}
		
		// Handle scalar output types - use generated response message
		outputType := method.OutputType
		if g.isScalarType(outputType) {
			responseName := method.Name
			// Capitalize first letter for message name
			if len(responseName) > 0 {
				responseName = strings.ToUpper(responseName[:1]) + responseName[1:]
			}
			outputType = responseName + "Response"
		}
		
		buf.WriteString(fmt.Sprintf("  rpc %s(%s) returns (%s);\n",
			method.Name, inputType, outputType))
	}
	buf.WriteString("}\n\n")
}

// mapToProtoType maps OKRA types to protobuf types
func (g *Generator) mapToProtoType(okraType string) string {
	switch okraType {
	case "String", "ID":
		return "string"
	case "Int":
		return "int32"
	case "Long":
		return "int64"
	case "Float":
		return "float"
	case "Double":
		return "double"
	case "Boolean":
		return "bool"
	case "Bytes":
		return "bytes"
	case "Time", "DateTime", "Timestamp":
		return "google.protobuf.Timestamp"
	default:
		// Custom types remain as-is
		return okraType
	}
}

// hasTimestampType checks if the schema uses timestamp types
func (g *Generator) hasTimestampType(s *schema.Schema) bool {
	for _, typ := range s.Types {
		for _, field := range typ.Fields {
			fieldType := strings.TrimSuffix(field.Type, "[]") // Remove list suffix if present
			if fieldType == "Time" || fieldType == "DateTime" || fieldType == "Timestamp" {
				return true
			}
		}
	}
	return false
}

// hasEmptyInputType checks if any service methods have empty input types
func (g *Generator) hasEmptyInputType(s *schema.Schema) bool {
	for _, service := range s.Services {
		for _, method := range service.Methods {
			if method.InputType == "" {
				return true
			}
		}
	}
	return false
}

// isScalarType checks if a type is a protobuf scalar type
func (g *Generator) isScalarType(typeName string) bool {
	switch typeName {
	case "String", "ID", "Int", "Long", "Float", "Double", "Boolean", "Bytes":
		return true
	case "Time", "DateTime", "Timestamp":
		// These map to google.protobuf.Timestamp which is a message type
		return false
	default:
		return false
	}
}

// generateScalarResponseMessages generates response message types for methods with scalar return types
func (g *Generator) generateScalarResponseMessages(buf *bytes.Buffer, s *schema.Schema) {
	// Collect all methods with scalar return types
	scalarReturns := make(map[string]string) // methodName -> scalarType
	
	for _, service := range s.Services {
		for _, method := range service.Methods {
			if g.isScalarType(method.OutputType) {
				responseName := method.Name
				// Capitalize first letter for message name
				if len(responseName) > 0 {
					responseName = strings.ToUpper(responseName[:1]) + responseName[1:]
				}
				responseName += "Response"
				scalarReturns[responseName] = method.OutputType
			}
		}
	}
	
	// Generate message definitions
	for responseName, scalarType := range scalarReturns {
		buf.WriteString(fmt.Sprintf("// %s is the response message for methods returning %s\n", responseName, scalarType))
		buf.WriteString(fmt.Sprintf("message %s {\n", responseName))
		protoType := g.mapToProtoType(scalarType)
		buf.WriteString(fmt.Sprintf("  %s response = 1;\n", protoType))
		buf.WriteString("}\n\n")
	}
}
