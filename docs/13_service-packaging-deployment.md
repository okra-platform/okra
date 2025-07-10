## OKRA Service Development & Packaging Model

### ✅ Service is the Unit of Deployment

- Every service is published as its own `.pkg` file.
- Each service has a stable Fully Qualified Name (FQN): `namespace.service.version` (e.g., `xyz.acme.user.User.v1`).
- Services are deployed, versioned, observed, and governed independently.

---

### 📁 Shared `okra.json` for Collocated Services

- Multiple services in the same directory/repo share a single `okra.json`.
- Keeps config minimal and simple for mini-monoliths or tight API surfaces.
- Service-level settings like exposure and path are embedded under a shared namespace/version.

---

### ♻️ Build-Time `.pkg` Generation

- Each service is compiled into a separate `.pkg` regardless of shared source.
- Each `.pkg` includes:
  - The service-specific interface, types, and compiled logic.
  - A generated `okra.json` scoped only to that service.
  - Automatically inferred dependencies from service usage (e.g., constructor injection).

---

### 🔍 Registry Model

- Registry is flat: each FQN maps directly to a `.pkg` file.
- No need to understand multi-service groupings or package structure.
- Example:
  ```json
  {
    "xyz.acme.user.User.v1": "cdn/xyz.acme.user.User.v1.pkg",
    "xyz.acme.user.Account.v1": "cdn/xyz.acme.user.Account.v1.pkg"
  }
  ```

---

### 🧠 Dependency Inference

- Internal service dependencies are inferred from DI or service call usage.
- External dependencies (e.g., `workspace:*`, published FQNs) are declared manually in `okra.json`.
- Ensures correct build-time wiring without requiring manual references between co-located services.

---

### 🌱 Three-Stage Lifecycle

| Stage | Features |
| ----- | -------- |
|       |          |

| **Stage 1** | Single `okra.json`, all services in one repo/file, fast iteration |
| ----------- | ----------------------------------------------------------------- |
| **Stage 2** | Services split into folders using `workspace:*` dependencies      |
| **Stage 3** | Services live in separate repos, published independently          |

---

### 🔒 Stable Behavior, Refactor-Friendly

- Moving a service to its own repo doesn't change:
  - Its FQN
  - Its packaging
  - Its runtime or deployment behavior
- Encourages gradual evolution without coupling runtime to authoring layout

---

### 🔹 TL;DR

> **OKRA enables service-level autonomy, developer simplicity, and enterprise scalability through stable FQNs, inferred dependencies, flat registries, and intelligent CLI tooling.**

