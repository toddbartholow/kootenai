-- Cloud Security Pathway Seed Data
-- Creates the Cloud Security pathway with 5 modules covering security across cloud platforms

-- =============================================================================
-- CLOUD SECURITY LAB TEMPLATES
-- =============================================================================

INSERT INTO lab_templates (id, name, slug, description, difficulty, platform, duration_minutes, max_points, tags, visibility, is_active, spec, created_at, updated_at)
VALUES
    -- Module 1: Cloud Security Fundamentals
    ('a3300001-0001-0001-0001-000000000001', 'Cloud Security Fundamentals', 'cloud-security-fundamentals',
     'Understand the shared responsibility model and cloud security basics. Learn identity management, network security, and compliance considerations across cloud platforms.',
     'beginner', 'cloudstack', 60, 100,
     ARRAY['cloud', 'security', 'fundamentals', 'compliance', 'iam'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 2: Identity and Access Management
    ('a3300001-0001-0001-0001-000000000002', 'Cloud Identity and Access Management', 'cloud-iam-deep-dive',
     'Deep dive into cloud IAM. Master least privilege principles, role-based access control, identity federation, and secure credential management.',
     'intermediate', 'cloudstack', 90, 120,
     ARRAY['cloud', 'security', 'iam', 'rbac', 'federation'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 3: Network Security in the Cloud
    ('a3300001-0001-0001-0001-000000000003', 'Cloud Network Security', 'cloud-network-security',
     'Secure cloud network architectures. Learn VPC security, security groups, NACLs, private connectivity, and DDoS protection strategies.',
     'intermediate', 'cloudstack', 90, 130,
     ARRAY['cloud', 'security', 'networking', 'vpc', 'firewall'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 4: Data Protection and Encryption
    ('a3300001-0001-0001-0001-000000000004', 'Cloud Data Protection', 'cloud-data-protection',
     'Protect data in the cloud. Master encryption at rest and in transit, key management, secrets management, and data loss prevention.',
     'intermediate', 'cloudstack', 75, 120,
     ARRAY['cloud', 'security', 'encryption', 'kms', 'data-protection'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 5: Security Monitoring and Incident Response
    ('a3300001-0001-0001-0001-000000000005', 'Cloud Security Monitoring', 'cloud-security-monitoring',
     'Monitor and respond to security events in the cloud. Learn cloud-native SIEM, audit logging, threat detection, and incident response procedures.',
     'advanced', 'cloudstack', 90, 140,
     ARRAY['cloud', 'security', 'monitoring', 'siem', 'incident-response'],
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
-- CLOUD SECURITY PATHWAY
-- =============================================================================

INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, status, is_active, is_featured, visibility, prerequisites, tags, icon, color, created_at, updated_at)
VALUES (
    'b2300001-0001-0001-0001-000000000001',
    'Cloud Security Essentials',
    'cloud-security-essentials',
    'Secure your cloud infrastructure from the ground up. This pathway covers cloud security fundamentals, identity management, network security, data protection, and security monitoring. Learn to implement defense-in-depth strategies across cloud platforms.',
    'Protect cloud infrastructure with IAM, network security, encryption, and monitoring.',
    'intermediate',
    7, -- Total estimated hours
    12, -- Display order
    'published',
    true,
    true,
    'global',
    '["Basic cloud computing knowledge", "Understanding of networking concepts", "Familiarity with security principles"]'::jsonb,
    ARRAY['cloud', 'security', 'iam', 'compliance', 'monitoring'],
    'pi pi-shield',
    '#7c3aed',
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
-- CLOUD SECURITY PATHWAY MODULES
-- =============================================================================

INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, icon, estimated_minutes, created_at)
VALUES
    ('c3300001-0001-0001-0001-000000000001', 'b2300001-0001-0001-0001-000000000001',
     'Security Fundamentals', 'security-fundamentals',
     'Understand cloud security models and shared responsibility.',
     1, 'always', true, 'pi pi-book', 60, NOW()),

    ('c3300001-0001-0001-0001-000000000002', 'b2300001-0001-0001-0001-000000000001',
     'Identity & Access Management', 'cloud-iam',
     'Master IAM for secure access control.',
     2, 'sequential', true, 'pi pi-users', 90, NOW()),

    ('c3300001-0001-0001-0001-000000000003', 'b2300001-0001-0001-0001-000000000001',
     'Network Security', 'cloud-network-sec',
     'Secure cloud network architectures.',
     3, 'sequential', true, 'pi pi-sitemap', 90, NOW()),

    ('c3300001-0001-0001-0001-000000000004', 'b2300001-0001-0001-0001-000000000001',
     'Data Protection', 'data-protection',
     'Encrypt and protect data in the cloud.',
     4, 'sequential', true, 'pi pi-lock', 75, NOW()),

    ('c3300001-0001-0001-0001-000000000005', 'b2300001-0001-0001-0001-000000000001',
     'Security Monitoring', 'security-monitoring',
     'Monitor, detect, and respond to threats.',
     5, 'all_previous', true, 'pi pi-eye', 90, NOW())
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
    ('d4300001-0001-0001-0001-000000000001', 'c3300001-0001-0001-0001-000000000001', 'a3300001-0001-0001-0001-000000000001', 1, true),
    ('d4300001-0001-0001-0001-000000000002', 'c3300001-0001-0001-0001-000000000002', 'a3300001-0001-0001-0001-000000000002', 1, true),
    ('d4300001-0001-0001-0001-000000000003', 'c3300001-0001-0001-0001-000000000003', 'a3300001-0001-0001-0001-000000000003', 1, true),
    ('d4300001-0001-0001-0001-000000000004', 'c3300001-0001-0001-0001-000000000004', 'a3300001-0001-0001-0001-000000000004', 1, true),
    ('d4300001-0001-0001-0001-000000000005', 'c3300001-0001-0001-0001-000000000005', 'a3300001-0001-0001-0001-000000000005', 1, true)
ON CONFLICT (id) DO UPDATE SET
    display_order = EXCLUDED.display_order,
    is_required = EXCLUDED.is_required;

-- =============================================================================
-- CLOUD SECURITY ACHIEVEMENTS
-- =============================================================================

INSERT INTO achievements (id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria, created_at, updated_at)
VALUES
    ('cloud-security-beginner', 'Cloud Security Beginner',
     'Complete the Cloud Security Fundamentals module',
     'pathway', 'bronze', '/icons/cloud-shield.svg', 75, false, true,
     '{"type": "pathway_modules", "pathway_slug": "cloud-security-essentials", "modules_required": 1}'::jsonb,
     NOW(), NOW()),

    ('iam-specialist', 'IAM Specialist',
     'Complete the IAM module',
     'pathway', 'silver', '/icons/users.svg', 150, false, true,
     '{"type": "pathway_modules", "pathway_slug": "cloud-security-essentials", "modules_required": 2}'::jsonb,
     NOW(), NOW()),

    ('cloud-defender', 'Cloud Defender',
     'Complete network security and data protection modules',
     'pathway', 'gold', '/icons/shield.svg', 250, false, true,
     '{"type": "pathway_modules", "pathway_slug": "cloud-security-essentials", "modules_required": 4}'::jsonb,
     NOW(), NOW()),

    ('cloud-security-master', 'Cloud Security Master',
     'Complete the entire Cloud Security pathway',
     'pathway', 'platinum', '/icons/trophy.svg', 500, false, true,
     '{"type": "pathway_complete", "pathway_slug": "cloud-security-essentials"}'::jsonb,
     NOW(), NOW()),

    ('cloud-guardian', 'Cloud Guardian',
     'Score 100% on any Cloud Security pathway lab',
     'perfect_score', 'gold', '/icons/star.svg', 100, false, true,
     '{"type": "lab_perfect_score", "pathway_slug": "cloud-security-essentials"}'::jsonb,
     NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    points = EXCLUDED.points,
    criteria = EXCLUDED.criteria,
    updated_at = NOW();
