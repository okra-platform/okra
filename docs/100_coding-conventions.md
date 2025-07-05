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
