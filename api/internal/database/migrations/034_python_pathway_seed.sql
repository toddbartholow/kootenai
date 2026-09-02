-- Python for IT Professionals Pathway Seed Data
-- Creates the Python pathway with 6 modules covering scripting, automation, and security tools

-- =============================================================================
-- PYTHON LAB TEMPLATES
-- =============================================================================

INSERT INTO lab_templates (id, name, slug, description, difficulty, platform, duration_minutes, max_points, tags, visibility, is_active, spec, created_at, updated_at)
VALUES
    -- Module 1: Python Basics
    ('a3200001-0001-0001-0001-000000000001', 'Python Fundamentals', 'python-fundamentals',
     'Start your Python journey with core programming concepts. Learn variables, data types, control flow, functions, and basic data structures like lists and dictionaries.',
     'beginner', 'proxmox', 60, 100,
     ARRAY['python', 'programming', 'scripting', 'beginner'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 2: File and System Operations
    ('a3200001-0001-0001-0001-000000000002', 'Python File and System Operations', 'python-file-system',
     'Learn to interact with the filesystem and operating system using Python. Cover file I/O, path manipulation, directory operations, and subprocess management.',
     'beginner', 'proxmox', 60, 100,
     ARRAY['python', 'scripting', 'filesystem', 'automation'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 3: Network Programming
    ('a3200001-0001-0001-0001-000000000003', 'Python Network Programming', 'python-networking',
     'Master network programming with Python. Learn sockets, HTTP requests, API consumption, and network protocol implementation.',
     'intermediate', 'proxmox', 75, 120,
     ARRAY['python', 'networking', 'sockets', 'apis', 'http'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 4: System Administration Scripting
    ('a3200001-0001-0001-0001-000000000004', 'Python for System Administration', 'python-sysadmin',
     'Automate system administration tasks with Python. Learn log parsing, configuration management, monitoring scripts, and scheduled task automation.',
     'intermediate', 'proxmox', 90, 130,
     ARRAY['python', 'sysadmin', 'automation', 'scripting', 'cron'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 5: Security Scripting
    ('a3200001-0001-0001-0001-000000000005', 'Python for Security', 'python-security',
     'Build security tools with Python. Learn to create port scanners, password tools, encryption utilities, and basic exploitation scripts for authorized testing.',
     'intermediate', 'proxmox', 90, 140,
     ARRAY['python', 'security', 'pentesting', 'scripting', 'cryptography'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 6: Data Analysis and Reporting
    ('a3200001-0001-0001-0001-000000000006', 'Python Data Analysis and Reporting', 'python-data-analysis',
     'Analyze data and generate reports with Python. Learn pandas for data manipulation, matplotlib for visualization, and automated report generation.',
     'intermediate', 'proxmox', 75, 120,
     ARRAY['python', 'data-analysis', 'pandas', 'reporting', 'visualization'],
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
-- PYTHON FOR IT PROFESSIONALS PATHWAY
-- =============================================================================

INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, status, is_active, is_featured, visibility, prerequisites, tags, icon, color, created_at, updated_at)
VALUES (
    'b2200001-0001-0001-0001-000000000001',
    'Python for IT Professionals',
    'python-it-professionals',
    'Learn Python with a focus on IT and security applications. This pathway teaches Python programming through practical examples relevant to system administrators, network engineers, and security professionals. Build automation scripts, network tools, and security utilities.',
    'Practical Python scripting for sysadmins, network engineers, and security pros.',
    'beginner',
    8, -- Total estimated hours
    11, -- Display order
    'published',
    true,
    true,
    'global',
    '["Basic command line familiarity", "No prior programming experience required"]'::jsonb,
    ARRAY['python', 'scripting', 'automation', 'security', 'sysadmin'],
    'pi pi-code',
    '#3776ab',
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
-- PYTHON PATHWAY MODULES
-- =============================================================================

INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, icon, estimated_minutes, created_at)
VALUES
    ('c3200001-0001-0001-0001-000000000001', 'b2200001-0001-0001-0001-000000000001',
     'Python Fundamentals', 'python-fundamentals',
     'Learn core Python programming concepts and syntax.',
     1, 'always', true, 'pi pi-play', 60, NOW()),

    ('c3200001-0001-0001-0001-000000000002', 'b2200001-0001-0001-0001-000000000001',
     'File & System Operations', 'file-system-ops',
     'Work with files, directories, and system resources.',
     2, 'sequential', true, 'pi pi-folder', 60, NOW()),

    ('c3200001-0001-0001-0001-000000000003', 'b2200001-0001-0001-0001-000000000001',
     'Network Programming', 'network-programming',
     'Build network tools and consume APIs.',
     3, 'sequential', true, 'pi pi-wifi', 75, NOW()),

    ('c3200001-0001-0001-0001-000000000004', 'b2200001-0001-0001-0001-000000000001',
     'Sysadmin Automation', 'sysadmin-automation',
     'Automate common system administration tasks.',
     4, 'sequential', true, 'pi pi-cog', 90, NOW()),

    ('c3200001-0001-0001-0001-000000000005', 'b2200001-0001-0001-0001-000000000001',
     'Security Scripting', 'security-scripting',
     'Build security tools and utilities.',
     5, 'sequential', true, 'pi pi-shield', 90, NOW()),

    ('c3200001-0001-0001-0001-000000000006', 'b2200001-0001-0001-0001-000000000001',
     'Data Analysis & Reporting', 'data-analysis-reporting',
     'Analyze data and create automated reports.',
     6, 'sequential', true, 'pi pi-chart-bar', 75, NOW())
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
    ('d4200001-0001-0001-0001-000000000001', 'c3200001-0001-0001-0001-000000000001', 'a3200001-0001-0001-0001-000000000001', 1, true),
    ('d4200001-0001-0001-0001-000000000002', 'c3200001-0001-0001-0001-000000000002', 'a3200001-0001-0001-0001-000000000002', 1, true),
    ('d4200001-0001-0001-0001-000000000003', 'c3200001-0001-0001-0001-000000000003', 'a3200001-0001-0001-0001-000000000003', 1, true),
    ('d4200001-0001-0001-0001-000000000004', 'c3200001-0001-0001-0001-000000000004', 'a3200001-0001-0001-0001-000000000004', 1, true),
    ('d4200001-0001-0001-0001-000000000005', 'c3200001-0001-0001-0001-000000000005', 'a3200001-0001-0001-0001-000000000005', 1, true),
    ('d4200001-0001-0001-0001-000000000006', 'c3200001-0001-0001-0001-000000000006', 'a3200001-0001-0001-0001-000000000006', 1, true)
ON CONFLICT (id) DO UPDATE SET
    display_order = EXCLUDED.display_order,
    is_required = EXCLUDED.is_required;

-- =============================================================================
-- PYTHON ACHIEVEMENTS
-- =============================================================================

INSERT INTO achievements (id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria, created_at, updated_at)
VALUES
    ('python-beginner', 'Python Beginner',
     'Complete the Python Fundamentals module',
     'pathway', 'bronze', '/icons/python.svg', 50, false, true,
     '{"type": "pathway_modules", "pathway_slug": "python-it-professionals", "modules_required": 1}'::jsonb,
     NOW(), NOW()),

    ('python-scripter', 'Python Scripter',
     'Complete Python file operations and networking modules',
     'pathway', 'silver', '/icons/python.svg', 150, false, true,
     '{"type": "pathway_modules", "pathway_slug": "python-it-professionals", "modules_required": 3}'::jsonb,
     NOW(), NOW()),

    ('python-automator', 'Python Automator',
     'Complete the sysadmin automation module',
     'pathway', 'silver', '/icons/automation.svg', 175, false, true,
     '{"type": "pathway_modules", "pathway_slug": "python-it-professionals", "modules_required": 4}'::jsonb,
     NOW(), NOW()),

    ('python-security-dev', 'Python Security Developer',
     'Complete the security scripting module',
     'pathway', 'gold', '/icons/security.svg', 200, false, true,
     '{"type": "pathway_modules", "pathway_slug": "python-it-professionals", "modules_required": 5}'::jsonb,
     NOW(), NOW()),

    ('python-master', 'Python Master',
     'Complete the entire Python for IT Professionals pathway',
     'pathway', 'platinum', '/icons/trophy.svg', 500, false, true,
     '{"type": "pathway_complete", "pathway_slug": "python-it-professionals"}'::jsonb,
     NOW(), NOW()),

    ('pythonic-perfectionist', 'Pythonic Perfectionist',
     'Score 100% on any Python pathway lab',
     'perfect_score', 'gold', '/icons/star.svg', 100, false, true,
     '{"type": "lab_perfect_score", "pathway_slug": "python-it-professionals"}'::jsonb,
     NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    points = EXCLUDED.points,
    criteria = EXCLUDED.criteria,
    updated_at = NOW();
