# Database Migrations

OKRA provides a powerful, intuitive database migration system that combines the best of Prisma's developer experience with the robustness of traditional migration tools. Using a GraphQL-inspired syntax for schema definition and Atlas under the hood for migration execution, OKRA makes database management seamless.

## The Schema-Driven Approach

OKRA migrations are **always schema-driven**. You modify your `models.okra.gql` file, and OKRA figures out what database changes are needed. No manual SQL writing, no migration naming - just pure schema evolution.

**Traditional approach:**
```bash
# 1. Think about what SQL you need
# 2. Create migration: rails generate migration AddEmailToUsers
# 3. Write SQL or ORM code
# 4. Run migration
```

**OKRA approach:**
```bash
# 1. Update your schema in models.okra.gql
# 2. Run: okra db:migrate:generate
# Done! OKRA generates the SQL for you
```

Behind the scenes, OKRA tracks your schema history to generate accurate diffs - but you never need to manage this manually. The system is powered by a robust migration engine that handles all the complexity.

## Quick Start

### 1. Define Your Schema

Create a `models.okra.gql` file in your service directory:

```graphql
@okra(namespace: "myapp", version: "v1")

model User {
  id: ID! @default(uuid)
  email: String! @unique
  name: String!
  createdAt: DateTime! @default(now)
  updatedAt: DateTime! @updatedAt
  
  posts: [Post!]! @relation(name: "UserPosts")
}

model Post {
  id: ID! @default(uuid)
  title: String!
  content: String?
  published: Boolean! @default(false)
  
  author: User! @relation(name: "UserPosts", fields: [authorId], references: [id])
  authorId: ID!
  
  createdAt: DateTime! @default(now)
}
```

### 2. Understand the Two-Phase Workflow

OKRA separates rapid development from migration generation:

#### Phase 1: Development (Rapid Iteration)

```bash
# Option 1: Auto-sync with okra dev
okra dev
# Automatically syncs database as you edit models.okra.gql

# Option 2: Manual sync
okra db:sync
# Manually sync database to match your schema
```

During development:
- Edit `models.okra.gql` freely
- Database updates immediately
- No migration files created
- Experiment without commitment

#### Phase 2: Commit (Create Migration)

```bash
# Check what changed since last snapshot
okra db:diff

# Create a migration snapshot
okra db:migrate:snapshot
# Generates one clean migration from all your changes

# Apply migrations in production
okra db:migrate:up
```

The key benefit: Experiment freely, commit clean migrations!

## Schema Definition

### Basic Types

OKRA supports all common database types with intuitive names:

```graphql
model Product {
  id: ID!                    # UUID by default
  name: String!              # Required string
  description: String?       # Optional string
  price: Float!              # Decimal number
  quantity: Int!             # Integer
  inStock: Boolean!          # Boolean
  metadata: JSON?            # JSON data
  createdAt: DateTime!       # Timestamp
}
```

### Field Directives

Enhance fields with powerful directives:

```graphql
model User {
  id: ID! @default(uuid)           # Auto-generate UUIDs
  email: String! @unique           # Unique constraint
  sequence: Int! @default(autoincrement)
  createdAt: DateTime! @default(now)
  updatedAt: DateTime! @updatedAt  # Auto-update on changes
  
  # Override database type
  bio: String? @db(type: "TEXT")
}
```

### Relationships

Define relationships between models:

```graphql
# One-to-Many
model User {
  id: ID!
  posts: [Post!]! @relation(name: "UserPosts")
}

model Post {
  id: ID!
  author: User! @relation(name: "UserPosts", fields: [authorId], references: [id])
  authorId: ID!
}

# Many-to-Many (implicit join table)
model Post {
  id: ID!
  tags: [Tag!]! @relation(name: "PostTags")
}

model Tag {
  id: ID!
  posts: [Post!]! @relation(name: "PostTags")
}

# One-to-One
model User {
  id: ID!
  profile: Profile? @relation(name: "UserProfile")
}

model Profile {
  id: ID!
  user: User! @relation(name: "UserProfile", fields: [userId], references: [id])
  userId: ID! @unique
}
```

### Indexes

Add indexes for better query performance:

```graphql
# Single field index
model User {
  email: String! @unique @index
}

# Composite indexes
@index(model: "Post", fields: ["published", "createdAt"])
@index(model: "Post", fields: ["authorId", "published"])
```

## Configuration

### Simple Configuration

For single database setups:

```json
{
  "database": {
    "provider": "postgres",
    "url": "${DATABASE_URL}",
    "migrations": {
      "generateSQL": false,  // true = generate SQL files for review
      "dir": "./migrations",
      "development": {
        "autoSync": true,    // Auto-sync during okra dev
        "syncDebounce": 500  // Wait 500ms after changes
      }
    },
    "schemas": ["./models.okra.gql"]
  }
}
```

### Multi-Environment Configuration

For different settings per environment:

```json
{
  "database": {
    "environments": {
      "local": {
        "provider": "sqlite",
        "url": "sqlite://local.db",
        "generateSQL": false,
        "autoSync": true      // Enable for development
      },
      "test": {
        "provider": "sqlite",
        "url": "sqlite://:memory:",
        "generateSQL": false,
        "autoSync": false     // Disable for tests
      },
      "production": {
        "provider": "postgres",
        "url": "${DATABASE_URL}",
        "generateSQL": true,  // Generate SQL for review
        "autoSync": false     // Never auto-sync production!
      }
    },
    "migrations": {
      "dir": "./migrations"
    }
  }
}
```

## Understanding Migration Storage

OKRA tracks schema states to enable its two-phase workflow:

```
migrations/
├── snapshot/
│   └── schema.json      # Last committed schema (for diffing)
├── current/
│   └── schema.json      # Current synced state (what's in DB)
├── 20240120_143022/
│   ├── schema.json      # Schema at time of migration
│   └── migration.sql    # Optional: only if generateSQL is true
└── 20240121_091545/
    ├── schema.json
    └── migration.sql
```

**Key directories:**
- `snapshot/`: What was last committed (used for creating migrations)
- `current/`: What's currently synced to the database
- Timestamped folders: Individual migrations

You only edit `models.okra.gql`; these tracking files are managed automatically.

## The Two-Phase Workflow

### Phase 1: Development (db:sync)

Rapid iteration without migrations:

```bash
# Auto-sync during development
okra dev
# [OKRA] Auto-sync enabled, watching models.okra.gql

# Or manual sync
okra db:sync
```

**What happens:**
- Direct database updates
- No migration files
- Updates `current/schema.json`
- Perfect for experimenting

### Phase 2: Commit (db:migrate:snapshot)

Create clean migrations for deployment:

```bash
# See what changed
okra db:diff
# Changes since last snapshot:
#   + Added field 'phoneNumber' to User
#   - Removed field 'faxNumber' from User

# Create migration snapshot
okra db:migrate:snapshot
```

**Safety check if schema not synced:**
```
⚠️  Schema mismatch detected!

Your models.okra.gql has changes that haven't been synced:
  + Added field 'email' to User

What would you like to do?
  1. Sync database first, then create snapshot (recommended)
  2. Create snapshot anyway (untested changes)
  3. Cancel

Choice [1]: _
```

### Complete Example Flow

```bash
# 1. Start developing
okra dev

# 2. Make many schema changes
# - Add fields
# - Rename things
# - Delete others
# (Database auto-syncs each time)

# 3. Ready to commit? Create snapshot
okra db:migrate:snapshot
# Generated migrations/20240120_143022/
#   Net changes: +1 field, -1 field (not all the intermediate steps!)

# 4. Deploy to production
OKRA_ENV=production okra db:migrate:up
```

## Type Generation

OKRA automatically generates type-safe models for your service language.

### Automatic Generation

Types are generated automatically during:
- `okra dev` - Watches for schema changes
- `okra build` - Generates before compilation

### Generated Types

For a `User` model, OKRA generates:

**Go:**
```go
type User struct {
    ID        string `json:"id"`
    Email     string `json:"email"`
    Name      string `json:"name"`
    CreatedAt string `json:"createdAt"`
    UpdatedAt string `json:"updatedAt"`
}
```

**TypeScript:**
```typescript
export interface User {
    id: string;
    email: string;
    name: string;
    createdAt: string;
    updatedAt: string;
}
```

**Python:**
```python
@dataclass
class User:
    id: str
    email: str
    name: str
    created_at: str
    updated_at: str
```

## Cross-Database Support

OKRA helps you use SQLite for development and PostgreSQL for production:

### Type Mapping

OKRA automatically handles type differences:

| OKRA Type | PostgreSQL | SQLite |
|-----------|------------|--------|
| ID | UUID | TEXT |
| String | VARCHAR | TEXT |
| Int | INTEGER | INTEGER |
| Float | DOUBLE PRECISION | REAL |
| Boolean | BOOLEAN | INTEGER (0/1) |
| DateTime | TIMESTAMP | TEXT (ISO8601) |
| JSON | JSONB | TEXT |

### Compatibility Warnings

OKRA warns about database-specific features:

```bash
okra db:migrate:create --name add_json_field

# Output:
# ⚠ PostgreSQL: Using JSONB for optimal performance
# ⚠ SQLite: JSON stored as TEXT (limited operations)
```

## Best Practices

### 1. Start Simple

Begin with auto-sync for new projects:
```json
{
  "database": {
    "provider": "postgres",
    "migrations": { 
      "development": {
        "autoSync": true
      }
    }
  }
}
```

### 2. Use Environments

Different settings for different environments:
- Local: Auto-sync enabled, SQLite for speed
- CI: Auto-sync disabled, snapshot testing
- Production: Auto-sync disabled, SQL files enabled

### 3. Schema Organization

Keep models organized:
```
models/
├── models.okra.gql     # Main schema
├── auth.okra.gql       # Auth-related models
└── content.okra.gql    # Content models
```

### 4. Schema Changes

Make atomic, well-documented schema changes:
```graphql
# Good: Clear, focused changes
model User {
  # Added for email verification feature
  emailVerified: Boolean! @default(false)
  emailVerificationToken: String?
}

# Migrations are auto-generated with timestamps:
# 20240120_143022_add_email_verification.sql
```

### 5. Review Before Production

Always review migrations before applying to production:
```bash
# 1. During development (auto-sync handles updates)
model User {
  phoneNumber: String?     # Added
  faxNumber: String?       # Added then removed
  mobileNumber: String?    # Added then renamed to phoneNumber
}

# 2. Create snapshot when ready
okra db:migrate:snapshot

# Output: Generated migrations/20240120_143022/
#   Net change: Added phoneNumber field only
#   (All experiments collapsed into one clean migration)

# 3. If generateSQL is true, review the SQL
cat migrations/20240120_143022/migration.sql
# ALTER TABLE users ADD COLUMN phone_number VARCHAR(255);

# 4. Test in staging first
OKRA_ENV=staging okra db:migrate:up
```

## Common Patterns

### Soft Deletes

```graphql
model Post {
  id: ID!
  title: String!
  deletedAt: DateTime?
  
  # Add index for efficient queries
  @@index([deletedAt])
}
```

### Audit Fields

```graphql
model BaseModel {
  createdAt: DateTime! @default(now)
  updatedAt: DateTime! @updatedAt
  createdBy: String?
  updatedBy: String?
}

model User {
  # Inherits audit fields
  ...BaseModel
  
  email: String!
  name: String!
}
```

### Enums (Future)

```graphql
enum UserRole {
  ADMIN
  USER
  GUEST
}

model User {
  role: UserRole! @default(USER)
}
```

## Troubleshooting

### Understanding Snapshots

The snapshot workflow prevents migration clutter:

```bash
# Without snapshots (traditional):
migration_001_add_field.sql
migration_002_rename_field.sql  
migration_003_remove_old_field.sql
migration_004_add_another_field.sql

# With snapshots (OKRA):
migration_20240120_143022.sql  # All changes in one clean migration
```

### Schema Conflicts

When working in teams:
```bash
# Fetch latest changes
git pull

# Merge conflicts in models.okra.gql (easy!)
# Sync your local database
okra db:sync

# Create new snapshot with merged changes
okra db:migrate:snapshot
```

The beauty: Conflicts happen in the schema file (easy to resolve), not in SQL migrations!

### Database Connection Issues

Check your configuration:
```bash
# Test connection
okra db:validate

# Use explicit environment
OKRA_ENV=production okra db:migrate:status
```

### Schema Validation Errors

OKRA validates your schema before migrations:
```
Error: Circular dependency detected between User and Post
Error: Field 'email' marked as @unique but type is JSON
```

## CLI Reference

### Development Commands

```bash
okra db:sync              # Sync database to schema (no migrations)
okra db:sync --watch      # Watch mode for auto-sync
okra db:diff              # Show changes since last snapshot
okra db:reset             # Reset database to schema
okra db:validate          # Validate schema syntax
```

### Migration Commands

```bash
okra db:init              # Initialize migrations directory
okra db:migrate:snapshot  # Create migration from changes since last snapshot
okra db:migrate:up        # Apply migrations
okra db:migrate:down      # Rollback migration
okra db:migrate:status    # Show migration status
```

Note: `db:migrate:snapshot` creates a single migration from all changes since your last snapshot. It ensures changes have been tested by checking if the schema is synced.

### Utility Commands

```bash
okra db:seed             # Run seed files
okra models:generate     # Manually generate types
```

## Next Steps

1. **Define your schema** in `models.okra.gql`
2. **Configure your database** in `okra.config.json`  
3. **Start developing** with `okra dev` (auto-sync enabled)
4. **Make schema changes** freely - database syncs automatically
5. **Create snapshots** when ready to commit:
   - Run `okra db:diff` to preview changes
   - Run `okra db:migrate:snapshot` to create migration
   - Commit both schema and migration files
6. **Deploy to production**:
   - Run `okra db:migrate:up` in production
   - Never use auto-sync in production!
7. **Query your data** using your preferred SQL library or query builder