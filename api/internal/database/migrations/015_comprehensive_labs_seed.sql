-- +goose Up
-- Comprehensive Lab Templates Seed Migration
-- Adds Cybersecurity, Windows AD, AWS, Linux Advanced, and Networking lab templates

-- Cybersecurity Labs
INSERT INTO lab_templates (id, name, slug, description, platform, duration_minutes, difficulty, tags, max_points, is_active, visibility, spec, created_at, updated_at)
VALUES
    ('b1000001-0001-0001-0001-000000000001', 'SOC Analyst Fundamentals', 'soc-analyst-fundamentals',
     'Learn the fundamentals of Security Operations Center (SOC) analysis. This lab covers log analysis, SIEM basics, alert triage, and incident documentation.',
     'proxmox', 90, 'beginner',
     ARRAY['security', 'soc', 'siem', 'log-analysis', 'blue-team'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b1000001-0001-0001-0001-000000000002', 'Network Traffic Analysis', 'network-traffic-analysis',
     'Master network traffic analysis using Wireshark and tcpdump. Learn to capture packets, analyze protocols, identify suspicious traffic patterns.',
     'proxmox', 120, 'intermediate',
     ARRAY['security', 'networking', 'wireshark', 'tcpdump', 'traffic-analysis'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b1000001-0001-0001-0001-000000000003', 'Vulnerability Assessment', 'vulnerability-assessment',
     'Learn to perform vulnerability assessments using industry-standard tools including Nmap and vulnerability scanners.',
     'proxmox', 120, 'intermediate',
     ARRAY['security', 'vulnerability', 'nmap', 'scanning', 'assessment'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b1000001-0001-0001-0001-000000000004', 'Web Application Security', 'web-application-security',
     'Identify and exploit common web vulnerabilities from the OWASP Top 10 including SQL injection, XSS, and CSRF.',
     'proxmox', 150, 'intermediate',
     ARRAY['security', 'web', 'owasp', 'sql-injection', 'xss', 'pentesting'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b1000001-0001-0001-0001-000000000005', 'Incident Response and Forensics', 'incident-response-forensics',
     'Practice incident response procedures on a compromised system. Learn evidence preservation, malware analysis, and timeline creation.',
     'proxmox', 180, 'advanced',
     ARRAY['security', 'dfir', 'forensics', 'incident-response', 'malware'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b1000001-0001-0001-0001-000000000006', 'Penetration Testing Basics', 'penetration-testing-basics',
     'Learn the fundamentals of penetration testing methodology including reconnaissance, scanning, exploitation, and reporting.',
     'proxmox', 180, 'intermediate',
     ARRAY['security', 'pentesting', 'metasploit', 'exploitation', 'red-team'],
     120, true, 'global', '{}'::jsonb, NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    difficulty = EXCLUDED.difficulty,
    duration_minutes = EXCLUDED.duration_minutes,
    tags = EXCLUDED.tags,
    max_points = EXCLUDED.max_points,
    updated_at = NOW();

-- Windows Active Directory Labs
INSERT INTO lab_templates (id, name, slug, description, platform, duration_minutes, difficulty, tags, max_points, is_active, visibility, spec, created_at, updated_at)
VALUES
    ('b2000001-0001-0001-0001-000000000001', 'Active Directory Fundamentals', 'ad-fundamentals',
     'Learn the fundamentals of Windows Active Directory including AD structure, OUs, user and group management.',
     'proxmox', 120, 'beginner',
     ARRAY['windows', 'active-directory', 'administration', 'powershell', 'identity'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b2000001-0001-0001-0001-000000000002', 'Group Policy Management', 'group-policy-management',
     'Master Windows Group Policy for centralized management. Create, link, and troubleshoot GPOs for security and configuration.',
     'proxmox', 120, 'intermediate',
     ARRAY['windows', 'active-directory', 'group-policy', 'gpo', 'security'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b2000001-0001-0001-0001-000000000003', 'Active Directory Security Hardening', 'ad-security-hardening',
     'Secure Active Directory against common attacks. Learn privileged access management, Kerberos security, and attack detection.',
     'proxmox', 150, 'advanced',
     ARRAY['windows', 'active-directory', 'security', 'hardening', 'kerberos'],
     110, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b2000001-0001-0001-0001-000000000004', 'Windows DNS and DHCP Services', 'windows-dns-dhcp',
     'Configure and manage Windows DNS and DHCP services integrated with Active Directory.',
     'proxmox', 90, 'intermediate',
     ARRAY['windows', 'dns', 'dhcp', 'networking', 'active-directory'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    difficulty = EXCLUDED.difficulty,
    duration_minutes = EXCLUDED.duration_minutes,
    tags = EXCLUDED.tags,
    max_points = EXCLUDED.max_points,
    updated_at = NOW();

-- AWS Cloud Labs
INSERT INTO lab_templates (id, name, slug, description, platform, duration_minutes, difficulty, tags, max_points, is_active, visibility, spec, created_at, updated_at)
VALUES
    ('b3000001-0001-0001-0001-000000000001', 'AWS Cloud Fundamentals', 'aws-fundamentals',
     'Get started with Amazon Web Services. Learn console navigation, IAM basics, EC2, and S3.',
     'cloudstack', 90, 'beginner',
     ARRAY['aws', 'cloud', 'ec2', 's3', 'iam', 'fundamentals'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b3000001-0001-0001-0001-000000000002', 'AWS VPC and Networking', 'aws-vpc-networking',
     'Design and implement AWS Virtual Private Cloud networking including subnets, route tables, and security groups.',
     'cloudstack', 120, 'intermediate',
     ARRAY['aws', 'cloud', 'vpc', 'networking', 'security-groups', 'subnets'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b3000001-0001-0001-0001-000000000003', 'AWS IAM and Security', 'aws-iam-security',
     'Master AWS Identity and Access Management for secure cloud operations with users, groups, roles, and policies.',
     'cloudstack', 120, 'intermediate',
     ARRAY['aws', 'cloud', 'iam', 'security', 'policies', 'roles'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b3000001-0001-0001-0001-000000000004', 'AWS Serverless with Lambda', 'aws-serverless-lambda',
     'Build serverless applications with AWS Lambda, API Gateway, and DynamoDB.',
     'cloudstack', 120, 'intermediate',
     ARRAY['aws', 'cloud', 'lambda', 'serverless', 'api-gateway', 'dynamodb'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    difficulty = EXCLUDED.difficulty,
    duration_minutes = EXCLUDED.duration_minutes,
    tags = EXCLUDED.tags,
    max_points = EXCLUDED.max_points,
    updated_at = NOW();

-- Linux Advanced Labs
INSERT INTO lab_templates (id, name, slug, description, platform, duration_minutes, difficulty, tags, max_points, is_active, visibility, spec, created_at, updated_at)
VALUES
    ('b4000001-0001-0001-0001-000000000001', 'Advanced Shell Scripting', 'advanced-shell-scripting',
     'Master Bash scripting with functions, error handling, command-line arguments, and automation best practices.',
     'proxmox', 120, 'intermediate',
     ARRAY['linux', 'bash', 'scripting', 'automation', 'shell'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b4000001-0001-0001-0001-000000000002', 'Container Fundamentals with Docker', 'container-fundamentals',
     'Learn containerization with Docker. Build images, run containers, use Docker Compose, and understand container networking.',
     'proxmox', 120, 'intermediate',
     ARRAY['linux', 'docker', 'containers', 'devops', 'compose'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b4000001-0001-0001-0001-000000000003', 'Linux System Administration', 'linux-system-administration',
     'Advanced Linux system administration skills. Learn system monitoring, performance tuning, cron scheduling, log management, and hardening.',
     'proxmox', 150, 'advanced',
     ARRAY['linux', 'sysadmin', 'monitoring', 'performance', 'hardening'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    difficulty = EXCLUDED.difficulty,
    duration_minutes = EXCLUDED.duration_minutes,
    tags = EXCLUDED.tags,
    max_points = EXCLUDED.max_points,
    updated_at = NOW();

-- Networking Advanced Labs
INSERT INTO lab_templates (id, name, slug, description, platform, duration_minutes, difficulty, tags, max_points, is_active, visibility, spec, created_at, updated_at)
VALUES
    ('b5000001-0001-0001-0001-000000000001', 'IP Routing Fundamentals', 'ip-routing-fundamentals',
     'Master IP routing concepts and configuration. Learn static routing, dynamic routing with OSPF and BGP basics, route summarization.',
     'proxmox', 120, 'intermediate',
     ARRAY['networking', 'routing', 'ospf', 'bgp', 'cisco', 'linux'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b5000001-0001-0001-0001-000000000002', 'VLANs and Layer 2 Switching', 'vlans-layer2-switching',
     'Learn VLAN configuration and Layer 2 switching concepts. This lab covers VLAN creation, trunk ports, inter-VLAN routing, and STP basics.',
     'proxmox', 120, 'intermediate',
     ARRAY['networking', 'vlan', 'switching', 'stp', 'layer2'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW()),

    ('b5000001-0001-0001-0001-000000000003', 'Network Firewall Configuration', 'network-firewall-configuration',
     'Learn to configure and manage network firewalls. This lab covers iptables, nftables, firewalld, and pfSense basics including NAT and port forwarding.',
     'proxmox', 150, 'intermediate',
     ARRAY['networking', 'firewall', 'security', 'iptables', 'pfsense', 'nat'],
     100, true, 'global', '{}'::jsonb, NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    difficulty = EXCLUDED.difficulty,
    duration_minutes = EXCLUDED.duration_minutes,
    tags = EXCLUDED.tags,
    max_points = EXCLUDED.max_points,
    updated_at = NOW();

-- Create Cybersecurity Pathway
INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, is_active, is_featured, visibility, prerequisites, tags, created_at, updated_at)
VALUES
    ('c1000001-0001-0001-0001-000000000001', 'Cybersecurity Fundamentals', 'cybersecurity-fundamentals',
     'A comprehensive introduction to cybersecurity covering SOC operations, network analysis, vulnerability assessment, web security, incident response, and penetration testing.',
     'Learn essential cybersecurity skills from SOC analysis to penetration testing',
     'intermediate', 15, 2, true, true, 'global',
     '[]'::jsonb,
     ARRAY['security', 'cybersecurity', 'soc', 'pentesting', 'blue-team', 'red-team'],
     NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    updated_at = NOW();

-- Create Windows Administration Pathway
INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, is_active, is_featured, visibility, prerequisites, tags, created_at, updated_at)
VALUES
    ('c2000001-0001-0001-0001-000000000001', 'Windows Server Administration', 'windows-server-administration',
     'Master Windows Server administration with Active Directory, Group Policy, security hardening, and network services.',
     'Build expertise in Windows Server and Active Directory',
     'intermediate', 10, 3, true, true, 'global',
     '[]'::jsonb,
     ARRAY['windows', 'active-directory', 'server', 'administration', 'powershell'],
     NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    updated_at = NOW();

-- Create AWS Cloud Pathway
INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, is_active, is_featured, visibility, prerequisites, tags, created_at, updated_at)
VALUES
    ('c3000001-0001-0001-0001-000000000001', 'AWS Cloud Practitioner', 'aws-cloud-practitioner',
     'Learn Amazon Web Services from the ground up. Cover EC2, S3, VPC networking, IAM security, and serverless computing.',
     'Start your cloud journey with AWS fundamentals',
     'intermediate', 8, 4, true, true, 'global',
     '[]'::jsonb,
     ARRAY['aws', 'cloud', 'devops', 'infrastructure', 'serverless'],
     NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    updated_at = NOW();

-- Create Network Engineering Pathway
INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, is_active, is_featured, visibility, prerequisites, tags, created_at, updated_at)
VALUES
    ('c4000001-0001-0001-0001-000000000001', 'Network Engineering', 'network-engineering',
     'Build network engineering skills from routing and switching to firewall configuration. Learn OSPF, VLANs, and network security.',
     'Master networking from routing to firewall security',
     'intermediate', 8, 5, true, false, 'global',
     '[]'::jsonb,
     ARRAY['networking', 'routing', 'switching', 'firewall', 'cisco'],
     NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    updated_at = NOW();

-- Cybersecurity Pathway Modules
INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, created_at)
VALUES
    ('d1000001-0001-0001-0001-000000000001', 'c1000001-0001-0001-0001-000000000001',
     'Security Operations', 'security-operations',
     'Learn SOC fundamentals and log analysis', 1, 'sequential', true, NOW()),
    ('d1000001-0001-0001-0001-000000000002', 'c1000001-0001-0001-0001-000000000001',
     'Network Security', 'network-security-module',
     'Master network traffic analysis and monitoring', 2, 'sequential', true, NOW()),
    ('d1000001-0001-0001-0001-000000000003', 'c1000001-0001-0001-0001-000000000001',
     'Vulnerability Management', 'vulnerability-management',
     'Learn vulnerability assessment techniques', 3, 'sequential', true, NOW()),
    ('d1000001-0001-0001-0001-000000000004', 'c1000001-0001-0001-0001-000000000001',
     'Web Security', 'web-security-module',
     'Understand and exploit web vulnerabilities', 4, 'sequential', true, NOW()),
    ('d1000001-0001-0001-0001-000000000005', 'c1000001-0001-0001-0001-000000000001',
     'Incident Response', 'incident-response-module',
     'Practice incident response and forensics', 5, 'sequential', true, NOW()),
    ('d1000001-0001-0001-0001-000000000006', 'c1000001-0001-0001-0001-000000000001',
     'Penetration Testing', 'pentesting-module',
     'Learn offensive security fundamentals', 6, 'sequential', true, NOW())
ON CONFLICT (pathway_id, slug) DO NOTHING;

-- Windows Pathway Modules
INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, created_at)
VALUES
    ('d2000001-0001-0001-0001-000000000001', 'c2000001-0001-0001-0001-000000000001',
     'Active Directory Basics', 'ad-basics',
     'Learn AD fundamentals and user management', 1, 'sequential', true, NOW()),
    ('d2000001-0001-0001-0001-000000000002', 'c2000001-0001-0001-0001-000000000001',
     'Group Policy', 'group-policy-module',
     'Master Group Policy management', 2, 'sequential', true, NOW()),
    ('d2000001-0001-0001-0001-000000000003', 'c2000001-0001-0001-0001-000000000001',
     'AD Security', 'ad-security-module',
     'Secure and harden Active Directory', 3, 'sequential', true, NOW()),
    ('d2000001-0001-0001-0001-000000000004', 'c2000001-0001-0001-0001-000000000001',
     'Network Services', 'network-services-module',
     'Configure DNS and DHCP services', 4, 'sequential', true, NOW())
ON CONFLICT (pathway_id, slug) DO NOTHING;

-- AWS Pathway Modules
INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, created_at)
VALUES
    ('d3000001-0001-0001-0001-000000000001', 'c3000001-0001-0001-0001-000000000001',
     'AWS Basics', 'aws-basics',
     'Get started with AWS console and core services', 1, 'sequential', true, NOW()),
    ('d3000001-0001-0001-0001-000000000002', 'c3000001-0001-0001-0001-000000000001',
     'VPC Networking', 'vpc-networking-module',
     'Design and implement VPC networks', 2, 'sequential', true, NOW()),
    ('d3000001-0001-0001-0001-000000000003', 'c3000001-0001-0001-0001-000000000001',
     'IAM Security', 'iam-security-module',
     'Master IAM for secure access control', 3, 'sequential', true, NOW()),
    ('d3000001-0001-0001-0001-000000000004', 'c3000001-0001-0001-0001-000000000001',
     'Serverless', 'serverless-module',
     'Build serverless applications', 4, 'sequential', true, NOW())
ON CONFLICT (pathway_id, slug) DO NOTHING;

-- Network Engineering Pathway Modules
INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, created_at)
VALUES
    ('d4000001-0001-0001-0001-000000000001', 'c4000001-0001-0001-0001-000000000001',
     'IP Routing', 'ip-routing-module',
     'Learn routing fundamentals and protocols', 1, 'sequential', true, NOW()),
    ('d4000001-0001-0001-0001-000000000002', 'c4000001-0001-0001-0001-000000000001',
     'Switching & VLANs', 'switching-vlans-module',
     'Master Layer 2 switching and VLANs', 2, 'sequential', true, NOW()),
    ('d4000001-0001-0001-0001-000000000003', 'c4000001-0001-0001-0001-000000000001',
     'Firewall Security', 'firewall-security-module',
     'Configure network firewalls', 3, 'sequential', true, NOW())
ON CONFLICT (pathway_id, slug) DO NOTHING;

-- Link Cybersecurity Labs to Modules
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
VALUES
    (uuid_generate_v4(), 'd1000001-0001-0001-0001-000000000001', 'b1000001-0001-0001-0001-000000000001', 1, true),
    (uuid_generate_v4(), 'd1000001-0001-0001-0001-000000000002', 'b1000001-0001-0001-0001-000000000002', 1, true),
    (uuid_generate_v4(), 'd1000001-0001-0001-0001-000000000003', 'b1000001-0001-0001-0001-000000000003', 1, true),
    (uuid_generate_v4(), 'd1000001-0001-0001-0001-000000000004', 'b1000001-0001-0001-0001-000000000004', 1, true),
    (uuid_generate_v4(), 'd1000001-0001-0001-0001-000000000005', 'b1000001-0001-0001-0001-000000000005', 1, true),
    (uuid_generate_v4(), 'd1000001-0001-0001-0001-000000000006', 'b1000001-0001-0001-0001-000000000006', 1, true)
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Link Windows Labs to Modules
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
VALUES
    (uuid_generate_v4(), 'd2000001-0001-0001-0001-000000000001', 'b2000001-0001-0001-0001-000000000001', 1, true),
    (uuid_generate_v4(), 'd2000001-0001-0001-0001-000000000002', 'b2000001-0001-0001-0001-000000000002', 1, true),
    (uuid_generate_v4(), 'd2000001-0001-0001-0001-000000000003', 'b2000001-0001-0001-0001-000000000003', 1, true),
    (uuid_generate_v4(), 'd2000001-0001-0001-0001-000000000004', 'b2000001-0001-0001-0001-000000000004', 1, true)
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Link AWS Labs to Modules
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
VALUES
    (uuid_generate_v4(), 'd3000001-0001-0001-0001-000000000001', 'b3000001-0001-0001-0001-000000000001', 1, true),
    (uuid_generate_v4(), 'd3000001-0001-0001-0001-000000000002', 'b3000001-0001-0001-0001-000000000002', 1, true),
    (uuid_generate_v4(), 'd3000001-0001-0001-0001-000000000003', 'b3000001-0001-0001-0001-000000000003', 1, true),
    (uuid_generate_v4(), 'd3000001-0001-0001-0001-000000000004', 'b3000001-0001-0001-0001-000000000004', 1, true)
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Link Networking Labs to Modules
INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
VALUES
    (uuid_generate_v4(), 'd4000001-0001-0001-0001-000000000001', 'b5000001-0001-0001-0001-000000000001', 1, true),
    (uuid_generate_v4(), 'd4000001-0001-0001-0001-000000000002', 'b5000001-0001-0001-0001-000000000002', 1, true),
    (uuid_generate_v4(), 'd4000001-0001-0001-0001-000000000003', 'b5000001-0001-0001-0001-000000000003', 1, true)
ON CONFLICT (module_id, lab_template_id) DO NOTHING;

-- Add Achievements for Cybersecurity Pathway
INSERT INTO achievements (id, name, description, icon_url, type, tier, points, criteria, is_active, created_at, updated_at)
VALUES
    ('e1000001-0001-0001-0001-000000000001', 'SOC Analyst', 'Complete the SOC Analyst Fundamentals lab', 'shield-check', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b1000001-0001-0001-0001-000000000001"}'::jsonb, true, NOW(), NOW()),
    ('e1000001-0001-0001-0001-000000000002', 'Traffic Inspector', 'Complete the Network Traffic Analysis lab', 'network', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b1000001-0001-0001-0001-000000000002"}'::jsonb, true, NOW(), NOW()),
    ('e1000001-0001-0001-0001-000000000003', 'Vulnerability Hunter', 'Complete the Vulnerability Assessment lab', 'search', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b1000001-0001-0001-0001-000000000003"}'::jsonb, true, NOW(), NOW()),
    ('e1000001-0001-0001-0001-000000000004', 'Web Defender', 'Complete the Web Application Security lab', 'globe', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b1000001-0001-0001-0001-000000000004"}'::jsonb, true, NOW(), NOW()),
    ('e1000001-0001-0001-0001-000000000005', 'Incident Responder', 'Complete the Incident Response lab', 'alert-triangle', 'lab_completion', 'silver', 75, '{"lab_template_id": "b1000001-0001-0001-0001-000000000005"}'::jsonb, true, NOW(), NOW()),
    ('e1000001-0001-0001-0001-000000000006', 'Ethical Hacker', 'Complete the Penetration Testing lab', 'terminal', 'lab_completion', 'silver', 75, '{"lab_template_id": "b1000001-0001-0001-0001-000000000006"}'::jsonb, true, NOW(), NOW()),
    ('e1000001-0001-0001-0001-000000000007', 'Cybersecurity Pro', 'Complete the entire Cybersecurity Fundamentals pathway', 'award', 'pathway_completion', 'gold', 200, '{"pathway_id": "c1000001-0001-0001-0001-000000000001"}'::jsonb, true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Add Achievements for Windows Pathway
INSERT INTO achievements (id, name, description, icon_url, type, tier, points, criteria, is_active, created_at, updated_at)
VALUES
    ('e2000001-0001-0001-0001-000000000001', 'AD Administrator', 'Complete the Active Directory Fundamentals lab', 'users', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b2000001-0001-0001-0001-000000000001"}'::jsonb, true, NOW(), NOW()),
    ('e2000001-0001-0001-0001-000000000002', 'Policy Master', 'Complete the Group Policy Management lab', 'settings', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b2000001-0001-0001-0001-000000000002"}'::jsonb, true, NOW(), NOW()),
    ('e2000001-0001-0001-0001-000000000003', 'Security Hardener', 'Complete the AD Security Hardening lab', 'lock', 'lab_completion', 'silver', 75, '{"lab_template_id": "b2000001-0001-0001-0001-000000000003"}'::jsonb, true, NOW(), NOW()),
    ('e2000001-0001-0001-0001-000000000004', 'Windows Pro', 'Complete the entire Windows Server Administration pathway', 'award', 'pathway_completion', 'gold', 150, '{"pathway_id": "c2000001-0001-0001-0001-000000000001"}'::jsonb, true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Add Achievements for AWS Pathway
INSERT INTO achievements (id, name, description, icon_url, type, tier, points, criteria, is_active, created_at, updated_at)
VALUES
    ('e3000001-0001-0001-0001-000000000001', 'Cloud Beginner', 'Complete the AWS Cloud Fundamentals lab', 'cloud', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b3000001-0001-0001-0001-000000000001"}'::jsonb, true, NOW(), NOW()),
    ('e3000001-0001-0001-0001-000000000002', 'Network Architect', 'Complete the AWS VPC and Networking lab', 'network', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b3000001-0001-0001-0001-000000000002"}'::jsonb, true, NOW(), NOW()),
    ('e3000001-0001-0001-0001-000000000003', 'IAM Expert', 'Complete the AWS IAM and Security lab', 'key', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b3000001-0001-0001-0001-000000000003"}'::jsonb, true, NOW(), NOW()),
    ('e3000001-0001-0001-0001-000000000004', 'Serverless Developer', 'Complete the AWS Serverless with Lambda lab', 'zap', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b3000001-0001-0001-0001-000000000004"}'::jsonb, true, NOW(), NOW()),
    ('e3000001-0001-0001-0001-000000000005', 'AWS Practitioner', 'Complete the entire AWS Cloud Practitioner pathway', 'award', 'pathway_completion', 'gold', 150, '{"pathway_id": "c3000001-0001-0001-0001-000000000001"}'::jsonb, true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Add Achievements for Networking Pathway
INSERT INTO achievements (id, name, description, icon_url, type, tier, points, criteria, is_active, created_at, updated_at)
VALUES
    ('e4000001-0001-0001-0001-000000000001', 'Router Pro', 'Complete the IP Routing Fundamentals lab', 'git-branch', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b5000001-0001-0001-0001-000000000001"}'::jsonb, true, NOW(), NOW()),
    ('e4000001-0001-0001-0001-000000000002', 'Switch Master', 'Complete the VLANs and Layer 2 Switching lab', 'layers', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b5000001-0001-0001-0001-000000000002"}'::jsonb, true, NOW(), NOW()),
    ('e4000001-0001-0001-0001-000000000003', 'Firewall Expert', 'Complete the Network Firewall Configuration lab', 'shield', 'lab_completion', 'bronze', 50, '{"lab_template_id": "b5000001-0001-0001-0001-000000000003"}'::jsonb, true, NOW(), NOW()),
    ('e4000001-0001-0001-0001-000000000004', 'Network Engineer', 'Complete the entire Network Engineering pathway', 'award', 'pathway_completion', 'gold', 150, '{"pathway_id": "c4000001-0001-0001-0001-000000000001"}'::jsonb, true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Note: Down migrations are not supported by the current migration system
-- To rollback, manually delete records with IDs starting with:
-- - Achievements: e1*, e2*, e3*, e4*
-- - Lab templates: b1*, b2*, b3*, b4*, b5*
-- - Pathways: c1*, c2*, c3*, c4*
