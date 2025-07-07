package schema

import (
	"regexp"
)

// okraDirectiveRegex matches @okra(...) at the start of a line.
// Uses a more robust pattern that handles nested parentheses better by matching
// balanced content or escaping after a reasonable depth.
var okraDirectiveRegex = regexp.MustCompile(`(?m)^@okra\s*\(((?:[^()]*|\([^)]*\))*)\)`)

// serviceStartRegex matches service declarations at the start of a line.
// Captures the service name which must be a valid GraphQL identifier.
var serviceStartRegex = regexp.MustCompile(`(?m)^service\s+(\w+)\s*{`)

// PreprocessGraphQL rewrites `@okra(...)` and `service` blocks into valid GraphQL `type` definitions.
func PreprocessGraphQL(input string) string {
	// 1. Rewrite @okra(...) to a _Schema type with a properly typed field
	// The field needs a type to be valid GraphQL
	input = okraDirectiveRegex.ReplaceAllStringFunc(input, func(match string) string {
		args := okraDirectiveRegex.FindStringSubmatch(match)[1]
		return `type _Schema {
  _: String @okra(` + args + `)
}`
	})

	// 2. Rewrite service blocks to type Service_X {
	input = serviceStartRegex.ReplaceAllStringFunc(input, func(match string) string {
		serviceName := serviceStartRegex.FindStringSubmatch(match)[1]
		return `type Service_` + serviceName + ` {`
	})

	return input
}
