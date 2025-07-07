# GraphQL-Go-Tools AST Value Extraction Guide

## Overview

When working with the `graphql-go-tools` AST, extracting values from directive arguments requires understanding the proper methods and the structure of the AST.

## Key Concepts

### 1. Value Structure
The `ast.Value` type has this structure:
```go
type Value struct {
    Kind     ValueKind // The type of value (string, int, boolean, etc.)
    Ref      int       // Reference/index to the actual value
    Position position.Position
}
```

### 2. Document Storage
The `ast.Document` stores different value types in separate arrays:
- `StringValues []StringValue` - for string literals
- `IntValues []IntValue` - for integer values
- `FloatValues []FloatValue` - for float values
- `BooleanValues [2]BooleanValue` - for true/false (index 0=false, 1=true)
- `EnumValues []EnumValue` - for enum values
- `Values []Value` - the main value references

### 3. Proper Extraction Methods

#### For Directive Arguments:
```go
// Get the argument value
value := doc.ArgumentValue(argRef)
```

#### For Different Value Types:

**Strings:**
```go
case ast.ValueKindString:
    return doc.StringValueContentString(value.Ref)
```

**Enums:**
```go
case ast.ValueKindEnum:
    if value.Ref >= 0 && value.Ref < len(doc.EnumValues) {
        return doc.Input.ByteSliceString(doc.EnumValues[value.Ref].Name)
    }
```

**Booleans:**
```go
case ast.ValueKindBoolean:
    if value.Ref >= 0 && value.Ref < len(doc.BooleanValues) {
        if doc.BooleanValues[value.Ref] {
            return "true"
        }
        return "false"
    }
```

**Integers:**
```go
case ast.ValueKindInteger:
    return fmt.Sprintf("%d", doc.IntValueAsInt(value.Ref))
```

**Floats:**
```go
case ast.ValueKindFloat:
    return fmt.Sprintf("%f", doc.FloatValueAsFloat32(value.Ref))
```

## Common Mistakes to Avoid

1. **Don't access Values array directly** - The `value.Ref` is NOT an index into `doc.Values`
2. **Don't create ByteSliceReference manually** - Use the provided methods
3. **Don't assume value.Ref structure** - It's an opaque reference to be used with document methods
4. **Always check array bounds** - Especially for enum values

## Complete Example

See the `parseValue` function in `parser.go` for a complete implementation of value extraction that handles all value types properly.

## Key Takeaways

1. Use `doc.ArgumentValue(argRef)` to get argument values
2. Use type-specific document methods like `StringValueContentString()`
3. The `value.Ref` is an index into type-specific arrays, not a direct value
4. Always handle all value kinds in your switch statement