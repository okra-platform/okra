package codegen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter_BasicWriting(t *testing.T) {
	// Test: Basic write operations
	w := NewWriter("\t")
	
	w.Write("hello")
	w.Write(" world")
	
	assert.Equal(t, "hello world", w.String())
}

func TestWriter_WriteLine(t *testing.T) {
	// Test: WriteLine adds newline
	w := NewWriter("\t")
	
	w.WriteLine("line1")
	w.WriteLine("line2")
	
	expected := "line1\nline2\n"
	assert.Equal(t, expected, w.String())
}

func TestWriter_Indentation(t *testing.T) {
	// Test: Proper indentation handling
	w := NewWriter("\t")
	
	w.WriteLine("func main() {")
	w.Indent()
	w.WriteLine("fmt.Println(\"hello\")")
	w.WriteLine("return")
	w.Dedent()
	w.WriteLine("}")
	
	expected := "func main() {\n\tfmt.Println(\"hello\")\n\treturn\n}\n"
	assert.Equal(t, expected, w.String())
}

func TestWriter_NestedIndentation(t *testing.T) {
	// Test: Multiple levels of indentation
	w := NewWriter("  ")
	
	w.WriteLine("if true {")
	w.Indent()
	w.WriteLine("if false {")
	w.Indent()
	w.WriteLine("return")
	w.Dedent()
	w.WriteLine("}")
	w.Dedent()
	w.WriteLine("}")
	
	expected := "if true {\n  if false {\n    return\n  }\n}\n"
	assert.Equal(t, expected, w.String())
}

func TestWriter_BlankLine(t *testing.T) {
	// Test: BlankLine prevents multiple blank lines
	w := NewWriter("\t")
	
	w.WriteLine("line1")
	w.BlankLine()
	w.WriteLine("line2")
	w.BlankLine()
	w.BlankLine() // Should not add another blank line
	w.WriteLine("line3")
	
	lines := strings.Split(w.String(), "\n")
	require.Len(t, lines, 6) // line1, blank, line2, blank, line3, empty
	assert.Equal(t, "line1", lines[0])
	assert.Equal(t, "", lines[1])
	assert.Equal(t, "line2", lines[2])
	assert.Equal(t, "", lines[3])
	assert.Equal(t, "line3", lines[4])
}

func TestWriter_WriteBlock(t *testing.T) {
	// Test: WriteBlock helper function
	w := NewWriter("\t")
	
	w.WriteBlock("func test() {", "}", func() {
		w.WriteLine("return nil")
	})
	
	expected := "func test() {\n\treturn nil\n}\n"
	assert.Equal(t, expected, w.String())
}

func TestWriter_Comments(t *testing.T) {
	// Test: Comment writing functions
	w := NewWriter("\t")
	
	w.WriteComment("Single line comment")
	w.WriteMultilineComment([]string{"Line 1", "Line 2", "Line 3"})
	
	expected := "// Single line comment\n// Line 1\n// Line 2\n// Line 3\n"
	assert.Equal(t, expected, w.String())
}

func TestWriter_DocComment(t *testing.T) {
	// Test: Documentation comment with multi-line string
	w := NewWriter("\t")
	
	doc := `This is a documentation comment
that spans multiple lines
and should be formatted properly`
	
	w.WriteDocComment(doc)
	
	expected := "// This is a documentation comment\n// that spans multiple lines\n// and should be formatted properly\n"
	assert.Equal(t, expected, w.String())
}

func TestWriter_DocCommentEmpty(t *testing.T) {
	// Test: Empty doc comment produces no output
	w := NewWriter("\t")
	
	w.WriteDocComment("")
	w.WriteLine("type Foo struct{}")
	
	assert.Equal(t, "type Foo struct{}\n", w.String())
}

func TestWriter_WriteFormatted(t *testing.T) {
	// Test: Formatted write operations
	w := NewWriter("\t")
	
	w.WriteLinef("var %s = %d", "count", 42)
	w.Indent()
	w.Writef("// %s: %v", "value", true)
	w.Newline()
	
	expected := "var count = 42\n\t// value: true\n"
	assert.Equal(t, expected, w.String())
}

func TestWriter_Reset(t *testing.T) {
	// Test: Reset clears writer state
	w := NewWriter("\t")
	
	w.WriteLine("some content")
	w.Indent()
	w.Indent()
	assert.Equal(t, 2, w.IndentLevel())
	
	w.Reset()
	
	assert.Equal(t, "", w.String())
	assert.Equal(t, 0, w.IndentLevel())
	
	// Should work normally after reset
	w.WriteLine("new content")
	assert.Equal(t, "new content\n", w.String())
}

func TestWriter_Bytes(t *testing.T) {
	// Test: Bytes returns byte slice
	w := NewWriter("\t")
	
	w.Write("hello")
	
	assert.Equal(t, []byte("hello"), w.Bytes())
}

func TestWriter_IndentDedentBounds(t *testing.T) {
	// Test: Dedent doesn't go below zero
	w := NewWriter("\t")
	
	assert.Equal(t, 0, w.IndentLevel())
	w.Dedent()
	assert.Equal(t, 0, w.IndentLevel())
	
	w.Indent()
	assert.Equal(t, 1, w.IndentLevel())
	w.Dedent()
	assert.Equal(t, 0, w.IndentLevel())
}

func TestWriter_ComplexExample(t *testing.T) {
	// Test: Complex code generation example
	w := NewWriter("\t")
	
	w.WriteLine("package main")
	w.BlankLine()
	w.WriteLine("import (")
	w.Indent()
	w.WriteLine(`"fmt"`)
	w.WriteLine(`"context"`)
	w.Dedent()
	w.WriteLine(")")
	w.BlankLine()
	
	w.WriteDocComment("UserService provides user management operations")
	w.WriteBlock("type UserService interface {", "}", func() {
		w.WriteComment("GetUser retrieves a user by ID")
		w.WriteLine("GetUser(ctx context.Context, id string) (*User, error)")
		w.BlankLine()
		w.WriteComment("CreateUser creates a new user")
		w.WriteLine("CreateUser(ctx context.Context, user *User) error")
	})
	
	// Verify structure but not exact formatting
	result := w.String()
	assert.Contains(t, result, "package main")
	assert.Contains(t, result, "import (")
	assert.Contains(t, result, "\"fmt\"")
	assert.Contains(t, result, "type UserService interface {")
	assert.Contains(t, result, "GetUser(ctx context.Context, id string) (*User, error)")
}