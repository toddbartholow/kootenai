# Multi-Tenancy Implementation Plan for Kootenai Platform

## Overview

Implement comprehensive multi-tenancy to support individuals, groups/classrooms, and organizations - similar to HackTheBox/TryHackMe. The platform will have three editions:

- **Community Edition**: Free, self-hosted, open source
- **Professional Edition**: Paid, self-hosted (license key)
- **Enterprise Edition**: Paid, self-hosted or managed SaaS

## Key Design Decisions

1. **Hierarchy**: Flexible - Orgs can optionally contain Teams, or be flat (Org → Users directly)
2. **Lab Templates**: Global platform templates + org-private templates by org admins/instructors
3. **Deployment**: Both self-hosted (license key) and managed SaaS
4. **Auth**: Current JWT + future SAML/OIDC/LDAP (not immediate priority)

---

## Database Schema

### New Tables

#### 1. Organizations
```sql
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(128) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL DEFAULT 'standard',  -- standard, educational, enterprise
    edition VARCHAR(50) NOT NULL DEFAULT 'community',
    license_key VARCHAR(512),
    license_expires_at TIMESTAMPTZ,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    max_users INT,
    max_concurrent_pods INT,
    logo_url VARCHAR(512),
    contact_email VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### 2. Teams (Optional Hierarchy)
```sql
CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(128) NOT NULL,
    description TEXT,
    parent_team_id UUID REFERENCES teams(id),  -- Nested teams support
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, slug)
);
```

#### 3. Organization Memberships
```sql
CREATE TABLE organization_memberships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(64) NOT NULL DEFAULT 'member',  -- owner, admin, instructor, member
    is_primary BOOLEAN NOT NULL DEFAULT false,
    invited_by UUID REFERENCES users(id),
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    UNIQUE(organization_id, user_id)
);
```

#### 4. Team Memberships
```sql
CREATE TABLE team_memberships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(64) NOT NULL DEFAULT 'member',  -- lead, member
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(team_id, user_id)
);
```

#### 5. Feature Flags
```sql
CREATE TABLE feature_flags (
    id VARCHAR(128) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    editions TEXT[] NOT NULL DEFAULT ARRAY['enterprise'],
    is_global BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE organization_features (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    feature_id VARCHAR(128) NOT NULL REFERENCES feature_flags(id),
    enabled BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ,
    PRIMARY KEY(organization_id, feature_id)
);
```

#### 6. Licenses
```sql
CREATE TABLE licenses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    license_key VARCHAR(512) NOT NULL UNIQUE,
    edition VARCHAR(50) NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    max_users INT,
    features TEXT[],
    is_active BOOLEAN NOT NULL DEFAULT true,
    validation_status VARCHAR(50) DEFAULT 'pending'
);
```

### Modify Existing Tables

```sql
-- Users: add default org
ALTER TABLE users ADD COLUMN default_organization_id UUID REFERENCES organizations(id);

-- Lab Templates: add org scope and visibility
ALTER TABLE lab_templates
    ADD COLUMN organization_id UUID REFERENCES organizations(id),
    ADD COLUMN visibility VARCHAR(50) NOT NULL DEFAULT 'global',  -- global, organization, private
    ADD COLUMN created_by UUID REFERENCES users(id);

-- Pods: add org/team context
ALTER TABLE pods
    ADD COLUMN organization_id UUID REFERENCES organizations(id),
    ADD COLUMN team_id UUID REFERENCES teams(id);

-- Sessions: add org/team context
ALTER TABLE lab_sessions
    ADD COLUMN organization_id UUID REFERENCES organizations(id),
    ADD COLUMN team_id UUID REFERENCES teams(id);

-- Audit log: add org context
ALTER TABLE audit_log ADD COLUMN organization_id UUID REFERENCES organizations(id);
```

---

## Feature Matrix

| Feature | Community | Professional | Enterprise |
|---------|-----------|--------------|------------|
| Max Users | 50 | 250 | Unlimited |
| Max Concurrent Pods | 10 | 50 | Unlimited |
| Organizations | 1 | 1 | Unlimited |
| Teams | No | Yes (5) | Unlimited |
| Custom Lab Templates | No | 10 | Unlimited |
| Achievement System | Basic | Full | Full + Custom |
| Leaderboards | No | Org-scoped | Multi-org |
| Canvas LTI | Basic | Full | Full |
| SSO/SAML/OIDC | No | No | Yes |
| Audit Logging | 7 days | 90 days | 1 year |
| API Access | Read-only | Full | Full + Webhooks |
| Custom Branding | No | Logo | White-label |
| Support | Community | Email | Priority |

---

## API Endpoints

### Organization Management
- `POST /api/v1/organizations` - Create org
- `GET /api/v1/organizations` - List user's orgs
- `GET /api/v1/organizations/:orgID` - Get org details
- `PUT /api/v1/organizations/:orgID` - Update org (admin)
- `DELETE /api/v1/organizations/:orgID` - Delete org (owner)

### Membership
- `GET /api/v1/organizations/:orgID/members` - List members
- `POST /api/v1/organizations/:orgID/members` - Add/invite member
- `PUT /api/v1/organizations/:orgID/members/:userID` - Update role
- `DELETE /api/v1/organizations/:orgID/members/:userID` - Remove member
- `POST /api/v1/invitations/:token/accept` - Accept invite

### Teams
- `POST /api/v1/organizations/:orgID/teams` - Create team
- `GET /api/v1/organizations/:orgID/teams` - List teams
- `GET /api/v1/teams/:teamID` - Get team
- `PUT /api/v1/teams/:teamID` - Update team
- `DELETE /api/v1/teams/:teamID` - Delete team
- `POST /api/v1/teams/:teamID/members` - Add member
- `DELETE /api/v1/teams/:teamID/members/:userID` - Remove member

### Org-Scoped Labs
- `POST /api/v1/organizations/:orgID/labs` - Create org template
- `GET /api/v1/organizations/:orgID/labs` - List org templates

### License & Features
- `POST /api/v1/organizations/:orgID/license` - Activate license
- `GET /api/v1/organizations/:orgID/license` - Get license status
- `GET /api/v1/organizations/:orgID/features` - Get enabled features

---

## Middleware

### Tenant Context Middleware
Extracts org context from:
1. URL path param (`/organizations/:orgID/...`)
2. `X-Organization` header
3. Subdomain (SaaS: `org-slug.example.com`)
4. User's default organization

### Feature Gate Middleware
```go
RequireFeature("teams")  // Returns 402 if not available
RequireFeature("custom_labs")
```

### Org Role Middleware
```go
RequireOrgRole(OrgRoleAdmin)      // owner, admin
RequireOrgRole(OrgRoleInstructor) // owner, admin, instructor
RequireOrgRole(OrgRoleMember)     // any member
```

---

## Implementation Phases

### Phase 1: Database Foundation ✅ COMPLETE
- [x] Create migration `012_multi_tenancy.sql`
- [x] Add Organization model to `models/organization.go`
- [x] Create OrganizationRepository
- [x] Create MembershipRepository
- [x] Create default "system" organization for migration

**Files:**
- `api/internal/database/migrations/012_multi_tenancy.sql`
- `api/internal/models/organization.go`
- `api/internal/database/repositories/organization_repo.go`
- `api/internal/database/repositories/organization_membership_repo.go`

### Phase 2: Teams & Memberships ✅ COMPLETE
- [x] Add Team model
- [x] Create TeamRepository
- [x] Create TeamMembershipRepository
- [x] Implement invitation system (email optional initially)

**Files:**
- `api/internal/database/repositories/team_repo.go`
- `api/internal/database/repositories/team_membership_repo.go`

### Phase 3: Tenant Middleware ✅ COMPLETE
- [x] Implement TenantMiddleware
- [x] Add TenantContext to request context
- [x] Update auth to include org context in JWT
- [x] Implement RequireOrgRole middleware

**Files:**
- `api/internal/middleware/tenant.go`
- `api/internal/server/auth_handlers.go` (updated for org context in JWT)
- `api/internal/server/server.go` (middleware wired)

### Phase 4: Feature Flags ✅ COMPLETE
- [x] Create FeatureRepository
- [x] Seed feature_flags table with edition features
- [x] Implement RequireFeature middleware
- [x] Add feature checks to existing handlers

**Files:**
- `api/internal/database/repositories/feature_repo.go`
- `api/internal/middleware/feature.go`
- `api/internal/server/feature_handlers.go`

### Phase 5: API Endpoints ✅ COMPLETE
- [x] Organization CRUD handlers
- [x] Membership handlers
- [x] Team handlers
- [x] License handlers

**Files:**
- `api/internal/server/organization_handlers.go`
- `api/internal/server/team_handlers.go`
- `api/internal/server/license_handlers.go`
- `api/internal/database/repositories/license_repo.go`

### Phase 6: Scoped Resources ✅ COMPLETE
- [x] Update LabTemplateRepository with org filter
- [x] Update PodRepository with org filter
- [x] Update SessionRepository with org filter
- [x] Add visibility controls to lab template queries

**Files:**
- `api/internal/server/lab_handlers.go` (org/visibility filtering)
- `api/internal/server/pod_handlers.go` (org filtering)
- `api/internal/server/session_handlers.go` (org filtering)

### Phase 7: License System ✅ COMPLETE
- [x] Implement license key generation utility
- [x] Create license handlers with validation
- [x] Add license enforcement to org operations
- [ ] Implement usage tracking (deferred)

**Files:**
- `api/internal/server/license_handlers.go`
- `api/internal/database/repositories/license_repo.go`

### Phase 8: Frontend Updates ✅ COMPLETE
- [x] Add organization switcher component
- [x] Create org management views
- [x] Create team management views
- [x] Update Pinia stores for multi-tenancy
- [x] Add feature flag guards to routes

**Files:**
- `web/src/components/OrganizationSwitcher.vue`
- `web/src/views/organizations/OrganizationView.vue`
- `web/src/views/organizations/OrganizationSettingsView.vue`
- `web/src/views/teams/TeamView.vue`
- `web/src/views/teams/CreateTeamView.vue`
- `web/src/stores/organization.ts`
- `web/src/router.ts` (feature flag guards)

### Phase 9: Canvas LTI Integration ✅ COMPLETE
- [x] Map Canvas courses to organizations
- [x] Auto-create teams from Canvas sections
- [x] Update LTI launch to set org context
- [x] Local Canvas development setup documented

**Files:**
- `api/internal/server/lti_handlers.go` (updated - org mapping integration)
- `api/internal/canvas/sync.go` (new - course/section sync service)
- `config/lti/` (new - RSA keys and configuration)
- `docs/admin/canvas-lms-integration.md` (new - setup guide)

---

## Migration Strategy

1. Create new tables without breaking existing functionality
2. Add nullable columns to existing tables
3. Create default "system" organization
4. Migrate existing users to system org as members
5. Existing labs/pods/sessions get `organization_id = NULL` (global)
6. Enable tenant middleware (initially permissive)
7. Gradually enforce org scoping

---

## Critical Files Reference

| Purpose | Path |
|---------|------|
| Base schema patterns | `api/internal/database/migrations/001_initial_schema.sql` |
| Existing models | `api/internal/models/types.go` |
| Repository interfaces | `api/internal/database/repositories/interfaces.go` |
| Server/routes | `api/internal/server/server.go` |
| Auth middleware | `api/internal/auth/auth.go` |
| LTI handlers | `api/internal/server/lti_handlers.go` |
| Frontend auth store | `web/src/stores/auth.ts` |
| Frontend router | `web/src/router/index.ts` |
