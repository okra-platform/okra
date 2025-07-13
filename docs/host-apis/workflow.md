# Host API: `workflow.*`

The `workflow.*` Host API provides capabilities for services to interact with OKRA workflows - running workflows, sending signals, and querying workflow status. This API enables services to orchestrate complex multi-service operations while maintaining proper governance and observability.

---

## Interface

```ts
interface WorkflowInput {
  workflowId: string; // Fully qualified: namespace.name.version
  input: any; // Must match workflow's inputSchema
  correlationId?: string; // For tracking related executions
  tags?: Record<string, string>; // Metadata for querying
}

interface WorkflowInstance {
  instanceId: string;
  workflowId: string;
  version: string;
  status: 'pending' | 'running' | 'waiting' | 'completed' | 'failed' | 'cancelled';
  currentStep?: string;
  startTime: string; // ISO 8601
  endTime?: string; // ISO 8601
  input: any;
  output?: any;
  error?: string;
  variables: Record<string, any>; // Current workflow variables
}

interface SignalInput {
  instanceId: string;
  signal: string;
  payload?: any;
}

interface QueryOptions {
  workflowId?: string;
  status?: WorkflowInstance['status'][];
  tags?: Record<string, string>;
  startedAfter?: string; // ISO 8601
  startedBefore?: string; // ISO 8601
  limit?: number;
  cursor?: string;
}

interface WorkflowHostAPI {
  // Run a new workflow instance
  run(workflow: WorkflowInput): Promise<string>; // Returns instance ID
  
  // Send a signal to a waiting workflow instance
  signal(signal: SignalInput): Promise<void>;
  
  // Get workflow instance status
  getStatus(instanceId: string): Promise<WorkflowInstance>;
  
  // Query workflow instances
  query(options: QueryOptions): Promise<{
    instances: WorkflowInstance[];
    cursor?: string;
  }>;
  
  // Cancel a running workflow
  cancel(instanceId: string, reason?: string): Promise<void>;
  
  // List available workflows
  listWorkflows(): Promise<{
    workflowId: string;
    version: string;
    triggers: string[];
  }[]>;
}
```

---

## Host API Configuration

Workflow execution behavior and limits can be configured per deployment.

```ts
interface WorkflowHostAPIConfig {
  maxConcurrentInstances?: number; // Per service
  maxInstanceDuration?: number; // Maximum runtime in seconds
  enableManualTriggers?: boolean;
  retentionDays?: number; // How long to keep completed instances
  defaultTimeout?: number; // Default step timeout
}
```

---

## Enforceable Okra Policies

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **Workflow ID validation** - Must be valid FQN format
2. **Input schema validation** - Input must match workflow's declared schema
3. **Signal name validation** - Only alphanumeric, underscore, dash allowed
4. **Maximum input size** - Prevent memory exhaustion (default: 1MB)
5. **Instance ID format** - Must be valid UUID or system-generated ID
6. **Query result limits** - Maximum 1000 instances per query
7. **Recursion prevention** - Workflows cannot run themselves directly

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### 1. **Workflow execution permissions**

```json
"policy.workflow.run.allowed": ["user.onboarding", "order.processing", "report.*"],
"policy.workflow.run.blocked": ["admin.dangerous", "legacy.*"]
```

#### 2. **Conditional execution**

```json
"policy.workflow.run.condition": "workflow.id != 'payment.process' || request.auth.claims.role == 'payment_admin'"
```

#### 3. **Signal permissions**

```json
"policy.workflow.signal.allowed": true,
"policy.workflow.signal.allowedSignals": ["approved", "rejected", "timeout"],
"policy.workflow.signal.condition": "instance.status == 'waiting' && request.auth.claims.canSignal"
```

#### 4. **Query restrictions**

```json
"policy.workflow.query.maxResults": 100,
"policy.workflow.query.allowedStatuses": ["completed", "failed"],
"policy.workflow.query.condition": "request.auth.claims.role in ['admin', 'operator']"
```

#### 5. **Rate limiting**

```json
"policy.workflow.run.rateLimit": "100/minute",
"policy.workflow.signal.rateLimit": "1000/minute",
"policy.workflow.query.rateLimit": "100/minute"
```

#### 6. **Instance limits**

```json
"policy.workflow.maxConcurrentInstances": 50,
"policy.workflow.maxInstancesPerWorkflow": {
  "user.onboarding": 100,
  "report.generation": 10
}
```

#### 7. **Environment-specific policies**

```json
"policy.workflow.condition": "env.DEPLOYMENT_ENV == 'production' ? workflow.id != 'test.*' : true"
```

#### 8. **Input validation**

```json
"policy.workflow.run.maxInputSizeKb": 100,
"policy.workflow.run.requiredInputFields": {
  "payment.process": ["amount", "currency", "customerId"]
}
```

#### 9. **Cancellation policies**

```json
"policy.workflow.cancel.allowed": true,
"policy.workflow.cancel.condition": "instance.status in ['pending', 'running'] && request.auth.claims.role == 'admin'",
"policy.workflow.cancel.requireReason": true
```

#### 10. **Audit and monitoring**

```json
"policy.workflow.audit": true,
"policy.workflow.auditLevel": "full", // "none", "summary", "full"
"policy.workflow.alertOnFailure": ["payment.*", "compliance.*"]
```

#### 11. **Cross-service restrictions**

```json
"policy.workflow.run.crossNamespace": false,
"policy.workflow.run.allowedNamespaces": ["app", "common"],
"policy.workflow.run.condition": "workflow.namespace == service.namespace || workflow.namespace == 'common'"
```

#### 12. **Timeout policies**

```json
"policy.workflow.maxDurationMinutes": 60,
"policy.workflow.defaultTimeoutMs": 300000,
"policy.workflow.timeoutAction": "fail" // "fail", "cancel", "continue"
```

---

## Integration with Scheduling

The schedule host API can trigger workflows at specified times:

```typescript
// In schedule API
await schedule.triggerWorkflow({
  workflowId: "reports.monthly",
  input: { month: "2024-01" },
  cronExpression: "0 0 1 * *" // First day of month
});
```

---

## Notes for Codegen / Shim Targets

- Workflow calls should be async and non-blocking
- Input validation should happen before workflow execution starts
- Signal delivery should be guaranteed (at-least-once)
- Query results should be paginated for large result sets
- Instance status should be eventually consistent
- Support correlation IDs for tracking related workflows
- Provide type-safe interfaces based on workflow schemas
- Handle workflow versioning properly (explicit versions)

---

This specification enables:

- Multi-service orchestration
- Long-running business processes
- Human-in-the-loop workflows via signals
- Event-driven automation
- Saga pattern implementation
- Process monitoring and debugging
- Compliance and audit trails
- Complex retry and compensation logic