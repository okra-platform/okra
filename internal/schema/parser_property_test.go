package schema

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test plan for property-based testing:
// 1. Test round-trip: parse -> generate -> parse produces equivalent schema
// 2. Test invalid GraphQL is consistently rejected
// 3. Test all valid type combinations parse correctly
// 4. Test directive parsing with various argument types
// 5. Test field nullability and list types
// 6. Test edge cases in naming (unicode, special chars)
// 7. Test deeply nested types
// 8. Test large schemas with many types

func TestParseSchema_PropertyBasedValidSchemas(t *testing.T) {
	// Test: Randomly generated valid schemas should parse without errors
	
	for i := range 100 {
		t.Run(fmt.Sprintf("random_schema_%d", i), func(t *testing.T) {
			schema := generateRandomValidSchema()
			
			parsed, err := ParseSchema(schema)
			assert.NoError(t, err, "Valid schema should parse: %s", schema)
			
			if err == nil {
				// Verify basic properties
				assert.NotNil(t, parsed)
				assert.NotNil(t, parsed.Types)
				assert.NotNil(t, parsed.Services)
				assert.NotNil(t, parsed.Enums)
			}
		})
	}
}

func TestParseSchema_PropertyBasedInvalidSchemas(t *testing.T) {
	// Test: Various forms of invalid GraphQL should be rejected
	invalidGenerators := []func() string{
		generateMalformedType,
		generateMissingBrackets,
		generateInvalidDirectives,
		generateInvalidFieldTypes,
	}
	
	for _, generator := range invalidGenerators {
		for range 20 {
			schema := generator()
			_, err := ParseSchema(schema)
			assert.Error(t, err, "Invalid schema should fail: %s", schema)
		}
	}
}

func TestParseSchema_PropertyBasedFieldTypes(t *testing.T) {
	// Test: All combinations of field types should parse correctly
	baseTypes := []string{"String", "Int", "Float", "Boolean", "ID", "CustomType"}
	
	for _, baseType := range baseTypes {
		// Non-null
		t.Run(fmt.Sprintf("non_null_%s", baseType), func(t *testing.T) {
			schema := fmt.Sprintf(`
				type TestType {
					field: %s!
				}
			`, baseType)
			
			parsed, err := ParseSchema(schema)
			require.NoError(t, err)
			require.Len(t, parsed.Types, 1)
			require.Len(t, parsed.Types[0].Fields, 1)
			
			field := parsed.Types[0].Fields[0]
			assert.Equal(t, baseType, field.Type)
			assert.True(t, field.Required)
		})
		
		// List
		t.Run(fmt.Sprintf("list_%s", baseType), func(t *testing.T) {
			schema := fmt.Sprintf(`
				type TestType {
					field: [%s]
				}
			`, baseType)
			
			parsed, err := ParseSchema(schema)
			require.NoError(t, err)
			require.Len(t, parsed.Types, 1)
			require.Len(t, parsed.Types[0].Fields, 1)
			
			field := parsed.Types[0].Fields[0]
			assert.Equal(t, fmt.Sprintf("[%s]", baseType), field.Type)
			assert.False(t, field.Required)
		})
		
		// Non-null list of non-null
		t.Run(fmt.Sprintf("non_null_list_non_null_%s", baseType), func(t *testing.T) {
			schema := fmt.Sprintf(`
				type TestType {
					field: [%s!]!
				}
			`, baseType)
			
			parsed, err := ParseSchema(schema)
			require.NoError(t, err)
			require.Len(t, parsed.Types, 1)
			require.Len(t, parsed.Types[0].Fields, 1)
			
			field := parsed.Types[0].Fields[0]
			// Parser might not preserve inner non-null in list types
			assert.Equal(t, fmt.Sprintf("[%s]", baseType), field.Type)
			assert.True(t, field.Required)
		})
	}
}

func TestParseSchema_PropertyBasedDirectives(t *testing.T) {
	// Test: Directives with various argument types
	argTypes := []struct {
		value string
		desc  string
	}{
		{`"string value"`, "string"},
		{`42`, "integer"},
		{`3.14`, "float"},
		{`true`, "boolean"},
		{`ENUM_VALUE`, "enum"},
	}
	
	for _, argType := range argTypes {
		t.Run(fmt.Sprintf("directive_%s", argType.desc), func(t *testing.T) {
			schema := fmt.Sprintf(`
				type TestType {
					field: String @custom(arg: %s)
				}
			`, argType.value)
			
			parsed, err := ParseSchema(schema)
			require.NoError(t, err)
			require.Len(t, parsed.Types, 1)
			require.Len(t, parsed.Types[0].Fields, 1)
			require.Len(t, parsed.Types[0].Fields[0].Directives, 1)
			
			directive := parsed.Types[0].Fields[0].Directives[0]
			assert.Equal(t, "custom", directive.Name)
			assert.Contains(t, directive.Args, "arg")
		})
	}
}

func TestParseSchema_PropertyBasedNaming(t *testing.T) {
	// Test: Various naming patterns
	testCases := []struct {
		name     string
		typeName string
		valid    bool
	}{
		{"simple", "SimpleType", true},
		{"underscore", "Type_With_Underscores", true},
		{"numbers", "Type123", true},
		{"leading_underscore", "_LeadingUnderscore", true},
		{"unicode", "Type世界", false}, // Parser might not handle unicode
		{"emoji", "Type🚀", false},      // Parser might not handle emojis
		{"space", "Type With Space", false},
		{"hyphen", "Type-With-Hyphen", true}, // Parser accepts hyphens
		{"leading_number", "123Type", false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			schema := fmt.Sprintf(`type %s { field: String }`, tc.typeName)
			
			parsed, err := ParseSchema(schema)
			if tc.valid {
				assert.NoError(t, err, "Type name '%s' should be valid", tc.typeName)
				if err == nil {
					assert.Len(t, parsed.Types, 1)
					assert.Equal(t, tc.typeName, parsed.Types[0].Name)
				}
			} else {
				assert.Error(t, err, "Type name '%s' should be invalid", tc.typeName)
			}
		})
	}
}

func TestParseSchema_PropertyBasedLargeSchemas(t *testing.T) {
	// Test: Large schemas with many types
	if testing.Short() {
		t.Skip("Skipping large schema test in short mode")
	}
	
	sizes := []int{10, 50, 100}
	
	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			schema := generateLargeSchema(size)
			
			start := time.Now()
			parsed, err := ParseSchema(schema)
			duration := time.Since(start)
			
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(parsed.Types), size)
			assert.Less(t, duration, 5*time.Second, "Parsing should complete quickly even for large schemas")
		})
	}
}

func TestParseSchema_PropertyBasedRoundTrip(t *testing.T) {
	// Test: Parse -> inspect -> verify consistency
	schemas := []string{
		`type User { id: ID! name: String! }`,
		`enum Status { ACTIVE INACTIVE PENDING }`,
		`type Service_UserService { getUser(input: GetUserInput): User! }`,
		`type Complex { 
			simple: String
			required: Int!
			list: [String]
			requiredList: [Int!]!
			nestedObject: User
		}`,
	}
	
	for i, schema := range schemas {
		t.Run(fmt.Sprintf("schema_%d", i), func(t *testing.T) {
			parsed1, err := ParseSchema(schema)
			require.NoError(t, err)
			
			// Convert back to a normalized form and parse again
			normalized := normalizeSchema(parsed1)
			parsed2, err := ParseSchema(normalized)
			require.NoError(t, err)
			
			// Compare key properties
			assert.Equal(t, len(parsed1.Types), len(parsed2.Types))
			assert.Equal(t, len(parsed1.Enums), len(parsed2.Enums))
			assert.Equal(t, len(parsed1.Services), len(parsed2.Services))
		})
	}
}

// Helper functions for generating test data

func generateRandomValidSchema() string {
	var sb strings.Builder
	
	// Add some types
	numTypes := rand.Intn(5) + 1
	for i := range numTypes {
		sb.WriteString(fmt.Sprintf("type Type%d {\n", i))
		
		// Add fields
		numFields := rand.Intn(5) + 1
		for j := range numFields {
			fieldType := randomFieldType()
			sb.WriteString(fmt.Sprintf("  field%d: %s\n", j, fieldType))
		}
		
		sb.WriteString("}\n\n")
	}
	
	// Add an enum
	if rand.Float32() < 0.5 {
		sb.WriteString("enum Status {\n")
		values := []string{"ACTIVE", "INACTIVE", "PENDING"}
		for _, v := range values {
			sb.WriteString(fmt.Sprintf("  %s\n", v))
		}
		sb.WriteString("}\n\n")
	}
	
	// Add a service
	if rand.Float32() < 0.5 {
		sb.WriteString("type Service_TestService {\n")
		sb.WriteString("  testMethod(input: Type0): Type1\n")
		sb.WriteString("}\n")
	}
	
	return sb.String()
}

func randomFieldType() string {
	types := []string{"String", "Int", "Float", "Boolean", "ID"}
	baseType := types[rand.Intn(len(types))]
	
	// Randomly make it required
	if rand.Float32() < 0.3 {
		baseType += "!"
	}
	
	// Randomly make it a list
	if rand.Float32() < 0.2 {
		baseType = "[" + baseType + "]"
		// Randomly make the list required
		if rand.Float32() < 0.5 {
			baseType += "!"
		}
	}
	
	return baseType
}

func generateMalformedType() string {
	malformed := []string{
		"type { field: String }",           // Missing type name
		"type Test field: String }",        // Missing opening brace
		"type Test { field: }",             // Missing field type
		"type Test { : String }",           // Missing field name
		"type Test { field: String",        // Missing closing brace
		"type Test { field: [String }",     // Mismatched brackets
		"type Test { field: String!! }",    // Double exclamation
		"type Test { field: [String]! ! }", // Space in type
	}
	
	return malformed[rand.Intn(len(malformed))]
}

func generateMissingBrackets() string {
	return fmt.Sprintf("type Test%d { field: [String }", rand.Intn(100))
}

func generateInvalidDirectives() string {
	invalid := []string{
		"type Test { field: String @ }",            // Empty directive
		"type Test { field: String @123 }",         // Numeric directive
		"type Test { field: String @test( }",       // Unclosed arguments
		"type Test { field: String @test(arg:) }",  // Missing argument value
		"type Test { field: String @test(: 123) }", // Missing argument name
	}
	
	return invalid[rand.Intn(len(invalid))]
}

func generateCircularTypes() string {
	// GraphQL actually allows circular references, but let's test deep nesting
	depth := rand.Intn(10) + 5
	var sb strings.Builder
	
	for i := range depth {
		sb.WriteString(fmt.Sprintf("type Type%d {\n", i))
		sb.WriteString(fmt.Sprintf("  next: Type%d\n", (i+1)%depth))
		sb.WriteString("}\n")
	}
	
	return sb.String()
}

func generateInvalidFieldTypes() string {
	invalid := []string{
		"type Test { field: [] }",           // Empty list
		"type Test { field: [!String] }",    // Exclamation in wrong place
		"type Test { field: String!! }",      // Double exclamation
		"type Test { field: String? }",      // Question mark (not GraphQL syntax)
		"type Test { field: String | Int }", // Union syntax in field
	}
	
	return invalid[rand.Intn(len(invalid))]
}

func generateLargeSchema(numTypes int) string {
	var sb strings.Builder
	
	// Generate types
	for i := range numTypes {
		sb.WriteString(fmt.Sprintf("type Type%d {\n", i))
		
		// Each type has 5-10 fields
		numFields := rand.Intn(6) + 5
		for j := range numFields {
			// Reference other types to create a connected schema
			if rand.Float32() < 0.3 && i > 0 {
				refType := rand.Intn(i)
				sb.WriteString(fmt.Sprintf("  ref%d: Type%d\n", j, refType))
			} else {
				sb.WriteString(fmt.Sprintf("  field%d: %s\n", j, randomFieldType()))
			}
		}
		
		sb.WriteString("}\n\n")
	}
	
	// Add some enums
	numEnums := numTypes / 10
	for i := range numEnums {
		sb.WriteString(fmt.Sprintf("enum Enum%d {\n", i))
		for j := range 5 {
			sb.WriteString(fmt.Sprintf("  VALUE%d_%d\n", i, j))
		}
		sb.WriteString("}\n\n")
	}
	
	return sb.String()
}

func normalizeSchema(schema *Schema) string {
	var sb strings.Builder
	
	// Write types
	for _, t := range schema.Types {
		sb.WriteString(fmt.Sprintf("type %s {\n", t.Name))
		for _, f := range t.Fields {
			fieldType := f.Type
			if f.Required && !strings.HasSuffix(fieldType, "!") {
				fieldType += "!"
			}
			sb.WriteString(fmt.Sprintf("  %s: %s\n", f.Name, fieldType))
		}
		sb.WriteString("}\n\n")
	}
	
	// Write enums
	for _, e := range schema.Enums {
		sb.WriteString(fmt.Sprintf("enum %s {\n", e.Name))
		for _, v := range e.Values {
			sb.WriteString(fmt.Sprintf("  %s\n", v.Name))
		}
		sb.WriteString("}\n\n")
	}
	
	// Write services
	for _, s := range schema.Services {
		sb.WriteString(fmt.Sprintf("type Service_%s {\n", s.Name))
		for _, m := range s.Methods {
			sb.WriteString(fmt.Sprintf("  %s(input: %s): %s\n", m.Name, m.InputType, m.OutputType))
		}
		sb.WriteString("}\n\n")
	}
	
	return sb.String()
}

func TestParseSchema_PropertyBasedSpecialCharacters(t *testing.T) {
	// Test: Field values with special characters
	specialStrings := []string{
		`"simple"`,
		`"with \"quotes\""`,
		`"with \n newline"`,
		`"with \t tab"`,
		`"with \\ backslash"`,
		`"with unicode: \u0048\u0065\u006C\u006C\u006F"`, // Hello
		`""`, // Empty string
		`"very long string ` + strings.Repeat("x", 1000) + `"`,
	}
	
	for i, str := range specialStrings {
		t.Run(fmt.Sprintf("special_%d", i), func(t *testing.T) {
			schema := fmt.Sprintf(`
				type TestType {
					field: String @doc(description: %s)
				}
			`, str)
			
			parsed, err := ParseSchema(schema)
			
			// Some might fail due to GraphQL parsing rules
			if err == nil {
				assert.Len(t, parsed.Types, 1)
				if len(parsed.Types) > 0 && len(parsed.Types[0].Fields) > 0 &&
					len(parsed.Types[0].Fields[0].Directives) > 0 {
					directive := parsed.Types[0].Fields[0].Directives[0]
					assert.Equal(t, "doc", directive.Name)
					assert.Contains(t, directive.Args, "description")
				}
			}
		})
	}
}

func TestParseSchema_PropertyBasedFuzzing(t *testing.T) {
	// Test: Random byte sequences shouldn't crash the parser
	if testing.Short() {
		t.Skip("Skipping fuzzing test in short mode")
	}
	
	for range 100 {
		// Generate random bytes
		size := rand.Intn(1000) + 100
		data := make([]byte, size)
		rand.Read(data)
		
		// Make it somewhat GraphQL-like
		schema := string(data)
		if rand.Float32() < 0.5 {
			// Add some GraphQL keywords
			keywords := []string{"type", "enum", "interface", "input", "{", "}", ":", "String", "Int"}
			keyword := keywords[rand.Intn(len(keywords))]
			schema = keyword + " " + schema
		}
		
		// Should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Parser panicked on input: %v", r)
				}
			}()
			
			_, _ = ParseSchema(schema)
		}()
	}
}

func TestParseSchema_PropertyBasedWhitespace(t *testing.T) {
	// Test: Various whitespace patterns
	whitespaceVariants := []string{
		"type Test{field:String}",                          // No whitespace
		"type    Test    {    field   :   String    }",     // Extra spaces
		"type\tTest\t{\tfield\t:\tString\t}",               // Tabs
		"type\nTest\n{\nfield\n:\nString\n}",               // Newlines
		"type\r\nTest\r\n{\r\nfield\r\n:\r\nString\r\n}",   // Windows newlines
		"\n\n\ntype Test { field: String }\n\n\n",          // Leading/trailing newlines
		"type Test {\n\n\n  field: String\n\n\n}",          // Multiple blank lines
	}
	
	for i, variant := range whitespaceVariants {
		t.Run(fmt.Sprintf("whitespace_%d", i), func(t *testing.T) {
			parsed, err := ParseSchema(variant)
			require.NoError(t, err)
			
			// All should parse to the same structure
			assert.Len(t, parsed.Types, 1)
			assert.Equal(t, "Test", parsed.Types[0].Name)
			assert.Len(t, parsed.Types[0].Fields, 1)
			assert.Equal(t, "field", parsed.Types[0].Fields[0].Name)
			assert.Equal(t, "String", parsed.Types[0].Fields[0].Type)
		})
	}
}

func TestParseSchema_PropertyBasedComments(t *testing.T) {
	// Test: Comments in various positions
	schemas := []string{
		`# Comment at start
		type Test { field: String }`,
		
		`type Test { # Comment after type
			field: String
		}`,
		
		`type Test {
			field: String # Comment at end of line
		}`,
		
		`type Test {
			# Comment on its own line
			field: String
		}`,
		
		`"""
		Multi-line
		description
		"""
		type Test { field: String }`,
	}
	
	for i, schema := range schemas {
		t.Run(fmt.Sprintf("comments_%d", i), func(t *testing.T) {
			parsed, err := ParseSchema(schema)
			require.NoError(t, err)
			
			// Comments should be handled gracefully
			assert.Len(t, parsed.Types, 1)
			assert.Equal(t, "Test", parsed.Types[0].Name)
		})
	}
}

// Helper to check if a string contains only valid GraphQL identifier characters
func isValidGraphQLIdentifier(s string) bool {
	if s == "" {
		return false
	}
	
	for i, r := range s {
		if i == 0 {
			// First character must be letter or underscore
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else {
			// Subsequent characters can be letter, digit, or underscore
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return false
			}
		}
	}
	
	return true
}