-- Multi-Tenancy Schema
-- Adds organizations, teams, memberships, feature flags, and licenses
-- for supporting individual users, groups, and organizations

-- =============================================================================
-- ENUM TYPES
-- =============================================================================

CREATE TYPE edition_type AS ENUM ('community', 'professional', 'enterprise');
CREATE TYPE org_type AS ENUM ('standard', 'educational', 'enterprise');
CREATE TYPE org_role AS ENUM ('owner', 'admin', 'instructor', 'member');
CREATE TYPE team_role AS ENUM ('lead', 'member');
CREATE TYPE lab_visibility AS ENUM ('global', 'organization', 'private');
CREATE TYPE license_status AS ENUM ('pending', 'valid', 'expired', 'revoked');

-- =============================================================================
-- ORGANIZATIONS
-- =============================================================================

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(128) NOT NULL UNIQUE,
    type org_type NOT NULL DEFAULT 'standard',
    edition edition_type NOT NULL DEFAULT 'community',

    -- License info (denormalized for quick access)
    license_key VARCHAR(512),
    license_expires_at TIMESTAMPTZ,

    -- Configuration
    settings JSONB NOT NULL DEFAULT '{
        "allowPublicSignup": false,
        "requireApproval": true,
        "defaultUserRole": "member",
        "allowedDomains": [],
        "enableTeams": true,
        "enableAchievements": true,
        "enableLeaderboards": false,
        "maxSessionDurationMins": 480,
        "retentionDays": 90
    }'::jsonb,

    -- Limits (NULL = unlimited)
    max_users INT,
    max_concurrent_pods INT,
    max_storage_gb INT,

    -- Branding
    logo_url VARCHAR(512),
    contact_email VARCHAR(255),

    -- Status
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_organizations_slug ON organizations(slug);
CREATE INDEX idx_organizations_edition ON organizations(edition);
CREATE INDEX idx_organizations_type ON organizations(type);
CREATE INDEX idx_organizations_active ON organizations(is_active) WHERE is_active = true;

-- Trigger for updated_at
CREATE TRIGGER trigger_organizations_updated
BEFORE UPDATE ON organizations
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- =============================================================================
-- TEAMS (Optional Hierarchy within Organizations)
-- =============================================================================

CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(128) NOT NULL,
    description TEXT,

    -- Nested teams support
    parent_team_id UUID REFERENCES teams(id) ON DELETE CASCADE,

    -- Canvas integration
    canvas_section_id VARCHAR(128),

    -- Settings
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Status
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(organization_id, slug)
);

CREATE INDEX idx_teams_org ON teams(organization_id);
CREATE INDEX idx_teams_parent ON teams(parent_team_id) WHERE parent_team_id IS NOT NULL;
CREATE INDEX idx_teams_canvas ON teams(canvas_section_id) WHERE canvas_section_id IS NOT NULL;
CREATE INDEX idx_teams_active ON teams(is_active) WHERE is_active = true;

CREATE TRIGGER trigger_teams_updated
BEFORE UPDATE ON teams
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- =============================================================================
-- ORGANIZATION MEMBERSHIPS
-- =============================================================================

CREATE TABLE organization_memberships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    role org_role NOT NULL DEFAULT 'member',

    -- Primary organization for user
    is_primary BOOLEAN NOT NULL DEFAULT false,

    -- Invitation tracking
    invited_by UUID REFERENCES users(id),
    invitation_token VARCHAR(128),
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,

    -- Status
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(organization_id, user_id)
);

CREATE INDEX idx_org_memberships_org ON organization_memberships(organization_id);
CREATE INDEX idx_org_memberships_user ON organization_memberships(user_id);
CREATE INDEX idx_org_memberships_role ON organization_memberships(role);
CREATE INDEX idx_org_memberships_primary ON organization_memberships(user_id, is_primary)
    WHERE is_primary = true;
CREATE INDEX idx_org_memberships_pending ON organization_memberships(invitation_token)
    WHERE accepted_at IS NULL AND invitation_token IS NOT NULL;

CREATE TRIGGER trigger_org_memberships_updated
BEFORE UPDATE ON organization_memberships
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- =============================================================================
-- TEAM MEMBERSHIPS
-- =============================================================================

CREATE TABLE team_memberships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    role team_role NOT NULL DEFAULT 'member',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(team_id, user_id)
);

CREATE INDEX idx_team_memberships_team ON team_memberships(team_id);
CREATE INDEX idx_team_memberships_user ON team_memberships(user_id);
CREATE INDEX idx_team_memberships_role ON team_memberships(role);

-- =============================================================================
-- FEATURE FLAGS
-- =============================================================================

-- System-defined features (immutable)
CREATE TABLE feature_flags (
    id VARCHAR(128) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,

    -- Which editions have this feature by default
    editions edition_type[] NOT NULL DEFAULT ARRAY['enterprise']::edition_type[],

    -- Global features available to all regardless of edition
    is_global BOOLEAN NOT NULL DEFAULT false,

    -- Default settings for this feature
    default_settings JSONB DEFAULT '{}'::jsonb,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Organization-specific feature overrides
CREATE TABLE organization_features (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    feature_id VARCHAR(128) NOT NULL REFERENCES feature_flags(id) ON DELETE CASCADE,

    enabled BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ,

    -- Override settings
    settings JSONB DEFAULT '{}'::jsonb,

    granted_by UUID REFERENCES users(id),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY(organization_id, feature_id)
);

CREATE INDEX idx_org_features_org ON organization_features(organization_id);
CREATE INDEX idx_org_features_expires ON organization_features(expires_at)
    WHERE expires_at IS NOT NULL;

-- =============================================================================
-- LICENSES
-- =============================================================================

CREATE TABLE licenses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    license_key VARCHAR(512) NOT NULL UNIQUE,
    edition edition_type NOT NULL,

    -- Validity
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,

    -- Limits encoded in license
    max_users INT,
    max_pods INT,
    max_storage_gb INT,

    -- Additional licensed features
    features TEXT[],

    -- Validation status
    is_active BOOLEAN NOT NULL DEFAULT true,
    validation_status license_status NOT NULL DEFAULT 'pending',
    last_validated_at TIMESTAMPTZ,
    validation_error TEXT,

    -- Tracking
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_licenses_org ON licenses(organization_id);
CREATE INDEX idx_licenses_key ON licenses(license_key);
CREATE INDEX idx_licenses_status ON licenses(validation_status);
CREATE INDEX idx_licenses_expires ON licenses(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_licenses_active ON licenses(is_active) WHERE is_active = true;

CREATE TRIGGER trigger_licenses_updated
BEFORE UPDATE ON licenses
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- =============================================================================
-- MODIFICATIONS TO EXISTING TABLES
-- =============================================================================

-- Users: add default organization
ALTER TABLE users
    ADD COLUMN default_organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;

CREATE INDEX idx_users_default_org ON users(default_organization_id)
    WHERE default_organization_id IS NOT NULL;

-- Lab Templates: add organization scope and visibility
ALTER TABLE lab_templates
    ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    ADD COLUMN visibility lab_visibility NOT NULL DEFAULT 'global',
    ADD COLUMN created_by UUID REFERENCES users(id);

CREATE INDEX idx_lab_templates_org ON lab_templates(organization_id)
    WHERE organization_id IS NOT NULL;
CREATE INDEX idx_lab_templates_visibility ON lab_templates(visibility);
CREATE INDEX idx_lab_templates_created_by ON lab_templates(created_by)
    WHERE created_by IS NOT NULL;

-- Pods: add organization and team context
ALTER TABLE pods
    ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    ADD COLUMN team_id UUID REFERENCES teams(id) ON DELETE SET NULL;

CREATE INDEX idx_pods_org ON pods(organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX idx_pods_team ON pods(team_id) WHERE team_id IS NOT NULL;

-- Lab Sessions: add organization and team context
ALTER TABLE lab_sessions
    ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    ADD COLUMN team_id UUID REFERENCES teams(id) ON DELETE SET NULL;

CREATE INDEX idx_sessions_org ON lab_sessions(organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX idx_sessions_team ON lab_sessions(team_id) WHERE team_id IS NOT NULL;

-- Audit Log: add organization context
ALTER TABLE audit_log
    ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;

CREATE INDEX idx_audit_log_org ON audit_log(organization_id) WHERE organization_id IS NOT NULL;

-- =============================================================================
-- SEED DATA: Default Organization
-- =============================================================================

-- Create default "system" organization for existing data
INSERT INTO organizations (id, name, slug, type, edition, is_active, settings)
VALUES (
    '00000000-0000-0000-0000-000000000000',
    'Default',
    'default',
    'standard',
    'community',
    true,
    '{
        "allowPublicSignup": true,
        "requireApproval": false,
        "defaultUserRole": "member",
        "allowedDomains": [],
        "enableTeams": true,
        "enableAchievements": true,
        "enableLeaderboards": true,
        "maxSessionDurationMins": 480,
        "retentionDays": 90
    }'::jsonb
);

-- Add existing users to default organization
INSERT INTO organization_memberships (organization_id, user_id, role, is_primary, accepted_at)
SELECT
    '00000000-0000-0000-0000-000000000000',
    id,
    CASE role
        WHEN 'admin' THEN 'admin'::org_role
        WHEN 'instructor' THEN 'instructor'::org_role
        ELSE 'member'::org_role
    END,
    true,
    NOW()
FROM users
WHERE id != '00000000-0000-0000-0000-000000000000';  -- Exclude system user

-- Set default organization for existing users
UPDATE users
SET default_organization_id = '00000000-0000-0000-0000-000000000000'
WHERE id != '00000000-0000-0000-0000-000000000000';

-- =============================================================================
-- SEED DATA: Feature Flags
-- =============================================================================

INSERT INTO feature_flags (id, name, description, editions, is_global) VALUES
-- Core features (all editions)
('labs.basic', 'Basic Lab Access', 'Access to run labs and complete checkpoints', ARRAY['community', 'professional', 'enterprise']::edition_type[], true),
('achievements.basic', 'Basic Achievements', 'Earn achievements for completing labs', ARRAY['community', 'professional', 'enterprise']::edition_type[], true),
('api.readonly', 'Read-only API Access', 'Access to read-only API endpoints', ARRAY['community', 'professional', 'enterprise']::edition_type[], true),

-- Professional features
('teams', 'Team Management', 'Create and manage teams within organization', ARRAY['professional', 'enterprise']::edition_type[], false),
('labs.custom', 'Custom Lab Templates', 'Create organization-specific lab templates', ARRAY['professional', 'enterprise']::edition_type[], false),
('achievements.full', 'Full Achievement System', 'Access to all achievement types and leaderboards', ARRAY['professional', 'enterprise']::edition_type[], false),
('api.full', 'Full API Access', 'Access to all API endpoints including write operations', ARRAY['professional', 'enterprise']::edition_type[], false),
('analytics.standard', 'Standard Analytics', 'Progress tracking and basic reporting', ARRAY['professional', 'enterprise']::edition_type[], false),
('canvas.full', 'Full Canvas LTI', 'Complete Canvas LMS integration with grade sync', ARRAY['professional', 'enterprise']::edition_type[], false),
('audit.extended', 'Extended Audit Logs', '90-day audit log retention', ARRAY['professional', 'enterprise']::edition_type[], false),

-- Enterprise features
('sso.saml', 'SAML SSO', 'Single Sign-On via SAML 2.0', ARRAY['enterprise']::edition_type[], false),
('sso.oidc', 'OIDC SSO', 'Single Sign-On via OpenID Connect', ARRAY['enterprise']::edition_type[], false),
('sso.ldap', 'LDAP Integration', 'Direct LDAP/Active Directory integration', ARRAY['enterprise']::edition_type[], false),
('rbac.custom', 'Custom Roles', 'Create custom roles with specific permissions', ARRAY['enterprise']::edition_type[], false),
('rbac.abac', 'Attribute-Based Access', 'Fine-grained resource-level permissions', ARRAY['enterprise']::edition_type[], false),
('branding.full', 'Full White-Label', 'Complete custom branding and theming', ARRAY['enterprise']::edition_type[], false),
('analytics.advanced', 'Advanced Analytics', 'AI-driven insights and custom reports', ARRAY['enterprise']::edition_type[], false),
('audit.compliance', 'Compliance Audit', '1-year audit log retention with export', ARRAY['enterprise']::edition_type[], false),
('api.webhooks', 'Webhook Integrations', 'Custom webhook notifications', ARRAY['enterprise']::edition_type[], false),
('multi_org', 'Multiple Organizations', 'Manage multiple organizations', ARRAY['enterprise']::edition_type[], false);

-- =============================================================================
-- HELPER FUNCTIONS
-- =============================================================================

-- Check if a user is a member of an organization
CREATE OR REPLACE FUNCTION is_org_member(p_user_id UUID, p_org_id UUID)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1 FROM organization_memberships
        WHERE user_id = p_user_id
        AND organization_id = p_org_id
        AND accepted_at IS NOT NULL
    );
END;
$$ LANGUAGE plpgsql STABLE;

-- Check if a user has a specific role or higher in an organization
CREATE OR REPLACE FUNCTION has_org_role(p_user_id UUID, p_org_id UUID, p_min_role org_role)
RETURNS BOOLEAN AS $$
DECLARE
    v_user_role org_role;
    v_role_priority INT;
    v_min_priority INT;
BEGIN
    -- Get user's role
    SELECT role INTO v_user_role
    FROM organization_memberships
    WHERE user_id = p_user_id
    AND organization_id = p_org_id
    AND accepted_at IS NOT NULL;

    IF v_user_role IS NULL THEN
        RETURN false;
    END IF;

    -- Define role priority (higher = more permissions)
    v_role_priority := CASE v_user_role
        WHEN 'owner' THEN 100
        WHEN 'admin' THEN 80
        WHEN 'instructor' THEN 50
        WHEN 'member' THEN 10
        ELSE 0
    END;

    v_min_priority := CASE p_min_role
        WHEN 'owner' THEN 100
        WHEN 'admin' THEN 80
        WHEN 'instructor' THEN 50
        WHEN 'member' THEN 10
        ELSE 0
    END;

    RETURN v_role_priority >= v_min_priority;
END;
$$ LANGUAGE plpgsql STABLE;

-- Check if a feature is enabled for an organization
CREATE OR REPLACE FUNCTION is_feature_enabled(p_org_id UUID, p_feature_id VARCHAR)
RETURNS BOOLEAN AS $$
DECLARE
    v_edition edition_type;
    v_feature_editions edition_type[];
    v_is_global BOOLEAN;
    v_override_enabled BOOLEAN;
    v_override_expires TIMESTAMPTZ;
BEGIN
    -- Get organization's edition
    SELECT edition INTO v_edition
    FROM organizations
    WHERE id = p_org_id AND is_active = true;

    IF v_edition IS NULL THEN
        RETURN false;
    END IF;

    -- Get feature info
    SELECT editions, is_global INTO v_feature_editions, v_is_global
    FROM feature_flags
    WHERE id = p_feature_id;

    IF v_feature_editions IS NULL THEN
        RETURN false;  -- Feature doesn't exist
    END IF;

    -- Check for organization-specific override
    SELECT enabled, expires_at INTO v_override_enabled, v_override_expires
    FROM organization_features
    WHERE organization_id = p_org_id AND feature_id = p_feature_id;

    IF v_override_enabled IS NOT NULL THEN
        -- Override exists, check if expired
        IF v_override_expires IS NOT NULL AND v_override_expires < NOW() THEN
            RETURN false;  -- Override expired
        END IF;
        RETURN v_override_enabled;
    END IF;

    -- No override, check if global or in edition's features
    IF v_is_global THEN
        RETURN true;
    END IF;

    RETURN v_edition = ANY(v_feature_editions);
END;
$$ LANGUAGE plpgsql STABLE;

-- Get user's effective organization count limit
CREATE OR REPLACE FUNCTION get_org_user_limit(p_org_id UUID)
RETURNS INT AS $$
DECLARE
    v_org_limit INT;
    v_license_limit INT;
    v_edition edition_type;
BEGIN
    -- Get organization's settings and edition
    SELECT max_users, edition INTO v_org_limit, v_edition
    FROM organizations
    WHERE id = p_org_id;

    -- Get active license limit
    SELECT max_users INTO v_license_limit
    FROM licenses
    WHERE organization_id = p_org_id
    AND is_active = true
    AND validation_status = 'valid'
    AND (expires_at IS NULL OR expires_at > NOW())
    ORDER BY expires_at DESC NULLS FIRST
    LIMIT 1;

    -- Use license limit if available, otherwise org limit
    IF v_license_limit IS NOT NULL THEN
        RETURN v_license_limit;
    END IF;

    -- Default limits by edition if no explicit limit set
    IF v_org_limit IS NULL THEN
        RETURN CASE v_edition
            WHEN 'community' THEN 50
            WHEN 'professional' THEN 250
            WHEN 'enterprise' THEN NULL  -- Unlimited
        END;
    END IF;

    RETURN v_org_limit;
END;
$$ LANGUAGE plpgsql STABLE;
