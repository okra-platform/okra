package typescript

import (
	"strings"

	"github.com/okra-platform/okra/internal/codegen/writer"
	"github.com/okra-platform/okra/internal/schema"
)

// Generator generates TypeScript code from an OKRA schema
type Generator struct {
	moduleName      string
	generateClasses bool // If true, generate classes instead of interfaces
}

// NewGenerator creates a new TypeScript code generator
func NewGenerator(moduleName string) *Generator {
	return &Generator{
		moduleName:      moduleName,
		generateClasses: false,
	}
}

// WithClasses configures the generator to produce classes instead of interfaces
func (g *Generator) WithClasses(useClasses bool) *Generator {
	g.generateClasses = useClasses
	return g
}

// Language returns the name of the target language
func (g *Generator) Language() string {
	return "typescript"
}

// FileExtension returns the file extension for generated files
func (g *Generator) FileExtension() string {
	return ".ts"
}

// Generate generates TypeScript interfaces and types from the schema
func (g *Generator) Generate(s *schema.Schema) ([]byte, error) {
	w := writer.NewWriter("  ") // TypeScript typically uses 2 spaces

	// Write module declaration if specified
	if g.moduleName != "" {
		w.WriteLinef("export module %s {", g.moduleName)
		w.Indent()
	}

	// Generate enums
	for i, enum := range s.Enums {
		g.generateEnum(w, enum)
		if i < len(s.Enums)-1 || len(s.Types) > 0 || len(s.Services) > 0 {
			w.BlankLine()
		}
	}

	// Generate types
	for i, typ := range s.Types {
		g.generateType(w, typ)
		if i < len(s.Types)-1 || len(s.Services) > 0 {
			w.BlankLine()
		}
	}

	// Generate service interfaces
	for i, svc := range s.Services {
		g.generateServiceInterface(w, svc)
		if i < len(s.Services)-1 {
			w.BlankLine()
		}
	}

	// Close module if opened
	if g.moduleName != "" {
		w.Dedent()
		w.WriteLine("}")
	}

	return w.Bytes(), nil
}

// generateEnum generates TypeScript enum
func (g *Generator) generateEnum(w *writer.Writer, enum schema.EnumType) {
	// Write documentation
	if enum.Doc != "" {
		g.writeJSDoc(w, enum.Doc)
	}

	// Generate enum
	w.WriteLinef("export enum %s {", enum.Name)
	w.Indent()

	for i, value := range enum.Values {
		if value.Doc != "" {
			g.writeJSDoc(w, value.Doc)
		}
		w.WriteLinef("%s = \"%s\",", value.Name, value.Name)
		if i < len(enum.Values)-1 && value.Doc == "" && enum.Values[i+1].Doc != "" {
			w.BlankLine()
		}
	}

	w.Dedent()
	w.WriteLine("}")

	// Generate type guard
	w.BlankLine()
	w.WriteLinef("export function is%s(value: any): value is %s {", enum.Name, enum.Name)
	w.Indent()
	w.WriteLine("return Object.values(" + enum.Name + ").includes(value);")
	w.Dedent()
	w.WriteLine("}")
}

// generateType generates TypeScript interface or class for an object type
func (g *Generator) generateType(w *writer.Writer, typ schema.ObjectType) {
	// Write documentation
	if typ.Doc != "" {
		g.writeJSDoc(w, typ.Doc)
	}

	// Generate interface or class
	keyword := "interface"
	if g.generateClasses {
		keyword = "class"
	}

	w.WriteLinef("export %s %s {", keyword, typ.Name)
	w.Indent()

	// Generate fields
	for _, field := range typ.Fields {
		// Write field documentation
		if field.Doc != "" {
			g.writeJSDoc(w, field.Doc)
		}

		// Generate field
		tsType := g.mapToTSType(field.Type)
		optional := ""
		if !field.Required {
			optional = "?"
		}

		if g.generateClasses {
			w.WriteLinef("%s%s: %s;", field.Name, optional, tsType)
		} else {
			w.WriteLinef("%s%s: %s;", field.Name, optional, tsType)
		}
	}

	w.Dedent()
	w.WriteLine("}")
}

// generateServiceInterface generates TypeScript interface for a service
func (g *Generator) generateServiceInterface(w *writer.Writer, svc schema.Service) {
	// Write documentation
	if svc.Doc != "" {
		g.writeJSDoc(w, svc.Doc)
	}

	// Generate interface
	w.WriteLinef("export interface %s {", svc.Name)
	w.Indent()

	// Generate methods
	for i, method := range svc.Methods {
		// Write method documentation
		if method.Doc != "" {
			g.writeJSDoc(w, method.Doc)
		}

		// Generate method signature
		methodName := g.toCamelCase(method.Name)
		inputType := g.mapToTSType(method.InputType)
		outputType := g.mapToTSType(method.OutputType)

		w.WriteLinef("%s(input: %s): Promise<%s>;", methodName, inputType, outputType)

		if i < len(svc.Methods)-1 {
			w.BlankLine()
		}
	}

	w.Dedent()
	w.WriteLine("}")

	// Generate client implementation stub
	w.BlankLine()
	w.WriteLinef("export abstract class %sClient implements %s {", svc.Name, svc.Name)
	w.Indent()

	for _, method := range svc.Methods {
		methodName := g.toCamelCase(method.Name)
		inputType := g.mapToTSType(method.InputType)
		outputType := g.mapToTSType(method.OutputType)

		w.WriteLinef("abstract %s(input: %s): Promise<%s>;", methodName, inputType, outputType)
	}

	w.Dedent()
	w.WriteLine("}")
}

// mapToTSType maps OKRA types to TypeScript types
func (g *Generator) mapToTSType(typ string) string {
	// Handle array types
	if strings.HasPrefix(typ, "[") && strings.HasSuffix(typ, "]") {
		inner := typ[1 : len(typ)-1]
		return g.mapToTSType(inner) + "[]"
	}

	// Map basic types
	switch typ {
	case "String":
		return "string"
	case "Int", "Int32", "Int64", "Float", "Float64":
		return "number"
	case "Boolean", "Bool":
		return "boolean"
	case "Bytes":
		return "Uint8Array"
	case "Time":
		return "Date"
	case "Any":
		return "any"
	default:
		// Assume it's a custom type
		return typ
	}
}

// writeJSDoc writes JSDoc style comments
func (g *Generator) writeJSDoc(w *writer.Writer, doc string) {
	if doc == "" {
		return
	}

	lines := strings.Split(strings.TrimSpace(doc), "\n")
	if len(lines) == 1 {
		w.WriteLinef("/** %s */", lines[0])
	} else {
		w.WriteLine("/**")
		for _, line := range lines {
			w.WriteLinef(" * %s", strings.TrimSpace(line))
		}
		w.WriteLine(" */")
	}
}

// toCamelCase converts a name to camelCase
func (g *Generator) toCamelCase(name string) string {
	if name == "" {
		return ""
	}
	// Simple conversion - just lowercase first letter
	return strings.ToLower(name[:1]) + name[1:]
}
