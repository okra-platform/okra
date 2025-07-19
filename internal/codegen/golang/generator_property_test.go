package golang

import (
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/okra-platform/okra/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test plan for property-based testing:
// 1. Generated Go code should always be syntactically valid
// 2. All schema types should be represented in generated code
// 3. Field types should map correctly to Go types
// 4. Required fields should use appropriate Go types (no pointers)
// 5. Method signatures should match schema definitions
// 6. Generated code should compile (syntax check)
// 7. Special characters in names should be handled correctly
// 8. Large schemas should generate without issues

func TestGenerator_PropertyBasedValidGo(t *testing.T) {
	// Test: All generated code should be valid Go syntax
	
	for i := 0; i < 50; i++ {
		t.Run(fmt.Sprintf("random_schema_%d", i), func(t *testing.T) {
			s := generateRandomSchema()
			gen := NewGenerator("testpkg")
			
			code, err := gen.Generate(s)
			require.NoError(t, err)
			
			// Verify it's valid Go code
			formatted, err := format.Source(code)
			if err != nil {
				t.Logf("Generated code:\n%s", string(code))
				t.Fatalf("Generated code is not valid Go: %v", err)
			}
			
			// Parse the code to ensure it's syntactically correct
			fset := token.NewFileSet()
			_, err = parser.ParseFile(fset, "generated.go", formatted, parser.AllErrors)
			assert.NoError(t, err, "Generated code should parse successfully")
		})
	}
}

func TestGenerator_PropertyBasedTypeMapping(t *testing.T) {
	// Test: Schema types should map correctly to Go types
	testCases := []struct {
		schemaType string
		required   bool
		expected   string
	}{
		// Scalar types
		{"String", true, "string"},
		{"String", false, "*string"},
		{"Int", true, "int"},
		{"Int", false, "*int"},
		{"Float", true, "float32"},
		{"Float", false, "*float32"},
		{"Boolean", true, "bool"},
		{"Boolean", false, "*bool"},
		{"ID", true, "ID"},
		{"ID", false, "*ID"},
		
		// List types
		{"[String]", true, "[]string"},
		{"[String]", false, "*[]string"}, // Lists get pointer wrapper
		{"[Int]", true, "[]int"},
		{"[Boolean]", true, "[]bool"},
		
		// Custom types
		{"CustomType", true, "CustomType"},
		{"CustomType", false, "*CustomType"},
		{"[CustomType]", true, "[]CustomType"},
		{"[CustomType]", false, "*[]CustomType"},
	}
	
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_required_%v", tc.schemaType, tc.required), func(t *testing.T) {
			s := &schema.Schema{
				Types: []schema.ObjectType{
					{
						Name: "TestType",
						Fields: []schema.Field{
							{
								Name:     "testField",
								Type:     tc.schemaType,
								Required: tc.required,
							},
						},
					},
				},
			}
			
			gen := NewGenerator("test")
			code, err := gen.Generate(s)
			require.NoError(t, err)
			
			codeStr := string(code)
			assert.Contains(t, codeStr, fmt.Sprintf("TestField %s", tc.expected),
				"Field should have correct Go type")
		})
	}
}

func TestGenerator_PropertyBasedFieldNames(t *testing.T) {
	// Test: Various field naming patterns should be handled correctly
	fieldNames := []struct {
		input    string
		expected string
	}{
		{"simple", "Simple"},
		{"twoWords", "TwoWords"},
		{"three_word_name", "Three_word_name"}, // Simple capitalization
		{"withACRONYM", "WithACRONYM"},
		{"id", "Id"}, // Simple capitalization, not special cased
		{"userId", "UserId"}, // Simple capitalization
		{"apiKey", "ApiKey"}, // Simple capitalization
		{"htmlContent", "HtmlContent"},
		{"xmlData", "XmlData"},
		{"jsonValue", "JsonValue"},
		{"httpRequest", "HttpRequest"},
		{"urlPath", "UrlPath"},
	}
	
	for _, fn := range fieldNames {
		t.Run(fn.input, func(t *testing.T) {
			s := &schema.Schema{
				Types: []schema.ObjectType{
					{
						Name: "TestType",
						Fields: []schema.Field{
							{
								Name:     fn.input,
								Type:     "String",
								Required: true,
							},
						},
					},
				},
			}
			
			gen := NewGenerator("test")
			code, err := gen.Generate(s)
			require.NoError(t, err)
			
			codeStr := string(code)
			assert.Contains(t, codeStr, fmt.Sprintf("%s string", fn.expected),
				"Field name should be properly capitalized")
		})
	}
}

func TestGenerator_PropertyBasedServices(t *testing.T) {
	// Test: Service methods should generate correct interfaces
	methodPatterns := []struct {
		name       string
		inputType  string
		outputType string
	}{
		{"simpleMethod", "Input", "Output"},
		{"getUser", "GetUserRequest", "User"},
		{"listItems", "ListItemsRequest", "ListItemsResponse"},
		{"createOrder", "Order", "OrderResponse"},
		{"updateStatus", "UpdateStatusInput", "Status"},
	}
	
	for _, pattern := range methodPatterns {
		t.Run(pattern.name, func(t *testing.T) {
			s := &schema.Schema{
				Services: []schema.Service{
					{
						Name: "TestService",
						Methods: []schema.Method{
							{
								Name:       pattern.name,
								InputType:  pattern.inputType,
								OutputType: pattern.outputType,
							},
						},
					},
				},
			}
			
			gen := NewGenerator("test")
			code, err := gen.Generate(s)
			require.NoError(t, err)
			
			codeStr := string(code)
			
			// Should generate interface
			assert.Contains(t, codeStr, "type TestService interface {")
			
			// Should have correct method signature
			expectedMethod := fmt.Sprintf("%s(input *%s) (*%s, error)",
				capitalize(pattern.name), pattern.inputType, pattern.outputType)
			assert.Contains(t, codeStr, expectedMethod)
		})
	}
}

func TestGenerator_PropertyBasedEnums(t *testing.T) {
	// Test: Enums should generate correct Go code
	enumTests := []struct {
		name   string
		values []string
	}{
		{"Status", []string{"ACTIVE", "INACTIVE", "PENDING"}},
		{"Priority", []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}},
		{"Color", []string{"RED", "GREEN", "BLUE"}},
	}
	
	for _, et := range enumTests {
		t.Run(et.name, func(t *testing.T) {
			s := &schema.Schema{
				Enums: []schema.EnumType{
					{
						Name: et.name,
						Values: func() []schema.EnumValue {
							values := make([]schema.EnumValue, len(et.values))
							for i, v := range et.values {
								values[i] = schema.EnumValue{Name: v}
							}
							return values
						}(),
					},
				},
			}
			
			gen := NewGenerator("test")
			code, err := gen.Generate(s)
			require.NoError(t, err)
			
			codeStr := string(code)
			
			// Should generate type
			assert.Contains(t, codeStr, fmt.Sprintf("type %s string", et.name))
			
			// Should generate constants
			assert.Contains(t, codeStr, "const (")
			for _, value := range et.values {
				expectedConst := fmt.Sprintf("%s%s %s = \"%s\"",
					et.name, value, et.name, value)
				assert.Contains(t, codeStr, expectedConst)
			}
			
			// Should generate Valid() method, not String()
			assert.Contains(t, codeStr, fmt.Sprintf("func (e %s) Valid() bool", et.name))
		})
	}
}

func TestGenerator_PropertyBasedComplexSchemas(t *testing.T) {
	// Test: Complex schemas with interdependencies
	s := &schema.Schema{
		Types: []schema.ObjectType{
			{
				Name: "User",
				Fields: []schema.Field{
					{Name: "id", Type: "ID", Required: true},
					{Name: "name", Type: "String", Required: true},
					{Name: "email", Type: "String", Required: false},
					{Name: "status", Type: "UserStatus", Required: true},
					{Name: "profile", Type: "Profile", Required: false},
					{Name: "tags", Type: "[String]", Required: true},
				},
			},
			{
				Name: "Profile",
				Fields: []schema.Field{
					{Name: "bio", Type: "String", Required: false},
					{Name: "avatar", Type: "String", Required: false},
					{Name: "settings", Type: "Settings", Required: true},
				},
			},
			{
				Name: "Settings",
				Fields: []schema.Field{
					{Name: "notifications", Type: "Boolean", Required: true},
					{Name: "privacy", Type: "PrivacyLevel", Required: true},
				},
			},
		},
		Enums: []schema.EnumType{
			{
				Name: "UserStatus",
				Values: []schema.EnumValue{
					{Name: "ACTIVE"},
					{Name: "INACTIVE"},
					{Name: "SUSPENDED"},
				},
			},
			{
				Name: "PrivacyLevel",
				Values: []schema.EnumValue{
					{Name: "PUBLIC"},
					{Name: "PRIVATE"},
					{Name: "FRIENDS_ONLY"},
				},
			},
		},
		Services: []schema.Service{
			{
				Name: "UserService",
				Methods: []schema.Method{
					{Name: "getUser", InputType: "GetUserRequest", OutputType: "User"},
					{Name: "updateUser", InputType: "User", OutputType: "User"},
					{Name: "listUsers", InputType: "ListUsersRequest", OutputType: "ListUsersResponse"},
				},
			},
		},
	}
	
	gen := NewGenerator("test")
	code, err := gen.Generate(s)
	require.NoError(t, err)
	
	// Should be valid Go
	_, err = format.Source(code)
	require.NoError(t, err, "Complex schema should generate valid Go code")
	
	codeStr := string(code)
	
	// Verify all types are generated
	assert.Contains(t, codeStr, "type User struct")
	assert.Contains(t, codeStr, "type Profile struct")
	assert.Contains(t, codeStr, "type Settings struct")
	
	// Verify enums
	assert.Contains(t, codeStr, "type UserStatus string")
	assert.Contains(t, codeStr, "type PrivacyLevel string")
	
	// Verify service interface
	assert.Contains(t, codeStr, "type UserService interface")
}

func TestGenerator_PropertyBasedLargeSchemas(t *testing.T) {
	// Test: Large schemas should generate without performance issues
	if testing.Short() {
		t.Skip("Skipping large schema test in short mode")
	}
	
	sizes := []int{10, 50, 100}
	
	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			s := generateLargeSchema(size)
			gen := NewGenerator("test")
			
			start := time.Now()
			code, err := gen.Generate(s)
			duration := time.Since(start)
			
			require.NoError(t, err)
			assert.NotEmpty(t, code)
			
			// Should complete quickly
			assert.Less(t, duration, 5*time.Second,
				"Generation should complete quickly even for large schemas")
			
			// Should be valid Go
			_, err = format.Source(code)
			assert.NoError(t, err, "Large schema should generate valid Go code")
		})
	}
}

func TestGenerator_PropertyBasedSpecialCases(t *testing.T) {
	// Test: Special cases and edge conditions
	testCases := []struct {
		name        string
		schema      *schema.Schema
		shouldError bool
		contains    []string
	}{
		{
			name: "empty_schema",
			schema: &schema.Schema{
				Types:    []schema.ObjectType{},
				Enums:    []schema.EnumType{},
				Services: []schema.Service{},
			},
			shouldError: false,
			contains:    []string{"package test"},
		},
		{
			name: "type_with_no_fields",
			schema: &schema.Schema{
				Types: []schema.ObjectType{
					{Name: "Empty", Fields: []schema.Field{}},
				},
			},
			shouldError: false,
			contains:    []string{"type Empty struct {", "}"},
		},
		{
			name: "enum_with_one_value",
			schema: &schema.Schema{
				Enums: []schema.EnumType{
					{
						Name:   "Single",
						Values: []schema.EnumValue{{Name: "ONLY"}},
					},
				},
			},
			shouldError: false,
			contains:    []string{"type Single string", "SingleONLY Single = \"ONLY\""},
		},
		{
			name: "deeply_nested_types",
			schema: &schema.Schema{
				Types: []schema.ObjectType{
					{
						Name: "Level1",
						Fields: []schema.Field{
							{Name: "level2", Type: "[Level2]", Required: true},
						},
					},
					{
						Name: "Level2",
						Fields: []schema.Field{
							{Name: "level3", Type: "Level3", Required: false},
						},
					},
					{
						Name: "Level3",
						Fields: []schema.Field{
							{Name: "value", Type: "String", Required: true},
						},
					},
				},
			},
			shouldError: false,
			contains:    []string{"Level2 []Level2", "Level3 *Level3"},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gen := NewGenerator("test")
			code, err := gen.Generate(tc.schema)
			
			if tc.shouldError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				codeStr := string(code)
				
				for _, expected := range tc.contains {
					assert.Contains(t, codeStr, expected)
				}
				
				// Should be valid Go
				_, err = format.Source(code)
				assert.NoError(t, err)
			}
		})
	}
}

// Helper functions

func generateRandomSchema() *schema.Schema {
	s := &schema.Schema{
		Types:    []schema.ObjectType{},
		Enums:    []schema.EnumType{},
		Services: []schema.Service{},
	}
	
	// Generate 1-5 types
	numTypes := rand.Intn(5) + 1
	for i := 0; i < numTypes; i++ {
		objType := schema.ObjectType{
			Name:   fmt.Sprintf("Type%d", i),
			Fields: []schema.Field{},
		}
		
		// Generate 1-5 fields
		numFields := rand.Intn(5) + 1
		for j := 0; j < numFields; j++ {
			field := schema.Field{
				Name:     fmt.Sprintf("field%d", j),
				Type:     randomType(),
				Required: rand.Float32() < 0.5,
			}
			objType.Fields = append(objType.Fields, field)
		}
		
		s.Types = append(s.Types, objType)
	}
	
	// Generate 0-2 enums
	numEnums := rand.Intn(3)
	for i := 0; i < numEnums; i++ {
		enum := schema.EnumType{
			Name:   fmt.Sprintf("Enum%d", i),
			Values: []schema.EnumValue{},
		}
		
		// Generate 2-5 values
		numValues := rand.Intn(4) + 2
		for j := 0; j < numValues; j++ {
			enum.Values = append(enum.Values, schema.EnumValue{
				Name: fmt.Sprintf("VALUE_%d", j),
			})
		}
		
		s.Enums = append(s.Enums, enum)
	}
	
	// Generate 0-1 service
	if rand.Float32() < 0.5 {
		service := schema.Service{
			Name:    "TestService",
			Methods: []schema.Method{},
		}
		
		// Generate 1-3 methods
		numMethods := rand.Intn(3) + 1
		for i := 0; i < numMethods; i++ {
			method := schema.Method{
				Name:       fmt.Sprintf("method%d", i),
				InputType:  fmt.Sprintf("Input%d", i),
				OutputType: fmt.Sprintf("Output%d", i),
			}
			service.Methods = append(service.Methods, method)
		}
		
		s.Services = append(s.Services, service)
	}
	
	return s
}

func randomType() string {
	types := []string{
		"String", "Int", "Float", "Boolean", "ID",
		"[String]", "[Int]", "[Boolean]",
		"CustomType", "[CustomType]",
	}
	return types[rand.Intn(len(types))]
}

func generateLargeSchema(numTypes int) *schema.Schema {
	s := &schema.Schema{
		Types:    make([]schema.ObjectType, numTypes),
		Enums:    []schema.EnumType{},
		Services: []schema.Service{},
	}
	
	// Generate types
	for i := 0; i < numTypes; i++ {
		s.Types[i] = schema.ObjectType{
			Name:   fmt.Sprintf("Type%d", i),
			Fields: make([]schema.Field, 10), // Each type has 10 fields
		}
		
		for j := 0; j < 10; j++ {
			fieldType := "String"
			if j < i && rand.Float32() < 0.3 {
				// Reference another type
				fieldType = fmt.Sprintf("Type%d", rand.Intn(i))
			}
			
			s.Types[i].Fields[j] = schema.Field{
				Name:     fmt.Sprintf("field%d", j),
				Type:     fieldType,
				Required: rand.Float32() < 0.5,
			}
		}
	}
	
	return s
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func titleCase(s string) string {
	words := strings.Split(strings.ToLower(s), "_")
	for i, word := range words {
		if word != "" {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, "")
}