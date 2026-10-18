-- Database Administration Pathway Seed Data
-- Creates the Database Administration pathway with 5 modules covering SQL, MySQL, PostgreSQL

-- =============================================================================
-- DATABASE ADMINISTRATION LAB TEMPLATES
-- =============================================================================

INSERT INTO lab_templates (id, name, slug, description, difficulty, platform, duration_minutes, max_points, tags, visibility, is_active, spec, created_at, updated_at)
VALUES
    -- Module 1: SQL Fundamentals
    ('a3400001-0001-0001-0001-000000000001', 'SQL Fundamentals', 'sql-fundamentals',
     'Master SQL from the ground up. Learn SELECT queries, filtering with WHERE, sorting, aggregations, joins, and subqueries using standard SQL syntax.',
     'beginner', 'proxmox', 75, 100,
     ARRAY['database', 'sql', 'queries', 'beginner'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 2: PostgreSQL Administration
    ('a3400001-0001-0001-0001-000000000002', 'PostgreSQL Administration', 'postgresql-administration',
     'Learn to install, configure, and manage PostgreSQL databases. Cover user management, backup and recovery, performance tuning, and monitoring.',
     'intermediate', 'proxmox', 90, 120,
     ARRAY['database', 'postgresql', 'administration', 'backup', 'performance'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 3: MySQL Administration
    ('a3400001-0001-0001-0001-000000000003', 'MySQL Administration', 'mysql-administration',
     'Master MySQL database administration. Learn installation, user management, replication, backup strategies, and query optimization.',
     'intermediate', 'proxmox', 90, 120,
     ARRAY['database', 'mysql', 'administration', 'replication', 'backup'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 4: Database Design and Modeling
    ('a3400001-0001-0001-0001-000000000004', 'Database Design and Modeling', 'database-design',
     'Learn database design principles. Master normalization, entity-relationship modeling, indexing strategies, and schema optimization.',
     'intermediate', 'proxmox', 75, 130,
     ARRAY['database', 'design', 'modeling', 'normalization', 'indexing'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 5: Database Security
    ('a3400001-0001-0001-0001-000000000005', 'Database Security', 'database-security',
     'Secure your databases against attacks. Learn authentication, authorization, encryption, SQL injection prevention, and audit logging.',
     'advanced', 'proxmox', 90, 140,
     ARRAY['database', 'security', 'encryption', 'audit', 'sql-injection'],
     'global', true, '{}'::jsonb, NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    difficulty = EXCLUDED.difficulty,
    duration_minutes = EXCLUDED.duration_minutes,
    max_points = EXCLUDED.max_points,
    tags = EXCLUDED.tags,
    updated_at = NOW();

-- =============================================================================
-- DATABASE ADMINISTRATION PATHWAY
-- Note: database-administration pathway already exists, so we'll use a new slug
-- =============================================================================

INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, status, is_active, is_featured, visibility, prerequisites, tags, icon, color, created_at, updated_at)
VALUES (
    'b2400001-0001-0001-0001-000000000001',
    'Database Essentials',
    'database-essentials',
    'Build comprehensive database skills from SQL basics to security hardening. This pathway covers SQL fundamentals, PostgreSQL and MySQL administration, database design principles, and security best practices. Perfect for aspiring DBAs and developers who want to master data management.',
    'From SQL queries to database security - master PostgreSQL, MySQL, and DBA best practices.',
    'beginner',
    7, -- Total estimated hours
    13, -- Display order
    'published',
    true,
    false,
    'global',
    '["Basic command line skills", "Understanding of client-server architecture"]'::jsonb,
    ARRAY['database', 'sql', 'postgresql', 'mysql', 'administration'],
    'pi pi-database',
    '#336791',
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    short_description = EXCLUDED.short_description,
    estimated_hours = EXCLUDED.estimated_hours,
    is_featured = EXCLUDED.is_featured,
    status = 'published',
    icon = EXCLUDED.icon,
    color = EXCLUDED.color,
    updated_at = NOW();

-- =============================================================================
-- DATABASE ADMIN PATHWAY MODULES
-- =============================================================================

INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, icon, estimated_minutes, created_at)
VALUES
    ('c3400001-0001-0001-0001-000000000001', 'b2400001-0001-0001-0001-000000000001',
     'SQL Fundamentals', 'sql-fundamentals',
     'Master SQL queries and data manipulation.',
     1, 'always', true, 'pi pi-table', 75, NOW()),

    ('c3400001-0001-0001-0001-000000000002', 'b2400001-0001-0001-0001-000000000001',
     'PostgreSQL Administration', 'postgresql-admin',
     'Install, configure, and manage PostgreSQL.',
     2, 'sequential', true, 'pi pi-database', 90, NOW()),

    ('c3400001-0001-0001-0001-000000000003', 'b2400001-0001-0001-0001-000000000001',
     'MySQL Administration', 'mysql-admin',
     'Master MySQL database management.',
     3, 'sequential', true, 'pi pi-database', 90, NOW()),

    ('c3400001-0001-0001-0001-000000000004', 'b2400001-0001-0001-0001-000000000001',
     'Database Design', 'database-design',
     'Design efficient and normalized schemas.',
     4, 'sequential', true, 'pi pi-sitemap', 75, NOW()),

    ('c3400001-0001-0001-0001-000000000005', 'b2400001-0001-0001-0001-000000000001',
     'Database Security', 'database-security',
     'Secure databases against attacks.',
     5, 'all_previous', true, 'pi pi-lock', 90, NOW())
ON CONFLICT (pathway_id, slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    display_order = EXCLUDED.display_order,
    icon = EXCLUDED.icon,
    estimated_minutes = EXCLUDED.estimated_minutes;

-- =============================================================================
-- LINK LABS TO MODULES
-- =============================================================================

INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
VALUES
    ('d4400001-0001-0001-0001-000000000001', 'c3400001-0001-0001-0001-000000000001', 'a3400001-0001-0001-0001-000000000001', 1, true),
    ('d4400001-0001-0001-0001-000000000002', 'c3400001-0001-0001-0001-000000000002', 'a3400001-0001-0001-0001-000000000002', 1, true),
    ('d4400001-0001-0001-0001-000000000003', 'c3400001-0001-0001-0001-000000000003', 'a3400001-0001-0001-0001-000000000003', 1, true),
    ('d4400001-0001-0001-0001-000000000004', 'c3400001-0001-0001-0001-000000000004', 'a3400001-0001-0001-0001-000000000004', 1, true),
    ('d4400001-0001-0001-0001-000000000005', 'c3400001-0001-0001-0001-000000000005', 'a3400001-0001-0001-0001-000000000005', 1, true)
ON CONFLICT (id) DO UPDATE SET
    display_order = EXCLUDED.display_order,
    is_required = EXCLUDED.is_required;

-- =============================================================================
-- DATABASE ADMIN ACHIEVEMENTS
-- =============================================================================

INSERT INTO achievements (id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria, created_at, updated_at)
VALUES
    ('sql-novice', 'SQL Novice',
     'Complete the SQL Fundamentals module',
     'pathway', 'bronze', '/icons/database.svg', 75, false, true,
     '{"type": "pathway_modules", "pathway_slug": "database-essentials", "modules_required": 1}'::jsonb,
     NOW(), NOW()),

    ('postgres-admin', 'PostgreSQL Administrator',
     'Complete the PostgreSQL Administration module',
     'pathway', 'silver', '/icons/postgresql.svg', 125, false, true,
     '{"type": "pathway_modules", "pathway_slug": "database-essentials", "modules_required": 2}'::jsonb,
     NOW(), NOW()),

    ('mysql-admin', 'MySQL Administrator',
     'Complete the MySQL Administration module',
     'pathway', 'silver', '/icons/mysql.svg', 125, false, true,
     '{"type": "pathway_modules", "pathway_slug": "database-essentials", "modules_required": 3}'::jsonb,
     NOW(), NOW()),

    ('database-architect', 'Database Architect',
     'Complete the Database Design module',
     'pathway', 'gold', '/icons/schema.svg', 200, false, true,
     '{"type": "pathway_modules", "pathway_slug": "database-essentials", "modules_required": 4}'::jsonb,
     NOW(), NOW()),

    ('database-essentials-master', 'Database Master',
     'Complete the entire Database Essentials pathway',
     'pathway', 'platinum', '/icons/trophy.svg', 500, false, true,
     '{"type": "pathway_complete", "pathway_slug": "database-essentials"}'::jsonb,
     NOW(), NOW()),

    ('query-perfectionist', 'Query Perfectionist',
     'Score 100% on any Database pathway lab',
     'perfect_score', 'gold', '/icons/star.svg', 100, false, true,
     '{"type": "lab_perfect_score", "pathway_slug": "database-essentials"}'::jsonb,
     NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    points = EXCLUDED.points,
    criteria = EXCLUDED.criteria,
    updated_at = NOW();
