-- Additional Learning Pathways
-- Creates 3 new pathways: Network Security Advanced, Blue Team Operations, Cloud Security (AWS)

-- ============================================================================
-- PATHWAY 1: Network Security Advanced
-- ============================================================================
INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, status, is_active, is_featured, visibility, prerequisites, tags, icon, color, created_at, updated_at)
VALUES (
    'b2000004-0001-0001-0001-000000000001',
    'Network Security Advanced',
    'network-security-advanced',
    'Take your network security skills to the next level. This advanced pathway covers firewall configuration, IDS/IPS deployment, network hardening, and security monitoring. Build on your networking and security fundamentals to defend enterprise networks.',
    'Advanced network security: firewalls, IDS/IPS, and enterprise defense.',
    'advanced',
    10,
    6,
    'published',
    true,
    true,
    'global',
    '["Network Engineering pathway", "Basic security knowledge", "Linux command line proficiency"]'::jsonb,
    ARRAY['security', 'networking', 'firewall', 'ids', 'enterprise', 'advanced'],
    'shield-check',
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

-- Network Security Advanced Modules
INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, icon, estimated_minutes, created_at)
VALUES
    ('c3000004-0001-0001-0001-000000000001', 'b2000004-0001-0001-0001-000000000001',
     'Firewall Security', 'firewall-security',
     'Master enterprise firewall configuration including pfSense, iptables, and zone-based firewalling.',
     1, 'always', true, 'shield', 150, NOW()),

    ('c3000004-0001-0001-0001-000000000002', 'b2000004-0001-0001-0001-000000000001',
     'Routing & Network Segmentation', 'routing-segmentation',
     'Implement secure network routing and segmentation strategies for defense in depth.',
     2, 'sequential', true, 'git-branch', 120, NOW()),

    ('c3000004-0001-0001-0001-000000000003', 'b2000004-0001-0001-0001-000000000001',
     'VLAN Security', 'vlan-security',
     'Configure VLANs for network isolation and implement inter-VLAN security policies.',
     3, 'sequential', true, 'layers', 120, NOW()),

    ('c3000004-0001-0001-0001-000000000004', 'b2000004-0001-0001-0001-000000000001',
     'Linux Firewall Hardening', 'linux-firewall-hardening',
     'Harden Linux hosts with iptables, nftables, and host-based firewalls.',
     4, 'all_previous', true, 'lock', 90, NOW())
ON CONFLICT (pathway_id, slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    display_order = EXCLUDED.display_order,
    icon = EXCLUDED.icon,
    estimated_minutes = EXCLUDED.estimated_minutes;

-- Link existing labs to Network Security Advanced modules
-- Module 1: Firewall Security -> networking-advanced/03-firewall-security
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000004-0001-0001-0001-000000000001',
    'c3000004-0001-0001-0001-000000000001',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = '03-firewall-security'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Module 2: Routing & Segmentation -> networking-advanced/01-routing-fundamentals
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000004-0001-0001-0001-000000000002',
    'c3000004-0001-0001-0001-000000000002',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = '01-routing-fundamentals'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Module 3: VLAN Security -> networking-advanced/02-vlan-switching
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000004-0001-0001-0001-000000000003',
    'c3000004-0001-0001-0001-000000000003',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = '02-vlan-switching'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Module 4: Linux Firewall Hardening -> security/linux-firewall-basics
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000004-0001-0001-0001-000000000004',
    'c3000004-0001-0001-0001-000000000004',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = 'linux-firewall-basics'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Network Security Advanced Achievements
INSERT INTO achievements (id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria, created_at, updated_at)
VALUES
    ('firewall-master', 'Firewall Master',
     'Complete firewall security and routing modules',
     'pathway', 'silver', '/icons/shield-check.svg', 200, false, true,
     '{"type": "pathway_modules", "pathway_slug": "network-security-advanced", "modules_required": 2}'::jsonb,
     NOW(), NOW()),

    ('network-defender', 'Network Defender',
     'Complete all Network Security Advanced modules',
     'pathway', 'platinum', '/icons/shield-plus.svg', 500, false, true,
     '{"type": "pathway_complete", "pathway_slug": "network-security-advanced"}'::jsonb,
     NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    points = EXCLUDED.points,
    criteria = EXCLUDED.criteria,
    updated_at = NOW();

-- ============================================================================
-- PATHWAY 2: Blue Team Operations
-- ============================================================================
INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, status, is_active, is_featured, visibility, prerequisites, tags, icon, color, created_at, updated_at)
VALUES (
    'b2000005-0001-0001-0001-000000000001',
    'Blue Team Operations',
    'blue-team-operations',
    'Develop essential blue team skills for defending enterprise networks. Learn SIEM operations, log analysis, threat detection, incident triage, and proactive threat hunting. This pathway prepares you for roles in Security Operations Centers.',
    'Master defensive security: SIEM, log analysis, threat detection, and incident response.',
    'intermediate',
    12,
    7,
    'published',
    true,
    true,
    'global',
    '["Cybersecurity Fundamentals pathway or equivalent", "Linux command line basics", "Basic networking"]'::jsonb,
    ARRAY['security', 'soc', 'blue-team', 'siem', 'threat-hunting', 'incident-response'],
    'eye',
    '#0ea5e9',
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

-- Blue Team Operations Modules
INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, icon, estimated_minutes, created_at)
VALUES
    ('c3000005-0001-0001-0001-000000000001', 'b2000005-0001-0001-0001-000000000001',
     'SOC Fundamentals', 'soc-fundamentals',
     'Learn Security Operations Center workflows, tools, and analyst responsibilities.',
     1, 'always', true, 'monitor', 90, NOW()),

    ('c3000005-0001-0001-0001-000000000002', 'b2000005-0001-0001-0001-000000000001',
     'Network Traffic Analysis', 'network-analysis',
     'Master packet capture, network flow analysis, and protocol-level threat detection.',
     2, 'sequential', true, 'activity', 120, NOW()),

    ('c3000005-0001-0001-0001-000000000003', 'b2000005-0001-0001-0001-000000000001',
     'Vulnerability Management', 'vuln-management',
     'Perform vulnerability assessments, prioritize findings, and track remediation.',
     3, 'sequential', true, 'search', 120, NOW()),

    ('c3000005-0001-0001-0001-000000000004', 'b2000005-0001-0001-0001-000000000001',
     'Incident Response', 'incident-response',
     'Execute incident response procedures from detection through containment and recovery.',
     4, 'sequential', true, 'alert-triangle', 180, NOW()),

    ('c3000005-0001-0001-0001-000000000005', 'b2000005-0001-0001-0001-000000000001',
     'Threat Hunting', 'threat-hunting',
     'Proactively hunt for threats using hypothesis-driven investigation techniques.',
     5, 'all_previous', true, 'crosshair', 180, NOW())
ON CONFLICT (pathway_id, slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    display_order = EXCLUDED.display_order,
    icon = EXCLUDED.icon,
    estimated_minutes = EXCLUDED.estimated_minutes;

-- Link existing labs to Blue Team Operations modules
-- Module 1: SOC Fundamentals -> cybersecurity/01-soc-analyst-fundamentals
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000005-0001-0001-0001-000000000001',
    'c3000005-0001-0001-0001-000000000001',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = '01-soc-analyst-fundamentals'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Module 2: Network Traffic Analysis -> cybersecurity/02-network-traffic-analysis
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000005-0001-0001-0001-000000000002',
    'c3000005-0001-0001-0001-000000000002',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = '02-network-traffic-analysis'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Module 3: Vulnerability Management -> cybersecurity/03-vulnerability-assessment
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000005-0001-0001-0001-000000000003',
    'c3000005-0001-0001-0001-000000000003',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = '03-vulnerability-assessment'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Module 4: Incident Response -> cybersecurity/05-incident-response
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000005-0001-0001-0001-000000000004',
    'c3000005-0001-0001-0001-000000000004',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = '05-incident-response'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Module 5: Threat Hunting -> security/blue-team-ir (if exists) or placeholder
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000005-0001-0001-0001-000000000005',
    'c3000005-0001-0001-0001-000000000005',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = 'blue-team-ir'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Blue Team Operations Achievements
INSERT INTO achievements (id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria, created_at, updated_at)
VALUES
    ('soc-analyst', 'SOC Analyst',
     'Complete SOC and network analysis modules',
     'pathway', 'bronze', '/icons/monitor.svg', 100, false, true,
     '{"type": "pathway_modules", "pathway_slug": "blue-team-operations", "modules_required": 2}'::jsonb,
     NOW(), NOW()),

    ('threat-hunter', 'Threat Hunter',
     'Complete all Blue Team Operations modules',
     'pathway', 'platinum', '/icons/eye-check.svg', 500, false, true,
     '{"type": "pathway_complete", "pathway_slug": "blue-team-operations"}'::jsonb,
     NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    points = EXCLUDED.points,
    criteria = EXCLUDED.criteria,
    updated_at = NOW();

-- ============================================================================
-- PATHWAY 3: Cloud Security (AWS)
-- ============================================================================
INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, status, is_active, is_featured, visibility, prerequisites, tags, icon, color, created_at, updated_at)
VALUES (
    'b2000006-0001-0001-0001-000000000001',
    'Cloud Security (AWS)',
    'cloud-security-aws',
    'Secure AWS cloud environments with defense-in-depth strategies. Learn IAM best practices, VPC security architecture, CloudTrail monitoring, GuardDuty threat detection, and security automation. Builds on AWS fundamentals to prepare you for cloud security roles.',
    'Secure AWS environments: IAM, VPC security, CloudTrail, and automation.',
    'intermediate',
    10,
    8,
    'published',
    true,
    true,
    'global',
    '["AWS Cloud Practitioner pathway or equivalent AWS experience", "Basic security concepts"]'::jsonb,
    ARRAY['aws', 'cloud', 'security', 'iam', 'vpc', 'devsecops'],
    'cloud-lock',
    '#ff9900',
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

-- Cloud Security (AWS) Modules
INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, icon, estimated_minutes, created_at)
VALUES
    ('c3000006-0001-0001-0001-000000000001', 'b2000006-0001-0001-0001-000000000001',
     'IAM Security Best Practices', 'iam-security',
     'Implement least privilege, IAM policies, roles, and identity federation securely.',
     1, 'always', true, 'users', 120, NOW()),

    ('c3000006-0001-0001-0001-000000000002', 'b2000006-0001-0001-0001-000000000001',
     'VPC Security Architecture', 'vpc-security',
     'Design secure VPC architectures with security groups, NACLs, and network segmentation.',
     2, 'sequential', true, 'network', 120, NOW()),

    ('c3000006-0001-0001-0001-000000000003', 'b2000006-0001-0001-0001-000000000001',
     'Monitoring & Threat Detection', 'cloud-monitoring',
     'Configure CloudTrail, CloudWatch, and GuardDuty for security monitoring and alerting.',
     3, 'sequential', true, 'bell', 120, NOW()),

    ('c3000006-0001-0001-0001-000000000004', 'b2000006-0001-0001-0001-000000000001',
     'Security Automation', 'security-automation',
     'Automate security responses with Lambda, Config rules, and infrastructure as code.',
     4, 'all_previous', true, 'zap', 120, NOW())
ON CONFLICT (pathway_id, slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    display_order = EXCLUDED.display_order,
    icon = EXCLUDED.icon,
    estimated_minutes = EXCLUDED.estimated_minutes;

-- Link existing AWS labs to Cloud Security modules
-- Module 1: IAM Security -> aws/03-iam-security
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000006-0001-0001-0001-000000000001',
    'c3000006-0001-0001-0001-000000000001',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = '03-iam-security'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Module 2: VPC Security -> aws/02-vpc-networking
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000006-0001-0001-0001-000000000002',
    'c3000006-0001-0001-0001-000000000002',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = '02-vpc-networking'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Module 3 & 4: Monitoring and Automation - will need new labs or use serverless as placeholder
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000006-0001-0001-0001-000000000003',
    'c3000006-0001-0001-0001-000000000003',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = '04-serverless-lambda'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
SELECT
    'd4000006-0001-0001-0001-000000000004',
    'c3000006-0001-0001-0001-000000000004',
    lt.id,
    1, true
FROM lab_templates lt WHERE lt.slug = '01-aws-fundamentals'
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Cloud Security (AWS) Achievements
INSERT INTO achievements (id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria, created_at, updated_at)
VALUES
    ('cloud-security-analyst', 'Cloud Security Analyst',
     'Complete IAM and VPC security modules',
     'pathway', 'silver', '/icons/cloud-shield.svg', 200, false, true,
     '{"type": "pathway_modules", "pathway_slug": "cloud-security-aws", "modules_required": 2}'::jsonb,
     NOW(), NOW()),

    ('aws-security-specialist', 'AWS Security Specialist',
     'Complete all Cloud Security (AWS) modules',
     'pathway', 'platinum', '/icons/cloud-check.svg', 500, false, true,
     '{"type": "pathway_complete", "pathway_slug": "cloud-security-aws"}'::jsonb,
     NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    points = EXCLUDED.points,
    criteria = EXCLUDED.criteria,
    updated_at = NOW();
