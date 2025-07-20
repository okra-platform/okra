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
# Done! Atlas figures out the SQL for you
```

Behind the scenes, OKRA tracks your schema history to generate accurate diffs - but you never need to manage this manually.

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

### 2. Choose Your Workflow

OKRA supports three migration workflows to match your development style:

#### Declarative Mode (Fastest for Development)

Perfect for prototyping and early development:

```bash
# Sync your database with the schema - no migration files!
okra db:sync

# Preview changes before applying
okra db:sync --dry-run
```

#### Versioned Mode (Production Ready)

Schema-driven migration files for production systems:

```bash
# 1. Edit models.okra.gql (your single source of truth)
# 2. Generate migrations automatically
okra db:migrate:generate

# Apply migrations
okra db:migrate:up

# Check status
okra db:migrate:status
```

The key benefit: You only maintain the schema, not the migrations!

#### Hybrid Mode (Best of Both Worlds)

Use declarative locally, versioned in production:

```json
{
  "database": {
    "environments": {
      "local": {
        "provider": "sqlite",
        "url": "sqlite://local.db",
        "migrations": { "mode": "declarative" }
      },
      "production": {
        "provider": "postgres",
        "url": "${DATABASE_URL}",
        "migrations": { 
          "mode": "versioned",
          "dir": "./migrations"
        }
      }
    }
  }
}
```

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
      "mode": "versioned",
      "dir": "./migrations"
    },
    "schemas": ["./models.okra.gql"]
  }
}
```

### Multi-Environment Configuration

For different databases per environment:

```json
{
  "database": {
    "environments": {
      "local": {
        "provider": "sqlite",
        "url": "sqlite://local.db",
        "migrations": {
          "mode": "declarative"
        }
      },
      "test": {
        "provider": "sqlite",
        "url": "sqlite://:memory:",
        "migrations": {
          "mode": "declarative"
        }
      },
      "production": {
        "provider": "postgres",
        "url": "${DATABASE_URL}",
        "migrations": {
          "mode": "versioned",
          "dir": "./migrations/postgres"
        }
      }
    }
  }
}
```

## Understanding Migration Storage

When using versioned mode, OKRA creates a structured directory:

```
migrations/
├── 20240120_143022_initial/
│   ├── migration.sql    # The SQL to run
│   └── schema.json      # Schema snapshot at this point
├── 20240121_091545_add_users/
│   ├── migration.sql
│   └── schema.json
└── current/
    └── schema.json      # Latest schema state
```

The `schema.json` files enable accurate diffing - OKRA knows exactly what changed between versions. You only edit `models.okra.gql`; these files are generated automatically.

## Migration Modes

### Declarative Mode

Like Prisma's `db push`, perfect for rapid development:

**Pros:**
- Zero friction - just sync and go
- No migration conflicts
- Always matches your schema
- Perfect for prototyping

**Cons:**
- No migration history
- Can't review changes
- Not suitable for production

**Commands:**
```bash
okra db:sync              # Apply schema changes
okra db:sync --dry-run    # Preview changes
okra db:sync --force      # Allow destructive changes
```

### Versioned Mode

Traditional migration files for production:

**Pros:**
- Full migration history
- Code review friendly
- Explicit rollback support
- Team collaboration ready

**Cons:**
- More steps required
- Potential conflicts
- Requires management

**Commands:**
```bash
okra db:migrate:generate    # Generate from schema diff
okra db:migrate:up
okra db:migrate:down
okra db:migrate:status
```

### Transitioning Modes

Start with declarative, move to versioned when ready:

```bash
# During early development
okra db:sync  # Fast iterations

# Ready for production?
okra db:migrate:generate --initial

# Creates initial migration from current database state
# Subsequent changes: just run okra db:migrate:generate
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

Begin with declarative mode for new projects:
```json
{
  "database": {
    "provider": "postgres",
    "migrations": { "mode": "declarative" }
  }
}
```

### 2. Use Environments

Different modes for different environments:
- Local: Declarative with SQLite (fast)
- CI: Versioned with PostgreSQL (accurate)
- Production: Always versioned

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
# 1. Make schema changes in models.okra.gql
model User {
  # Add new field
  phoneNumber: String?
}

# 2. Generate migration from schema diff
okra db:migrate:generate

# Output: Generated migrations/20240120_143022.sql

# 3. Review what Atlas generated
cat migrations/20240120_143022.sql
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

### Schema Conflicts

When working in teams:
```bash
# Fetch latest changes including schema
git pull

# Merge any schema conflicts in models.okra.gql
# Then regenerate migrations
okra db:migrate:generate

# Atlas figures out the right migrations
# based on the merged schema
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

### Schema Commands

```bash
okra db:validate           # Validate schema syntax
okra db:sync              # Sync database (declarative)
okra db:sync --dry-run    # Preview changes
okra db:sync --force      # Force destructive changes
```

### Migration Commands

```bash
okra db:init              # Initialize migrations
okra db:migrate:generate  # Diff schema vs database, generate migrations
okra db:migrate:up        # Apply migrations
okra db:migrate:down      # Rollback migration
okra db:migrate:status    # Show migration status
okra db:migrate:validate  # Validate all migrations
```

Note: `db:migrate:generate` compares your `models.okra.gql` against the current database state and generates the necessary SQL to bring the database in sync with your schema.

### Utility Commands

```bash
okra db:reset            # Drop and recreate database
okra db:seed             # Run seed files
okra models:generate     # Manually generate types
```

## Next Steps

1. **Define your schema** in `models.okra.gql`
2. **Configure your database** in `okra.config.json`  
3. **Start with declarative mode** (`okra db:sync`) for quick development
4. **Switch to versioned** when ready for production:
   - Edit `models.okra.gql`
   - Run `okra db:migrate:generate`
   - Review and commit generated SQL
5. **Use the SQL Host API** to query your data

For more details on querying data, see the [SQL Host API documentation](./host-apis/sql.md).