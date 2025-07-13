# Host API: `env.get`

The `env.get` Host API provides services with access to runtime environment variables. Unlike `secrets.get`, this API is intended for non-sensitive configuration values such as service mode, region, or feature toggles.

---

## Interface

```ts
interface EnvHostAPI {
  get(key: string): string | null;
}
```

---

## Host API Configuration

Environment variables are injected by the host at deployment or runtime and scoped per service.

```ts
interface EnvHostAPIConfig {
  env: Record<string, string>; // e.g. { "MODE": "prod", "REGION": "us-west" }
  scoped?: boolean; // if true, restricts access to service-scoped keys
}
```

---

## Enforceable Okra Policies

Policies can constrain which environment keys are accessible.

### Examples:

#### 1. **Allowlist of keys**

```json
"policy.env.get.allowedKeys": ["MODE", "REGION", "LOG_LEVEL"]
```

#### 2. **Deny access to specific keys**

```json
"policy.env.get.blockedKeys": ["INTERNAL_DEBUG_FLAG"]
```

#### 3. **Conditional access**

```json
"policy.env.get.condition": "request.auth.claims.role == 'admin' || request.key != 'DEBUG_MODE'"
```

---

## Notes for Codegen / Shim Targets

- Unlike secrets, `env.get` may return default or fallback values.
- Guests may cache values unless explicitly disabled.
- Host may optimize via lazy resolution or pre-injection.
- `null` return value should be treated as undefined.

---

This document defines the interface and policy model for `env.get`, used to generate:

- Guest shims
- Host-side injection logic
- Policy enforcement wrappers
- Developer documentation

