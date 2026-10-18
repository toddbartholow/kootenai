-- RBAC (Role-Based Access Control) Schema
-- Provides granular permissions beyond organization roles

-- =============================================================================
-- PERMISSIONS TABLE
-- Atomic permission definitions (e.g., "pods.create", "labs.delete")
-- =============================================================================

CREATE TABLE permissions (
    id VARCHAR(128) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    resource VARCHAR(64) NOT NULL,  -- Resource type: pods, labs, sessions, teams, etc.
    action VARCHAR(64) NOT NULL,    -- Action: create, read, update, delete, list, etc.
    is_system BOOLEAN NOT NULL DEFAULT true,  -- System permissions can't be deleted
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_permissions_resource ON permissions(resource);
CREATE INDEX idx_permissions_action ON permissions(action);

-- =============================================================================
-- PERMISSION SETS TABLE
-- Reusable groups of permissions (e.g., "lab_manager", "pod_viewer")
-- =============================================================================

CREATE TABLE permission_sets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(128) NOT NULL UNIQUE,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trigger_permission_sets_updated
BEFORE UPDATE ON permission_sets
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Junction table for permission set -> permissions
CREATE TABLE permission_set_permissions (
    permission_set_id UUID NOT NULL REFERENCES permission_sets(id) ON DELETE CASCADE,
    permission_id VARCHAR(128) NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (permission_set_id, permission_id)
);

CREATE INDEX idx_psp_permission_set ON permission_set_permissions(permission_set_id);
CREATE INDEX idx_psp_permission ON permission_set_permissions(permission_id);

-- =============================================================================
-- ROLES TABLE
-- Named collections of permissions and permission sets
-- =============================================================================

CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(128) NOT NULL,
    slug VARCHAR(128) NOT NULL UNIQUE,
    description TEXT,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,  -- NULL = system role
    is_system BOOLEAN NOT NULL DEFAULT false,  -- System roles can't be deleted
    is_default BOOLEAN NOT NULL DEFAULT false, -- Default role for new users
    priority INT NOT NULL DEFAULT 0,           -- Higher = more privileged
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_roles_org ON roles(organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX idx_roles_system ON roles(is_system) WHERE is_system = true;
CREATE INDEX idx_roles_default ON roles(is_default) WHERE is_default = true;
CREATE INDEX idx_roles_priority ON roles(priority DESC);

CREATE TRIGGER trigger_roles_updated
BEFORE UPDATE ON roles
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Junction table for role -> permission sets
CREATE TABLE role_permission_sets (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_set_id UUID NOT NULL REFERENCES permission_sets(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_set_id)
);

CREATE INDEX idx_rps_role ON role_permission_sets(role_id);
CREATE INDEX idx_rps_permission_set ON role_permission_sets(permission_set_id);

-- Junction table for role -> direct permissions (override/additions)
CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id VARCHAR(128) NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    granted BOOLEAN NOT NULL DEFAULT true,  -- false = explicitly denied
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX idx_rp_role ON role_permissions(role_id);
CREATE INDEX idx_rp_permission ON role_permissions(permission_id);

-- =============================================================================
-- USER ROLES TABLE
-- Role assignments to users (scoped by organization)
-- =============================================================================

CREATE TABLE user_roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,  -- NULL = global assignment
    granted_by UUID REFERENCES users(id),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,  -- Optional expiration

    UNIQUE(user_id, role_id, organization_id)
);

CREATE INDEX idx_user_roles_user ON user_roles(user_id);
CREATE INDEX idx_user_roles_role ON user_roles(role_id);
CREATE INDEX idx_user_roles_org ON user_roles(organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX idx_user_roles_expires ON user_roles(expires_at) WHERE expires_at IS NOT NULL;

-- =============================================================================
-- SEED DATA: System Permissions
-- =============================================================================

INSERT INTO permissions (id, name, description, resource, action) VALUES
-- Lab template permissions
('labs.list', 'List Labs', 'View list of lab templates', 'labs', 'list'),
('labs.read', 'View Lab Details', 'View individual lab template details', 'labs', 'read'),
('labs.create', 'Create Labs', 'Create new lab templates', 'labs', 'create'),
('labs.update', 'Update Labs', 'Modify existing lab templates', 'labs', 'update'),
('labs.delete', 'Delete Labs', 'Remove lab templates', 'labs', 'delete'),
('labs.publish', 'Publish Labs', 'Publish lab templates for use', 'labs', 'publish'),

-- Pod permissions
('pods.list', 'List Own Pods', 'View list of own pods', 'pods', 'list'),
('pods.list.all', 'List All Pods', 'View all pods in organization', 'pods', 'list.all'),
('pods.read', 'View Pod Details', 'View individual pod details', 'pods', 'read'),
('pods.create', 'Create Pods', 'Create new pods from templates', 'pods', 'create'),
('pods.delete', 'Delete Pods', 'Destroy pods', 'pods', 'delete'),
('pods.start', 'Start Pods', 'Start stopped pods', 'pods', 'start'),
('pods.stop', 'Stop Pods', 'Stop running pods', 'pods', 'stop'),
('pods.console', 'Access Console', 'Access VM console', 'pods', 'console'),
('pods.snapshot', 'Manage Snapshots', 'Create and revert snapshots', 'pods', 'snapshot'),

-- Session permissions
('sessions.list', 'List Own Sessions', 'View list of own sessions', 'sessions', 'list'),
('sessions.list.all', 'List All Sessions', 'View all sessions in organization', 'sessions', 'list.all'),
('sessions.read', 'View Session Details', 'View individual session details', 'sessions', 'read'),
('sessions.create', 'Create Sessions', 'Start new lab sessions', 'sessions', 'create'),
('sessions.grade', 'Grade Sessions', 'Submit grades for sessions', 'sessions', 'grade'),
('sessions.end', 'End Sessions', 'End active sessions', 'sessions', 'end'),

-- Team permissions
('teams.list', 'List Teams', 'View list of teams', 'teams', 'list'),
('teams.read', 'View Team Details', 'View team details and members', 'teams', 'read'),
('teams.create', 'Create Teams', 'Create new teams', 'teams', 'create'),
('teams.update', 'Update Teams', 'Modify team settings', 'teams', 'update'),
('teams.delete', 'Delete Teams', 'Remove teams', 'teams', 'delete'),
('teams.manage', 'Manage Members', 'Add/remove team members', 'teams', 'manage'),

-- Organization permissions
('org.read', 'View Organization', 'View organization details', 'org', 'read'),
('org.settings', 'Manage Settings', 'Modify organization settings', 'org', 'settings'),
('org.members.list', 'List Members', 'View organization members', 'org', 'members.list'),
('org.members.invite', 'Invite Members', 'Invite new members', 'org', 'members.invite'),
('org.members.remove', 'Remove Members', 'Remove organization members', 'org', 'members.remove'),
('org.members.role', 'Assign Roles', 'Change member roles', 'org', 'members.role'),

-- Pathway permissions
('pathways.list', 'List Pathways', 'View list of pathways', 'pathways', 'list'),
('pathways.read', 'View Pathway Details', 'View pathway details', 'pathways', 'read'),
('pathways.create', 'Create Pathways', 'Create new pathways', 'pathways', 'create'),
('pathways.update', 'Update Pathways', 'Modify pathways', 'pathways', 'update'),
('pathways.delete', 'Delete Pathways', 'Remove pathways', 'pathways', 'delete'),
('pathways.enroll', 'Enroll Users', 'Enroll users in pathways', 'pathways', 'enroll'),

-- RBAC permissions (enterprise)
('rbac.roles.list', 'List Roles', 'View available roles', 'rbac', 'roles.list'),
('rbac.roles.read', 'View Role Details', 'View role permissions', 'rbac', 'roles.read'),
('rbac.roles.create', 'Create Roles', 'Create custom roles', 'rbac', 'roles.create'),
('rbac.roles.update', 'Update Roles', 'Modify role permissions', 'rbac', 'roles.update'),
('rbac.roles.delete', 'Delete Roles', 'Remove custom roles', 'rbac', 'roles.delete'),
('rbac.roles.assign', 'Assign Roles', 'Assign roles to users', 'rbac', 'roles.assign'),
('rbac.permissions.read', 'View Permissions', 'View available permissions', 'rbac', 'permissions.read'),

-- Analytics permissions
('analytics.read', 'View Analytics', 'View analytics dashboards', 'analytics', 'read'),
('analytics.export', 'Export Analytics', 'Export analytics data', 'analytics', 'export'),

-- Audit permissions
('audit.read', 'View Audit Log', 'View audit log entries', 'audit', 'read'),
('audit.export', 'Export Audit Log', 'Export audit log data', 'audit', 'export'),

-- Admin permissions
('admin.users', 'Manage Users', 'Full user management', 'admin', 'users'),
('admin.system', 'System Administration', 'System-level administration', 'admin', 'system');

-- =============================================================================
-- SEED DATA: Permission Sets
-- =============================================================================

INSERT INTO permission_sets (id, name, description, is_system) VALUES
('a0000000-0000-0000-0000-000000000001', 'lab_viewer', 'View lab templates', true),
('a0000000-0000-0000-0000-000000000002', 'lab_manager', 'Full lab template management', true),
('a0000000-0000-0000-0000-000000000003', 'pod_user', 'Basic pod operations', true),
('a0000000-0000-0000-0000-000000000004', 'pod_manager', 'Full pod management', true),
('a0000000-0000-0000-0000-000000000005', 'session_user', 'Basic session operations', true),
('a0000000-0000-0000-0000-000000000006', 'session_manager', 'Full session management', true),
('a0000000-0000-0000-0000-000000000007', 'team_manager', 'Team management', true),
('a0000000-0000-0000-0000-000000000008', 'org_admin', 'Organization administration', true),
('a0000000-0000-0000-0000-000000000009', 'pathway_viewer', 'View pathways', true),
('a0000000-0000-0000-0000-00000000000a', 'pathway_manager', 'Full pathway management', true);

-- Lab viewer permissions
INSERT INTO permission_set_permissions (permission_set_id, permission_id) VALUES
('a0000000-0000-0000-0000-000000000001', 'labs.list'),
('a0000000-0000-0000-0000-000000000001', 'labs.read');

-- Lab manager permissions
INSERT INTO permission_set_permissions (permission_set_id, permission_id) VALUES
('a0000000-0000-0000-0000-000000000002', 'labs.list'),
('a0000000-0000-0000-0000-000000000002', 'labs.read'),
('a0000000-0000-0000-0000-000000000002', 'labs.create'),
('a0000000-0000-0000-0000-000000000002', 'labs.update'),
('a0000000-0000-0000-0000-000000000002', 'labs.delete'),
('a0000000-0000-0000-0000-000000000002', 'labs.publish');

-- Pod user permissions
INSERT INTO permission_set_permissions (permission_set_id, permission_id) VALUES
('a0000000-0000-0000-0000-000000000003', 'pods.list'),
('a0000000-0000-0000-0000-000000000003', 'pods.read'),
('a0000000-0000-0000-0000-000000000003', 'pods.create'),
('a0000000-0000-0000-0000-000000000003', 'pods.start'),
('a0000000-0000-0000-0000-000000000003', 'pods.stop'),
('a0000000-0000-0000-0000-000000000003', 'pods.console');

-- Pod manager permissions
INSERT INTO permission_set_permissions (permission_set_id, permission_id) VALUES
('a0000000-0000-0000-0000-000000000004', 'pods.list'),
('a0000000-0000-0000-0000-000000000004', 'pods.list.all'),
('a0000000-0000-0000-0000-000000000004', 'pods.read'),
('a0000000-0000-0000-0000-000000000004', 'pods.create'),
('a0000000-0000-0000-0000-000000000004', 'pods.delete'),
('a0000000-0000-0000-0000-000000000004', 'pods.start'),
('a0000000-0000-0000-0000-000000000004', 'pods.stop'),
('a0000000-0000-0000-0000-000000000004', 'pods.console'),
('a0000000-0000-0000-0000-000000000004', 'pods.snapshot');

-- Session user permissions
INSERT INTO permission_set_permissions (permission_set_id, permission_id) VALUES
('a0000000-0000-0000-0000-000000000005', 'sessions.list'),
('a0000000-0000-0000-0000-000000000005', 'sessions.read'),
('a0000000-0000-0000-0000-000000000005', 'sessions.create');

-- Session manager permissions
INSERT INTO permission_set_permissions (permission_set_id, permission_id) VALUES
('a0000000-0000-0000-0000-000000000006', 'sessions.list'),
('a0000000-0000-0000-0000-000000000006', 'sessions.list.all'),
('a0000000-0000-0000-0000-000000000006', 'sessions.read'),
('a0000000-0000-0000-0000-000000000006', 'sessions.create'),
('a0000000-0000-0000-0000-000000000006', 'sessions.grade'),
('a0000000-0000-0000-0000-000000000006', 'sessions.end');

-- Team manager permissions
INSERT INTO permission_set_permissions (permission_set_id, permission_id) VALUES
('a0000000-0000-0000-0000-000000000007', 'teams.list'),
('a0000000-0000-0000-0000-000000000007', 'teams.read'),
('a0000000-0000-0000-0000-000000000007', 'teams.create'),
('a0000000-0000-0000-0000-000000000007', 'teams.update'),
('a0000000-0000-0000-0000-000000000007', 'teams.delete'),
('a0000000-0000-0000-0000-000000000007', 'teams.manage');

-- Org admin permissions
INSERT INTO permission_set_permissions (permission_set_id, permission_id) VALUES
('a0000000-0000-0000-0000-000000000008', 'org.read'),
('a0000000-0000-0000-0000-000000000008', 'org.settings'),
('a0000000-0000-0000-0000-000000000008', 'org.members.list'),
('a0000000-0000-0000-0000-000000000008', 'org.members.invite'),
('a0000000-0000-0000-0000-000000000008', 'org.members.remove'),
('a0000000-0000-0000-0000-000000000008', 'org.members.role'),
('a0000000-0000-0000-0000-000000000008', 'analytics.read'),
('a0000000-0000-0000-0000-000000000008', 'audit.read');

-- Pathway viewer permissions
INSERT INTO permission_set_permissions (permission_set_id, permission_id) VALUES
('a0000000-0000-0000-0000-000000000009', 'pathways.list'),
('a0000000-0000-0000-0000-000000000009', 'pathways.read');

-- Pathway manager permissions
INSERT INTO permission_set_permissions (permission_set_id, permission_id) VALUES
('a0000000-0000-0000-0000-00000000000a', 'pathways.list'),
('a0000000-0000-0000-0000-00000000000a', 'pathways.read'),
('a0000000-0000-0000-0000-00000000000a', 'pathways.create'),
('a0000000-0000-0000-0000-00000000000a', 'pathways.update'),
('a0000000-0000-0000-0000-00000000000a', 'pathways.delete'),
('a0000000-0000-0000-0000-00000000000a', 'pathways.enroll');

-- =============================================================================
-- SEED DATA: System Roles
-- =============================================================================

INSERT INTO roles (id, name, slug, description, is_system, is_default, priority) VALUES
('b0000000-0000-0000-0000-000000000001', 'Student', 'student', 'Default student role with basic access', true, true, 10),
('b0000000-0000-0000-0000-000000000002', 'Instructor', 'instructor', 'Instructor role with teaching capabilities', true, false, 50),
('b0000000-0000-0000-0000-000000000003', 'Organization Admin', 'org-admin', 'Full organization administration', true, false, 80),
('b0000000-0000-0000-0000-000000000004', 'System Admin', 'system-admin', 'System-wide administration', true, false, 100);

-- Student role permission sets
INSERT INTO role_permission_sets (role_id, permission_set_id) VALUES
('b0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001'), -- lab_viewer
('b0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000003'), -- pod_user
('b0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000005'), -- session_user
('b0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000009'); -- pathway_viewer

-- Instructor role permission sets
INSERT INTO role_permission_sets (role_id, permission_set_id) VALUES
('b0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000002'), -- lab_manager
('b0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000004'), -- pod_manager
('b0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000006'), -- session_manager
('b0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000007'), -- team_manager
('b0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-00000000000a'); -- pathway_manager

-- Org admin role permission sets
INSERT INTO role_permission_sets (role_id, permission_set_id) VALUES
('b0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000002'), -- lab_manager
('b0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000004'), -- pod_manager
('b0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000006'), -- session_manager
('b0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000007'), -- team_manager
('b0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-000000000008'), -- org_admin
('b0000000-0000-0000-0000-000000000003', 'a0000000-0000-0000-0000-00000000000a'); -- pathway_manager

-- Org admin direct permissions (RBAC management)
INSERT INTO role_permissions (role_id, permission_id, granted) VALUES
('b0000000-0000-0000-0000-000000000003', 'rbac.roles.list', true),
('b0000000-0000-0000-0000-000000000003', 'rbac.roles.read', true),
('b0000000-0000-0000-0000-000000000003', 'rbac.roles.assign', true),
('b0000000-0000-0000-0000-000000000003', 'rbac.permissions.read', true);

-- System admin gets all permissions
INSERT INTO role_permission_sets (role_id, permission_set_id) VALUES
('b0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000002'),
('b0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000004'),
('b0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000006'),
('b0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000007'),
('b0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-000000000008'),
('b0000000-0000-0000-0000-000000000004', 'a0000000-0000-0000-0000-00000000000a');

INSERT INTO role_permissions (role_id, permission_id, granted) VALUES
('b0000000-0000-0000-0000-000000000004', 'rbac.roles.list', true),
('b0000000-0000-0000-0000-000000000004', 'rbac.roles.read', true),
('b0000000-0000-0000-0000-000000000004', 'rbac.roles.create', true),
('b0000000-0000-0000-0000-000000000004', 'rbac.roles.update', true),
('b0000000-0000-0000-0000-000000000004', 'rbac.roles.delete', true),
('b0000000-0000-0000-0000-000000000004', 'rbac.roles.assign', true),
('b0000000-0000-0000-0000-000000000004', 'rbac.permissions.read', true),
('b0000000-0000-0000-0000-000000000004', 'analytics.export', true),
('b0000000-0000-0000-0000-000000000004', 'audit.export', true),
('b0000000-0000-0000-0000-000000000004', 'admin.users', true),
('b0000000-0000-0000-0000-000000000004', 'admin.system', true);

-- =============================================================================
-- HELPER FUNCTION: Check if user has permission
-- =============================================================================

CREATE OR REPLACE FUNCTION has_permission(
    p_user_id UUID,
    p_permission_id VARCHAR,
    p_org_id UUID DEFAULT NULL
)
RETURNS BOOLEAN AS $$
DECLARE
    v_has_permission BOOLEAN := false;
BEGIN
    -- Check direct role permissions and permission sets
    SELECT EXISTS (
        SELECT 1
        FROM user_roles ur
        JOIN roles r ON ur.role_id = r.id
        LEFT JOIN role_permissions rp ON r.id = rp.role_id AND rp.permission_id = p_permission_id
        LEFT JOIN role_permission_sets rps ON r.id = rps.role_id
        LEFT JOIN permission_set_permissions psp ON rps.permission_set_id = psp.permission_set_id
        WHERE ur.user_id = p_user_id
        AND (ur.organization_id IS NULL OR ur.organization_id = p_org_id OR p_org_id IS NULL)
        AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
        AND (
            (rp.permission_id = p_permission_id AND rp.granted = true)
            OR psp.permission_id = p_permission_id
        )
        -- Ensure no explicit denial
        AND NOT EXISTS (
            SELECT 1 FROM role_permissions rp2
            WHERE rp2.role_id = r.id
            AND rp2.permission_id = p_permission_id
            AND rp2.granted = false
        )
    ) INTO v_has_permission;

    RETURN v_has_permission;
END;
$$ LANGUAGE plpgsql STABLE;

-- =============================================================================
-- HELPER FUNCTION: Get user's effective permissions
-- =============================================================================

CREATE OR REPLACE FUNCTION get_user_permissions(
    p_user_id UUID,
    p_org_id UUID DEFAULT NULL
)
RETURNS TABLE (permission_id VARCHAR) AS $$
BEGIN
    RETURN QUERY
    SELECT DISTINCT p.id
    FROM permissions p
    WHERE p.id IN (
        -- From permission sets via roles
        SELECT psp.permission_id
        FROM user_roles ur
        JOIN roles r ON ur.role_id = r.id
        JOIN role_permission_sets rps ON r.id = rps.role_id
        JOIN permission_set_permissions psp ON rps.permission_set_id = psp.permission_set_id
        WHERE ur.user_id = p_user_id
        AND (ur.organization_id IS NULL OR ur.organization_id = p_org_id OR p_org_id IS NULL)
        AND (ur.expires_at IS NULL OR ur.expires_at > NOW())

        UNION

        -- Direct role permissions (granted)
        SELECT rp.permission_id
        FROM user_roles ur
        JOIN roles r ON ur.role_id = r.id
        JOIN role_permissions rp ON r.id = rp.role_id
        WHERE ur.user_id = p_user_id
        AND (ur.organization_id IS NULL OR ur.organization_id = p_org_id OR p_org_id IS NULL)
        AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
        AND rp.granted = true
    )
    -- Exclude explicitly denied permissions
    AND p.id NOT IN (
        SELECT rp.permission_id
        FROM user_roles ur
        JOIN roles r ON ur.role_id = r.id
        JOIN role_permissions rp ON r.id = rp.role_id
        WHERE ur.user_id = p_user_id
        AND (ur.organization_id IS NULL OR ur.organization_id = p_org_id OR p_org_id IS NULL)
        AND (ur.expires_at IS NULL OR ur.expires_at > NOW())
        AND rp.granted = false
    );
END;
$$ LANGUAGE plpgsql STABLE;
