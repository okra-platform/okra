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

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **Key name validation** - Only alphanumeric, underscore, and dash characters
2. **Maximum key length** - Prevent excessive memory usage (default: 256 characters)
3. **Reserved key prevention** - Block access to host-internal keys (e.g., `OKRA_*`, `HOST_*`)
4. **Null byte protection** - Prevent injection via key names

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### 1. **Allowlist of keys**

```json
"policy.env.get.allowedKeys": ["MODE", "REGION", "LOG_LEVEL"]
```

#### 2. **Deny access to specific keys**

```json
"policy.env.get.blockedKeys": ["INTERNAL_DEBUG_FLAG", "LEGACY_*"]
```

#### 3. **Conditional access based on role**

```json
"policy.env.get.condition": "request.auth.claims.role == 'admin' || !request.key.startsWith('DEBUG_')"
```

#### 4. **Environment-specific restrictions**

```json
"policy.env.get.condition": "env.DEPLOYMENT_ENV == 'production' ? request.key in ['MODE', 'REGION'] : true"
```

#### 5. **Service-scoped access**

```json
"policy.env.get.scopedAccess": true,
"policy.env.get.keyPrefix": "SERVICE_${service.namespace}_${service.name}_"
```

#### 6. **Pattern-based allow/deny**

```json
"policy.env.get.allowedPatterns": ["^CONFIG_.*", "^FEATURE_.*"],
"policy.env.get.deniedPatterns": [".*_SECRET$", ".*_PASSWORD$"]
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

