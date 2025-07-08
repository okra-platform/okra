package schema

// Schema is the root of a parsed .okra.gql file
type Schema struct {
	Types    []ObjectType `json:"types"`
	Enums    []EnumType   `json:"enums"`
	Services []Service    `json:"services"`
	Meta     Metadata     `json:"meta"`
}

// Metadata represents global metadata for the IDL file
type Metadata struct {
	Namespace string `json:"namespace"`
	Version   string `json:"version"`
	Service   string `json:"service"`
}

// ObjectType represents a top-level "type" block
type ObjectType struct {
	Name   string  `json:"name"`
	Doc    string  `json:"doc"`
	Fields []Field `json:"fields"`
}

// Field represents a field inside a type or input object
type Field struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Required   bool        `json:"required"`
	Directives []Directive `json:"directives"`
	Doc        string      `json:"doc"`
}

// EnumType represents an enum definition
type EnumType struct {
	Name   string      `json:"name"`
	Doc    string      `json:"doc"`
	Values []EnumValue `json:"values"`
}

// EnumValue represents a single value inside an enum
type EnumValue struct {
	Name string `json:"name"`
	Doc  string `json:"doc"`
}

// Service represents a "service" block (transformed from type Service_*)
type Service struct {
	Name      string   `json:"name"`
	Doc       string   `json:"doc"`
	Namespace string   `json:"namespace"`
	Version   string   `json:"version"`
	Methods   []Method `json:"methods"`
}

// Method represents a single service method
type Method struct {
	Name       string      `json:"name"`
	InputType  string      `json:"inputType"`
	OutputType string      `json:"outputType"`
	Directives []Directive `json:"directives"`
	Doc        string      `json:"doc"`
}

// Directive represents an attached directive (e.g. @auth, @validate)
type Directive struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args"`
	Doc  string            `json:"doc"`
}
