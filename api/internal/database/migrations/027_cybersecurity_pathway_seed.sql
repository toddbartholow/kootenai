-- Cybersecurity Learning Pathway Seed Data
-- Enriches the Cybersecurity pathway and its 6 modules, which 015 already seeded.
--
-- This migration originally minted its own ids for the pathway and its modules, but
-- 015_comprehensive_labs_seed.sql already owns 'cybersecurity-fundamentals' (pathway
-- c1000001-...) and its six modules (d1000001-...). The pathway upsert therefore hit
-- ON CONFLICT (slug), which leaves the surviving row's id alone, and the module inserts
-- then failed the pathway_modules -> pathways foreign key.
--
-- So we reuse 015's ids and slugs throughout: every statement below upserts onto the
-- rows 015 created rather than creating a parallel, duplicate set of modules pointing
-- at the same six lab templates.

-- Create the Cybersecurity Learning Pathway
INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, status, is_active, is_featured, visibility, prerequisites, tags, icon, color, created_at, updated_at)
VALUES (
    'c1000001-0001-0001-0001-000000000001',
    'Cybersecurity Fundamentals',
    'cybersecurity-fundamentals',
    'A comprehensive pathway to building cybersecurity skills from the ground up. Start with SOC fundamentals and progress through network analysis, vulnerability assessment, web security, incident response, and penetration testing. This pathway covers both defensive (blue team) and offensive (red team) perspectives.',
    'Build a complete cybersecurity skillset - from SOC operations to penetration testing.',
    'beginner',
    14, -- Total estimated hours
    2,
    'published',
    true,
    true,
    'global',
    '["Basic networking knowledge", "Familiarity with Linux command line", "Understanding of TCP/IP"]'::jsonb,
    ARRAY['security', 'soc', 'pentesting', 'blue-team', 'red-team', 'certification-prep'],
    'shield',
    '#dc2626',
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

-- Create pathway modules (6 modules, matching the cybersecurity labs)
INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, icon, estimated_minutes, created_at)
VALUES
    ('d1000001-0001-0001-0001-000000000001', 'c1000001-0001-0001-0001-000000000001',
     'SOC Analyst Fundamentals', 'security-operations',
     'Begin your cybersecurity journey by learning Security Operations Center fundamentals including log analysis and SIEM basics.',
     1, 'always', true, 'eye', 90, NOW()),

    ('d1000001-0001-0001-0001-000000000002', 'c1000001-0001-0001-0001-000000000001',
     'Network Traffic Analysis', 'network-security-module',
     'Master packet capture and network traffic analysis using Wireshark and tcpdump.',
     2, 'sequential', true, 'activity', 120, NOW()),

    ('d1000001-0001-0001-0001-000000000003', 'c1000001-0001-0001-0001-000000000001',
     'Vulnerability Assessment', 'vulnerability-management',
     'Learn to perform vulnerability assessments using Nmap and vulnerability scanners.',
     3, 'sequential', true, 'search', 120, NOW()),

    ('d1000001-0001-0001-0001-000000000004', 'c1000001-0001-0001-0001-000000000001',
     'Web Application Security', 'web-security-module',
     'Identify and exploit OWASP Top 10 vulnerabilities including SQL injection and XSS.',
     4, 'sequential', true, 'globe', 150, NOW()),

    ('d1000001-0001-0001-0001-000000000005', 'c1000001-0001-0001-0001-000000000001',
     'Incident Response & Forensics', 'incident-response-module',
     'Practice incident response procedures and digital forensics on compromised systems.',
     5, 'sequential', true, 'alert-triangle', 180, NOW()),

    ('d1000001-0001-0001-0001-000000000006', 'c1000001-0001-0001-0001-000000000001',
     'Penetration Testing Basics', 'pentesting-module',
     'Learn penetration testing methodology from reconnaissance to exploitation.',
     6, 'all_previous', true, 'target', 180, NOW())
ON CONFLICT (pathway_id, slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    display_order = EXCLUDED.display_order,
    -- unlock_type MUST be carried across. 015 seeds every module of this
    -- pathway as 'sequential' at display_order 1..6, and enrollment unlocks a
    -- module only when `unlock_type = 'always' OR display_order = 0`
    -- (enrollment_repo.go, create_module_progress). With none matching, every
    -- module enrolls 'locked' and UnlockNextModules -- which only fires after a
    -- completion -- has nothing to start from, so the pathway cannot be entered
    -- at all. Declaring 'always' on module 1 below is the fix; omitting it here
    -- silently discards it.
    unlock_type = EXCLUDED.unlock_type,
    icon = EXCLUDED.icon,
    estimated_minutes = EXCLUDED.estimated_minutes;

-- Link labs to modules
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
VALUES
    ('d4000002-0001-0001-0001-000000000001', 'd1000001-0001-0001-0001-000000000001', 'b1000001-0001-0001-0001-000000000001', 1, true),
    ('d4000002-0001-0001-0001-000000000002', 'd1000001-0001-0001-0001-000000000002', 'b1000001-0001-0001-0001-000000000002', 1, true),
    ('d4000002-0001-0001-0001-000000000003', 'd1000001-0001-0001-0001-000000000003', 'b1000001-0001-0001-0001-000000000003', 1, true),
    ('d4000002-0001-0001-0001-000000000004', 'd1000001-0001-0001-0001-000000000004', 'b1000001-0001-0001-0001-000000000004', 1, true),
    ('d4000002-0001-0001-0001-000000000005', 'd1000001-0001-0001-0001-000000000005', 'b1000001-0001-0001-0001-000000000005', 1, true),
    ('d4000002-0001-0001-0001-000000000006', 'd1000001-0001-0001-0001-000000000006', 'b1000001-0001-0001-0001-000000000006', 1, true)
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Create Cybersecurity pathway achievements
INSERT INTO achievements (id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria, created_at, updated_at)
VALUES
    ('security-analyst', 'Security Analyst',
     'Complete the SOC and Network Analysis modules',
     'pathway', 'bronze', '/icons/shield.svg', 100, false, true,
     '{"type": "pathway_modules", "pathway_slug": "cybersecurity-fundamentals", "modules_required": 2}'::jsonb,
     NOW(), NOW()),

    ('security-assessor', 'Security Assessor',
     'Complete vulnerability assessment and web security modules',
     'pathway', 'silver', '/icons/search-check.svg', 200, false, true,
     '{"type": "pathway_modules", "pathway_slug": "cybersecurity-fundamentals", "modules_required": 4}'::jsonb,
     NOW(), NOW()),

    ('incident-responder', 'Incident Responder',
     'Complete the incident response and forensics module',
     'pathway', 'gold', '/icons/alert-circle.svg', 250, false, true,
     '{"type": "pathway_modules", "pathway_slug": "cybersecurity-fundamentals", "modules_required": 5}'::jsonb,
     NOW(), NOW()),

    ('penetration-tester', 'Penetration Tester',
     'Complete all cybersecurity pathway modules including penetration testing',
     'pathway', 'platinum', '/icons/crosshair.svg', 500, false, true,
     '{"type": "pathway_complete", "pathway_slug": "cybersecurity-fundamentals"}'::jsonb,
     NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    points = EXCLUDED.points,
    criteria = EXCLUDED.criteria,
    updated_at = NOW();

-- Create Windows Active Directory Pathway
INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, status, is_active, is_featured, visibility, prerequisites, tags, icon, color, created_at, updated_at)
VALUES (
    'b2000003-0001-0001-0001-000000000001',
    'Windows Active Directory',
    'windows-active-directory',
    'Master Windows Active Directory administration from fundamentals to security hardening. Learn AD structure, Group Policy management, DNS/DHCP integration, and security best practices for enterprise environments.',
    'Master Windows AD administration and security from fundamentals to enterprise hardening.',
    'intermediate',
    8,
    3,
    'published',
    true,
    false,
    'global',
    '["Basic Windows Server knowledge", "Understanding of networking concepts"]'::jsonb,
    ARRAY['windows', 'active-directory', 'administration', 'enterprise', 'security'],
    'server',
    '#0078d4',
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    short_description = EXCLUDED.short_description,
    estimated_hours = EXCLUDED.estimated_hours,
    status = 'published',
    icon = EXCLUDED.icon,
    color = EXCLUDED.color,
    updated_at = NOW();

-- Create Windows AD pathway modules
INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, icon, estimated_minutes, created_at)
VALUES
    ('c3000003-0001-0001-0001-000000000001', 'b2000003-0001-0001-0001-000000000001',
     'Active Directory Fundamentals', 'ad-fundamentals',
     'Learn AD structure, OUs, and user/group management basics.',
     1, 'always', true, 'users', 120, NOW()),

    ('c3000003-0001-0001-0001-000000000002', 'b2000003-0001-0001-0001-000000000001',
     'Group Policy Management', 'group-policy-management',
     'Master GPO creation, linking, and troubleshooting.',
     2, 'sequential', true, 'settings', 120, NOW()),

    ('c3000003-0001-0001-0001-000000000003', 'b2000003-0001-0001-0001-000000000001',
     'DNS and DHCP Services', 'windows-dns-dhcp',
     'Configure and manage Windows DNS and DHCP integrated with AD.',
     3, 'sequential', true, 'network', 90, NOW()),

    ('c3000003-0001-0001-0001-000000000004', 'b2000003-0001-0001-0001-000000000001',
     'Security Hardening', 'ad-security-hardening',
     'Secure AD against common attacks with privileged access management.',
     4, 'all_previous', true, 'lock', 150, NOW())
ON CONFLICT (pathway_id, slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    display_order = EXCLUDED.display_order,
    -- Carried for the same reason as the cybersecurity modules above: a
    -- re-application must not silently revert module 1 to a locked state.
    unlock_type = EXCLUDED.unlock_type,
    icon = EXCLUDED.icon,
    estimated_minutes = EXCLUDED.estimated_minutes;

-- Link labs to Windows AD modules
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
VALUES
    ('d4000003-0001-0001-0001-000000000001', 'c3000003-0001-0001-0001-000000000001', 'b2000001-0001-0001-0001-000000000001', 1, true),
    ('d4000003-0001-0001-0001-000000000002', 'c3000003-0001-0001-0001-000000000002', 'b2000001-0001-0001-0001-000000000002', 1, true),
    ('d4000003-0001-0001-0001-000000000003', 'c3000003-0001-0001-0001-000000000003', 'b2000001-0001-0001-0001-000000000004', 1, true),
    ('d4000003-0001-0001-0001-000000000004', 'c3000003-0001-0001-0001-000000000004', 'b2000001-0001-0001-0001-000000000003', 1, true)
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Windows AD achievements
INSERT INTO achievements (id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria, created_at, updated_at)
VALUES
    ('ad-admin', 'AD Administrator',
     'Complete AD Fundamentals and Group Policy modules',
     'pathway', 'silver', '/icons/server.svg', 150, false, true,
     '{"type": "pathway_modules", "pathway_slug": "windows-active-directory", "modules_required": 2}'::jsonb,
     NOW(), NOW()),

    ('ad-master', 'AD Master',
     'Complete the entire Windows Active Directory pathway',
     'pathway', 'gold', '/icons/crown.svg', 400, false, true,
     '{"type": "pathway_complete", "pathway_slug": "windows-active-directory"}'::jsonb,
     NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    points = EXCLUDED.points,
    criteria = EXCLUDED.criteria,
    updated_at = NOW();
