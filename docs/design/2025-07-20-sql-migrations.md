# Design: SQL Migrations System for OKRA

## Overview

This document outlines the design for an IDL-driven database schema and migration system for OKRA. The system provides a Prisma-like developer experience using GraphQL syntax with OKRA-specific extensions, while leveraging Atlas for migration execution under the hood.

### Core Philosophy: Schema as Single Source of Truth

Unlike traditional migration systems where developers write individual migration files, OKRA takes a **schema-first approach**:

1. **Define desired state** in `models.okra.gql`
2. **System determines changes** by comparing with current state  
3. **Migrations generated automatically** with proper SQL for each database

This eliminates entire classes of problems:
- No manual SQL writing or debugging
- No migration naming decisions
- No out-of-sync schemas and migrations
- Conflicts happen in schema files (easy to merge), not SQL

The approach is inspired by Prisma but leverages Atlas's mature migration engine for robustness.

## Goals

1. **Intuitive Schema Definition**: Use familiar GraphQL syntax with extensions for database modeling
2. **Type-Safe Models**: Generate model definitions from schema for each service's language (Go structs, TypeScript interfaces, Python dataclasses, etc.)
3. **Seamless DX**: Hide implementation complexity (Atlas HCL) while providing powerful migration capabilities
4. **OKRA Integration**: Natural fit with existing OKRA tooling and conventions
5. **Multi-Database Support**: PostgreSQL, MySQL, SQLite through Atlas abstraction
6. **Schema-Driven Workflow**: Always generate migrations from schema changes, never write SQL manually

## Non-Goals

1. **ORM Functionality**: This is purely for schema definition and migrations
2. **Query Building**: Handled separately by the SQL Host API
3. **Direct Atlas Exposure**: Users should not need to know about Atlas or HCL

## Design

### Schema Definition Language

Database schemas are defined in `models.okra.gql` files using GraphQL syntax with OKRA extensions:

```graphql
# models.okra.gql
@okra(namespace: "auth.users", version: "v1")

model User {
  id: ID! @default(uuid)
  email: String! @unique
  name: String!
  passwordHash: String!
  
  createdAt: DateTime! @default(now)
  updatedAt: DateTime! @updatedAt
  
  posts: [Post!]! @relation(name: "UserPosts")
  profile: Profile? @relation(name: "UserProfile")
}

model Post {
  id: ID! @default(uuid)
  title: String!
  content: String?
  published: Boolean! @default(false)
  
  author: User! @relation(name: "UserPosts", fields: [authorId], references: [id])
  authorId: ID!
  
  tags: [Tag!]! @relation(name: "PostTags")
  
  createdAt: DateTime! @default(now)
  updatedAt: DateTime! @updatedAt
}

model Tag {
  id: ID! @default(uuid)
  name: String! @unique
  
  posts: [Post!]! @relation(name: "PostTags")
}

model Profile {
  id: ID! @default(uuid)
  bio: String?
  avatarUrl: String?
  
  user: User! @relation(name: "UserProfile", fields: [userId], references: [id])
  userId: ID! @unique
}

# Indexes
@index(model: "Post", fields: ["published", "createdAt"])
@index(model: "Post", fields: ["authorId"])
@index(model: "Tag", fields: ["name"])
```

### Schema Directives

#### Model-Level Directives
- `@table(name: "custom_name")`: Override table name (default: pluralized model name)
- `@database(name: "specific_db")`: Target specific database in multi-db setups

#### Field-Level Directives
- `@id`: Primary key (implicit for `id: ID!`)
- `@unique`: Unique constraint
- `@index`: Create index on field
- `@default(value)`: Default values (`uuid`, `now`, `autoincrement`, literals)
- `@updatedAt`: Auto-update timestamp
- `@db(type: "TEXT")`: Override database type
- `@relation(...)`: Define relationships

#### Top-Level Directives
- `@index(model: "Model", fields: [...])`: Composite indexes
- `@unique(model: "Model", fields: [...])`: Composite unique constraints

### Implementation Architecture

```
┌─────────────────────┐
│  models.okra.gql    │  ← Developer writes schema
└──────────┬──────────┘
           │
           │ Parse (reuse service IDL parser approach)
           ▼
┌─────────────────────┐
│   Go Structs AST    │  ← Internal representation
│  (SchemaDefinition) │
└──────────┬──────────┘
           │
           │ Generate
           ▼
┌─────────────────────┐
│   Atlas HCL File    │  ← Hidden in temp directory
│    (temp/*.hcl)     │
└──────────┬──────────┘
           │
           │ Atlas CLI
           ├─────────────────────────┐
           │                         │
           ▼                         ▼
┌─────────────────────┐   ┌─────────────────────┐
│  Declarative Mode   │   │   Versioned Mode    │
│                     │   │                     │
│  Direct DB Apply    │   │ Generate SQL Files  │
│  (no files)         │   │ (migrations/*.sql)  │
└─────────────────────┘   └─────────────────────┘
```

### Configuration Mode Detection

The system automatically detects the configuration mode:

```go
// internal/migrations/config.go
type DatabaseConfig struct {
    // Simple mode: single provider
    Provider string
    URL      string
    
    // Multi-environment mode
    Environments map[string]EnvironmentConfig
}

func (c *DatabaseConfig) IsMultiEnvironment() bool {
    return len(c.Environments) > 0
}

func (c *DatabaseConfig) GetProviders() []string {
    if c.IsMultiEnvironment() {
        providers := make(map[string]bool)
        for _, env := range c.Environments {
            providers[env.Provider] = true
        }
        return maps.Keys(providers)
    }
    return []string{c.Provider}
}
```

### Parser Implementation

Extend the existing OKRA schema parser to handle database models:

```go
// internal/schema/models/parser.go
package models

import (
    "github.com/okra-io/okra/internal/schema"
)

type SchemaDefinition struct {
    Namespace string
    Version   string
    Models    []Model
    Indexes   []Index
}

type Model struct {
    Name       string
    TableName  string // From @table directive or pluralized
    Fields     []Field
    Relations  []Relation
}

type Field struct {
    Name         string
    Type         FieldType
    Required     bool
    Unique       bool
    Default      *DefaultValue
    DbType       string // From @db directive
    IsPrimaryKey bool
    IsUpdatedAt  bool
}

type Relation struct {
    Name       string
    Type       RelationType // OneToOne, OneToMany, ManyToMany
    Model      string
    Fields     []string
    References []string
    OnDelete   string // CASCADE, SET_NULL, etc.
}

func ParseModelsFile(path string) (*SchemaDefinition, error) {
    // 1. Preprocess to handle @okra and model keywords
    // 2. Parse with graphql-go-tools
    // 3. Extract models, fields, relations
    // 4. Validate schema consistency
}
```

### Schema Conversion & State Manager

Convert OKRA schemas to Atlas schemas and manage schema snapshots:

```go
// internal/migrations/converter.go
package migrations

import (
    "ariga.io/atlas/sql/schema"
    "github.com/okra-io/okra/internal/schema/models"
)

type SchemaConverter struct {
    dialect string // postgres, mysql, sqlite
}

func (c *SchemaConverter) ToAtlasSchema(okraSchema *models.SchemaDefinition) (*schema.Schema, error) {
    s := &schema.Schema{
        Name: okraSchema.Namespace,
    }
    
    for _, model := range okraSchema.Models {
        table := &schema.Table{
            Name: c.pluralize(model.Name),
        }
        
        // Convert fields to columns
        for _, field := range model.Fields {
            col := &schema.Column{
                Name: c.toSnakeCase(field.Name),
                Type: c.mapFieldType(field.Type),
            }
            table.AddColumn(col)
        }
        
        s.AddTable(table)
    }
    
    return s, nil
}

// internal/migrations/state.go
type SchemaStateManager struct {
    migrationsDir string
}

func (m *SchemaStateManager) GetCurrentSchema() (*models.SchemaDefinition, error) {
    // Read from migrations/current/schema.json
}

func (m *SchemaStateManager) SaveMigrationWithSchema(
    migration string, 
    schema *models.SchemaDefinition,
    timestamp string,
) error {
    // Save to migrations/{timestamp}/
    // Update migrations/current/schema.json
}

func (m *SchemaStateManager) GetSchemaForMigration(version string) (*models.SchemaDefinition, error) {
    // Read from migrations/{version}/schema.json
}
```

### CLI Commands

New commands under `okra db:*`:

```bash
# Declarative Mode Commands
# ========================

# Sync database with current schema (no migration files)
okra db:sync

# Preview changes without applying (dry-run)
okra db:sync --dry-run

# Force sync (destructive changes allowed)
okra db:sync --force


# Versioned Mode Commands
# =======================

# Initialize migrations (create migrations directory)
okra db:init

# Generate migration from schema changes
okra db:migrate:generate

# Apply pending migrations
okra db:migrate:up

# Rollback last migration
okra db:migrate:down

# Show migration status
okra db:migrate:status


# Common Commands (Both Modes)
# ============================

# Reset database (drop all, re-create from schema)
okra db:reset

# Seed database from seed files
okra db:seed

# Validate schema across all configured databases
okra db:validate
```

### Schema State Tracking

A critical aspect of schema-driven migrations is tracking the schema state to enable proper diffing. Atlas needs to know both the current state and the desired state to generate accurate migrations.

#### Versioned Mode State Tracking

For versioned migrations, we store schema snapshots alongside each migration:

```
migrations/
├── 20240120_143022_initial/
│   ├── migration.sql         # The actual SQL migration
│   └── schema.json          # Snapshot of models.okra.gql at this point
├── 20240121_091545_add_users/
│   ├── migration.sql
│   └── schema.json
└── current/
    └── schema.json          # Current schema state (latest applied)
```

**Migration Generation Process:**
1. Parse current `models.okra.gql` → new schema state
2. Read `migrations/current/schema.json` → previous schema state
3. Generate HCL for both states
4. Use Atlas to diff HCL files → SQL migration
5. Store new migration with its schema snapshot
6. Update `current/schema.json` after successful apply

#### Declarative Mode State Tracking

For declarative mode, we have two approaches:

**Approach 1: Database Introspection (Default)**
- Atlas introspects current database state directly
- No schema snapshots needed
- Works well for simple schemas
- May miss some metadata not reflected in DB

**Approach 2: Shadow Schema Tracking**
- Store schema snapshots in `.okra/schema/` directory
- Track applied schema versions in a `_okra_schema_history` table
- More accurate for complex schemas with metadata

```sql
-- _okra_schema_history table
CREATE TABLE _okra_schema_history (
    version VARCHAR(255) PRIMARY KEY,
    schema_hash VARCHAR(64) NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    schema_snapshot TEXT NOT NULL
);
```

### Migration Workflow

The workflow adapts based on migration mode:

#### Declarative Mode (Runtime Generation)

Similar to Prisma's `db push`, ideal for rapid development:

```bash
# Sync database with schema (no migration files)
okra db:sync

# What happens internally:
# 1. Parse models.okra.gql
# 2. Generate Atlas HCL in temp directory  
# 3. Atlas compares HCL with current database state
# 4. Generate and apply migration plan directly
# 5. No migration files created
```

**Benefits:**
- ✅ Zero friction development
- ✅ Always in sync with schema
- ✅ No migration conflicts
- ✅ Perfect for prototyping

**Trade-offs:**
- ❌ No migration history
- ❌ Can't review changes before applying
- ❌ Not suitable for production

#### Versioned Mode (Explicit Migrations)

Schema-driven migration approach with explicit files, ideal for production:

```bash
# Generate migrations from schema changes
okra db:migrate:generate

# What happens internally:
# 1. Parse models.okra.gql → new schema state
# 2. Load migrations/current/schema.json → previous state
# 3. Generate Atlas HCL for both states
# 4. Atlas diffs HCL files → SQL migration
# 5. Save migration + schema snapshot

# Apply migrations
okra db:migrate:up

# Rollback
okra db:migrate:down
```

**Benefits:**
- ✅ Full migration history with schema snapshots
- ✅ Code review for database changes
- ✅ Explicit rollback support
- ✅ Team collaboration friendly
- ✅ Accurate diffing based on schema, not DB state

**Trade-offs:**
- ❌ More steps in development
- ❌ Requires schema snapshot management
- ❌ Larger repository size (includes JSON snapshots)

#### Hybrid Workflow

Use different modes for different environments:

```bash
# Local development (declarative)
OKRA_ENV=local okra db:sync  # Fast, no files

# Before committing (switch to versioned)
OKRA_ENV=development okra db:migrate:generate

# CI/Production (versioned)
OKRA_ENV=production okra db:migrate:up
```

### Mode Comparison

| Feature | Declarative | Versioned |
|---------|-------------|-----------|
| Migration files | No | Yes |
| Development speed | Very fast | Moderate |
| Production ready | No | Yes |
| Team collaboration | Limited | Excellent |
| Rollback support | No | Yes |
| Code review | No | Yes |
| Conflict resolution | Automatic | Manual |
| Best for | Prototyping, Solo dev | Production, Teams |

### Transitioning Between Modes

OKRA makes it easy to transition from declarative to versioned mode as your project matures:

```bash
# Start with declarative mode during prototyping
okra db:sync  # Quick iterations

# When ready to stabilize, generate initial migration
okra db:migrate:init-from-schema

# This creates:
# migrations/
# ├── 20240120_143022_initial/
# │   ├── migration.sql    # Full schema as SQL
# │   └── schema.json      # Current models.okra.gql snapshot
# └── current/
#     └── schema.json      # Same as above, for next diff

# Continue with versioned migrations
okra db:migrate:generate  # Will diff against current/schema.json
```

The transition command captures both the current database state (as SQL) and the current schema definition (as JSON), providing a clean starting point for versioned migrations.

**Transition Guidelines:**
- Start with declarative for new projects
- Switch to versioned before first deployment
- Use versioned for any multi-developer project
- Consider hybrid approach for different environments

### Configuration

OKRA supports flexible database configurations to match different development workflows:

#### Option 1: Declarative Mode (Runtime Generation)

Best for early development, prototyping, and solo projects:

```json
{
  "database": {
    "provider": "postgres",
    "url": "${DATABASE_URL}",
    "migrations": {
      "mode": "declarative",  // No migration files, runtime generation
      "table": "_okra_migrations"
    },
    "schemas": [
      "./models.okra.gql"
    ]
  }
}
```

#### Option 2: Versioned Mode (Explicit Migrations)

Best for production systems and team collaboration:

```json
{
  "database": {
    "provider": "postgres",
    "url": "${DATABASE_URL}",
    "migrations": {
      "mode": "versioned",  // Explicit migration files
      "dir": "./migrations",
      "table": "_okra_migrations"
    },
    "schemas": [
      "./models.okra.gql"
    ]
  }
}
```

#### Option 3: Multi-Environment with Mode Selection

Combines environment flexibility with migration mode choice:

```json
{
  "database": {
    "environments": {
      "local": {
        "provider": "sqlite",
        "url": "sqlite://local.db",
        "migrations": {
          "mode": "declarative"  // Fast iteration, no files
        }
      },
      "test": {
        "provider": "sqlite", 
        "url": "sqlite://:memory:",
        "migrations": {
          "mode": "declarative"
        }
      },
      "development": {
        "provider": "postgres",
        "url": "${DATABASE_URL}",
        "migrations": {
          "mode": "versioned",
          "dir": "./migrations/postgres"
        }
      },
      "production": {
        "provider": "postgres",
        "url": "${DATABASE_URL}",
        "migrations": {
          "mode": "versioned",  // Always explicit in production
          "dir": "./migrations/postgres"
        }
      }
    },
    "migrations": {
      "table": "_okra_migrations"
    },
    "schemas": [
      "./models.okra.gql"
    ]
  }
}
```

### Environment-Based Migration Strategy

When using multi-environment configuration:

1. **Migration Generation**:
   ```bash
   # Generates migrations for all configured providers
   okra db:migrate:generate
   
   # Creates (with timestamp):
   # migrations/
   # ├── postgres/
   # │   ├── 20240120_143022/
   # │   │   ├── migration.sql
   # │   │   └── schema.json
   # │   └── current/
   # │       └── schema.json
   # └── sqlite/
   #     ├── 20240120_143022/
   #     │   ├── migration.sql
   #     │   └── schema.json
   #     └── current/
   #         └── schema.json
   ```
   
   Each provider maintains its own schema history to handle provider-specific differences.

2. **Migration Application**:
   ```bash
   # Uses environment to determine which migrations to run
   OKRA_ENV=local okra db:migrate:up    # Uses SQLite migrations
   OKRA_ENV=production okra db:migrate:up # Uses PostgreSQL migrations
   
   # During development
   okra dev  # Automatically uses 'local' environment
   ```

3. **Testing Strategy**:
   - Unit tests: Use SQLite in-memory (`:memory:`) for speed
   - Integration tests: Optionally use PostgreSQL for accuracy
   - CI/CD: Can use either based on pipeline requirements

### Multi-Database Support

Support multiple databases in one project:

```graphql
# models/auth.okra.gql
@okra(namespace: "auth", database: "auth_db")

model User {
  # ...
}

# models/content.okra.gql  
@okra(namespace: "content", database: "content_db")

model Post {
  # ...
}
```

### Type Generation

Generate types from models for use in services. Since all service code runs in WASM containers and communicates via serialized JSON, generated structs use JSON tags:

```go
// Generated: internal/generated/models/user.go
package models

import "time"

type User struct {
    ID           string    `json:"id"`
    Email        string    `json:"email"`
    Name         string    `json:"name"`
    PasswordHash string    `json:"passwordHash"`
    CreatedAt    time.Time `json:"createdAt"`
    UpdatedAt    time.Time `json:"updatedAt"`
}
```

### Multi-Language Model Generation

Following OKRA's existing codegen patterns, model generation supports multiple languages through a common interface:

#### Generator Interface

```go
// internal/codegen/models/generator.go
package models

type Generator interface {
    Generate(schema *models.SchemaDefinition) ([]byte, error)
    Language() string
    FileExtension() string
}
```

#### Language Implementations

**Go Models Generator:**
```go
// internal/codegen/models/golang/generator.go
type GoGenerator struct {
    packageName string
}

func (g *GoGenerator) Generate(schema *models.SchemaDefinition) ([]byte, error) {
    // Generate Go structs with JSON tags
    // Handle relationships as IDs (no ORM-style navigation)
    // Include validation tags if needed
}
```

**TypeScript Models Generator:**
```go
// internal/codegen/models/typescript/generator.go
type TypeScriptGenerator struct {
    moduleType string // "esm" or "commonjs"
}

func (g *TypeScriptGenerator) Generate(schema *models.SchemaDefinition) ([]byte, error) {
    // Generate TypeScript interfaces
    // Include JSDoc comments
    // Generate type guards for runtime validation
}
```

**Python Models Generator:**
```go
// internal/codegen/models/python/generator.go
type PythonGenerator struct {
    useDataclasses bool
}

func (g *PythonGenerator) Generate(schema *models.SchemaDefinition) ([]byte, error) {
    // Generate Python dataclasses or TypedDict
    // Include type hints
    // Generate Pydantic models if configured
}
```

#### Registration Pattern

```go
// internal/codegen/models/init.go
func init() {
    registry.Register("go", func(opts map[string]string) Generator {
        return golang.NewGenerator(opts["package"])
    })
    
    registry.Register("typescript", func(opts map[string]string) Generator {
        return typescript.NewGenerator(opts["module"])
    })
    
    registry.Register("python", func(opts map[string]string) Generator {
        return python.NewGenerator(opts["style"] == "dataclass")
    })
}
```

#### Generated Code Examples

**Go:**
```go
package models

type Post struct {
    ID        string   `json:"id"`
    Title     string   `json:"title"`
    Content   *string  `json:"content,omitempty"`
    Published bool     `json:"published"`
    AuthorID  string   `json:"authorId"`
    TagIDs    []string `json:"tagIds"`
    CreatedAt string   `json:"createdAt"`
    UpdatedAt string   `json:"updatedAt"`
}
```

**TypeScript:**
```typescript
export interface Post {
    id: string;
    title: string;
    content?: string;
    published: boolean;
    authorId: string;
    tagIds: string[];
    createdAt: string;
    updatedAt: string;
}

export function isPost(value: unknown): value is Post {
    // Type guard implementation
}
```

**Python:**
```python
from dataclasses import dataclass
from typing import Optional, List

@dataclass
class Post:
    id: str
    title: str
    content: Optional[str]
    published: bool
    author_id: str
    tag_ids: List[str]
    created_at: str
    updated_at: str
```

#### CLI Integration

Model generation is automatically integrated into the development and build process:

**Automatic Generation:**
- During `okra dev`: Watches `models.okra.gql` and regenerates types on changes
- During `okra build`: Generates types before compilation
- Service language is detected from `okra.service.json`

**Manual Generation:**
```bash
# Manually regenerate models (uses service's configured language)
okra models:generate

# Generate with custom output directory
okra models:generate --out ./src/models
```

Configuration in `okra.config.json`:
```json
{
  "models": {
    "generators": {
      "go": {
        "package": "models",
        "output": "./internal/generated/models"
      },
      "typescript": {
        "module": "esm",
        "output": "./src/generated/models"
      }
    }
  }
}
```

## Atlas Integration Details

### Library vs CLI Decision

OKRA embeds Atlas as a Go library rather than shelling out to the CLI for several reasons:

1. **Runtime Migration Support**: Enables future auto-migration during service deployment
2. **Better Error Handling**: Direct access to Atlas errors and types
3. **Performance**: No process overhead for each operation
4. **Deployment Simplicity**: No need to bundle Atlas CLI with OKRA
5. **API Stability**: Library API more stable than CLI interface
6. **Custom Integration**: Can extend Atlas behavior if needed

### Schema Diffing Process

OKRA uses Atlas's Go API for schema operations:

```go
// internal/migrations/atlas.go
import (
    "ariga.io/atlas/sql/schema"
    "ariga.io/atlas/sql/sqlite"
    "ariga.io/atlas/sql/postgres" 
    "ariga.io/atlas/sql/mysql"
    "ariga.io/atlas/sql/migrate"
)

type MigrationEngine struct {
    driver  migrate.Driver
    devConn schema.ExecQuerier
}

func (e *MigrationEngine) GenerateMigration(current, desired *models.SchemaDefinition) (*migrate.Plan, error) {
    // 1. Convert OKRA schemas to Atlas schemas
    currentSchema := e.toAtlasSchema(current)
    desiredSchema := e.toAtlasSchema(desired)
    
    // 2. Create change set
    changes, err := e.driver.SchemaDiff(currentSchema, desiredSchema)
    if err != nil {
        return nil, err
    }
    
    // 3. Plan migration with dev database validation
    plan, err := e.driver.PlanChanges(context.Background(), "migration", changes, 
        migrate.WithDevConnection(e.devConn))
    if err != nil {
        return nil, err
    }
    
    return plan, nil
}

func (e *MigrationEngine) toAtlasSchema(def *models.SchemaDefinition) *schema.Schema {
    // Convert OKRA schema to Atlas schema representation
    // This replaces HCL generation entirely
    converter := &SchemaConverter{dialect: e.driver.Dialect()}
    return converter.ToAtlasSchema(def)
}
```

### Development Database Management

Atlas requires a "dev database" for validating migrations. OKRA manages this programmatically:

```go
func NewDevConnection(provider string) (schema.ExecQuerier, error) {
    switch provider {
    case "sqlite":
        // In-memory SQLite for development
        return sql.Open("sqlite3", ":memory:")
    
    case "postgres":
        // Use testcontainers or dedicated dev instance
        return postgres.Open(getDevPostgresURL())
        
    case "mysql":
        return mysql.Open(getDevMySQLURL())
    }
}
```

This ensures migrations are validated against a real database engine without external dependencies.

### Runtime Migration Execution

The library integration enables future auto-migration support during service deployment:

```go
func (e *MigrationEngine) ApplyMigrations(ctx context.Context, target *sql.DB) error {
    // Read current schema state from database
    current, err := e.getCurrentSchemaVersion(target)
    if err != nil {
        return err
    }
    
    // Get desired schema from models.okra.gql
    desired, err := models.ParseModelsFile("models.okra.gql")
    if err != nil {
        return err
    }
    
    // Generate and apply migration if needed
    plan, err := e.GenerateMigration(current, desired)
    if err != nil {
        return err
    }
    
    if len(plan.Changes) > 0 {
        return e.driver.ApplyChanges(ctx, target, plan.Changes)
    }
    
    return nil
}
```

This enables OKRA services to optionally auto-migrate their databases on startup, similar to Prisma's approach but with more control.

## Implementation Plan

### Phase 1: Core Schema Parsing
1. Extend schema parser for `model` keyword
2. Parse field types and directives
3. Build internal AST representation
4. Add validation for schema consistency

### Phase 2: Atlas Library Integration
1. Implement OKRA to Atlas schema converter
2. Integrate Atlas Go library (not CLI)
3. Set up development database connections
4. Implement migration planning and execution
5. Add schema state management

### Phase 3: CLI Commands
1. Add `db:*` command structure
2. Implement development workflow
3. Add production migration commands
4. Handle configuration loading

### Phase 4: Multi-Language Model Generation
1. Create model generator interface following existing codegen patterns
2. Implement Go generator with JSON tags
3. Implement TypeScript generator with interfaces and type guards
4. Implement Python generator with dataclasses
5. Add registry and language selection logic
6. Integrate with CLI and build process

### Phase 5: Advanced Features
1. Multi-database support
2. Seed file handling
3. Migration rollback strategies
4. Schema validation commands

## Cross-Database Compatibility

When using multi-environment mode with different database providers:

### Type Mapping

OKRA automatically maps types appropriately for each database:

| OKRA Type | PostgreSQL | SQLite | MySQL |
|-----------|------------|--------|-------|
| ID | UUID | TEXT | VARCHAR(36) |
| String | VARCHAR/TEXT | TEXT | VARCHAR/TEXT |
| Int | INTEGER | INTEGER | INT |
| Float | DOUBLE PRECISION | REAL | DOUBLE |
| Boolean | BOOLEAN | INTEGER (0/1) | BOOLEAN |
| DateTime | TIMESTAMP | TEXT (ISO8601) | DATETIME |
| JSON | JSONB | TEXT | JSON |

### Feature Compatibility

The system warns about database-specific features:

```bash
# After modifying schema to add JSON field
okra db:migrate:generate

# Output:
# ⚠ Warning: PostgreSQL uses JSONB for optimal performance
# ⚠ Warning: SQLite stores JSON as TEXT (no native JSON operations)
# Generated: migrations/postgres/20240120_143022.sql
# Generated: migrations/sqlite/20240120_143022.sql
```

### Best Practices for Cross-Database Support

1. **Stick to common features**: Use standard SQL types when possible
2. **Test both databases**: Run test suite against both SQLite and PostgreSQL in CI
3. **Document limitations**: Note any SQLite limitations in comments
4. **Use OKRA's query builder**: The SQL Host API abstracts database differences

## Security Considerations

1. **SQL Injection**: All migrations go through Atlas, which handles parameterization
2. **Connection Security**: Support SSL/TLS database connections
3. **Credential Management**: Use environment variables, never store in schema files
4. **Migration Safety**: Destructive operations require confirmation in production

### Declarative Mode Safety

When using declarative mode, additional safety measures are implemented:

1. **Environment Restrictions**: Declarative mode is disabled by default in production environments
2. **Destructive Change Detection**: Warns before dropping columns/tables:
   ```
   ⚠️  Destructive changes detected:
   - Drop column: users.legacy_field
   - Drop table: old_logs
   
   Run with --force to apply destructive changes
   ```
3. **Dry Run by Default**: First-time sync requires explicit confirmation
4. **Schema Snapshots**: Automatic backup of current schema before changes

## Testing Strategy

1. **Unit Tests**: Parser, generator, and CLI command logic
2. **Integration Tests**: Full migration workflow with test databases
3. **Fixture Tests**: Example schemas and their expected outputs
4. **Multi-DB Tests**: Ensure consistency across database providers

## Dependencies

### Required Go Libraries
- `ariga.io/atlas` - Core migration engine (as library, not CLI)
- `ariga.io/atlas/sql/schema` - Schema representation
- `ariga.io/atlas/sql/migrate` - Migration planning and execution
- `ariga.io/atlas/sql/sqlite` - SQLite driver
- `ariga.io/atlas/sql/postgres` - PostgreSQL driver
- `ariga.io/atlas/sql/mysql` - MySQL driver
- `github.com/vektah/gqlparser/v2` - GraphQL parsing (already in use)
- `github.com/urfave/cli/v3` - CLI commands (already in use)

### Optional Libraries
- `github.com/golang-migrate/migrate/v4` - Alternative if Atlas integration issues
- `github.com/pressly/goose/v3` - Another migration alternative
- `github.com/lib/pq` - PostgreSQL driver
- `github.com/go-sql-driver/mysql` - MySQL driver
- `github.com/mattn/go-sqlite3` - SQLite driver

## Design Decisions

1. **Migration Storage**: Flexible based on mode:
   - **Declarative Mode**: No migration files, optional schema snapshots in `_okra_schema_history` table
   - **Versioned Mode**: 
     - Migration SQL files in `migrations/{timestamp}/migration.sql`
     - Schema snapshots in `migrations/{timestamp}/schema.json`
     - Current state in `migrations/current/schema.json`
   - Parsed schema as `models.okra.json` always included in `.okra.pkg` bundle

2. **Schema State Tracking**: Essential for accurate diffing:
   - **Why**: Atlas needs both current and desired states for proper diff generation
   - **Versioned Mode**: Store schema.json with each migration for history
   - **Declarative Mode**: Use DB introspection or optional shadow tracking
   - **Benefits**: Accurate diffs, migration history, easier debugging

3. **Schema-Driven Workflow**: No manual migration creation:
   - **Single Source of Truth**: models.okra.gql defines the desired state
   - **Automatic Generation**: okra db:migrate:generate handles all SQL creation
   - **No Migration Naming**: Timestamps provide unique, sortable identifiers
   - **Conflict Resolution**: Happens at schema level, not SQL level

4. **Service Data Isolation**: Services operate only on their own data:
   - No cross-service model sharing
   - Services requiring data from other services use service-to-service communication
   - Maintains clear service boundaries and ownership

5. **Runtime Schema Access**: Not implemented in initial version
   - Services don't need runtime access to schema definitions
   - Can be added later if use cases emerge

6. **Backward Compatibility**: Leverages Atlas capabilities with OKRA enhancements:
   - **Atlas Features**:
     - Schema diffing and validation
     - Dry-run mode to preview changes
     - Reversible migration detection
   - **OKRA Additions**:
     - Migration testing framework: `okra db:migrate:test`
     - Rollback simulation: `okra db:migrate:dry-run`
     - Schema compatibility checker for breaking changes
     - Development mode for iterative testing

## Schema-Driven Edge Cases

### Handling Data Migrations

Sometimes schema changes require data transformation:

```graphql
# Before: Single name field
model User {
  name: String!
}

# After: Split into first/last
model User {
  firstName: String!
  lastName: String!
}
```

Solution: Use migration hooks:
```bash
okra db:migrate:generate --with-hooks

# Generates:
# migrations/20240120_143022/
# ├── migration.sql      # Schema changes
# ├── schema.json        # New schema
# └── hooks.sql          # Data migration template
```

### Renaming Detection

Atlas may not detect renames automatically. OKRA provides hints:

```graphql
model User {
  # @renamed(from: "username")
  email: String! @unique
}
```

### Migration Squashing

For long-lived projects, squash old migrations:

```bash
okra db:migrate:squash --before 2023-01-01

# Creates new initial migration combining all previous ones
# Preserves schema history for accurate diffs
```

## Future Enhancements

1. **Schema Versioning**: Track schema versions for compatibility
2. **Auto-Migration**: Optionally run migrations on service startup
3. **Service Schema Integration**: Generate service.okra.gql types from database models (one-time helper)
4. **Validation Rules**: Add business logic validations to models
5. **Computed Fields**: Support for database-level computed columns
6. **Migration Hooks**: Pre/post migration scripts for data transformations
7. **Schema Linting**: Validate naming conventions and best practices
