# Service IDL
Services are defined using an IDL that is "GraphQL-ish".

It reuses the `type` and `enum` declaration and adds a `service` declarations. It also supports directives for things like @validation, @auth, etc.

At build time `okra` cli parses the IDL. While running `okra dev` it uses the IDL info to generate the interface / struct file. `okra build` generates a `service.discovery.json` file with a serialized description of the service.

There is a top-level built-in directive `@okra` that allows users to specify namespace and version info:

```graphql
@okra(
    namespace: "auth.users", 
    version: "v1"
)
```

## Supported Scalar Types
| Scalar     | Description                     | Format / Standard                | Go Type   | JSON Type            |
| ---------- | ------------------------------- | -------------------------------- | --------- | -------------------- |
| `Int`      | 32-bit signed integer           | —                                | `int32`   | `number`             |
| `Float`    | 64-bit floating point number    | —                                | `float64` | `number`             |
| `String`   | UTF-8 encoded string            | —                                | `string`  | `string`             |
| `Boolean`  | Boolean true/false              | —                                | `bool`    | `boolean`            |
| `ID`       | Unique identifier               | —                                | `string`  | `string`             |
| `Date`     | Calendar date                   | ISO 8601 (`YYYY-MM-DD`)          | `string`  | `string`             |
| `Time`     | Time of day                     | ISO 8601 (`HH:MM[:SS]`)          | `string`  | `string`             |
| `DateTime` | Full timestamp with timezone    | RFC 3339 / ISO 8601              | `string`  | `string`             |
| `Duration` | Elapsed time duration           | ISO 8601 Duration (`PnDTnHnMnS`) | `string`  | `string`             |
| `UUID`     | Universally unique identifier   | UUID v4                          | `string`  | `string`             |
| `URL`      | Web or network resource locator | URI (RFC 3986)                   | `string`  | `string`             |
| `BigInt`   | Arbitrary-precision integer     | —                                | `string`  | `string` or `number` |
| `Decimal`  | Arbitrary-precision decimal     | —                                | `string`  | `string`             |
| `Bytes`    | Binary blob                     | Base64-encoded                   | `[]byte`  | `string`             |
| `Currency` | ISO currency code               | ISO 4217 (e.g., `USD`)           | `string`  | `string`             |


# OKRA IDL Preprocessing Rules

Before parsing `.okra.gql` files with a standard GraphQL parser, the OKRA CLI applies a set of preprocessing transforms to support custom syntax extensions. These transforms rewrite non-standard constructs into valid GraphQL types, allowing us to leverage existing parsing tools while preserving semantic intent.

see `schema/preprocess.go`

---

## 🔧 Supported Preprocessor Transforms

### 1. `@okra(...)` Directive → `_Schema` Wrapper

Top-level `@okra(...)` directives are transformed into a synthetic GraphQL `type` called `_Schema`, which wraps the directive on a dummy field.

#### Original:

```graphql
@okra(namespace: "auth.users", version: "v1")
```

#### Transformed:

```graphql
type _Schema {
  OkraDirective @okra(namespace: "auth.users", version: "v1")
}
```

This allows the directive and its arguments to be parsed and preserved in the AST, while clearly identifying it as schema-level metadata.

---

### 2. `service` Blocks → `type Service_*` Replacement

Custom `service ServiceName { ... }` blocks are rewritten as standard GraphQL `type` declarations, with the name prefixed by `Service_`.

#### Original:

```graphql
service UserService {
  createUser(input: CreateUser): CreateUserResponse
    @auth(cel: "auth.role == 'admin'")
}
```

#### Transformed:

```graphql
type Service_UserService {
  createUser(input: CreateUser): CreateUserResponse
    @auth(cel: "auth.role == 'admin'")
}
```

This allows the standard GraphQL parser to process the block as a regular type, while the OKRA toolchain can later extract service definitions by looking for types with the `Service_` prefix.

---

## Benefits of This Approach

* Allows custom syntax without modifying the GraphQL parser
* Leverages existing AST tooling and directive parsing
* Keeps the IDL readable and concise
* Ensures round-trip compatibility for tooling

---

These transforms are applied automatically as part of the OKRA CLI's parsing pipeline. They are invisible to most users but critical for bridging OKRA’s extended semantics with standard GraphQL tooling.
