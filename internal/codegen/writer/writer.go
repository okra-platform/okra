package writer

import (
	"fmt"
	"strings"
)

// Writer provides utilities for generating formatted code with proper indentation
type Writer struct {
	sb           strings.Builder
	indentLevel  int
	indentString string
	linePrefix   string
	needsIndent  bool
}

// NewWriter creates a new code writer with specified indentation string
func NewWriter(indentString string) *Writer {
	return &Writer{
		indentString: indentString,
		needsIndent:  true,
	}
}

// Indent increases the indentation level
func (w *Writer) Indent() {
	w.indentLevel++
	w.updatePrefix()
}

// Dedent decreases the indentation level
func (w *Writer) Dedent() {
	if w.indentLevel > 0 {
		w.indentLevel--
		w.updatePrefix()
	}
}

// Write writes a string without adding a newline
func (w *Writer) Write(s string) {
	if w.needsIndent && s != "" {
		w.sb.WriteString(w.linePrefix)
		w.needsIndent = false
	}
	w.sb.WriteString(s)
}

// Writef writes a formatted string without adding a newline
func (w *Writer) Writef(format string, args ...interface{}) {
	w.Write(fmt.Sprintf(format, args...))
}

// WriteLine writes a string and adds a newline
func (w *Writer) WriteLine(s string) {
	w.Write(s)
	w.Newline()
}

// WriteLinef writes a formatted string and adds a newline
func (w *Writer) WriteLinef(format string, args ...interface{}) {
	w.Writef(format, args...)
	w.Newline()
}

// Newline adds a newline character
func (w *Writer) Newline() {
	w.sb.WriteString("\n")
	w.needsIndent = true
}

// BlankLine adds an empty line
func (w *Writer) BlankLine() {
	if w.sb.Len() > 0 && !strings.HasSuffix(w.sb.String(), "\n\n") {
		w.Newline()
	}
}

// IndentLevel returns the current indentation level
func (w *Writer) IndentLevel() int {
	return w.indentLevel
}

// String returns the generated code as a string
func (w *Writer) String() string {
	return w.sb.String()
}

// Bytes returns the generated code as a byte slice
func (w *Writer) Bytes() []byte {
	return []byte(w.sb.String())
}

// Reset clears the writer's content and resets indentation
func (w *Writer) Reset() {
	w.sb.Reset()
	w.indentLevel = 0
	w.linePrefix = ""
	w.needsIndent = true
}

// updatePrefix updates the line prefix based on current indentation
func (w *Writer) updatePrefix() {
	w.linePrefix = strings.Repeat(w.indentString, w.indentLevel)
}

// WriteBlock writes content inside a block with proper indentation
// Example: WriteBlock("if true {", "}", func() { w.WriteLine("fmt.Println()") })
func (w *Writer) WriteBlock(opener, closer string, content func()) {
	w.WriteLine(opener)
	w.Indent()
	content()
	w.Dedent()
	w.WriteLine(closer)
}

// WriteComment writes a single-line comment
func (w *Writer) WriteComment(comment string) {
	w.WriteLinef("// %s", comment)
}

// WriteMultilineComment writes a multi-line comment
func (w *Writer) WriteMultilineComment(lines []string) {
	for _, line := range lines {
		w.WriteComment(line)
	}
}

// WriteDocComment writes a documentation comment block
func (w *Writer) WriteDocComment(doc string) {
	if doc == "" {
		return
	}
	lines := strings.Split(strings.TrimSpace(doc), "\n")
	for _, line := range lines {
		w.WriteComment(strings.TrimSpace(line))
	}
}
