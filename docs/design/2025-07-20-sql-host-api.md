# SQL Host API Design

**Date:** 2025-07-20  
**Status:** Design Phase  
**Complexity:** High  
**Dependencies:** Host API Interface, Policy Engine, Database Infrastructure

## Executive Summary

The SQL Host API provides OKRA services with secure, policy-controlled access to SQL databases. It implements a three-tier access model: declarative query building for safe SELECT operations, structured mutations for INSERT/UPDATE/DELETE, and heavily restricted raw SQL for edge cases. The API enforces security through both code-level protections (parameterized queries, timeouts, limits) and configurable CEL policies (table/column restrictions, audit requirements).

## Problem Statement

OKRA services need to interact with SQL databases for:
- Querying application data with complex filters and joins
- Performing CRUD operations on business entities
- Running analytical queries for reporting
- Managing transactional data with consistency guarantees

Current challenges:
- Direct database access from WASM modules is not possible
- Services need protection against SQL injection and resource abuse
- Different environments require different access policies
- Large result sets need efficient streaming mechanisms
- Database operations must be observable and auditable

## Design Goals

1. **Security First**: Prevent SQL injection, enforce access controls, audit sensitive operations
2. **Developer Experience**: Intuitive query builders, clear error messages, type safety in guest languages
3. **Performance**: Efficient query execution, result streaming, connection pooling
4. **Flexibility**: Support common SQL patterns while maintaining safety
5. **Observability**: Comprehensive metrics, tracing, and audit logs

## Proposed Solution

### API Structure

The SQL Host API (`okra.sql`) provides three methods with increasing levels of power and restriction:

#### 1. `sql.query` - Declarative Query Builder

```typescript
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
}

// WHERE clauses with full logical operators
export type SqlCondition =
  | { and: SqlCondition[] }
  | { or: SqlCondition[] }
  | { not: SqlCondition }
  | SqlComparison;

interface SqlComparison {
  column: string;
  op: '=' | '!=' | '<' | '<=' | '>' | '>=' | 'in' | 'notIn' | 'like' | 'isNull' | 'exists';
  value?: any;
  subquery?: SqlQuery; // for in, notIn, exists - enables nested queries
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

#### 2. `sql.mutate` - Structured Mutations

```typescript
interface SqlMutation {
  table: string;
  action: 'insert' | 'update' | 'delete';
  id?: string | number; // for update/delete by primary key
  values?: Record<string, any>; // for insert/update
  where?: SqlCondition; // optional, required if id is not provided
  returning?: string[]; // optional columns to return
}
```

#### 3. `sql.raw` - Raw SQL Execution

```typescript
interface SqlRawRequest {
  sql: string;
  parameters?: any[];
}

interface SqlResult {
  rows: Record<string, any>[];
  rowCount: number;
}
```

### Implementation Architecture

```
┌─────────────────────┐
│   WASM Service      │
│  ┌───────────────┐  │
│  │ SQL Guest Stub│  │
│  └───────┬───────┘  │
└──────────┼──────────┘
           │ JSON Request
           ▼
┌─────────────────────┐
│   Host Runtime      │
│  ┌───────────────┐  │
│  │ HostAPISet    │  │
│  └───────┬───────┘  │
│          │          │
│  ┌───────▼───────┐  │
│  │ SQL Host API  │  │
│  └───────┬───────┘  │
└──────────┼──────────┘
           │
    ┌──────┴──────┐
    ▼             ▼
┌────────┐  ┌──────────┐
│Query   │  │Policy    │
│Builder │  │Engine    │
└────┬───┘  └────┬─────┘
     │           │
     ▼           ▼
┌─────────────────────┐
│  Database Driver    │
│  - Connection Pool  │
│  - Prepared Stmts   │
└──────────┬──────────┘
           │
           ▼
     ┌──────────┐
     │ Database │
     └──────────┘
```

### Key Components

#### 1. SQL Host API Factory
```go
package sql

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log/slog"
    
    "go.opentelemetry.io/otel/trace"
    
    "github.com/okra/internal/hostapi"
    "github.com/okra/internal/hostapi/spec"
)

type sqlHostAPIFactory struct{}

func NewSQLAPIFactory() hostapi.HostAPIFactory {
    return &sqlHostAPIFactory{}
}

func (f *sqlHostAPIFactory) Name() string {
    return "okra.sql"
}

func (f *sqlHostAPIFactory) Version() string {
    return "v1.0.0"
}

func (f *sqlHostAPIFactory) Create(config hostapi.HostAPIConfig) (hostapi.HostAPI, error) {
    // Parse database URL to determine driver
    dbURL, err := dburl.Parse(config.Get("databaseUrl"))
    if err != nil {
        return nil, fmt.Errorf("invalid database URL: %w", err)
    }
    
    // Initialize connection pool based on driver type
    var pool *sql.DB
    switch dbURL.Driver {
    case "postgres", "pgx":
        pool, err = pgx.Open(dbURL.String())
    case "mysql":
        pool, err = mysql.Open(dbURL.String())
    case "sqlite3":
        pool, err = sqlite.Open(dbURL.String())
    default:
        return nil, fmt.Errorf("unsupported database driver: %s", dbURL.Driver)
    }
    
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    
    // Configure pool settings
    pool.SetMaxOpenConns(config.GetInt("maxConnections", 10))
    pool.SetMaxIdleConns(config.GetInt("maxConnections", 10) / 2)
    pool.SetConnMaxLifetime(5 * time.Minute)
    
    return &sqlHostAPI{
        pool:         pool,
        driver:       dbURL.Driver,
        queryBuilder: newQueryBuilder(dbURL.Driver),
        policyEngine: config.PolicyEngine,
        config:       config,
        logger:       config.Logger,
        tracer:       config.Tracer,
    }, nil
}

func (f *sqlHostAPIFactory) Methods() []hostapi.MethodMetadata {
    return []hostapi.MethodMetadata{
        {
            Name:        "query",
            Description: "Execute a declarative SQL query",
            Parameters:  spec.Object(sqlQuerySchema),
            Response:    spec.Object(sqlResultSchema),
        },
        {
            Name:        "mutate",
            Description: "Execute a structured mutation",
            Parameters:  spec.Object(sqlMutationSchema),
            Response:    spec.Object(sqlResultSchema),
        },
        {
            Name:        "raw",
            Description: "Execute raw SQL (requires special permission)",
            Parameters:  spec.Object(sqlRawSchema),
            Response:    spec.Object(sqlResultSchema),
        },
    }
}
```

#### 2. SQL Host API Implementation
```go
type sqlHostAPI struct {
    name         string
    version      string
    pool         *sql.DB
    driver       string  // "postgres", "mysql", or "sqlite3"
    queryBuilder *queryBuilder
    policyEngine hostapi.PolicyEngine
    config       hostapi.HostAPIConfig
    logger       *slog.Logger
    tracer       trace.Tracer
}

func (s *sqlHostAPI) Name() string    { return s.name }
func (s *sqlHostAPI) Version() string { return s.version }

func (s *sqlHostAPI) Execute(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
    // Standard non-streaming execution
    switch method {
    case "query":
        return s.executeQuery(ctx, params)
    case "mutate":
        return s.executeMutation(ctx, params)
    case "raw":
        return s.executeRaw(ctx, params)
    default:
        return nil, &hostapi.HostAPIError{
            Code:    hostapi.ErrorCodeMethodNotFound,
            Message: fmt.Sprintf("Unknown method: %s", method),
        }
    }
}

func (s *sqlHostAPI) Close() error {
    if s.pool != nil {
        return s.pool.Close()
    }
    return nil
}
```

#### 3. Query Builder

The query builder will be implemented using [Squirrel](https://github.com/Masterminds/squirrel), a fluent SQL query builder for Go that provides:

- Type-safe query construction
- Automatic parameterization to prevent SQL injection
- Support for complex queries with joins, subqueries, and aggregates
- Database-agnostic query building

Example implementation:
```go
import sq "github.com/Masterminds/squirrel"

func (s *sqlHostAPI) buildSelectQuery(query SqlQuery) (string, []interface{}, error) {
    // Start with base query
    q := sq.Select(query.Columns...).From(query.Table)
    
    // Add WHERE conditions
    if query.Where != nil {
        predicate, err := s.buildPredicate(query.Where)
        if err != nil {
            return "", nil, err
        }
        q = q.Where(predicate)
    }
    
    // Add JOINs
    for _, join := range query.Join {
        joinCond := fmt.Sprintf("%s = %s", join.On.LocalColumn, join.On.ForeignColumn)
        switch join.Type {
        case "inner":
            q = q.InnerJoin(join.Table + " ON " + joinCond)
        case "left":
            q = q.LeftJoin(join.Table + " ON " + joinCond)
        case "right":
            q = q.RightJoin(join.Table + " ON " + joinCond)
        }
    }
    
    // Add GROUP BY, ORDER BY, LIMIT, OFFSET
    if len(query.GroupBy) > 0 {
        q = q.GroupBy(query.GroupBy...)
    }
    for _, order := range query.OrderBy {
        col := order.Column
        if order.Direction == "desc" {
            col += " DESC"
        }
        q = q.OrderBy(col)
    }
    if query.Limit > 0 {
        q = q.Limit(uint64(query.Limit))
    }
    if query.Offset > 0 {
        q = q.Offset(uint64(query.Offset))
    }
    
    return q.ToSql()
}
```

Key features:
- Converts JSON query structure to parameterized SQL
- Validates table/column names against schema
- Enforces query complexity limits
- Generates efficient SQL with proper indexing hints

#### 4. Mutation Handler

Also implemented using Squirrel for consistent SQL generation:

```go
func (s *sqlHostAPI) buildMutation(mutation SqlMutation) (string, []interface{}, error) {
    switch mutation.Action {
    case "insert":
        q := sq.Insert(mutation.Table)
        if len(mutation.Values) > 0 {
            cols := make([]string, 0, len(mutation.Values))
            vals := make([]interface{}, 0, len(mutation.Values))
            for col, val := range mutation.Values {
                cols = append(cols, col)
                vals = append(vals, val)
            }
            q = q.Columns(cols...).Values(vals...)
        }
        if len(mutation.Returning) > 0 {
            q = q.Suffix("RETURNING " + strings.Join(mutation.Returning, ", "))
        }
        return q.ToSql()
        
    case "update":
        q := sq.Update(mutation.Table)
        for col, val := range mutation.Values {
            q = q.Set(col, val)
        }
        if mutation.Where != nil {
            predicate, err := s.buildPredicate(mutation.Where)
            if err != nil {
                return "", nil, err
            }
            q = q.Where(predicate)
        } else if mutation.Id != nil {
            q = q.Where(sq.Eq{"id": mutation.Id})
        }
        if len(mutation.Returning) > 0 {
            q = q.Suffix("RETURNING " + strings.Join(mutation.Returning, ", "))
        }
        return q.ToSql()
        
    case "delete":
        q := sq.Delete(mutation.Table)
        if mutation.Where != nil {
            predicate, err := s.buildPredicate(mutation.Where)
            if err != nil {
                return "", nil, err
            }
            q = q.Where(predicate)
        } else if mutation.Id != nil {
            q = q.Where(sq.Eq{"id": mutation.Id})
        }
        if len(mutation.Returning) > 0 {
            q = q.Suffix("RETURNING " + strings.Join(mutation.Returning, ", "))
        }
        return q.ToSql()
    }
    
    return "", nil, fmt.Errorf("invalid mutation action: %s", mutation.Action)
}
```

Key features:
- Supports INSERT, UPDATE, DELETE operations
- Handles RETURNING clauses for updated data
- Manages transaction boundaries
- Services implement their own concurrency control patterns

#### 5. Result Streaming

For large result sets, the SQL Host API implements `StreamingHostAPI`:

```go
// sqlHostAPI implements both HostAPI and StreamingHostAPI
type sqlHostAPI struct {
    hostapi.HostAPI
    // ... other fields
}

func (s *sqlHostAPI) ExecuteStreaming(ctx context.Context, method string, params json.RawMessage) (*hostapi.StreamingResponse, error) {
    switch method {
    case "query":
        var query SqlQuery
        if err := json.Unmarshal(params, &query); err != nil {
            return nil, &hostapi.HostAPIError{
                Code:    hostapi.ErrorCodeInvalidRequest,
                Message: "Invalid query parameters",
            }
        }
        
        // For large queries, return an iterator
        if query.Limit > 1000 || query.Limit == 0 {
            iterator, err := s.createQueryIterator(ctx, query)
            if err != nil {
                return nil, err
            }
            
            // First batch of results
            firstBatch, hasMore := iterator.Next(100)
            
            return &hostapi.StreamingResponse{
                Response: hostapi.Response{
                    Data: map[string]interface{}{
                        "rows":     firstBatch,
                        "hasMore":  hasMore,
                        "iteratorId": iterator.ID(),
                    },
                },
                Iterator: iterator,
            }, nil
        }
        
        // Small queries use regular Execute
        return nil, s.Execute(ctx, method, params)
    default:
        // Mutations don't support streaming
        return nil, s.Execute(ctx, method, params)
    }
}
```

- Returns iterator for row-by-row processing
- Automatic cleanup on completion or timeout
- Memory-efficient cursor-based pagination
- Guest SDK handles iterator transparently

### Policy Enforcement

#### Code-Level Policies (Always Enforced)
1. **SQL Injection Prevention**: All queries use parameterized statements
2. **Query Timeouts**: 30-second hard limit, configurable soft limits
3. **Result Set Limits**: Maximum 1M rows, configurable per query
4. **Reserved Tables**: System tables blocked (pg_*, information_schema)
5. **Connection Limits**: Per-service connection pool limits

#### CEL-Based Policies (Configurable)
```yaml
sql_policies:
  - name: "production_restrictions"
    when: "request.environment == 'production'"
    rules:
      - "request.method != 'sql.raw'"  # No raw SQL in production
      - "request.table not in ['audit_logs', 'system_config']"
      
  - name: "column_restrictions"
    when: "request.table == 'users'"
    rules:
      - "'ssn' not in request.columns"  # PII protection
      - "request.limit <= 1000"  # Limit user queries
      
  - name: "audit_requirements"
    when: "request.table in ['payments', 'transactions']"
    metadata:
      audit: true
      reason: "Financial data access"
```

### Error Handling

All errors use the `HostAPIError` struct with predefined codes from `internal/hostapi/errors.go`:

```go
// Predefined error codes
const (
    ErrorCodeSQLSyntax         = "SQL_SYNTAX_ERROR"
    ErrorCodeSQLTimeout        = "SQL_TIMEOUT"
    ErrorCodeSQLConnection     = "SQL_CONNECTION_ERROR"
    ErrorCodeSQLConstraint     = "SQL_CONSTRAINT_VIOLATION"
)

// Example error handling
if err := validateQuery(query); err != nil {
    return nil, &hostapi.HostAPIError{
        Code:    ErrorCodeSQLSyntax,
        Message: fmt.Sprintf("Invalid query structure: %v", err),
        Details: map[string]interface{}{"query": query},
    }
}

// Policy violations use existing code
if denied := s.checkPolicy(ctx, query); denied {
    return nil, &hostapi.HostAPIError{
        Code:    hostapi.ErrorCodePolicyDenied,
        Message: "Query denied by policy",
        Details: map[string]interface{}{"table": query.Table},
    }
}
```

### Guest-Side Implementation

TypeScript example:
```typescript
import { sql } from '@okra/sdk';

// Query builder
const users = await sql.query({
  table: 'users',
  columns: ['id', 'name', 'email'],
  where: sql.and([
    sql.eq('status', 'active'),
    sql.gte('created_at', '2025-01-01')
  ]),
  orderBy: [{ column: 'created_at', desc: true }],
  limit: 100
});

// Streaming large results
const allOrders = await sql.query({
  table: 'orders',
  columns: ['*'],
  limit: 0  // No limit triggers streaming
});

// The SDK detects streaming response and returns AsyncIterator
for await (const order of allOrders) {
  processOrder(order);
}

// Behind the scenes, the SDK:
// 1. Receives initial response with iteratorId
// 2. Calls okra.next(iteratorId) to fetch more rows
// 3. Handles cleanup when iteration completes

// Mutations - concurrency control is service's responsibility
const updated = await sql.mutate({
  table: 'users',
  action: 'update',
  values: { 
    email: 'new@example.com',
    updated_at: new Date().toISOString()
  },
  where: sql.eq('id', userId),
  returning: ['id', 'email', 'updated_at']
});

// Services can implement their own concurrency patterns:
// - Version fields: where: sql.and([sql.eq('id', id), sql.eq('version', version)])
// - Timestamps: where: sql.and([sql.eq('id', id), sql.eq('updated_at', lastKnownUpdate)])
// - No checking: where: sql.eq('id', id)
```

### Configuration

In `okra.json`:
```json
{
  "hostApis": {
    "sql": {
      "enabled": true,
      "databaseUrl": "${SQL_DATABASE_URL}",
      "maxConnections": 10,
      "queryTimeout": "30s",
      "maxRows": 1000000,
      "allowedTables": ["users", "orders", "products"],
      "auditMode": "mutations"  // "all", "mutations", "none"
    }
  }
}
```

Database URL examples:
- PostgreSQL: `postgres://user:pass@localhost:5432/dbname?sslmode=disable`
- MySQL: `mysql://user:pass@tcp(localhost:3306)/dbname?parseTime=true`
- SQLite: `sqlite3://file:test.db?cache=shared&mode=rwc` or `sqlite3://:memory:`

The driver is automatically selected based on the URL scheme. SQLite is recommended only for local development due to its limitations with concurrent writes.

## Security Considerations

1. **Connection String Security**: Database URLs stored in secrets, never in config
2. **Prepared Statements**: All queries parameterized, no string concatenation
3. **Schema Validation**: Table/column names validated against whitelist
4. **Row-Level Security**: Optional integration with database RLS
5. **Audit Trail**: All operations logged with service identity

## Performance Considerations

1. **Connection Pooling**: Per-service pools with health checks
2. **Query Optimization**: Automatic EXPLAIN analysis in dev mode
3. **Result Streaming**: Cursor-based pagination for large sets
4. **Caching**: Optional query result caching (separate host API)
5. **Batch Operations**: Support for bulk inserts/updates

## Testing Strategy

1. **Unit Tests**: Query builder logic, SQL generation
2. **Integration Tests**: Against test database with fixtures
3. **Policy Tests**: CEL policy evaluation scenarios
4. **Performance Tests**: Large result sets, concurrent queries
5. **Security Tests**: SQL injection attempts, access control

## Migration Path

For services currently using direct database access:
1. Identify all SQL queries in service code
2. Convert to appropriate API tier (query/mutate/raw)
3. Define policies for table/column access
4. Test with production-like data volumes
5. Monitor performance and adjust limits

## Future Enhancements

1. **Transaction Support**: Multi-statement transactions
2. **Stored Procedures**: Safe procedure invocation
3. **Database Migrations**: Schema management integration
4. **Query Analytics**: Performance insights and optimization suggestions
5. **Multi-Database**: Support for PostgreSQL, MySQL, SQLite

## Alternatives Considered

1. **GraphQL-to-SQL**: Too complex, performance concerns
2. **ORM Integration**: Too opinionated, limits flexibility
3. **Direct Database Access**: Security and isolation concerns
4. **SQL Proxy**: Additional infrastructure complexity

## Success Metrics

1. **Security**: Zero SQL injection vulnerabilities
2. **Performance**: <10ms overhead vs direct queries
3. **Adoption**: 80% of database operations use query/mutate
4. **Developer Satisfaction**: Positive feedback on API ergonomics
5. **Operational**: <0.01% timeout rate, comprehensive audit logs

## Dependencies

The SQL Host API implementation will use the following Go libraries:

### Core Dependencies

1. **[Squirrel](https://github.com/Masterminds/squirrel)** - SQL query builder
   - Type-safe query construction
   - Prevents SQL injection through parameterization
   - Supports complex queries and subqueries

2. **[sqlx](https://github.com/jmoiron/sqlx)** - Extended database/sql functionality
   - Named query parameters
   - Struct scanning for result sets
   - Better connection pool management
   - Prepared statement caching

3. **Database Drivers** - Support for multiple databases
   - **[pgx/v5](https://github.com/jackc/pgx)** - PostgreSQL driver
     - High-performance native PostgreSQL driver
     - Support for PostgreSQL-specific features
     - Better connection pooling than standard lib/pq
     - Context cancellation support
   - **[go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)** - MySQL driver
     - Mature MySQL/MariaDB driver
     - Support for MySQL-specific features
     - Connection pooling and prepared statements
   - **[mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)** - SQLite driver
     - For local development environments
     - Zero-configuration database
     - File-based storage
     - CGO dependency (note for builds)

### Schema & Validation

4. **[go-playground/validator](https://github.com/go-playground/validator)** - Struct validation
   - Validate query parameters before execution
   - Custom validation rules for SQL identifiers
   - Table/column name validation

5. **[xo/dburl](https://github.com/xo/dburl)** - Database URL parsing
   - Parse connection strings safely
   - Support multiple database URL formats
   - Extract connection parameters

### Testing & Development

6. **[DATA-DOG/go-sqlmock](https://github.com/DATA-DOG/go-sqlmock)** - SQL mocking for tests
   - Mock database interactions in unit tests
   - Verify generated SQL queries
   - Test error conditions

7. **[golang-migrate/migrate](https://github.com/golang-migrate/migrate)** - Database migrations
   - Schema versioning for test databases
   - Development environment setup
   - Integration test fixtures

### Monitoring & Performance

8. **[SQLC](https://github.com/kyleconroy/sqlc)** (optional) - Compile-time SQL checking
   - Validate SQL at build time
   - Generate type-safe Go code from SQL
   - Could be used for internal queries

9. **Database-specific features**
   - Squirrel supports all three target databases
   - Database detection from connection string
   - Dialect-specific SQL generation where needed

### Observability & Metrics

10. **[sqlstats](https://github.com/dlmiddlecote/sqlstats)** - SQL database metrics
    - Export connection pool metrics
    - Monitor query performance
    - Integrate with OpenTelemetry

11. **[ocsql](https://github.com/opencensus-integrations/ocsql)** - OpenCensus SQL driver wrapper
    - Automatic query tracing
    - Query latency metrics
    - Connection pool monitoring

### Security & Rate Limiting

12. **[ulule/limiter](https://github.com/ulule/limiter)** - Rate limiting
    - Per-service query rate limits
    - Prevent resource exhaustion
    - Memory-efficient counting

13. **[securego/gosec](https://github.com/securego/gosec)** - Security linter (dev dependency)
    - Scan for SQL injection vulnerabilities
    - Ensure secure coding practices
    - CI/CD integration

### Example go.mod additions:
```go
require (
    // Core SQL functionality
    github.com/Masterminds/squirrel v1.5.4
    github.com/jmoiron/sqlx v1.3.5
    
    // Database drivers
    github.com/jackc/pgx/v5 v5.5.1              // PostgreSQL
    github.com/go-sql-driver/mysql v1.7.1       // MySQL
    github.com/mattn/go-sqlite3 v1.14.19        // SQLite (local dev)
    
    // Validation and parsing
    github.com/go-playground/validator/v10 v10.16.0
    github.com/xo/dburl v0.20.0
    
    // Observability
    github.com/dlmiddlecote/sqlstats v1.0.0
    contrib.go.opencensus.io/integrations/ocsql v0.1.7
    
    // Security and rate limiting
    github.com/ulule/limiter/v3 v3.11.2
)

require (
    // Test dependencies
    github.com/DATA-DOG/go-sqlmock v1.5.0
    github.com/golang-migrate/migrate/v4 v4.17.0
    
    // Development tools
    github.com/securego/gosec/v2 v2.18.2
)
```

## Implementation Checklist

- [ ] Implement SQL Host API factory and registration
  ```go
  // In internal/hostapi/init.go
  func InitializeHostAPIs(registry *hostapi.Registry) error {
      // ... other APIs
      if err := registry.Register(sql.NewSQLAPIFactory()); err != nil {
          return fmt.Errorf("failed to register SQL API: %w", err)
      }
      return nil
  }
  ```
- [ ] Build query builder with full SQL support
- [ ] Add mutation handler with transaction support
- [ ] Implement result streaming with iterators
- [ ] Create comprehensive test suite
- [ ] Write guest-side stubs (Go, TypeScript)
- [ ] Add policy templates and examples
- [ ] Create migration guide from direct SQL
- [ ] Set up performance benchmarks
- [ ] Document security best practices