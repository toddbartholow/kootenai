-- Migration: Add edition support to lab_templates
-- This allows labs to be restricted by license edition (community, professional, enterprise)

-- Add min_edition column to lab_templates
ALTER TABLE lab_templates
ADD COLUMN IF NOT EXISTS min_edition edition_type NOT NULL DEFAULT 'community';

-- Create index for edition filtering
CREATE INDEX IF NOT EXISTS idx_lab_templates_edition ON lab_templates(min_edition);

-- Set CE labs (Linux Fundamentals, Vim, basic Python, Git, SQL, basic networking)
-- These are foundational labs available in all editions

-- Linux Fundamentals Pathway labs - CE
UPDATE lab_templates SET min_edition = 'community' WHERE name IN (
    'Linux Foundations',
    'Shell Essentials',
    'File Mastery',
    'Text Processing',
    'Process & User Management',
    'Services & System Management',
    'Storage & Package Management',
    'Networking & Troubleshooting',
    'Simple Linux Introduction',
    'base-linux-lab',
    'Linux Firewall Basics',
    'Linux Routing Fundamentals',
    'Advanced Shell Scripting'
);

-- Vim Pathway labs - CE
UPDATE lab_templates SET min_edition = 'community' WHERE name IN (
    'Vim Fundamentals',
    'Essential Editing Commands',
    'Essential Editing',
    'Navigation Mastery',
    'Search and Replace',
    'Visual Mode & Selection',
    'Buffers, Windows & Tabs',
    'Configuration & Customization',
    'Advanced Vim Features'
);

-- Python basics - CE (not security/network programming)
UPDATE lab_templates SET min_edition = 'community' WHERE name IN (
    'Python Fundamentals',
    'Python File and System Operations',
    'Python for System Administration'
);

-- Git and SQL basics - CE
UPDATE lab_templates SET min_edition = 'community' WHERE name IN (
    'Git Version Control',
    'SQL Fundamentals'
);

-- Basic networking - CE
UPDATE lab_templates SET min_edition = 'community' WHERE name IN (
    'IP Routing Fundamentals',
    'Network Firewall Configuration'
);

-- Professional tier labs (security, containers, intermediate networking)
UPDATE lab_templates SET min_edition = 'professional' WHERE name IN (
    'Python Network Programming',
    'Python Data Analysis and Reporting',
    'Container Fundamentals with Docker',
    'Docker Mastery',
    'VLANs and Layer 2 Switching',
    'Network Traffic Analysis',
    'Linux System Administration',
    'MySQL Administration',
    'PostgreSQL Administration',
    'Database Design and Modeling',
    'CI/CD Pipeline Fundamentals',
    'Monitoring and Observability',
    'network-security-intro'
);

-- Enterprise tier labs (cloud, security, AD, pentesting, advanced)
UPDATE lab_templates SET min_edition = 'enterprise' WHERE name IN (
    -- AWS/Cloud
    'AWS Cloud Fundamentals',
    'AWS IAM and Security',
    'AWS VPC and Networking',
    'AWS Serverless with Lambda',
    'Cloud Security Fundamentals',
    'Cloud Data Protection',
    'Cloud Identity and Access Management',
    'Cloud Network Security',
    'Cloud Security Monitoring',
    'Infrastructure as Code with Terraform',
    'cloudstack-iaas-fundamentals',
    -- Windows/AD
    'Active Directory Fundamentals',
    'Active Directory Security Hardening',
    'Group Policy Management',
    'Windows DNS and DHCP Services',
    -- Security/Pentesting
    'Penetration Testing Basics',
    'Vulnerability Assessment',
    'Web Application Security',
    'Python for Security',
    'SOC Analyst Fundamentals',
    'Incident Response and Forensics',
    'blue-team-incident-response',
    'Database Security',
    -- Kubernetes
    'Kubernetes Fundamentals'
);

-- Also update pathways with edition requirements
ALTER TABLE pathways
ADD COLUMN IF NOT EXISTS min_edition edition_type NOT NULL DEFAULT 'community';

CREATE INDEX IF NOT EXISTS idx_pathways_edition ON pathways(min_edition);

-- CE pathways
UPDATE pathways SET min_edition = 'community' WHERE name IN (
    'Linux Fundamentals',
    'Vim Mastery',
    'Python Fundamentals'
);

-- Professional pathways
UPDATE pathways SET min_edition = 'professional' WHERE name IN (
    'DevOps Fundamentals',
    'Database Administration'
);

-- Enterprise pathways
UPDATE pathways SET min_edition = 'enterprise' WHERE name IN (
    'Cloud Security',
    'Network Security',
    'SOC Analyst',
    'Penetration Testing'
);

-- Add comment
COMMENT ON COLUMN lab_templates.min_edition IS 'Minimum license edition required to access this lab';
COMMENT ON COLUMN pathways.min_edition IS 'Minimum license edition required to access this pathway';
