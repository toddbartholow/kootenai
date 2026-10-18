-- DevOps Fundamentals Pathway Seed Data
-- Creates the DevOps pathway with 6 modules covering CI/CD, Git, containers, and automation

-- =============================================================================
-- DEVOPS LAB TEMPLATES
-- =============================================================================

INSERT INTO lab_templates (id, name, slug, description, difficulty, platform, duration_minutes, max_points, tags, visibility, is_active, spec, created_at, updated_at)
VALUES
    -- Module 1: Git Version Control
    ('a3100001-0001-0001-0001-000000000001', 'Git Version Control', 'git-version-control',
     'Master Git version control from basics to advanced workflows. Learn commits, branches, merging, rebasing, and collaborative workflows including pull requests and code review.',
     'beginner', 'proxmox', 60, 100,
     ARRAY['devops', 'git', 'version-control', 'collaboration', 'beginner'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 2: CI/CD Fundamentals
    ('a3100001-0001-0001-0001-000000000002', 'CI/CD Pipeline Fundamentals', 'cicd-fundamentals',
     'Learn Continuous Integration and Continuous Deployment concepts. Build automated pipelines using GitHub Actions and understand testing, building, and deployment stages.',
     'intermediate', 'proxmox', 90, 120,
     ARRAY['devops', 'cicd', 'github-actions', 'automation', 'pipelines'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 3: Docker Mastery
    ('a3100001-0001-0001-0001-000000000003', 'Docker Mastery', 'docker-mastery',
     'Go beyond container basics. Learn multi-stage builds, Docker Compose for multi-container applications, networking, volumes, and production best practices.',
     'intermediate', 'proxmox', 90, 130,
     ARRAY['devops', 'docker', 'containers', 'compose', 'networking'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 4: Infrastructure as Code
    ('a3100001-0001-0001-0001-000000000004', 'Infrastructure as Code with Terraform', 'infrastructure-as-code',
     'Learn Infrastructure as Code principles using Terraform. Define, provision, and manage cloud infrastructure through declarative configuration files.',
     'intermediate', 'cloudstack', 120, 140,
     ARRAY['devops', 'terraform', 'iac', 'cloud', 'automation'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 5: Kubernetes Basics
    ('a3100001-0001-0001-0001-000000000005', 'Kubernetes Fundamentals', 'kubernetes-fundamentals',
     'Get started with Kubernetes container orchestration. Learn pods, deployments, services, and basic cluster management.',
     'intermediate', 'proxmox', 120, 150,
     ARRAY['devops', 'kubernetes', 'k8s', 'containers', 'orchestration'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 6: Monitoring and Observability
    ('a3100001-0001-0001-0001-000000000006', 'Monitoring and Observability', 'monitoring-observability',
     'Implement monitoring, logging, and observability for your applications. Learn Prometheus, Grafana, and log aggregation strategies.',
     'advanced', 'proxmox', 90, 140,
     ARRAY['devops', 'monitoring', 'prometheus', 'grafana', 'logging', 'observability'],
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
-- DEVOPS FUNDAMENTALS PATHWAY
-- =============================================================================

INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, status, is_active, is_featured, visibility, prerequisites, tags, icon, color, created_at, updated_at)
VALUES (
    'b2100001-0001-0001-0001-000000000001',
    'DevOps Fundamentals',
    'devops-fundamentals',
    'Master the DevOps mindset and toolchain. This pathway takes you from version control basics through CI/CD pipelines, containerization, infrastructure as code, and observability. Learn the tools and practices that enable teams to deliver software faster and more reliably.',
    'From Git to Kubernetes - master the tools and practices of modern DevOps.',
    'intermediate',
    10, -- Total estimated hours
    10, -- Display order
    'published',
    true,
    true,
    'global',
    '["Basic Linux command line skills", "Understanding of software development lifecycle"]'::jsonb,
    ARRAY['devops', 'cicd', 'containers', 'automation', 'cloud'],
    'pi pi-cog',
    '#f97316',
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
-- DEVOPS PATHWAY MODULES
-- =============================================================================

INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, icon, estimated_minutes, created_at)
VALUES
    ('c3100001-0001-0001-0001-000000000001', 'b2100001-0001-0001-0001-000000000001',
     'Git Version Control', 'git-version-control',
     'Master Git for version control and collaboration.',
     1, 'always', true, 'pi pi-github', 60, NOW()),

    ('c3100001-0001-0001-0001-000000000002', 'b2100001-0001-0001-0001-000000000001',
     'CI/CD Pipelines', 'cicd-pipelines',
     'Build automated pipelines for testing, building, and deploying.',
     2, 'sequential', true, 'pi pi-sync', 90, NOW()),

    ('c3100001-0001-0001-0001-000000000003', 'b2100001-0001-0001-0001-000000000001',
     'Docker Mastery', 'docker-mastery',
     'Advanced containerization with Docker and Docker Compose.',
     3, 'sequential', true, 'pi pi-box', 90, NOW()),

    ('c3100001-0001-0001-0001-000000000004', 'b2100001-0001-0001-0001-000000000001',
     'Infrastructure as Code', 'infrastructure-as-code',
     'Define and provision infrastructure with Terraform.',
     4, 'sequential', true, 'pi pi-code', 120, NOW()),

    ('c3100001-0001-0001-0001-000000000005', 'b2100001-0001-0001-0001-000000000001',
     'Kubernetes Basics', 'kubernetes-basics',
     'Container orchestration fundamentals with Kubernetes.',
     5, 'sequential', true, 'pi pi-server', 120, NOW()),

    ('c3100001-0001-0001-0001-000000000006', 'b2100001-0001-0001-0001-000000000001',
     'Monitoring & Observability', 'monitoring-observability',
     'Implement metrics, logging, and alerting.',
     6, 'sequential', true, 'pi pi-chart-line', 90, NOW())
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
    ('d4100001-0001-0001-0001-000000000001', 'c3100001-0001-0001-0001-000000000001', 'a3100001-0001-0001-0001-000000000001', 1, true),
    ('d4100001-0001-0001-0001-000000000002', 'c3100001-0001-0001-0001-000000000002', 'a3100001-0001-0001-0001-000000000002', 1, true),
    ('d4100001-0001-0001-0001-000000000003', 'c3100001-0001-0001-0001-000000000003', 'a3100001-0001-0001-0001-000000000003', 1, true),
    ('d4100001-0001-0001-0001-000000000004', 'c3100001-0001-0001-0001-000000000004', 'a3100001-0001-0001-0001-000000000004', 1, true),
    ('d4100001-0001-0001-0001-000000000005', 'c3100001-0001-0001-0001-000000000005', 'a3100001-0001-0001-0001-000000000005', 1, true),
    ('d4100001-0001-0001-0001-000000000006', 'c3100001-0001-0001-0001-000000000006', 'a3100001-0001-0001-0001-000000000006', 1, true)
ON CONFLICT (id) DO UPDATE SET
    display_order = EXCLUDED.display_order,
    is_required = EXCLUDED.is_required;

-- =============================================================================
-- DEVOPS ACHIEVEMENTS
-- =============================================================================

INSERT INTO achievements (id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria, created_at, updated_at)
VALUES
    ('devops-beginner', 'DevOps Beginner',
     'Complete the Git and CI/CD modules',
     'pathway', 'bronze', '/icons/git.svg', 100, false, true,
     '{"type": "pathway_modules", "pathway_slug": "devops-fundamentals", "modules_required": 2}'::jsonb,
     NOW(), NOW()),

    ('container-specialist', 'Container Specialist',
     'Complete the Docker Mastery module',
     'pathway', 'silver', '/icons/docker.svg', 150, false, true,
     '{"type": "pathway_modules", "pathway_slug": "devops-fundamentals", "modules_required": 3}'::jsonb,
     NOW(), NOW()),

    ('infrastructure-engineer', 'Infrastructure Engineer',
     'Complete IaC and Kubernetes modules',
     'pathway', 'gold', '/icons/terraform.svg', 250, false, true,
     '{"type": "pathway_modules", "pathway_slug": "devops-fundamentals", "modules_required": 5}'::jsonb,
     NOW(), NOW()),

    ('devops-master', 'DevOps Master',
     'Complete the entire DevOps Fundamentals pathway',
     'pathway', 'platinum', '/icons/trophy.svg', 500, false, true,
     '{"type": "pathway_complete", "pathway_slug": "devops-fundamentals"}'::jsonb,
     NOW(), NOW()),

    ('pipeline-perfectionist', 'Pipeline Perfectionist',
     'Score 100% on any DevOps pathway lab',
     'perfect_score', 'gold', '/icons/star.svg', 100, false, true,
     '{"type": "lab_perfect_score", "pathway_slug": "devops-fundamentals"}'::jsonb,
     NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    points = EXCLUDED.points,
    criteria = EXCLUDED.criteria,
    updated_at = NOW();
