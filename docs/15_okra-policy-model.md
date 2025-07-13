# Okra Policy Model: Defense-in-Depth & Zero-Trust Capability Governance

## 🎯 Overview

Okra introduces a powerful enterprise-oriented policy model that enforces **capability-level governance** at runtime, bringing zero-trust principles deep into service execution. Policies describe *what APIs a service can use*, *how they can use them*, and *under what conditions*, independent of the service implementation itself.

---

## 🧱 Core Concepts

### 1. **Hosts Define the Capability Surface**

- Each host publishes a schema describing which Host APIs it exposes and what constraints are enforceable.
- Examples:
  - `http.fetch.allowedDomains`
  - `sql.query.readOnly`
  - `fs.readFile.pathPrefix`

Hosts **can restrict the APIs they expose**, creating hard boundaries even before policies are applied.

### 2. **Policies Target Services or Tags**

- Policies are stored in the registry under `/policy/<host>/<policy>.yaml`
- They apply to one or more:
  - Fully Qualified Services (FQNs): `org/team/service`
  - Service Tags: `internal`, `edge-facing`, `trusted`

### 3. **Capability Policy Structure**

- Each policy file defines the **capability constraints** for the target services/tags.
- Constraints are:
  - Declarative: booleans, strings, enums, arrays, numbers
  - Optional CEL expressions: for context-aware logic (e.g. roles, environment)

Example:

```yaml
appliesTo:
  - service: "org-a/logs-service"
  - tag: "internal"

capabilities:
  http.fetch:
    allowedDomains: ["*.internal"]
    rateLimit:
      maxRequestsPerMinute: 500
      condition: "request.auth.claims.role == 'admin'"

  sql.query:
    readOnly: true
    allowedTables: ["logs_aggregate"]
```

### 4. **CEL Integration**

- Policies can use [CEL (Common Expression Language)] to define advanced logic.
- Examples:
  - `request.metadata.environment == 'prod'`
  - `request.auth.claims.role == 'admin'`

This allows dynamic policies that adapt to runtime context.

### 5. **Registry-Backed Governance**

- Policies are versioned, stored, and enforced via the Okra Registry.
- Hosts sync policies based on registry state.
- Open Source: pull-only + manual notification
- Enterprise: real-time push + audit trail + RBAC + policy review flows

---

## 🛡️ Defense-in-Depth Strategy

### Hybrid Policy Enforcement Model

Okra employs a **hybrid approach** to policy enforcement, combining code-level security checks with flexible CEL-based policies:

1. **Code-Level Policies** (Implemented in Host API code)
   - Security-critical validations that should never be bypassed
   - Performance-sensitive checks (e.g., size limits, format validation)
   - Protection against malformed requests and injection attacks
   - Examples: maximum payload sizes, valid parameter ranges, input sanitization

2. **CEL-Based Policies** (Configured via Registry)
   - Business logic and access control rules
   - Environment-specific configurations
   - Dynamic conditions based on runtime context
   - Examples: allowed domains, rate limits, feature flags

This hybrid approach ensures that critical security boundaries are always enforced while maintaining flexibility for business rules.

### Zero-Trust Posture

| Layer              | Enforced By        | Description                                           |
| ------------------ | ------------------ | ----------------------------------------------------- |
| Service Identity   | WorkOS + Host      | Strong authN with scoped claims                       |
| Host API Surface   | Host Config        | Only a subset of APIs are injected                    |
| Code-Level Policy  | Host API Code      | Critical security checks (sizes, formats, injection)  |
| CEL-Based Policy   | Registry + Host    | Business rules, access control, dynamic conditions    |
| Runtime Conditions | CEL                | Dynamic rules based on environment or identity        |
| Approval Workflow  | Registry + UI      | Human-in-the-loop policy creation                     |

---

## 🚀 Enterprise Differentiators

| Feature                       | OSS | Enterprise |
| ----------------------------- | --- | ---------- |
| Per-service policies          | ✅   | ✅          |
| CEL expressions               | ✅   | ✅          |
| Registry storage              | ✅   | ✅          |
| Visual Policy Editor          | ❌   | ✅          |
| AI policy assistant           | ❌   | ✅          |
| RBAC on policy editing        | ❌   | ✅          |
| Policy approval flows         | ❌   | ✅          |
| Real-time host sync           | ❌   | ✅          |
| Audit trail & version history | ❌   | ✅          |

---

## 🔄 Policy Review Loop (Enterprise)

1. Developer pushes new service
2. Registry detects missing or outdated policy
3. Notification sent to Ops/Sec team
4. Team creates or updates a capability policy
5. Policy is reviewed, approved, and versioned
6. Host pulls latest policy and enforces at runtime

---

## 🔒 Strategic Messaging

- **"Bring zero-trust to the runtime surface."**
- **"Not just who can deploy — what can they do once deployed."**
- **"Programmable governance, declarative by default."**
- **"Stop lateral movement and shadow privileges before they happen."**

