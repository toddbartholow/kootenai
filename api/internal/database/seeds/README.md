# Database Seeds

Standalone seed data files for populating a fresh database after schema migrations.

## Usage

```bash
mage db:seed              # Run all seeds
mage db:seed demo         # Run only demo seeds
```

## Design

Existing seed data lives in migration files (005, 010, 011, 015, 027, 029, 032-036, 038, 040)
and is automatically applied during migration. Those files cannot be moved.

New seed data should be placed here as standalone `.sql` files that can be run
independently via `mage db:seed`. Seeds should be idempotent (use `ON CONFLICT DO NOTHING`
or `INSERT ... WHERE NOT EXISTS`).

## File Naming

```
001_demo_users.sql        # Demo/dev users
002_lab_templates.sql     # Lab template definitions
003_pathways.sql          # Learning pathways
```
