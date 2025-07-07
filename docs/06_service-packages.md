# OKRA Service Package (.pkg)

An OKRA service package is a compiled bundle that contains everything needed to execute a service in a WASM-based runtime. It enables dynamic deployment and invocation without embedding service logic directly into the platform.

---

## 📦 Package Contents

Each `.pkg` file includes:

- `service.wasm` – Compiled WASM binary exposing `handle_request`
- `service.description.json` – JSON description of the parsed GraphQL IDL for validation and code generation
- `okra.service.json` – Describes supported methods, required host APIs, configuration, etc.

Packages are versioned and uploaded to object storage (e.g., S3, R2, GCS).

---

## Exported Method: `handle_request`

All WASM services must export a single entrypoint:

```wasm
(export "handle_request" (func $handle_request))
```

The OKRA runtime will:
* Allocate memory inside the module
* Write the JSON-encoded request bytes ([]byte)
* Call handle_request(ptr, len)
* Expect a pointer/length response containing the JSON-encoded response

## Internal Dispatch Logic: `handle_request` Implementation

The `handle_request` function is generated in Go or TypeScript and compiled to WASM.

It performs three main steps:

1. **Match the method name**  
   A static `switch` or `match` block routes the call based on the method string.

2. **Deserialize the input**  
   The raw `[]byte` input is parsed from JSON into the expected type for that method.

3. **Call the method and serialize the result**  
   The matching handler function is called with the typed input.  
   The return value is serialized to JSON and returned to the host.

There is no dynamic reflection or descriptor inspection in the WASM module.  
All routing and type handling is done statically in the generated code.

### Example

```go
func handle_request(method string, input []byte) ([]byte, error) {
    switch method {
    case "GetUser":
        var req GetUserRequest
        if err := json.Unmarshal(input, &req); err != nil {
            return nil, err
        }
        res, err := GetUser(&req)
        if err != nil {
            return nil, err
        }
        return json.Marshal(res)

    case "CreateUser":
        var req CreateUserRequest
        if err := json.Unmarshal(input, &req); err != nil {
            return nil, err
        }
        res, err := CreateUser(&req)
        if err != nil {
            return nil, err
        }
        return json.Marshal(res)

    // ... additional methods ...

    default:
        return nil, fmt.Errorf("unknown method: %s", method)
    }
}
```