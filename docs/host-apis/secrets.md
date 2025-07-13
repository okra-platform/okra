# Host API: `secrets.get`

The `secrets.get` Host API provides a secure, policy-enforced mechanism for services to retrieve secret values at runtime. This includes API keys, credentials, tokens, and other sensitive values.

---

## Interface

```ts
interface SecretsHostAPI {
  get(key: string): Promise<string | null>;
}
```

---

## Host API Configuration

Secrets are configured statically by the host and never exposed directly in code. They may be scoped globally or per service.

```ts
interface SecretsHostAPIConfig {
  secrets: Record<string, string>; // e.g. { "db/password": "...", "api/key": "..." }
  scoped?: boolean; // if true, service can only access its own declared secrets
}
```

---

## Enforceable Okra Policies

Policies can restrict access to specific secrets or conditionally allow access based on CEL expressions.

### Examples:

#### 1. **Allow only specific keys**

```json
"policy.secrets.get.allowedKeys": ["db/password", "api/key"]
```

#### 2. **Require role or context**

```json
"policy.secrets.get.condition": "request.auth.claims.role == 'internal' && request.key.startsWith('db/')"
```

#### 3. **Audit access**

```json
"policy.secrets.get.audit": true
```

#### 4. **Max access count (rate limit-like)**

```json
"policy.secrets.get.maxCallsPerMinute": 60
```

---

## Notes for Codegen / Shim Targets

- Shims should expose a simple async `get` method.
- No secret value should be persisted or logged.
- Policy evaluation should occur before any access attempt.
- Responses must distinguish between "not found" and "not allowed".

---

This specification will be used to:

- Generate client bindings in guest environments
- Enforce host-level secret management
- Document available secrets and access rules
- Enable audit/logging infrastructure for secret access

