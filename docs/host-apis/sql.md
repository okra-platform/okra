# Host API: `sql.query`, `sql.mutate`, `sql.raw`

The `sql.*` Host APIs expose structured database access to guest services via three tiers of capability:

- `sql.query`: Declarative, policy-safe, query-builder based interface for SELECT-like queries.
- `sql.mutate`: Structured mutation interface for INSERT, UPDATE, DELETE operations.
- `sql.raw`: Unsafe string-based SQL interface for advanced use cases (rarely granted).

---

## Interface Overview

```ts
interface SqlHostAPI {
  query(query: SqlQuery): Promise<SqlResult>;
  mutate(mutation: SqlMutation): Promise<SqlResult>;
  raw(sql: string, parameters?: any[]): Promise<SqlResult>;
}

interface SqlResult {
  rows: Record<string, any>[];
  rowCount: number;
}
```

---

## Query Builder Format (`sql.query`)

```ts
interface SqlQuery {
  table: string;
  columns?: string[]; // Default: ['*']
  where?: SqlCondition;
  join?: SqlJoin[];
  orderBy?: SqlOrderBy[];
  groupBy?: string[];
  limit?: number;
  offset?: number;
  aggregate?: SqlAggregate[];
  allowSubqueries?: boolean;
}

// WHERE clauses
export type SqlCondition =
  | { and: SqlCondition[] }
  | { or: SqlCondition[] }
  | { not: SqlCondition }
  | SqlComparison;

interface SqlComparison {
  column: string;
  op: '=' | '!=' | '<' | '<=' | '>' | '>=' | 'in' | 'notIn' | 'like' | 'isNull' | 'exists';
  value?: any;
  subquery?: SqlQuery; // for in, notIn, exists
}

// JOINs
interface SqlJoin {
  type: 'inner' | 'left' | 'right';
  table: string;
  on: {
    localColumn: string;
    foreignColumn: string;
  };
}

// ORDER BY
interface SqlOrderBy {
  column: string;
  direction?: 'asc' | 'desc';
}

// Aggregates
interface SqlAggregate {
  function: 'count' | 'sum' | 'avg' | 'min' | 'max';
  column: string;
  alias?: string;
}
```

---

## Mutate Format (`sql.mutate`)

```ts
interface SqlMutation {
  table: string;
  action: 'insert' | 'update' | 'delete';
  id?: string | number; // for update/delete by primary key
  values?: Record<string, any>; // for insert/update
  where?: SqlCondition; // optional, required if id is not provided
  returning?: string[]; // optional columns to return
}
```

---

## Host API Configuration

```ts
interface SqlHostAPIConfig {
  databaseUrl: string;
  allowRaw?: boolean; // usually false
  readonly?: boolean;
  maxQueryTimeMs?: number;
  logQueries?: boolean;
}
```

---

## Enforceable Okra Policies

OKRA uses a hybrid approach to policy enforcement, combining code-level security checks with flexible CEL-based policies.

### Code-Level Policies (Always Enforced)

These policies are implemented directly in the host API code for security and performance:

1. **SQL injection prevention** - All queries use parameterized statements
2. **Query timeout** - Hard limit of 30 seconds to prevent runaway queries
3. **Maximum query complexity** - Limits on nested subqueries (max depth: 5)
4. **Result set size** - Maximum 1M rows to prevent memory exhaustion
5. **Column/table name validation** - Only alphanumeric, underscore allowed
6. **Reserved table protection** - Cannot access system tables (pg_*, mysql.*, etc.)
7. **Query plan validation** - Reject queries with excessive cost estimates
8. **Connection pool limits** - Maximum connections per service
9. **Transaction safety** - Automatic rollback on errors

### CEL-Based Policies (Dynamic/Configurable)

These policies can be configured and updated at runtime:

#### `sql.query` Policies

```json
"policy.sql.query.allowedTables": ["users", "orders", "products"]
"policy.sql.query.blockedTables": ["audit_logs", "system_config"]
"policy.sql.query.condition": "request.auth.claims.role == 'admin' || !query.table.startsWith('admin_')"
"policy.sql.query.maxRows": 1000
"policy.sql.query.defaultLimit": 100
"policy.sql.query.maxJoinTables": 3
"policy.sql.query.allowSubqueries": true
"policy.sql.query.maxSubqueryDepth": 2
"policy.sql.query.maxAggregates": 5
"policy.sql.query.allowedAggregates": ["count", "sum", "avg"]
"policy.sql.query.columnRestrictions": {
  "users": ["password", "api_key", "totp_secret"]
}
"policy.sql.query.timeWindowRestriction": "created_at >= now() - interval '90 days'"
"policy.sql.query.audit": true
"policy.sql.query.auditLevel": "summary" // "none", "summary", "full"
```

#### `sql.mutate` Policies

```json
"policy.sql.mutate.allowedTables": ["orders", "inventory", "user_preferences"]
"policy.sql.mutate.blockedTables": ["users", "permissions", "audit_logs"]
"policy.sql.mutate.blockedColumns": {
  "users": ["password", "email", "role"],
  "orders": ["total_amount", "payment_status"]
}
"policy.sql.mutate.condition": "request.auth.claims.role in ['editor', 'admin']"
"policy.sql.mutate.requireIdForUpdate": true
"policy.sql.mutate.requireIdForDelete": true
"policy.sql.mutate.maxRowsPerMutation": 100
"policy.sql.mutate.allowBulkInsert": true
"policy.sql.mutate.maxBulkInsertRows": 1000
"policy.sql.mutate.allowTruncate": false
"policy.sql.mutate.validationRules": {
  "orders.status": "value in ['pending', 'processing', 'completed', 'cancelled']"
}
"policy.sql.mutate.audit": true
"policy.sql.mutate.requireAuditReason": true
```

#### `sql.raw` Policies (Rarely Granted)

```json
"policy.sql.raw.allow": false
"policy.sql.raw.condition": "request.auth.claims.role == 'dba' && env.DEPLOYMENT_ENV != 'production'"
"policy.sql.raw.allowedStatements": ["SELECT", "WITH"]
"policy.sql.raw.blockedKeywords": ["DROP", "TRUNCATE", "ALTER", "GRANT", "REVOKE"]
"policy.sql.raw.maxLength": 10000
"policy.sql.raw.timeout": 5000
"policy.sql.raw.audit": true
"policy.sql.raw.requireApproval": true
```

#### Environment-Specific Policies

```json
"policy.sql.query.condition": "env.DEPLOYMENT_ENV == 'production' ? query.table != 'debug_logs' : true"
"policy.sql.mutate.condition": "env.DEPLOYMENT_ENV == 'production' ? mutation.table != 'test_data' : true"
```

#### Rate Limiting

```json
"policy.sql.query.rateLimit": "1000/minute"
"policy.sql.mutate.rateLimit": "100/minute"
"policy.sql.raw.rateLimit": "10/hour"
```

---

## Notes for Codegen / Shim Targets

- Shims should generate strongly typed query/mutation builders.
- Raw SQL should be gated by clear warnings and policy checks.
- SQL conditions and joins should compile to safe, parameterized queries.
- Policy enforcement should occur before query execution.
- Results should be wrapped with row count + typed rows.
- Subqueries must be recursively validated and policy-checked.

---

This unified SQL API definition allows:
- Rich, structured access with visibility and security
- Clear separation between declarative (`query`, `mutate`) and raw modes
- Policy hooks for data governance
- Automatic instrumentation and tracing
- Support for future schema-aware code generation and IDE autocomplete

