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

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **Key format validation** - Only alphanumeric, slash, underscore, dash allowed
2. **Maximum key length** - Prevent excessive memory usage (default: 256 characters)
3. **Key path traversal protection** - Prevent ../ or absolute paths
4. **Response time obfuscation** - Constant time comparison to prevent timing attacks
5. **Memory zeroing** - Secret values cleared from memory after use
6. **No logging** - Secret values never logged, even in debug mode
7. **Encryption at rest** - Secrets stored encrypted in host configuration

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### 1. **Allow only specific keys**

```json
"policy.secrets.get.allowedKeys": ["db/password", "api/key", "smtp/credentials"]
```

#### 2. **Key pattern restrictions**

```json
"policy.secrets.get.allowedPatterns": ["^db/.*", "^api/.*"],
"policy.secrets.get.blockedPatterns": [".*_test$", ".*_dev$"]
```

#### 3. **Conditional access based on role**

```json
"policy.secrets.get.condition": "request.auth.claims.role in ['admin', 'service'] || request.key.startsWith('public/')"
```

#### 4. **Environment-specific access**

```json
"policy.secrets.get.condition": "env.DEPLOYMENT_ENV == 'production' ? !request.key.endsWith('_dev') : true"
```

#### 5. **Service-scoped secrets**

```json
"policy.secrets.get.scopedAccess": true,
"policy.secrets.get.keyPrefix": "${service.namespace}/${service.name}/"
```

#### 6. **Audit and alerting**

```json
"policy.secrets.get.audit": true,
"policy.secrets.get.alertOnAccess": ["db/master_password", "payment/api_key"]
```

#### 7. **Rate limiting**

```json
"policy.secrets.get.rateLimit": "60/minute",
"policy.secrets.get.burstLimit": 10
```

#### 8. **Time-based access**

```json
"policy.secrets.get.condition": "now().hour() >= 9 && now().hour() <= 17 || request.auth.claims.oncall == true"
```

#### 9. **Rotation enforcement**

```json
"policy.secrets.get.maxAge": "90d",
"policy.secrets.get.warnAge": "75d"
```

#### 10. **Multi-factor requirements**

```json
"policy.secrets.get.requireMFA": ["payment/*", "admin/*"],
"policy.secrets.get.condition": "request.key.startsWith('payment/') ? request.auth.mfa_verified : true"
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

