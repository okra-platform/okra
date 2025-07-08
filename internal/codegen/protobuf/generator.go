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
	if hasTimestamp {
		buf.WriteString("import \"google/protobuf/timestamp.proto\";\n\n")
	}

	// Generate enums
	for _, enum := range s.Enums {
		g.generateEnum(&buf, &enum)
	}

	// Generate message types
	for _, typ := range s.Types {
		g.generateMessage(&buf, &typ)
	}

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
		buf.WriteString(fmt.Sprintf("  rpc %s(%s) returns (%s);\n", 
			method.Name, method.InputType, method.OutputType))
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