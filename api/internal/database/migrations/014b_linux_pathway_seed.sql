-- Linux Learning Pathway Seed Data
-- Renamed from 014_linux_pathway_seed.sql to 014b to resolve collision with
-- 014_certificates.sql. For existing deployments, this file was already applied
-- under the old name; the migration runner uses filename-as-version.
-- This migration creates the Linux pathway with 8 modules and their associated labs

-- First, create lab templates for each Linux lab
-- (In a production system, these would be created by importing YAML files)

-- Insert Linux lab templates
INSERT INTO lab_templates (id, name, slug, description, difficulty, platform, duration_minutes, tags, visibility, is_active, spec, created_at, updated_at)
VALUES
    ('a1000001-0001-0001-0001-000000000001', 'Linux Foundations', 'linux-foundations',
     'Learn the fundamentals of Linux navigation and file editing. This lab covers essential commands like pwd, ls, cd, and introduces both vim and nano editors.',
     'beginner', 'proxmox', 45, ARRAY['linux', 'beginner', 'navigation', 'vim', 'nano'], 'global', true, '{}'::jsonb, NOW(), NOW()),

    ('a1000001-0001-0001-0001-000000000002', 'Shell Essentials', 'shell-essentials',
     'Master essential shell concepts including PATH, environment variables, I/O redirection, pipes, and sudo.',
     'beginner', 'proxmox', 45, ARRAY['linux', 'beginner', 'shell', 'environment', 'pipes'], 'global', true, '{}'::jsonb, NOW(), NOW()),

    ('a1000001-0001-0001-0001-000000000003', 'File Mastery', 'file-mastery',
     'Deep dive into Linux file operations including permissions, ownership, archive creation/extraction, and symbolic/hard links.',
     'intermediate', 'proxmox', 60, ARRAY['linux', 'intermediate', 'permissions', 'archives', 'links'], 'global', true, '{}'::jsonb, NOW(), NOW()),

    ('a1000001-0001-0001-0001-000000000004', 'Text Processing', 'text-processing',
     'Master Linux text processing tools including grep, awk, sed, cut, and sort.',
     'intermediate', 'proxmox', 60, ARRAY['linux', 'intermediate', 'grep', 'awk', 'sed', 'pipes'], 'global', true, '{}'::jsonb, NOW(), NOW()),

    ('a1000001-0001-0001-0001-000000000005', 'Process & User Management', 'process-user-management',
     'Learn to manage Linux processes and user accounts. Cover process monitoring, process control, and user/group administration.',
     'intermediate', 'proxmox', 75, ARRAY['linux', 'intermediate', 'processes', 'users', 'groups'], 'global', true, '{}'::jsonb, NOW(), NOW()),

    ('a1000001-0001-0001-0001-000000000006', 'Services & System Management', 'services-system-management',
     'Master systemd service management, journalctl log analysis, and system monitoring.',
     'intermediate', 'proxmox', 60, ARRAY['linux', 'intermediate', 'systemd', 'services', 'logs'], 'global', true, '{}'::jsonb, NOW(), NOW()),

    ('a1000001-0001-0001-0001-000000000007', 'Storage & Package Management', 'storage-packages',
     'Learn Linux storage management including disk partitioning, filesystems, mounting, and package management with apt.',
     'advanced', 'proxmox', 90, ARRAY['linux', 'advanced', 'storage', 'partitions', 'apt', 'lvm'], 'global', true, '{}'::jsonb, NOW(), NOW()),

    ('a1000001-0001-0001-0001-000000000008', 'Networking & Troubleshooting', 'networking-troubleshooting',
     'Master Linux networking configuration and troubleshooting. Learn IP addressing, SSH, firewall management, and network diagnostics.',
     'advanced', 'proxmox', 90, ARRAY['linux', 'advanced', 'networking', 'ssh', 'firewall', 'tcpdump'], 'global', true, '{}'::jsonb, NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    difficulty = EXCLUDED.difficulty,
    duration_minutes = EXCLUDED.duration_minutes,
    tags = EXCLUDED.tags,
    updated_at = NOW();

-- Create the Linux Learning Pathway
INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, status, is_active, is_featured, visibility, prerequisites, tags, created_at, updated_at)
VALUES (
    'b2000001-0001-0001-0001-000000000001',
    'Linux Fundamentals',
    'linux-fundamentals',
    'A comprehensive pathway to mastering Linux system administration. Starting from basic navigation and shell usage, progress through file management, text processing, user administration, and advanced topics like storage management and networking. Each module builds on previous knowledge to give you a solid foundation in Linux.',
    'Master Linux from the command line up - navigation, shell, files, processes, services, storage, and networking.',
    'beginner',
    9, -- Total estimated hours (45+45+60+60+75+60+90+90 = 525 minutes = ~9 hours)
    1,
    'published', -- Set to published so it appears in the list
    true,
    true,
    'global',
    '["Basic computer literacy", "Familiarity with command-line concepts helpful but not required"]'::jsonb,
    ARRAY['linux', 'sysadmin', 'fundamentals', 'certification-prep'],
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
    updated_at = NOW();

-- Create pathway modules (8 modules, one per lab)
INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, created_at)
VALUES
    ('c3000001-0001-0001-0001-000000000001', 'b2000001-0001-0001-0001-000000000001',
     'Linux Foundations', 'linux-foundations',
     'Start your Linux journey by learning essential navigation commands and file editing with vim and nano.',
     1, 'always', true, NOW()),

    ('c3000001-0001-0001-0001-000000000002', 'b2000001-0001-0001-0001-000000000001',
     'Shell Essentials', 'shell-essentials',
     'Master the shell environment including PATH, environment variables, I/O redirection, and sudo.',
     2, 'sequential', true, NOW()),

    ('c3000001-0001-0001-0001-000000000003', 'b2000001-0001-0001-0001-000000000001',
     'File Mastery', 'file-mastery',
     'Deep dive into permissions, ownership, archives, and links.',
     3, 'sequential', true, NOW()),

    ('c3000001-0001-0001-0001-000000000004', 'b2000001-0001-0001-0001-000000000001',
     'Text Processing', 'text-processing',
     'Master grep, awk, sed, and powerful text manipulation pipelines.',
     4, 'sequential', true, NOW()),

    ('c3000001-0001-0001-0001-000000000005', 'b2000001-0001-0001-0001-000000000001',
     'Process & User Management', 'process-user-management',
     'Learn process monitoring, control, and user/group administration.',
     5, 'sequential', true, NOW()),

    ('c3000001-0001-0001-0001-000000000006', 'b2000001-0001-0001-0001-000000000001',
     'Services & System Management', 'services-system-management',
     'Master systemd, journalctl, and system targets.',
     6, 'sequential', true, NOW()),

    ('c3000001-0001-0001-0001-000000000007', 'b2000001-0001-0001-0001-000000000001',
     'Storage & Package Management', 'storage-packages',
     'Learn disk management, filesystems, LVM, and package management.',
     7, 'sequential', true, NOW()),

    ('c3000001-0001-0001-0001-000000000008', 'b2000001-0001-0001-0001-000000000001',
     'Networking & Troubleshooting', 'networking-troubleshooting',
     'Master networking configuration, SSH, firewalls, and diagnostics.',
     8, 'sequential', true, NOW())
ON CONFLICT (pathway_id, slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    display_order = EXCLUDED.display_order;

-- Link labs to modules (each module has one lab)
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
VALUES
    ('d4000001-0001-0001-0001-000000000001', 'c3000001-0001-0001-0001-000000000001', 'a1000001-0001-0001-0001-000000000001', 1, true),
    ('d4000001-0001-0001-0001-000000000002', 'c3000001-0001-0001-0001-000000000002', 'a1000001-0001-0001-0001-000000000002', 1, true),
    ('d4000001-0001-0001-0001-000000000003', 'c3000001-0001-0001-0001-000000000003', 'a1000001-0001-0001-0001-000000000003', 1, true),
    ('d4000001-0001-0001-0001-000000000004', 'c3000001-0001-0001-0001-000000000004', 'a1000001-0001-0001-0001-000000000004', 1, true),
    ('d4000001-0001-0001-0001-000000000005', 'c3000001-0001-0001-0001-000000000005', 'a1000001-0001-0001-0001-000000000005', 1, true),
    ('d4000001-0001-0001-0001-000000000006', 'c3000001-0001-0001-0001-000000000006', 'a1000001-0001-0001-0001-000000000006', 1, true),
    ('d4000001-0001-0001-0001-000000000007', 'c3000001-0001-0001-0001-000000000007', 'a1000001-0001-0001-0001-000000000007', 1, true),
    ('d4000001-0001-0001-0001-000000000008', 'c3000001-0001-0001-0001-000000000008', 'a1000001-0001-0001-0001-000000000008', 1, true)
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Create Linux pathway achievements
-- Uses existing achievements schema: id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria
INSERT INTO achievements (id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria, created_at, updated_at)
VALUES
    ('linux-beginner', 'Linux Beginner',
     'Complete the first two Linux pathway modules (Foundations and Shell Essentials)',
     'pathway', 'bronze', '/icons/terminal.svg', 100, false, true,
     '{"type": "pathway_modules", "pathway_slug": "linux-fundamentals", "modules_required": 2}'::jsonb,
     NOW(), NOW()),

    ('linux-intermediate', 'Linux Intermediate',
     'Complete modules 3-6 of the Linux pathway (File Mastery through Services)',
     'pathway', 'silver', '/icons/cog.svg', 200, false, true,
     '{"type": "pathway_modules", "pathway_slug": "linux-fundamentals", "modules_required": 6}'::jsonb,
     NOW(), NOW()),

    ('linux-advanced', 'Linux Advanced',
     'Complete the advanced Linux modules (Storage and Networking)',
     'pathway', 'gold', '/icons/server.svg', 300, false, true,
     '{"type": "pathway_modules", "pathway_slug": "linux-fundamentals", "modules_required": 8}'::jsonb,
     NOW(), NOW()),

    ('linux-master', 'Linux Master',
     'Complete the entire Linux Fundamentals pathway',
     'pathway', 'platinum', '/icons/trophy.svg', 500, false, true,
     '{"type": "pathway_complete", "pathway_slug": "linux-fundamentals"}'::jsonb,
     NOW(), NOW()),

    ('linux-perfectionist', 'Linux Perfectionist',
     'Score 100% on any Linux pathway lab',
     'perfect_score', 'gold', '/icons/star.svg', 150, false, true,
     '{"type": "lab_perfect_score", "pathway_slug": "linux-fundamentals"}'::jsonb,
     NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    points = EXCLUDED.points,
    criteria = EXCLUDED.criteria,
    updated_at = NOW();

-- Add points to lab templates (for scoring)
-- Points are based on: beginner=100, intermediate=120-150, advanced=150-160
UPDATE lab_templates SET max_points = 100 WHERE slug IN ('linux-foundations', 'shell-essentials');
UPDATE lab_templates SET max_points = 120 WHERE slug IN ('file-mastery', 'text-processing');
UPDATE lab_templates SET max_points = 150 WHERE slug IN ('process-user-management', 'storage-packages');
UPDATE lab_templates SET max_points = 130 WHERE slug = 'services-system-management';
UPDATE lab_templates SET max_points = 160 WHERE slug = 'networking-troubleshooting';
