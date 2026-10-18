---
title: Organizations API
description: API endpoints for organizations, teams, features, and licenses
tags:
  - api
  - organizations
  - teams
  - multi-tenancy
---

# Organizations API

This section covers API endpoints for multi-tenancy, including organizations, teams, features, and licenses.

## Overview

Kootenai supports multi-tenancy through:

- **Organizations** - Top-level tenants (schools, companies)
- **Teams** - Groups within organizations (classes, departments)
- **Features** - Edition-based feature access
- **Licenses** - Commercial license management

---

## Organization Endpoints

### List Organizations

<span class="api-method get">GET</span> `/api/v1/organizations`

Get organizations the current user belongs to.

**Response (200 OK):**

```json
{
  "organizations": [
    {
      "id": "org-uuid-1",
      "name": "Acme University",
      "slug": "acme-university",
      "type": "educational",
      "edition": "professional",
      "isActive": true,
      "memberCount": 150,
      "teamCount": 5
    }
  ]
}
```

---

### Create Organization

<span class="api-method post">POST</span> `/api/v1/organizations`

Create a new organization. Requires appropriate permissions.

**Request Body:**

```json
{
  "name": "My Organization",
  "slug": "my-org",
  "type": "standard",
  "contactEmail": "admin@example.com"
}
```

**Organization Types:**

| Type | Description |
|------|-------------|
| `educational` | Schools, universities |
| `enterprise` | Businesses |
| `standard` | General use |

**Response (201 Created):**

```json
{
  "id": "org-new-uuid",
  "name": "My Organization",
  "slug": "my-org",
  "type": "standard",
  "edition": "community",
  "createdAt": "2025-01-15T10:00:00Z"
}
```

---

### Get Organization

<span class="api-method get">GET</span> `/api/v1/organizations/{orgId}`

Get organization details.

**Response (200 OK):**

```json
{
  "id": "org-uuid-1",
  "name": "Acme University",
  "slug": "acme-university",
  "type": "educational",
  "edition": "professional",
  "isActive": true,
  "memberCount": 150,
  "teamCount": 5,
  "settings": {
    "allowSelfRegistration": false,
    "requireApproval": true,
    "defaultRole": "student"
  },
  "createdAt": "2025-01-01T00:00:00Z"
}
```

---

### Update Organization

<span class="api-method put">PUT</span> `/api/v1/organizations/{orgId}`

Update organization settings. Requires admin role.

**Request Body:**

```json
{
  "name": "Updated Name",
  "settings": {
    "allowSelfRegistration": true
  }
}
```

---

### Delete Organization

<span class="api-method delete">DELETE</span> `/api/v1/organizations/{orgId}`

Delete an organization. Requires owner role.

!!! warning "Destructive"
    This deletes all teams, members, and associated data.

---

## Member Management

### List Members

<span class="api-method get">GET</span> `/api/v1/organizations/{orgId}/members`

Get all members in an organization.

**Response (200 OK):**

```json
{
  "members": [
    {
      "id": "member-uuid",
      "userId": "user-uuid",
      "email": "admin@example.com",
      "displayName": "John Admin",
      "role": "admin",
      "isPrimary": true,
      "status": "active",
      "acceptedAt": "2025-01-01T00:00:00Z"
    },
    {
      "id": "member-uuid-2",
      "userId": "user-uuid-2",
      "email": "instructor@example.com",
      "displayName": "Jane Instructor",
      "role": "instructor",
      "isPrimary": false,
      "status": "active",
      "acceptedAt": "2025-01-05T00:00:00Z"
    }
  ]
}
```

**Member Roles:**

| Role | Permissions |
|------|-------------|
| `owner` | Full control, billing, delete org |
| `admin` | Manage members, teams, settings |
| `instructor` | Manage labs, view reports |
| `member` | Participate in labs |

---

### Invite Member

<span class="api-method post">POST</span> `/api/v1/organizations/{orgId}/members`

Invite a new member to the organization.

**Request Body:**

```json
{
  "email": "newuser@example.com",
  "role": "member"
}
```

**Response (201 Created):**

```json
{
  "id": "member-uuid",
  "email": "newuser@example.com",
  "role": "member",
  "status": "invited",
  "invitedAt": "2025-01-15T10:00:00Z"
}
```

---

### Update Member Role

<span class="api-method put">PUT</span> `/api/v1/organizations/{orgId}/members/{userId}`

Update a member's role.

**Request Body:**

```json
{
  "role": "instructor"
}
```

---

### Remove Member

<span class="api-method delete">DELETE</span> `/api/v1/organizations/{orgId}/members/{userId}`

Remove a member from the organization.

---

## Team Endpoints

### List Teams

<span class="api-method get">GET</span> `/api/v1/organizations/{orgId}/teams`

Get all teams in an organization.

**Response (200 OK):**

```json
{
  "teams": [
    {
      "id": "team-uuid-1",
      "name": "CYB 101 - Fall 2025",
      "slug": "cyb-101-fall-2025",
      "description": "Introduction to Cybersecurity",
      "memberCount": 35,
      "isActive": true,
      "createdAt": "2025-01-01T00:00:00Z"
    }
  ]
}
```

---

### Create Team

<span class="api-method post">POST</span> `/api/v1/organizations/{orgId}/teams`

Create a new team. Requires instructor+ role.

**Request Body:**

```json
{
  "name": "Advanced Security - Spring 2025",
  "slug": "adv-security-spring-2025",
  "description": "Advanced security concepts course"
}
```

---

### Get Team

<span class="api-method get">GET</span> `/api/v1/teams/{teamId}`

Get team details.

---

### Update Team

<span class="api-method put">PUT</span> `/api/v1/teams/{teamId}`

Update team settings.

---

### Delete Team

<span class="api-method delete">DELETE</span> `/api/v1/teams/{teamId}`

Delete a team. Requires org admin.

---

### Add Team Member

<span class="api-method post">POST</span> `/api/v1/teams/{teamId}/members`

Add a member to a team.

**Request Body:**

```json
{
  "userId": "user-uuid",
  "role": "member"
}
```

**Team Roles:**

| Role | Permissions |
|------|-------------|
| `lead` | Manage team members |
| `member` | Participate in team labs |

---

### Remove Team Member

<span class="api-method delete">DELETE</span> `/api/v1/teams/{teamId}/members/{userId}`

Remove a member from a team.

---

## Feature Management

### List Features

<span class="api-method get">GET</span> `/api/v1/features`

Get all available features with edition requirements.

**Response (200 OK):**

```json
{
  "features": [
    {
      "id": "teams",
      "name": "Teams Support",
      "description": "Create and manage teams within organizations",
      "editions": ["professional", "enterprise"],
      "category": "collaboration"
    },
    {
      "id": "custom_labs",
      "name": "Custom Lab Templates",
      "description": "Create organization-specific lab templates",
      "editions": ["professional", "enterprise"],
      "category": "labs"
    },
    {
      "id": "sso",
      "name": "Single Sign-On",
      "description": "SAML/OIDC authentication",
      "editions": ["enterprise"],
      "category": "security"
    }
  ]
}
```

---

### Get Organization Features

<span class="api-method get">GET</span> `/api/v1/organizations/{orgId}/features`

Get features enabled for an organization.

**Response (200 OK):**

```json
{
  "features": {
    "teams": true,
    "custom_labs": true,
    "sso": false,
    "audit_extended": true,
    "api_advanced": true
  },
  "edition": "professional"
}
```

---

## License Management

### Get License

<span class="api-method get">GET</span> `/api/v1/organizations/{orgId}/license`

Get current license status. Requires admin.

**Response (200 OK):**

```json
{
  "id": "license-uuid",
  "edition": "professional",
  "maskedKey": "PRO-XXXX-XXXX-3456",
  "issuedAt": "2025-01-01T00:00:00Z",
  "expiresAt": "2026-01-01T00:00:00Z",
  "maxUsers": 100,
  "maxPods": 20,
  "maxStorageGb": 100,
  "features": ["teams", "custom_labs", "analytics.standard"],
  "isActive": true,
  "validationStatus": "valid",
  "isValid": true,
  "daysRemaining": 351
}
```

---

### Activate License

<span class="api-method post">POST</span> `/api/v1/organizations/{orgId}/license`

Activate a license key. Requires owner.

**Request Body:**

```json
{
  "licenseKey": "PRO-XXXX-XXXX-XXXX-XXXX"
}
```

**Response (200 OK):**

```json
{
  "id": "license-uuid",
  "edition": "professional",
  "isActive": true,
  "activatedAt": "2025-01-15T10:00:00Z"
}
```

---

### Deactivate License

<span class="api-method delete">DELETE</span> `/api/v1/organizations/{orgId}/license`

Deactivate license and downgrade to Community. Requires owner.

---

### Validate License

<span class="api-method post">POST</span> `/api/v1/organizations/{orgId}/license/validate`

Manually trigger license validation.

---

## Edition Comparison

| Feature | Community | Professional | Enterprise |
|---------|-----------|--------------|------------|
| Labs | 5 | Unlimited | Unlimited |
| Users | 10 | 100 | Unlimited |
| Teams | No | Yes | Yes |
| Custom Labs | No | Yes | Yes |
| SSO | No | No | Yes |
| API Access | Basic | Standard | Full |
| Support | Community | Email | Priority |
