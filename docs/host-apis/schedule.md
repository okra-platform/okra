# Host API: `schedule.*`

The `schedule.*` Host API provides cron-based scheduling capabilities for services to execute their own methods at specified intervals. This API enables services to schedule periodic tasks while maintaining proper encapsulation - services can only schedule their own methods, not methods of other services.

---

## Scheduling Boundaries

The schedule API enforces proper encapsulation and service boundaries:

### ✅ Can Schedule:
- **Your own service methods** - Any method defined in your service
- **Workflow triggers** - For cross-service orchestration needs

### ❌ Cannot Schedule:
- **Other services' methods directly** - Maintains service isolation
- **Arbitrary code/callbacks** - WASM doesn't support dynamic functions
- **Host API calls** - Can't schedule direct API invocations

### Why These Boundaries Matter:
1. **Service isolation** - Services remain independent and deployable
2. **Clear dependencies** - No hidden cross-service scheduling
3. **Predictable behavior** - All scheduled work is visible in the IDL
4. **Security** - Services can't interfere with each other's execution

### Example:
```graphql
service OrderService @okra(namespace: "shop", version: "1.0") {
  # ✅ Can schedule this - own method
  @scheduleOnly
  dailyOrderReport(): void
  
  # ✅ Can trigger workflows for cross-service needs
  # await schedule.triggerWorkflow({
  #   workflowId: "shop.reconciliation.v1",
  #   cronExpression: "0 0 * * *"
  # })
}

service InventoryService @okra(namespace: "shop", version: "1.0") {
  # ❌ OrderService CANNOT schedule this method
  updateStock(): void
}
```

For cross-service orchestration, use workflows which provide proper visibility, versioning, and governance.

---

## Interface

```ts
interface ScheduleOptions {
  cronExpression: string; // Standard cron format
  method: string; // Method name from this service's IDL
  input?: any; // Optional input to pass to the method
  timezone?: string; // IANA timezone for schedule evaluation
  maxExecutions?: number; // Limit number of executions
  startTime?: string; // ISO 8601, when to start schedule
  endTime?: string; // ISO 8601, when to stop schedule
}

interface ScheduleHandle {
  id: string;
  method: string;
  cronExpression: string;
  nextExecution: string; // ISO 8601
  executionCount: number;
  status: 'active' | 'paused' | 'completed';
}

interface WorkflowTrigger {
  workflowId: string; // Fully qualified workflow ID
  input?: any; // Input to pass to workflow
  at?: string; // ISO 8601, when to trigger (one-time)
  cronExpression?: string; // For recurring workflow triggers
}

interface ScheduleHostAPI {
  // Schedule a method from the current service
  schedule(options: ScheduleOptions): Promise<string>; // Returns schedule ID
  
  // Cancel a schedule
  cancel(scheduleId: string): Promise<void>;
  
  // List active schedules for this service
  list(): Promise<ScheduleHandle[]>;
  
  // Pause/resume a schedule
  pause(scheduleId: string): Promise<void>;
  resume(scheduleId: string): Promise<void>;
  
  // Trigger a workflow at a specific time or schedule
  triggerWorkflow(trigger: WorkflowTrigger): Promise<string>; // Returns trigger ID
}
```

---

## Host API Configuration

Scheduling behavior can be configured per deployment with different backends and limits.

```ts
interface ScheduleHostAPIConfig {
  backend: 'memory' | 'redis' | 'postgres' | 'etcd';
  maxSchedulesPerService?: number; // Maximum concurrent schedules
  minIntervalSeconds?: number; // Minimum time between executions
  allowDynamicScheduling?: boolean; // Enable runtime scheduling
  persistSchedules?: boolean; // Survive service restarts
  defaultTimezone?: string; // Default if not specified
}
```

---

## IDL Support

Services can declare scheduled methods in their IDL:

```graphql
service Tasks @okra(namespace: "app", version: "1.0") {
  # Regular API method
  getTasks(): TaskList
  
  # Can be scheduled or called directly
  processDaily(input: ProcessInput): Result
  
  # Schedule-only method (cannot be called via RPC)
  cleanupOld(): Result @scheduleOnly
}
```

---

## Configuration-Based Scheduling

Static schedules can be defined in `okra.json`:

```json
{
  "schedules": [
    {
      "cron": "0 0 * * *",
      "method": "processDaily",
      "input": { "mode": "full" },
      "timezone": "UTC"
    },
    {
      "cron": "0 * * * *",
      "method": "cleanupOld"
    }
  ]
}
```

---

## Enforceable Okra Policies

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **Cron expression validation** - Must be valid standard cron syntax
2. **Method name validation** - Must exist in service IDL
3. **Maximum concurrent schedules** - Prevent resource exhaustion (default: 100 per service)
4. **Schedule frequency limits** - Minimum interval between executions (default: 1 minute)
5. **Input size limits** - Maximum size of input data (default: 1MB)
6. **Timezone validation** - Must be valid IANA timezone identifier
7. **Method accessibility** - Cannot schedule private or @scheduleOnly methods via RPC

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### 1. **Scheduling restrictions**

```json
"policy.schedule.allowed": true,
"policy.schedule.allowedCronPatterns": ["0 * * * *", "*/15 * * * *", "0 0 * * *"],
"policy.schedule.maxFrequencySeconds": 300
```

#### 2. **Method restrictions**

```json
"policy.schedule.allowedMethods": ["processDaily", "cleanupOld", "syncData"],
"policy.schedule.blockedMethods": ["deleteAll", "resetSystem"]
```

#### 3. **Environment-specific policies**

```json
"policy.schedule.condition": "env.DEPLOYMENT_ENV != 'production' || request.method in ['processDaily', 'generateReports']"
```

#### 4. **Schedule execution limits**

```json
"policy.schedule.maxExecutions": 1000,
"policy.schedule.maxDurationDays": 365,
"policy.schedule.requiredEndTime": true
```

#### 5. **Rate limiting**

```json
"policy.schedule.create.rateLimit": "10/hour",
"policy.schedule.modify.rateLimit": "20/hour"
```

#### 6. **Input validation**

```json
"policy.schedule.maxInputSizeKb": 100,
"policy.schedule.requireInputValidation": true
```

#### 7. **Business hour restrictions**

```json
"policy.schedule.businessHoursOnly": true,
"policy.schedule.businessHours": "09:00-17:00",
"policy.schedule.businessDays": ["Mon", "Tue", "Wed", "Thu", "Fri"]
```

#### 8. **Audit and monitoring**

```json
"policy.schedule.audit": true,
"policy.schedule.auditMethods": ["processPayments", "cleanupData"],
"policy.schedule.alertOnFailure": true
```

#### 9. **Workflow triggering**

```json
"policy.schedule.workflow.allowed": true,
"policy.schedule.workflow.allowedWorkflows": ["daily-report", "cleanup-workflow"],
"policy.schedule.workflow.maxTriggersPerDay": 100
```

#### 10. **Dynamic scheduling**

```json
"policy.schedule.dynamic.allowed": true,
"policy.schedule.dynamic.requireApproval": false,
"policy.schedule.dynamic.condition": "request.auth.claims.role in ['admin', 'scheduler']"
```

---

## Execution Model

When a scheduled method is triggered:

1. The runtime constructs a service call with the configured input
2. The call is executed as if it came from an internal system caller
3. The method has access to all the same Host APIs as a regular call
4. Results are logged but not returned (fire-and-forget)
5. Failures are retried based on service configuration

---

## Notes for Codegen / Shim Targets

- Scheduled method calls should appear as regular service invocations
- Support both static (config) and dynamic (runtime) scheduling
- Validate that scheduled methods exist in the service IDL at deploy time
- Handle timezone conversions properly
- Persist schedules across service restarts when configured
- Provide metrics on schedule execution (success/failure rates)
- Respect @scheduleOnly decorator - these methods cannot be called via RPC
- Generate appropriate types for schedule input validation

---

This specification enables:

- Periodic data processing and cleanup
- Report generation on schedules
- Cache warming and refresh
- Batch job processing
- Maintenance operations
- Workflow orchestration via scheduled triggers
- Time-based business logic execution