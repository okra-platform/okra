# Okra Workflow Specification

This document defines the architecture, structure, and rationale behind `okra.workflow.yaml` files. These workflows represent typed, declarative, durable execution graphs for orchestrating service calls, callbacks, AI steps, and logic — all governed by YAML + CEL and fully inspectable by the platform.

---

## 🧠 Philosophy

Okra workflows:

- Are **declarative**: logic is defined in YAML, not code
- Are **governed**: logic, transitions, and schema are auditable and policy-enforced
- Are **typed**: every step has input/output schemas, enabling strong validation and developer tooling
- Are **composable**: workflows can depend on services, events, and other workflows
- Are **observable**: fully indexable, introspectable, and queryable across an org
- Are **AI-safe**: agents and callbacks provide data, but cannot dictate flow

---

## 📄 File: `okra.workflow.yaml`

```yaml
name: user.onboarding
version: v1

inputSchema:
  type: object
  properties:
    userId:
      type: string

triggers:
  - type: event
    event: user.created
    condition: "event.payload.accountType == 'pro'"

steps:
  - name: createAccount
    call:
      type: service
      target: account.create
      args:
        userId: "{{ input.userId }}"
    outputSchema:
      type: object
      properties:
        accountId:
          type: string
    saveResultAs: accountResult

  - name: waitForActivation
    waitForSignal: activated
    timeoutMs: 86400000
    onTimeout:
      next: markAbandoned

  - name: markAbandoned
    call:
      type: service
      target: user.flagInactive
      args:
        userId: "{{ input.userId }}"
```

---

## ✨ Step Anatomy

Each step can include:

```yaml
- name: stepName
  call:
    type: service | callback | ai
    target: service.method or functionName
    args: {...}
  inputSchema: {...}      # Optional validation for step input
  outputSchema: {...}     # Validation of result
  saveResultAs: varName   # Binds result for use in conditions or future args
  waitForSignal: signalName  # Blocks until received
  timeoutMs: number       # Fails or reroutes on timeout
  condition: CEL          # Conditional execution (if true, continue)
  next: stepName          # Override next step
  onFailure: fail | retry | { goto: stepName }
```

---

## 🔄 Triggers

Workflows can be triggered by:

```yaml
triggers:
  - type: event
    event: user.created
    condition: CEL

  - type: manual
    condition: CEL

  - type: signal
    signal: wake
    
  - type: schedule
    cron: "0 0 * * *"  # Daily at midnight
    timezone: "UTC"
```

Triggering creates a new workflow instance, bound to input and version. Scheduled triggers can be configured statically in the workflow definition or dynamically via the schedule host API.

---

## 📦 Dependencies

```yaml
dependsOn:
  - service: user
  - service: email
  - workflow: common.auditTrail@v2
```

Used to:

- Generate a dependency graph
- Validate available services/workflows
- Track breaking changes or drift

---

## 🔐 Policy Control

Workflows are subject to Okra policies:

```json
"policy.workflow.run.allowed": ["user.onboarding"]
"policy.workflow.step.maxSteps": 20
"policy.workflow.trigger.allowedEvents": ["user.*"]
"policy.workflow.callback.allowedTargets": ["kyc.validate", "plugin.*"]
"policy.workflow.schema.validation": true
```

---

## ⚡ Tooling Powered by This Spec

- `okra run user.onboarding --input userId=abc123`
- `okra signal user.onboarding@v1 --id wf-123 --signal activated`
- `okra validate workflows/*.yaml`
- `okra graph` — visualizes flow
- `okra who-calls billing.charge` — full dependency resolution
- `okra test` — simulates workflow execution
- `okra workflow status --id wf-123` — query instance status
- `okra workflow list --status running` — list active workflows

---

## 🛠️ Runtime Contract

The runtime ensures:

- Step execution conforms to input/output schemas
- CEL expressions only access valid, typed bindings
- Timeouts, retries, and signal waits are host-enforced
- All execution is logged and observable

---

## 🔌 Programmatic Interaction

Services interact with workflows through two host APIs:

### Workflow Host API (`workflow.*`)

Services can programmatically run workflows, send signals, and query status:

```typescript
// Run a workflow
const instanceId = await workflow.run({
  workflowId: "user.onboarding.v1",
  input: { userId: "user-123", plan: "pro" }
});

// Send a signal to a waiting workflow
await workflow.signal({
  instanceId: instanceId,
  signal: "activated",
  payload: { activatedAt: new Date() }
});

// Query workflow status
const status = await workflow.getStatus(instanceId);
if (status.status === 'failed') {
  // Handle failure
}
```

### Schedule Host API (`schedule.*`)

Services can schedule workflows to run at specific times or intervals:

```typescript
// Schedule a workflow to run daily
await schedule.triggerWorkflow({
  workflowId: "reports.daily.v1",
  cronExpression: "0 0 * * *",
  input: { reportType: "summary" }
});

// One-time scheduled workflow
await schedule.triggerWorkflow({
  workflowId: "user.followup.v1",
  at: "2024-12-01T10:00:00Z",
  input: { userId: "user-123" }
});
```

See the [workflow host API](./host-apis/workflow.md) and [schedule host API](./host-apis/schedule.md) documentation for complete details.

---

## 🧩 Future Extensions

- Parallel execution (`fork`, `join`)
- Subworkflows
- Typed signal schemas
- Schema inheritance or shared types
- Workflow templates or DSL for partial reuse

---

This spec forms the foundation of Okra's safe, composable workflow engine. All orchestration logic lives in these files — not in code — enabling unparalleled clarity, safety, and tooling leverage across engineering, AI, and enterprise governance.

