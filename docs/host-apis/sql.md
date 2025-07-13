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

### `sql.query` Policies

```json
"policy.sql.query.allowedTables": ["users", "orders"]
"policy.sql.query.condition": "request.auth.claims.role == 'admin' || query.table != 'payments'"
"policy.sql.query.maxRows": 1000
"policy.sql.query.maxJoinDepth": 2
"policy.sql.query.allowSubqueries": false
"policy.sql.query.maxSubqueryDepth": 1
"policy.sql.query.maxAggregates": 3
"policy.sql.query.audit": true
```

### `sql.mutate` Policies

```json
"policy.sql.mutate.allowedTables": ["orders", "inventory"]
"policy.sql.mutate.restrictedColumns": ["users.password"]
"policy.sql.mutate.condition": "request.auth.claims.role == 'editor'"

"policy.sql.mutate.allowUpdateWithCondition": false
"policy.sql.mutate.allowDeleteWithCondition": false
"policy.sql.mutate.allowMultiRowUpdate": false
"policy.sql.mutate.allowMultiRowDelete": false
```

### `sql.raw` Policies (rare)

```json
"policy.sql.raw.allow": false
"policy.sql.raw.condition": "request.auth.claims.role == 'superuser'"
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

