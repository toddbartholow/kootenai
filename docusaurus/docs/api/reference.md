# API Reference

The Kootenai API provides RESTful endpoints for managing labs, pods, sessions, and achievements.

**Base URL**: `http://localhost:8080` (development) or your deployed API URL

**Authentication**: Most endpoints require a JWT token in the `Authorization: Bearer <token>` header.

## Quick Links

- [Health & Status](#health--status)
- [Authentication](#authentication)
- [Labs](#labs)
- [Pods](#pods)
- [Sessions](#sessions)
- [Achievements](#achievements)
- [Users](#users-admin)
- [Pathways](#pathways)
- [Dashboard](#dashboard)
- [Organizations](#organizations)
- [Teams](#teams)
- [Features](#features)
- [Licenses](#licenses)
- [Classroom Simulation (Enterprise)](#classroom-simulation-enterprise)

---

## Health & Status

### GET /health
Basic liveness check.

**Response**:
```json
{
  "status": "ok"
}
```

### GET /ready
Readiness check with service status.

**Response**:
```json
{
  "status": "ready",
  "checks": {
    "database": { "status": "healthy" },
    "nats": { "status": "healthy" },
    "redis": { "status": "healthy" },
    "proxmox": { "status": "healthy", "circuit_state": "closed" }
  }
}
```

### GET /version
API version information.

**Response**:
```json
{
  "version": "1.0.0",
  "commit": "abc1234",
  "buildTime": "2025-01-07T12:00:00Z"
}
```

---

## Authentication

### POST /api/v1/auth/login
Authenticate and receive JWT tokens.

**Request**:
```json
{
  "username": "user@example.com",
  "password": "password123"
}
```

**Response**:
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIs...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIs...",
  "expiresIn": 3600,
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "displayName": "John Doe",
    "role": "student"
  }
}
```

### POST /api/v1/auth/refresh
Refresh an expired access token.

**Request**:
```json
{
  "refreshToken": "eyJhbGciOiJIUzI1NiIs..."
}
```

### GET /api/v1/auth/me
Get current authenticated user.

**Response**:
```json
{
  "id": "uuid",
  "email": "user@example.com",
  "displayName": "John Doe",
  "role": "student",
  "createdAt": "2025-01-01T00:00:00Z"
}
```

### POST /api/v1/auth/password/reset-request
Request a password reset email.

**Request**:
```json
{
  "email": "user@example.com"
}
```

### POST /api/v1/auth/password/reset-confirm
Confirm password reset with token.

**Request**:
```json
{
  "token": "reset-token-from-email",
  "newPassword": "newpassword123"
}
```

---

## Labs

### GET /api/v1/labs
List all available lab templates.

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| platform | string | Filter by platform (proxmox, cloudstack) |
| active | boolean | Filter by active status (default: true) |

**Response**:
```json
{
  "labs": [
    {
      "id": "uuid",
      "name": "Linux Foundations",
      "description": "Introduction to Linux commands",
      "version": "1.0.0",
      "platform": "proxmox",
      "durationMinutes": 60,
      "difficulty": "beginner",
      "maxPoints": 100,
      "passThreshold": 70,
      "isActive": true,
      "createdAt": "2025-01-01T00:00:00Z"
    }
  ],
  "count": 1
}
```

### GET /api/v1/labs/:labID
Get a specific lab template.

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| include_spec | boolean | Include full YAML spec (default: false) |

**Response**:
```json
{
  "id": "uuid",
  "name": "Linux Foundations",
  "description": "Introduction to Linux commands",
  "version": "1.0.0",
  "platform": "proxmox",
  "durationMinutes": 60,
  "difficulty": "beginner",
  "maxPoints": 100,
  "passThreshold": 70,
  "isActive": true,
  "spec": { ... },
  "checkpoints": [ ... ]
}
```

### POST /api/v1/labs (Admin)
Create a new lab template.

**Request**:
```json
{
  "name": "My New Lab",
  "description": "Lab description",
  "version": "1.0.0",
  "platform": "proxmox",
  "durationMinutes": 60,
  "difficulty": "beginner",
  "maxPoints": 100,
  "passThreshold": 70,
  "spec": "apiVersion: v1\nkind: LabTemplate\n...",
  "isActive": true,
  "visibility": "global"
}
```

**Response**:
```json
{
  "id": "uuid",
  "name": "My New Lab",
  "slug": "my-new-lab",
  "createdAt": "2025-01-07T00:00:00Z"
}
```

### PUT /api/v1/labs/:labID (Admin)
Update a lab template.

**Request**:
```json
{
  "name": "Updated Lab Name",
  "description": "Updated description",
  "isActive": false
}
```

### DELETE /api/v1/labs/:labID (Admin)
Delete a lab template.

**Response**:
```json
{
  "message": "lab template deleted successfully"
}
```

### PUT /api/v1/labs/:labID/active (Admin)
Activate or deactivate a lab template.

**Request**:
```json
{
  "isActive": true
}
```

### GET /api/v1/labs/:labID/instructions
Get lab instructions and learning content.

**Response**:
```json
{
  "labId": "uuid",
  "labName": "Linux Foundations",
  "overview": "In this lab you will learn...",
  "learning_objectives": ["Objective 1", "Objective 2"],
  "prerequisites": ["Basic computer skills"],
  "steps": [
    {
      "title": "Step 1: Login",
      "content": "Connect to the VM...",
      "hints": ["Use SSH"]
    }
  ],
  "summary": "You learned...",
  "tips": ["Pro tip: Use tab completion"],
  "resources": [{"title": "Linux Manual", "url": "https://..."}]
}
```

---

## Pods

### GET /api/v1/pods
List user's pods.

**Response**:
```json
{
  "pods": [
    {
      "id": "uuid",
      "name": "linux-foundations-abc123",
      "labTemplateId": "uuid",
      "labName": "Linux Foundations",
      "status": "running",
      "platform": "proxmox",
      "vms": [
        {
          "name": "student-vm",
          "vmid": 100,
          "status": "running",
          "ip": "10.0.100.10"
        }
      ],
      "createdAt": "2025-01-07T00:00:00Z",
      "expiresAt": "2025-01-07T02:00:00Z"
    }
  ]
}
```

### POST /api/v1/pods
Create a new pod from a lab template.

**Request**:
```json
{
  "labTemplateId": "uuid",
  "name": "my-lab-pod"
}
```

**Response**:
```json
{
  "id": "uuid",
  "name": "my-lab-pod",
  "status": "provisioning",
  "createdAt": "2025-01-07T00:00:00Z"
}
```

### GET /api/v1/pods/:podID
Get pod details.

**Response**:
```json
{
  "id": "uuid",
  "name": "my-lab-pod",
  "labTemplateId": "uuid",
  "status": "running",
  "vms": [
    {
      "name": "student-vm",
      "vmid": 100,
      "status": "running",
      "ip": "10.0.100.10",
      "memory": 2048,
      "cores": 2
    }
  ],
  "network": {
    "vlan": 100,
    "subnet": "10.0.100.0/24"
  },
  "createdAt": "2025-01-07T00:00:00Z"
}
```

### DELETE /api/v1/pods/:podID
Delete a pod and all its VMs.

### POST /api/v1/pods/:podID/start
Start all VMs in the pod.

### POST /api/v1/pods/:podID/stop
Stop all VMs in the pod.

### POST /api/v1/pods/:podID/reset
Reset pod to initial state.

### POST /api/v1/pods/:podID/vms/:vmName/start
Start a specific VM.

### POST /api/v1/pods/:podID/vms/:vmName/stop
Stop a specific VM.

### POST /api/v1/pods/:podID/vms/:vmName/reset
Reset a specific VM to snapshot.

### GET /api/v1/pods/:podID/vms/:vmName/console
Get VNC/SPICE console connection info.

**Response**:
```json
{
  "type": "vnc",
  "host": "proxmox.local",
  "port": 5900,
  "ticket": "vnc-ticket-abc123",
  "websocketUrl": "wss://api.example.com/api/v1/pods/uuid/vms/student-vm/vnc"
}
```

### GET /api/v1/pods/:podID/vms/:vmName/vnc
WebSocket endpoint for VNC proxy (use with noVNC client).

---

## Sessions

### GET /api/v1/sessions
List user's lab sessions.

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| status | string | Filter by status (active, ended, graded) |

**Response**:
```json
{
  "sessions": [
    {
      "id": "uuid",
      "podId": "uuid",
      "labTemplateId": "uuid",
      "labName": "Linux Foundations",
      "status": "active",
      "startedAt": "2025-01-07T00:00:00Z",
      "earnedPoints": 50,
      "maxPoints": 100,
      "passed": false
    }
  ]
}
```

### POST /api/v1/sessions
Start a new lab session.

**Request**:
```json
{
  "podId": "uuid"
}
```

**Response**:
```json
{
  "id": "uuid",
  "podId": "uuid",
  "status": "active",
  "startedAt": "2025-01-07T00:00:00Z"
}
```

### GET /api/v1/sessions/:sessionID
Get session details.

### GET /api/v1/sessions/:sessionID/progress
Get current progress with checkpoint status.

**Response**:
```json
{
  "sessionId": "uuid",
  "earnedPoints": 50,
  "maxPoints": 100,
  "percentComplete": 50,
  "checkpoints": [
    {
      "id": "obj-1",
      "name": "Create a file",
      "completed": true,
      "points": 25,
      "completedAt": "2025-01-07T00:30:00Z"
    },
    {
      "id": "obj-2",
      "name": "Install package",
      "completed": false,
      "points": 25
    }
  ]
}
```

### GET /api/v1/sessions/:sessionID/checkpoints
Get detailed checkpoint status.

### POST /api/v1/sessions/:sessionID/end
End a session without grading.

### POST /api/v1/sessions/:sessionID/submit
Submit session for grading. Achievement evaluation may be performed asynchronously when NATS is available.

**Response**:
```json
{
  "sessionId": "uuid",
  "status": "graded",
  "earnedPoints": 75,
  "maxPoints": 100,
  "percentage": 75.0,
  "passed": true,
  "passThreshold": 70,
  "checkpoints": [
    {
      "id": "checkpoint-1",
      "description": "Install required packages",
      "points": 25,
      "earnedPoints": 25,
      "passed": true,
      "completedAt": "2025-01-07T12:30:00Z"
    }
  ],
  "submittedAt": "2025-01-07T12:35:00Z",
  "achievements": [
    {
      "id": "uuid",
      "achievementId": "ach-first-lab",
      "name": "First Lab Complete",
      "tier": "bronze",
      "points": 10,
      "earnedAt": "2025-01-07T12:35:00Z"
    }
  ],
  "achievementsPending": false
}
```

**Response Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `achievements` | array | Achievements awarded (only present for sync evaluation) |
| `achievementsPending` | boolean | `true` when achievements are being evaluated asynchronously |

**Note**: When `achievementsPending: true`, achievements will be processed in the background. Poll `/api/v1/users/:userId/achievements` or use WebSocket notifications to get newly awarded achievements.

---

## Achievements

### GET /api/v1/achievements
List all achievements.

**Response**:
```json
{
  "achievements": [
    {
      "id": "uuid",
      "name": "First Steps",
      "description": "Complete your first lab",
      "tier": "bronze",
      "points": 10,
      "icon": "pi-star",
      "criteria": {
        "type": "lab_completion",
        "count": 1
      }
    }
  ]
}
```

### GET /api/v1/achievements/:achievementID
Get achievement details.

### GET /api/v1/achievements/recent
Get recently earned achievements (leaderboard).

**Response**:
```json
{
  "achievements": [
    {
      "userId": "uuid",
      "displayName": "John Doe",
      "achievement": {
        "id": "uuid",
        "name": "Speed Demon",
        "tier": "silver"
      },
      "earnedAt": "2025-01-07T00:00:00Z"
    }
  ]
}
```

### GET /api/v1/users/:userID/achievements
Get user's earned achievements.

**Response**:
```json
{
  "earned": [
    {
      "id": "uuid",
      "name": "First Steps",
      "tier": "bronze",
      "earnedAt": "2025-01-01T00:00:00Z"
    }
  ],
  "progress": [
    {
      "id": "uuid",
      "name": "Lab Master",
      "progress": 5,
      "required": 10,
      "percentComplete": 50
    }
  ]
}
```

### GET /api/v1/users/:userID/achievements/summary
Get user's achievement statistics.

**Response**:
```json
{
  "totalEarned": 5,
  "totalPoints": 150,
  "byTier": {
    "bronze": 3,
    "silver": 1,
    "gold": 1
  },
  "recentAchievements": [...]
}
```

---

## Users (Admin)

### GET /api/v1/users
List all users (admin only).

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| limit | int | Results per page (default: 50) |
| offset | int | Pagination offset |
| role | string | Filter by role |

**Response**:
```json
{
  "users": [
    {
      "id": "uuid",
      "email": "user@example.com",
      "displayName": "John Doe",
      "role": "student",
      "createdAt": "2025-01-01T00:00:00Z",
      "lastLoginAt": "2025-01-07T00:00:00Z"
    }
  ],
  "total": 100
}
```

### GET /api/v1/users/:userID
Get user details.

### PUT /api/v1/users/:userID
Update user.

### DELETE /api/v1/users/:userID
Delete user.

### PUT /api/v1/users/:userID/password/reset (Admin)
Force password reset for user.

---

## Pathways

### GET /api/v1/pathways
List learning pathways.

**Response**:
```json
{
  "pathways": [
    {
      "id": "uuid",
      "slug": "linux-fundamentals",
      "name": "Linux Fundamentals",
      "description": "Master Linux basics",
      "difficulty": "beginner",
      "estimatedHours": 10,
      "modules": [
        {
          "id": "uuid",
          "name": "Shell Basics",
          "labs": ["uuid1", "uuid2"]
        }
      ]
    }
  ]
}
```

### GET /api/v1/pathways/:slug
Get pathway details.

### GET /api/v1/enrollments
List user's pathway enrollments.

### POST /api/v1/enrollments
Enroll in a pathway.

**Request**:
```json
{
  "pathwayId": "uuid"
}
```

### GET /api/v1/enrollments/:id
Get enrollment progress.

---

## Dashboard

### GET /api/v1/dashboard
Get user's dashboard data.

**Response**:
```json
{
  "user": {
    "id": "uuid",
    "displayName": "John Doe"
  },
  "stats": {
    "labsCompleted": 5,
    "totalPoints": 450,
    "achievementsEarned": 3,
    "currentStreak": 2
  },
  "recentActivity": [
    {
      "type": "lab_completed",
      "labName": "Linux Foundations",
      "timestamp": "2025-01-07T00:00:00Z"
    }
  ],
  "activeSessions": [...],
  "recentAchievements": [...]
}
```

### GET /api/v1/activity
Get platform activity feed.

### GET /api/v1/leaderboard
Get points leaderboard.

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| limit | int | Number of entries (default: 10) |

**Response**:
```json
{
  "entries": [
    {
      "rank": 1,
      "userId": "uuid",
      "displayName": "Jane Doe",
      "totalPoints": 1500,
      "achievementCount": 12,
      "labsCompleted": 15
    }
  ],
  "currentUser": {
    "rank": 5,
    "totalPoints": 450
  },
  "totalUsers": 100
}
```

---

## Reservations

### GET /api/v1/reservations
List user's reservations.

### POST /api/v1/reservations
Create a lab reservation.

**Request**:
```json
{
  "labTemplateId": "uuid",
  "startTime": "2025-01-08T09:00:00Z",
  "endTime": "2025-01-08T11:00:00Z"
}
```

### GET /api/v1/reservations/availability
Check lab availability for scheduling.

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| labTemplateId | uuid | Lab to check |
| date | string | Date to check (YYYY-MM-DD) |

### DELETE /api/v1/reservations/:reservationID
Cancel a reservation.

---

## WebSocket Endpoints

### GET /ws
General WebSocket for real-time updates.

**Events Received**:
- `pod.status` - Pod status changes
- `session.progress` - Checkpoint completions
- `achievement.earned` - New achievements

### GET /ws/:podID
Pod-specific WebSocket for VM events.

---

## Organizations

Multi-tenancy support allows users to belong to organizations with teams and feature-based access control.

### GET /api/v1/organizations
List organizations the user belongs to.

**Response**:
```json
{
  "organizations": [
    {
      "id": "uuid",
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

### POST /api/v1/organizations
Create a new organization (requires appropriate permissions).

**Request**:
```json
{
  "name": "My Organization",
  "slug": "my-org",
  "type": "standard",
  "contactEmail": "admin@example.com"
}
```

### GET /api/v1/organizations/:orgID
Get organization details.

### PUT /api/v1/organizations/:orgID
Update organization (admin only).

### DELETE /api/v1/organizations/:orgID
Delete organization (owner only).

### GET /api/v1/organizations/:orgID/members
List organization members.

**Response**:
```json
{
  "members": [
    {
      "id": "uuid",
      "userId": "uuid",
      "email": "user@example.com",
      "displayName": "John Doe",
      "role": "admin",
      "isPrimary": true,
      "acceptedAt": "2025-01-01T00:00:00Z"
    }
  ]
}
```

### POST /api/v1/organizations/:orgID/members
Invite a member to the organization (admin only).

**Request**:
```json
{
  "email": "newuser@example.com",
  "role": "member"
}
```

### PUT /api/v1/organizations/:orgID/members/:userID
Update member role (admin only).

### DELETE /api/v1/organizations/:orgID/members/:userID
Remove member from organization.

---

## Teams

Teams provide sub-grouping within organizations.

### GET /api/v1/organizations/:orgID/teams
List teams in an organization.

**Response**:
```json
{
  "teams": [
    {
      "id": "uuid",
      "name": "Security 101",
      "slug": "security-101",
      "description": "Intro to security course",
      "memberCount": 25,
      "isActive": true
    }
  ]
}
```

### POST /api/v1/organizations/:orgID/teams
Create a team (instructor+ role required).

**Request**:
```json
{
  "name": "Advanced Networking",
  "slug": "advanced-networking",
  "description": "Advanced network concepts"
}
```

### GET /api/v1/teams/:teamID
Get team details.

### PUT /api/v1/teams/:teamID
Update team (team lead or org admin).

### DELETE /api/v1/teams/:teamID
Delete team (org admin only).

### POST /api/v1/teams/:teamID/members
Add member to team.

**Request**:
```json
{
  "userId": "uuid",
  "role": "member"
}
```

### DELETE /api/v1/teams/:teamID/members/:userID
Remove member from team.

---

## Features

Feature flags control access to edition-specific functionality.

### GET /api/v1/features
List all available features with edition requirements.

**Response**:
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
    }
  ]
}
```

### GET /api/v1/organizations/:orgID/features
Get features enabled for an organization.

**Response**:
```json
{
  "features": {
    "teams": true,
    "custom_labs": true,
    "sso": false,
    "audit_extended": true
  },
  "edition": "professional"
}
```

### PUT /api/v1/organizations/:orgID/features/:featureID
Enable/disable a feature (admin only, subject to edition limits).

### DELETE /api/v1/organizations/:orgID/features/:featureID
Disable a feature.

---

## Licenses

License management for paid editions.

### GET /api/v1/organizations/:orgID/license
Get current license status (admin only).

**Response**:
```json
{
  "id": "uuid",
  "edition": "professional",
  "maskedKey": "PRO-...3456",
  "issuedAt": "2025-01-01T00:00:00Z",
  "expiresAt": "2026-01-01T00:00:00Z",
  "maxUsers": 100,
  "maxPods": 20,
  "maxStorageGb": 100,
  "features": ["teams", "custom_labs", "analytics.standard"],
  "isActive": true,
  "validationStatus": "valid",
  "isValid": true,
  "daysRemaining": 358
}
```

### POST /api/v1/organizations/:orgID/license
Activate a license key (owner only).

**Request**:
```json
{
  "licenseKey": "PRO-XXXX-XXXX-XXXX-XXXX"
}
```

### DELETE /api/v1/organizations/:orgID/license
Deactivate license, downgrade to Community edition (owner only).

### POST /api/v1/organizations/:orgID/license/validate
Manually trigger license validation (admin only).

---

## Classroom Simulation (Enterprise)

AI classroom simulation endpoints for orchestrating virtual students. Requires `ai_classroom` enterprise feature.

For full endpoint documentation, see [Classroom Simulation API](classroom-simulation.md).

### POST /api/v1/simulation/classrooms
Create a new simulation with auto-generated AI students.

### GET /api/v1/simulation/classrooms
List all simulations.

### GET /api/v1/simulation/classrooms/:id
Get simulation details with students.

### POST /api/v1/simulation/classrooms/:id/start
Start a pending/paused simulation.

### POST /api/v1/simulation/classrooms/:id/pause
Pause a running simulation.

### POST /api/v1/simulation/classrooms/:id/stop
Stop a simulation.

### DELETE /api/v1/simulation/classrooms/:id
Delete a simulation (cascades to students and activities).

### GET /api/v1/simulation/classrooms/:id/activities
Paginated activity feed (query params: `limit`, `offset`).

### GET /api/v1/simulation/classrooms/:id/students/:studentId
Get AI student details including runtime state.

---

## Error Responses

All errors follow this format:

```json
{
  "error": "Error message describing what went wrong"
}
```

### Common HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Invalid/missing token |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found |
| 409 | Conflict - Resource already exists |
| 429 | Too Many Requests - Rate limited |
| 500 | Internal Server Error |
| 503 | Service Unavailable |

---

## Rate Limiting

The API implements rate limiting:

- **General**: 100 requests/minute per IP
- **Pod Creation**: 10 pods/hour per user
- **Login**: 5 attempts/minute per IP

Rate limit headers are included in responses:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1704628800
```

---

## Swagger/OpenAPI

Interactive API documentation is available at:

```
http://localhost:8080/swagger/
```
