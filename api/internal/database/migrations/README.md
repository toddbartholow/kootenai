# Database Migrations

## Naming Convention

Migrations use the format `NNN_description.sql` where `NNN` is a zero-padded sequential number:

```
001_initial_schema.sql
002_assessment_results.sql
...
042_sql_review_fixes.sql
```

## Rules

1. **Never rename existing migrations.** The migration runner tracks applied migrations by full filename. Renaming a file causes it to re-execute on existing databases, which can corrupt data or fail.

2. **Separate schema from seed data (043+).** New migrations should not mix DDL (CREATE TABLE, ALTER TABLE) with INSERT statements. Use separate files:
   - `043_add_foo_table.sql` (schema)
   - `044_seed_foo_data.sql` (seed data)

3. **Migrations must be idempotent where possible.** Use `IF NOT EXISTS`, `IF EXISTS`, and similar guards to prevent errors on re-run.

4. **No destructive operations in production.** Avoid `DROP TABLE` or `DROP COLUMN` without a deprecation period.

## Known Issues

### Duplicate number 014

Both `014_certificates.sql` and `014_linux_pathway_seed.sql` exist. The migration runner tracks by **full filename**, not by number prefix, so both are tracked independently and execute correctly. Do not renumber them -- renaming risks re-execution on existing databases.
