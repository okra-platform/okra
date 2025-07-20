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
3. **Seamless DX**: Users only interact with OKRA CLI and models.okra.gql files
4. **OKRA Integration**: Natural fit with existing OKRA tooling and conventions
5. **Multi-Database Support**: PostgreSQL, MySQL, SQLite through Atlas library
6. **Schema-Driven Workflow**: Always generate migrations from schema changes, never write SQL manually

## Non-Goals

1. **ORM Functionality**: This is purely for schema definition and migrations
2. **Query Building**: Services handle their own database queries
3. **Direct Atlas Exposure**: Users should not need to install Atlas CLI or understand HCL

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
│   Schema AST        │  ← Internal representation
│  (SchemaDefinition) │
└──────────┬──────────┘
           │
           │ Convert to Atlas schema objects
           ▼
┌─────────────────────┐
│  Atlas Schema API   │  ← In-memory Atlas schemas
│  (schema.Schema)    │     No HCL files created
└──────────┬──────────┘
           │
           │ Atlas library generates SQL
           ▼
┌─────────────────────┐
│   SQL Migrations    │  ← Applied to database
│                     │     SQL files optional
└─────────────────────┘
```

The system uses Atlas as a Go library to handle schema diffing and SQL generation. All Atlas operations happen in-memory - no HCL files are written to disk. Users only see and interact with their `models.okra.gql` file and the OKRA CLI.

### Configuration

The system uses a simplified configuration approach:

```go
// internal/migrations/config.go
type DatabaseConfig struct {
    Provider    string
    URL         string
    GenerateSQL bool   // false = development mode (no SQL files)
                      // true = enterprise mode (SQL files for review)
    Table       string // defaults to "_okra_migrations"
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

// Public interface following OKRA conventions
type SchemaParser interface {
    ParseModelsFile(path string) (*SchemaDefinition, error)
}

// Public constructor
func NewSchemaParser() SchemaParser {
    return &schemaParser{}
}

// Private implementation
type schemaParser struct{}

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

func (p *schemaParser) ParseModelsFile(path string) (*SchemaDefinition, error) {
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

// Public interface
type SchemaConverter interface {
    ToAtlasSchema(okraSchema *models.SchemaDefinition) (*schema.Schema, error)
}

// Public constructor
func NewSchemaConverter(dialect string) SchemaConverter {
    return &schemaConverter{dialect: dialect}
}

// Private implementation
type schemaConverter struct {
    dialect string // postgres, mysql, sqlite
}

func (c *schemaConverter) ToAtlasSchema(okraSchema *models.SchemaDefinition) (*schema.Schema, error) {
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

// Public interface
type SchemaStateManager interface {
    GetCurrentSchema() (*models.SchemaDefinition, error)
    SaveMigrationWithSchema(migration string, schema *models.SchemaDefinition, timestamp string, generateSQL bool) error
    GetSchemaForMigration(version string) (*models.SchemaDefinition, error)
    Close() error
}

// Public constructor
func NewSchemaStateManager(migrationsDir string) SchemaStateManager {
    return &schemaStateManager{
        migrationsDir: migrationsDir,
    }
}

// Private implementation
type schemaStateManager struct {
    migrationsDir string
}

func (m *schemaStateManager) GetCurrentSchema() (*models.SchemaDefinition, error) {
    // Read from migrations/current/schema.json
}

func (m *schemaStateManager) SaveMigrationWithSchema(
    migration string, 
    schema *models.SchemaDefinition,
    timestamp string,
    generateSQL bool,
) error {
    // Always save to migrations/{timestamp}/schema.json
    // Optionally save SQL to migrations/{timestamp}/migration.sql if generateSQL is true
    // Update migrations/current/schema.json
}

func (m *schemaStateManager) GetSchemaForMigration(version string) (*models.SchemaDefinition, error) {
    // Read from migrations/{version}/schema.json
}

func (m *schemaStateManager) Close() error {
    // Cleanup resources if any
    return nil
}
```

### CLI Commands

New commands under `okra db:*`:

```bash
# Development Commands
# ===================
# Sync database to match models.okra.gql (no migration files)
okra db:sync

# Watch mode - auto-sync on schema changes (used by okra dev)
okra db:sync --watch

# Reset database to match schema
okra db:reset

# Migration Commands
# ==================
# Create migration from all changes since last snapshot
okra db:migrate:snapshot

# Apply pending migrations
okra db:migrate:up

# Rollback last migration
okra db:migrate:down

# Show migration status
okra db:migrate:status

# Utility Commands
# ================
# Preview changes since last snapshot
okra db:diff

# Initialize migrations directory
okra db:init

# Validate schema syntax
okra db:validate

# Seed database from seed files
okra db:seed

# Future: Preview SQL without generating files
# okra db:migrate:preview
```

### Development Workflow

The new workflow separates rapid development from migration generation:

1. **Development Phase**: Use `okra db:sync` or auto-sync during `okra dev`
2. **Commit Phase**: Use `okra db:migrate:snapshot` to create a clean migration

### Schema State Tracking

The system tracks three key schema states for accurate migration generation:

```
migrations/
├── snapshot/
│   └── schema.json          # Last snapshot state (for migration diffing)
├── current/
│   └── schema.json          # Current synced state (what's in the database)
├── 20240120_143022/
│   ├── schema.json          # Schema at time of migration
│   └── migration.sql        # Optional: only if generateSQL is true
└── 20240121_091545/
    ├── schema.json
    └── migration.sql
```

**Development Sync Process (`okra db:sync`):**
1. Parse `models.okra.gql` → desired schema state
2. Apply changes directly to development database
3. Update `migrations/current/schema.json` with new state
4. No migration files created

**Migration Snapshot Process (`okra db:migrate:snapshot`):**
1. Check if `models.okra.gql` matches `migrations/current/schema.json`
2. If not, prompt user to sync first or proceed with untested changes
3. Compare `migrations/snapshot/schema.json` → `migrations/current/schema.json`
4. Generate migration for the net changes
5. Create `migrations/{timestamp}/` with schema and optional SQL
6. Update `migrations/snapshot/schema.json` to current state

This approach ensures:
- Fast local iteration without migration clutter
- Clean migration history (one per feature, not per experiment)
- Safety checks prevent untested migrations
- Always know what was last "committed" via snapshot

### Migration Workflow

The new workflow separates rapid development from migration generation:

#### Development Phase

```bash
# Option 1: Auto-sync during development
okra dev
# Automatically syncs database on schema changes

# Option 2: Manual sync
okra db:sync
# Manually sync database to match models.okra.gql
```

**What happens during sync:**
1. Parse `models.okra.gql`
2. Apply changes directly to development database
3. Update `migrations/current/schema.json`
4. No migration files created

#### Commit Phase

```bash
# Create a migration snapshot when ready to commit
okra db:migrate:snapshot

# What happens internally:
# 1. Verify models.okra.gql matches current/schema.json
# 2. If not, prompt to sync first (recommended) or proceed
# 3. Compare snapshot/schema.json → current/schema.json
# 4. Generate migration for net changes (not intermediate states)
# 5. Create migrations/{timestamp}/ with schema.json
# 6. Optionally create migration.sql (based on generateSQL config)
# 7. Update snapshot/schema.json
```

**Safety Check Example:**
```
⚠️  Schema mismatch detected!

Your models.okra.gql has changes that haven't been synced:
  + Added field 'phoneNumber' to User
  ~ Changed field 'email' from optional to required

What would you like to do?
  1. Sync database first, then create snapshot (recommended)
  2. Create snapshot from models.okra.gql anyway (untested changes)
  3. Cancel and sync manually

Choice [1]: _
```

This workflow ensures:
- Fast, friction-free local development
- Clean migration history
- Safety against untested changes
- Flexibility when needed

### Workflow Comparison

| Feature | Development (db:sync) | Production (db:migrate:snapshot) |
|---------|----------------------|----------------------------------|
| Migration files | No | Yes |
| Schema tracking | current/schema.json only | Full history with snapshots |
| Database changes | Immediate | Through migrations |
| Intermediate states | Not recorded | Collapsed into one migration |
| Safety checks | None | Prompts if schema unsynced |
| Best for | Local experimentation | Committing features |

### Example Development Flow

```bash
# Start developing with auto-sync
okra dev
# [OKRA] Auto-sync enabled, watching models.okra.gql

# Edit schema - add field
# [OKRA] Schema changed, syncing database...
# [OKRA] ✓ Database synced

# Edit schema - rename field  
# [OKRA] Schema changed, syncing database...
# [OKRA] ✓ Database synced

# Happy with changes? Check what changed
okra db:diff
# Changes since last snapshot:
#   + Added field 'phoneNumber' to User
#   - Removed field 'oldField' from User

# Create snapshot for commit
okra db:migrate:snapshot
# ✓ Schema is synced, creating snapshot...
# Generated migrations/20240120_143022/
#   Added field 'phoneNumber' to User
#   Removed field 'oldField' from User

# Commit and push
git add migrations/ models.okra.gql
git commit -m "Add phone number to users"
```

**Configuration for SQL Generation:**
```json
{
  "database": {
    "migrations": {
      "generateSQL": true  // Enable to generate SQL files in migrations
    }
  }
}
```

### Configuration

OKRA uses a streamlined configuration approach:

```json
{
  "database": {
    "provider": "postgres",
    "url": "${DATABASE_URL}",
    "migrations": {
      "generateSQL": false,  // false = no SQL files, true = generate SQL files
      "table": "_okra_migrations",
      "dir": "./migrations",
      "development": {
        "autoSync": true,        // Enable auto-sync during okra dev
        "syncDebounce": 500      // ms to wait after schema changes
      }
    },
    "schemas": [
      "./models.okra.gql"
    ]
  }
}
```

#### Multi-Environment Configuration

For projects needing different settings per environment:

```json
{
  "database": {
    "environments": {
      "local": {
        "provider": "sqlite",
        "url": "sqlite://local.db",
        "generateSQL": false  // Development mode
      },
      "test": {
        "provider": "sqlite", 
        "url": "sqlite://:memory:",
        "generateSQL": false
      },
      "production": {
        "provider": "postgres",
        "url": "${DATABASE_URL}",
        "generateSQL": true  // Enterprise mode for production
      }
    },
    "migrations": {
      "table": "_okra_migrations",
      "dir": "./migrations"
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
   # Generates migrations based on current environment
   OKRA_ENV=production okra db:migrate:generate
   
   # Creates (with timestamp):
   # migrations/
   # ├── 20240120_143022/
   # │   ├── schema.json      # Always generated
   # │   └── migration.sql    # Only if generateSQL: true
   # └── current/
   #     └── schema.json      # Current state
   ```
   
   All environments use the same migration structure for consistency.

2. **Migration Application**:
   ```bash
   # Uses environment to determine database and settings
   OKRA_ENV=local okra db:migrate:up       # SQLite, no SQL files
   OKRA_ENV=production okra db:migrate:up  # PostgreSQL, with SQL files
   
   # During development
   okra dev  # Automatically uses 'local' environment
   ```

3. **Testing Strategy**:
   - Unit tests: Use SQLite in-memory (`:memory:`) for speed
   - Integration tests: Match production database type
   - CI/CD: Use enterprise mode for SQL validation

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

### Why Atlas?

OKRA uses Atlas as a Go library to provide robust, battle-tested migration capabilities. This is an implementation detail that users don't need to know about - they only interact with OKRA CLI and their `models.okra.gql` files.

### Library Integration Benefits

1. **No External Dependencies**: Users don't need to install Atlas CLI
2. **In-Memory Operations**: All schema conversion happens in memory, no temporary files
3. **Better Error Handling**: Direct access to Atlas errors and types
4. **Performance**: No process overhead for operations
5. **Future Flexibility**: Enables auto-migration capabilities

### Schema Diffing Process

The migration engine uses Atlas's Go API directly:

```go
// internal/migrations/engine.go
import (
    "ariga.io/atlas/sql/schema"
    "ariga.io/atlas/sql/migrate"
)

// Public interface
type MigrationEngine interface {
    GenerateMigration(current, desired *models.SchemaDefinition) (string, error)
    ApplyMigration(ctx context.Context, sql string) error
    Shutdown() error
}

// Public constructor
func NewMigrationEngine(driver migrate.Driver, devConn schema.ExecQuerier) MigrationEngine {
    return &migrationEngine{
        driver:  driver,
        devConn: devConn,
    }
}

// Private implementation
type migrationEngine struct {
    driver  migrate.Driver
    devConn schema.ExecQuerier
}

func (e *migrationEngine) GenerateMigration(current, desired *models.SchemaDefinition) (string, error) {
    // 1. Convert OKRA schemas to Atlas schemas (in-memory)
    converter := NewSchemaConverter(e.driver.Dialect())
    currentSchema := converter.ToAtlasSchema(current)
    desiredSchema := converter.ToAtlasSchema(desired)
    
    // 2. Use Atlas to diff schemas
    changes, err := e.driver.SchemaDiff(currentSchema, desiredSchema)
    if err != nil {
        return "", err
    }
    
    // 3. Generate SQL directly (no HCL involved)
    return e.driver.GenerateSQL(changes)
}
```

This approach:
- Converts OKRA schema → Atlas schema objects directly
- Never generates HCL files
- Produces pure SQL output
- All operations happen in memory

### Development Database Management

The migration engine uses a development database for validation. This is managed transparently:

```go
// internal/migrations/devdb.go
func NewDevConnection(provider string) (schema.ExecQuerier, error) {
    switch provider {
    case "sqlite":
        // In-memory SQLite for fast validation
        return sql.Open("sqlite3", ":memory:")
    
    case "postgres":
        // Use local instance or testcontainers
        return postgres.Open(getDevPostgresURL())
        
    case "mysql":
        return mysql.Open(getDevMySQLURL())
    }
}
```

Users never interact with this - it's purely an internal validation mechanism.

### Runtime Migration Execution

The system supports both explicit and future auto-migration capabilities:

```go
func (e *migrationEngine) ApplyMigration(ctx context.Context, sql string) error {
    // Apply the generated SQL to the target database
    return e.driver.ApplySQL(ctx, sql)
}

// Future enhancement: auto-migration on service startup
func (e *migrationEngine) AutoMigrate(ctx context.Context) error {
    // This would enable Prisma-like auto-migration
    // Currently migrations are always explicit via CLI
    return fmt.Errorf("auto-migration not yet implemented")
}
```

The current design focuses on explicit migrations via CLI commands, maintaining full control over when database changes are applied.

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

### Phase 6: Enhanced Developer Experience
1. Colorized diff output for `db:diff` command
2. Add `db:migrate:preview` command to show SQL without generating files
3. Syntax highlighting for schema changes
4. Better visualization of migration complexity

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

### Core Go Libraries
- `ariga.io/atlas` - Migration engine (used as library, not CLI)
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

1. **Two-Phase Development Workflow**:
   - **Development Phase**: `okra db:sync` for rapid iteration without migrations
   - **Commit Phase**: `okra db:migrate:snapshot` for clean, production-ready migrations
   - **Benefits**: Fast experimentation, clean history, one migration per feature

2. **Three-State Schema Tracking**:
   - **snapshot/**: Last committed schema state
   - **current/**: Current synced database state  
   - **{timestamp}/**: Individual migration states
   - **Benefits**: Accurate diffing, safety checks, clear state management

3. **Atlas as Implementation Detail**: 
   - Users only interact with OKRA CLI and `models.okra.gql`
   - Atlas library provides robust migration engine
   - All operations happen in-memory (no HCL files)
   - No external Atlas CLI dependency

4. **Schema-Driven Workflow**: No manual SQL writing:
   - **Single Source of Truth**: models.okra.gql defines the desired state
   - **Automatic Sync**: `okra dev` watches and syncs automatically
   - **Clean Snapshots**: Collapse all experiments into one migration
   - **Conflict Resolution**: Happens at schema level, not SQL level

5. **Safety First Design**:
   - **Sync Verification**: Snapshot command checks if schema is tested
   - **User Prompts**: Clear choices when schema is out of sync
   - **Escape Hatches**: Can proceed with untested changes if needed
   - **Diff Command**: Preview changes before creating snapshots

6. **Configuration Simplicity**:
   - `generateSQL`: Control SQL file generation
   - `autoSync`: Enable/disable auto-sync during development
   - Same core workflow regardless of settings

7. **Future Enhancements**: Design enables future capabilities:
   - Custom SQL migrations alongside generated ones
   - Migration testing and validation framework
   - Cross-database compatibility checking
   - Advanced merge strategies for team collaboration

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
