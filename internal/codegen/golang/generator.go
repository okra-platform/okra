package golang

import (
	"strings"

	"github.com/okra-platform/okra/internal/codegen/writer"
	"github.com/okra-platform/okra/internal/schema"
)

// Generator generates Go code from an OKRA schema
type Generator struct {
	packageName string
	imports     map[string]bool
}

// NewGenerator creates a new Go code generator
func NewGenerator(packageName string) *Generator {
	return &Generator{
		packageName: packageName,
		imports:     make(map[string]bool),
	}
}

// Language returns the name of the target language
func (g *Generator) Language() string {
	return "go"
}

// FileExtension returns the file extension for generated files
func (g *Generator) FileExtension() string {
	return ".go"
}

// Generate generates Go interfaces and types from the schema
func (g *Generator) Generate(s *schema.Schema) ([]byte, error) {
	// For service interface, always use "types" package
	if g.packageName == "" {
		g.packageName = "types"
	}
	
	w := writer.NewWriter("\t")

	// Write package declaration
	w.WriteLinef("package %s", g.packageName)
	w.BlankLine()

	// Collect imports
	g.collectImports(s)

	// Write imports if any
	if len(g.imports) > 0 {
		w.WriteLine("import (")
		w.Indent()
		for imp := range g.imports {
			w.WriteLinef(`"%s"`, imp)
		}
		w.Dedent()
		w.WriteLine(")")
		w.BlankLine()
	}

	// Generate enums
	for _, enum := range s.Enums {
		g.generateEnum(w, enum)
		w.BlankLine()
	}

	// Generate types
	for _, typ := range s.Types {
		g.generateType(w, typ)
		w.BlankLine()
	}

	// Generate service interfaces
	for _, svc := range s.Services {
		g.generateServiceInterface(w, svc)
		w.BlankLine()
	}

	return w.Bytes(), nil
}


// collectImports analyzes the schema and collects required imports
func (g *Generator) collectImports(s *schema.Schema) {
	// No context needed for WASM services

	// Check for types that might need additional imports
	for _, typ := range s.Types {
		for _, field := range typ.Fields {
			g.checkTypeImports(field.Type)
		}
	}

	// Check service method types
	for _, svc := range s.Services {
		for _, method := range svc.Methods {
			g.checkTypeImports(method.InputType)
			g.checkTypeImports(method.OutputType)
		}
	}
}

// checkTypeImports checks if a type requires additional imports
func (g *Generator) checkTypeImports(typ string) {
	// Handle array types
	if strings.HasPrefix(typ, "[") && strings.HasSuffix(typ, "]") {
		inner := typ[1 : len(typ)-1]
		g.checkTypeImports(inner)
		return
	}

	// Check for time types
	if typ == "Time" || typ == "time.Time" {
		g.imports["time"] = true
	}
}

// generateEnum generates Go code for an enum type
func (g *Generator) generateEnum(w *writer.Writer, enum schema.EnumType) {
	// Write documentation
	w.WriteDocComment(enum.Doc)

	// Generate the base type
	w.WriteLinef("type %s string", enum.Name)
	w.BlankLine()

	// Generate constants
	w.WriteLine("const (")
	w.Indent()
	for i, value := range enum.Values {
		if value.Doc != "" {
			w.WriteDocComment(value.Doc)
		}
		prefix := enum.Name
		w.WriteLinef("%s%s %s = \"%s\"", prefix, value.Name, enum.Name, value.Name)
		if i < len(enum.Values)-1 {
			w.BlankLine()
		}
	}
	w.Dedent()
	w.WriteLine(")")

	// Generate validation method
	w.BlankLine()
	w.WriteLinef("// Valid returns true if the %s is a valid value", enum.Name)
	w.WriteLinef("func (e %s) Valid() bool {", enum.Name)
	w.Indent()
	w.WriteLine("switch e {")
	w.Write("case ")
	for i, value := range enum.Values {
		w.Writef("%s%s", enum.Name, value.Name)
		if i < len(enum.Values)-1 {
			w.Write(", ")
		}
	}
	w.WriteLine(":")
	w.Indent()
	w.WriteLine("return true")
	w.Dedent()
	w.WriteLine("default:")
	w.Indent()
	w.WriteLine("return false")
	w.Dedent()
	w.WriteLine("}")
	w.Dedent()
	w.WriteLine("}")
}

// generateType generates Go struct for an object type
func (g *Generator) generateType(w *writer.Writer, typ schema.ObjectType) {
	// Write documentation
	w.WriteDocComment(typ.Doc)

	// Generate struct
	w.WriteLinef("type %s struct {", typ.Name)
	w.Indent()

	// Generate fields
	for _, field := range typ.Fields {
		// Write field documentation
		if field.Doc != "" {
			w.WriteDocComment(field.Doc)
		}

		// Generate field
		goType := g.mapToGoType(field.Type, field.Required)
		jsonTag := g.generateJSONTag(field)
		w.WriteLinef("%s %s `json:\"%s\"`", g.exportedName(field.Name), goType, jsonTag)
	}

	w.Dedent()
	w.WriteLine("}")
}

// generateServiceInterface generates Go interface for a service
func (g *Generator) generateServiceInterface(w *writer.Writer, svc schema.Service) {
	// Write documentation
	if svc.Doc != "" {
		w.WriteDocComment(svc.Doc)
	} else {
		w.WriteLinef("// %s defines the service interface", svc.Name)
	}

	// Generate interface
	w.WriteLinef("type %s interface {", svc.Name)
	w.Indent()

	// Generate methods
	for i, method := range svc.Methods {
		// Write method documentation
		if method.Doc != "" {
			w.WriteDocComment(method.Doc)
		}

		// Generate method signature (no context for WASM)
		// Always use pointers for input/output types for consistency with WASM wrapper
		inputType := "*" + g.mapToGoType(method.InputType, true)
		outputType := "*" + g.mapToGoType(method.OutputType, true)
		w.WriteLinef("%s(input %s) (%s, error)", g.exportedName(method.Name), inputType, outputType)

		if i < len(svc.Methods)-1 {
			w.BlankLine()
		}
	}

	w.Dedent()
	w.WriteLine("}")
}

// mapToGoType maps OKRA types to Go types
func (g *Generator) mapToGoType(typ string, required bool) string {
	// Handle optional types first
	if !required {
		// For array types, don't double-nest the pointer
		if strings.HasPrefix(typ, "[") && strings.HasSuffix(typ, "]") {
			inner := typ[1 : len(typ)-1]
			return "*[]" + g.mapToGoType(inner, true)
		}
		return "*" + g.mapToGoType(typ, true)
	}

	// Handle array types
	if strings.HasPrefix(typ, "[") && strings.HasSuffix(typ, "]") {
		inner := typ[1 : len(typ)-1]
		return "[]" + g.mapToGoType(inner, true)
	}

	// Map basic types
	switch typ {
	case "String":
		return "string"
	case "Int":
		return "int"
	case "Int32":
		return "int32"
	case "Int64":
		return "int64"
	case "Float":
		return "float32"
	case "Float64":
		return "float64"
	case "Boolean", "Bool":
		return "bool"
	case "Bytes":
		return "[]byte"
	case "Time":
		return "time.Time"
	case "Any":
		return "interface{}"
	default:
		// Assume it's a custom type
		return typ
	}
}

// generateJSONTag generates JSON struct tag
func (g *Generator) generateJSONTag(field schema.Field) string {
	tag := field.Name
	if !field.Required {
		tag += ",omitempty"
	}
	return tag
}

// exportedName converts a field name to an exported Go name
func (g *Generator) exportedName(name string) string {
	if name == "" {
		return ""
	}
	// Simple conversion - just capitalize first letter
	// In production, might want to handle snake_case, etc.
	return strings.ToUpper(name[:1]) + name[1:]
}

