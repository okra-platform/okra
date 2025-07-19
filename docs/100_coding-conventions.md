# Coding Conventions

## Public Interfaces, Internal Implementations

### ✅ Summary

All major components in the OKRA platform should expose a **public interface** and return an **unexported struct implementation** through a **public constructor** (`NewX(...)`). This pattern ensures clarity, flexibility, and maintainability across the codebase.

---

### Why This Matters

- **Encapsulation**  
  Hides implementation details from consumers and enables safe refactoring.

- **Testability**  
  Encourages interface-driven development, mock injection, and contract-based testing.

- **Clarity**  
  Exposes only the intended usage surface of a package.

- **Stability**  
  Allows internals to evolve without breaking consumers.

- **Consistency**  
  Follows idiomatic Go design and reinforces architectural discipline.

---

### Example

```go
// pool.go (public API)

type WASMWorkerPool interface {
    Invoke(ctx context.Context, method string, input []byte) ([]byte, error)
    ActiveWorkers() int
    Shutdown(ctx context.Context) error
}

func NewWASMWorkerPool(...) WASMWorkerPool {
    return &wasmWorkerPool{...}
}

// pool_internal.go (implementation)

type wasmWorkerPool struct {
    // internal fields
}
```

---

## JSON Serialization Conventions

### ✅ Summary

All JSON field names must use **camelCase** convention (not snake_case) to follow JSON standards and ensure consistency with JavaScript/TypeScript ecosystems.

---

### Why This Matters

- **JSON Standards Compliance**  
  camelCase is the established convention in JSON and JavaScript ecosystems.

- **API Consistency**  
  Provides a consistent experience for API consumers, especially web clients.

- **Tooling Compatibility**  
  Better support from JSON Schema validators, OpenAPI generators, and TypeScript tooling.

- **Cross-Language Consistency**  
  Aligns with naming conventions in JavaScript, TypeScript, and other web technologies.

---

### Example

```go
// ✅ CORRECT - Use camelCase for JSON tags
type ServiceInfo struct {
    Name    string `json:"name"`
    Version string `json:"version"`
}

type RequestMetadata struct {
    TraceID     string            `json:"traceId,omitempty"`     // NOT trace_id
    SpanID      string            `json:"spanId,omitempty"`      // NOT span_id
    ServiceInfo ServiceInfo       `json:"serviceInfo"`           // NOT service_info
}

// ❌ INCORRECT - Don't use snake_case
type RequestMetadata struct {
    TraceID     string            `json:"trace_id,omitempty"`    // Wrong!
    SpanID      string            `json:"span_id,omitempty"`     // Wrong!
    ServiceInfo ServiceInfo       `json:"service_info"`          // Wrong!
}
```

### Guidelines

1. **Always use camelCase** for JSON field names, even if the Go field uses a different convention
2. **Be consistent** - if a field appears in multiple structs, use the same JSON name
3. **Acronyms** - Treat acronyms as words: `id` not `ID`, `url` not `URL`, `api` not `API`
4. **Multi-word fields** - Capitalize each word except the first: `serviceInfo`, `traceId`, `hostApiName`
