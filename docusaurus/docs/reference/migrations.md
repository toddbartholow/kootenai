---
title: Database Migrations
description: Database migration history, patterns, and management
tags:
  - database
  - migrations
  - schema
---

# Database Migrations

This document covers the database migration system, history, and best practices for schema changes.

## Overview

Kootenai uses SQL migrations to manage database schema changes. Migrations are:

- **Sequential** - Applied in order by version number
- **Idempotent** - Safe to run multiple times
- **Versioned** - Tracked in `schema_migrations` table

## Migration Files

Location: `api/internal/database/migrations/`

```
migrations/
├── 001_initial_schema.sql
├── 002_add_sessions.sql
├── 003_add_achievements.sql
├── 004_schema_improvements.sql
├── 005_add_organizations.sql
├── 006_add_audit_log.sql
└── 007_schema_constraints.sql
```

## Migration History

### 001_initial_schema.sql

Initial database schema:

- `users` - User accounts
- `lab_templates` - Lab definitions
- `pods` - Pod instances
- `events` - Event log

### 002_add_sessions.sql

Session tracking:

- `lab_sessions` - Session records
- `checkpoint_progress` - Checkpoint tracking

### 003_add_achievements.sql

Gamification system:

- `achievements` - Achievement definitions
- `user_achievements` - Earned achievements

### 004_schema_improvements.sql

Performance and integrity:

- Added indexes for common queries
- Added foreign key constraints
- Added check constraints

### 005_add_organizations.sql

Multi-tenancy support:

- `organizations` - Tenant organizations
- `organization_members` - Membership
- `teams` - Team groupings
- `team_members` - Team membership

### 006_add_audit_log.sql

Audit logging:

- `audit_log` - Action audit trail

### 007_schema_constraints.sql

Data integrity:

- Additional constraints
- Partial indexes
- Trigger updates

### 039_classroom_simulation.sql

AI Classroom Simulation (Enterprise):

- `classroom_simulations` — Simulation runs with status, config, pathway reference
- `ai_students` — Per-student personality, traits, tech skills, behavioral config, runtime state
- `ai_student_activities` — Activity log (lab starts/completions, assignment submissions, discussion posts, quiz attempts)
- Indexes on `simulation_id` and `student_id` for activity queries

## Running Migrations

### Using Mage

```bash
# Run pending migrations
mage db:migrate

# Run on remote (infra VM)
mage db:migrateRemote

# Check migration status
mage db:status
```

### Manual

```bash
cd api
go run cmd/labctl/main.go db migrate
```

### Docker

```bash
docker compose exec api ./labctl db migrate
```

## Migration Status

Check which migrations have been applied:

```sql
SELECT * FROM schema_migrations ORDER BY version;
```

Output:

```
 version |         applied_at
---------+----------------------------
       1 | 2025-01-01 10:00:00.000000
       2 | 2025-01-01 10:00:01.000000
       3 | 2025-01-01 10:00:02.000000
       ...
```

## Writing Migrations

### Naming Convention

```
{version}_{description}.sql
```

- `version`: 3-digit zero-padded number
- `description`: snake_case description

Examples:
- `008_add_reservations.sql`
- `009_add_canvas_integration.sql`

### Migration Template

```sql
-- Migration: {version} - {description}
-- Applied: {date}

-- Up migration
BEGIN;

-- Add your schema changes here
CREATE TABLE example (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Add indexes
CREATE INDEX idx_example_name ON example(name);

-- Add constraints
ALTER TABLE example ADD CONSTRAINT chk_name_length
    CHECK (char_length(name) > 0);

COMMIT;

-- Down migration (comment for reference)
-- DROP TABLE example;
```

### Best Practices

#### Do

- Use transactions (`BEGIN`/`COMMIT`)
- Add indexes for foreign keys
- Include down migration as comments
- Test on a copy first
- Handle NULL values explicitly

#### Don't

- Delete columns in production (mark deprecated)
- Change column types without data migration
- Add NOT NULL without default
- Run untested migrations

## Common Patterns

### Adding a Table

```sql
BEGIN;

CREATE TABLE IF NOT EXISTS new_table (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_new_table_user ON new_table(user_id);

-- Add updated_at trigger
CREATE TRIGGER trigger_new_table_updated
    BEFORE UPDATE ON new_table
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

COMMIT;
```

### Adding a Column

```sql
BEGIN;

-- Add column with default
ALTER TABLE existing_table
    ADD COLUMN new_column TEXT DEFAULT 'default_value';

-- For NOT NULL, add in two steps:
-- 1. Add nullable
ALTER TABLE existing_table ADD COLUMN required_column TEXT;

-- 2. Backfill data
UPDATE existing_table SET required_column = 'value' WHERE required_column IS NULL;

-- 3. Add NOT NULL constraint
ALTER TABLE existing_table ALTER COLUMN required_column SET NOT NULL;

COMMIT;
```

### Adding an Index

```sql
-- For large tables, use CONCURRENTLY (outside transaction)
CREATE INDEX CONCURRENTLY idx_large_table_column ON large_table(column);

-- For smaller tables or during maintenance window
BEGIN;
CREATE INDEX idx_table_column ON table(column);
COMMIT;
```

### Adding a Foreign Key

```sql
BEGIN;

-- Add FK constraint
ALTER TABLE child_table
    ADD CONSTRAINT fk_child_parent
    FOREIGN KEY (parent_id)
    REFERENCES parent_table(id)
    ON DELETE CASCADE;

-- Add index for FK (important for performance)
CREATE INDEX idx_child_parent ON child_table(parent_id);

COMMIT;
```

### Modifying ENUMs

```sql
BEGIN;

-- Add new value to enum
ALTER TYPE pod_status ADD VALUE IF NOT EXISTS 'maintenance';

-- Note: Removing enum values requires recreating the type
-- which is complex - prefer adding new values only

COMMIT;
```

## Rollback Procedures

Migrations are generally forward-only. For rollbacks:

### Manual Rollback

1. Identify the migration to rollback
2. Create a new migration that reverses the changes
3. Apply the new migration

```sql
-- 010_rollback_feature_x.sql
BEGIN;

-- Reverse changes from 009
DROP TABLE IF EXISTS feature_x;
ALTER TABLE related_table DROP COLUMN IF EXISTS feature_x_id;

COMMIT;
```

### Point-in-Time Recovery

For major issues, restore from backup:

```bash
# Stop API
docker compose stop api

# Restore backup
gunzip -c backup-20250115.sql.gz | docker compose exec -T postgres psql -U labadmin virtuallab

# Run pending migrations
mage db:migrate

# Restart API
docker compose start api
```

## Testing Migrations

### Local Testing

```bash
# Create test database
docker compose exec postgres createdb -U labadmin virtuallab_test

# Run migrations
DATABASE_NAME=virtuallab_test mage db:migrate

# Verify
docker compose exec postgres psql -U labadmin -d virtuallab_test -c "\dt"

# Clean up
docker compose exec postgres dropdb -U labadmin virtuallab_test
```

### CI Testing

Migrations are tested in CI pipeline:

```yaml
# .github/workflows/ci.yml
- name: Test migrations
  run: |
    docker compose up -d postgres
    sleep 5
    mage db:migrate
    mage db:status
```

## Performance Considerations

### Large Table Migrations

For tables with millions of rows:

1. **Add columns without NOT NULL** first
2. **Backfill in batches**:
   ```sql
   -- Backfill in chunks
   UPDATE large_table
   SET new_column = 'value'
   WHERE id IN (SELECT id FROM large_table WHERE new_column IS NULL LIMIT 10000);
   ```
3. **Add constraint after backfill**
4. **Create indexes CONCURRENTLY**

### Lock Considerations

Operations that lock tables:

| Operation | Lock Type | Duration |
|-----------|-----------|----------|
| CREATE INDEX | SHARE | Long |
| CREATE INDEX CONCURRENTLY | SHARE UPDATE EXCLUSIVE | Longer but non-blocking |
| ALTER TABLE ADD COLUMN | ACCESS EXCLUSIVE | Brief |
| ALTER TABLE ADD CONSTRAINT | ACCESS EXCLUSIVE | Brief |

## Related Documentation

- [Schema Overview](database-schema.md) - Current schema
- [Query Patterns](query-patterns.md) - Common queries
- [Production Deployment](../admin/production-deployment.md) - Deployment guide
