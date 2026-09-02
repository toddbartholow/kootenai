---
title: Authentication API
description: API endpoints for authentication, JWT tokens, and LTI integration
tags:
  - api
  - authentication
  - jwt
  - lti
---

# Authentication API

This section covers authentication endpoints including login, token management, and LTI 1.3 integration.

## Overview

Kootenai supports multiple authentication methods:

| Method | Use Case |
|--------|----------|
| Username/Password | Direct login |
| JWT Tokens | API access |
| LTI 1.3 | Canvas LMS integration |
| Demo Mode | Development/testing |

## Authentication Flow

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant DB

    Client->>API: POST /auth/login
    API->>DB: Validate credentials
    DB-->>API: User found
    API-->>Client: Access + Refresh tokens

    Client->>API: GET /api/v1/labs<br/>Authorization: Bearer {token}
    API->>API: Validate token
    API-->>Client: Labs data

    Note over Client,API: When token expires...
    Client->>API: POST /auth/refresh
    API-->>Client: New access token
```

## Endpoints

### Login

<span class="api-method post">POST</span> `/api/v1/auth/login`

Authenticate with username and password to receive JWT tokens.

**Request Body:**

```json
{
  "username": "user@example.com",
  "password": "password123"
}
```

**Response (200 OK):**

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 3600,
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "displayName": "John Doe",
    "role": "student"
  }
}
```

**Error Responses:**

| Status | Meaning |
|--------|---------|
| 400 | Invalid request body |
| 401 | Invalid credentials |
| 429 | Too many login attempts |

---

### Refresh Token

<span class="api-method post">POST</span> `/api/v1/auth/refresh`

Exchange a refresh token for a new access token.

**Request Body:**

```json
{
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response (200 OK):**

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 3600
}
```

**Error Responses:**

| Status | Meaning |
|--------|---------|
| 400 | Invalid refresh token format |
| 401 | Refresh token expired or invalid |

---

### Get Current User

<span class="api-method get">GET</span> `/api/v1/auth/me`

Get the currently authenticated user's information.

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "displayName": "John Doe",
  "role": "student",
  "createdAt": "2025-01-01T00:00:00Z",
  "lastLoginAt": "2025-01-15T10:30:00Z"
}
```

---

### Request Password Reset

<span class="api-method post">POST</span> `/api/v1/auth/password/reset-request`

Request a password reset email.

**Request Body:**

```json
{
  "email": "user@example.com"
}
```

**Response (200 OK):**

```json
{
  "message": "If an account exists with this email, a reset link has been sent."
}
```

!!! note "Security"
    The response is intentionally vague to prevent email enumeration attacks.

---

### Confirm Password Reset

<span class="api-method post">POST</span> `/api/v1/auth/password/reset-confirm`

Complete password reset with the token from email.

**Request Body:**

```json
{
  "token": "reset-token-from-email",
  "newPassword": "newSecurePassword123!"
}
```

**Response (200 OK):**

```json
{
  "message": "Password successfully reset"
}
```

**Error Responses:**

| Status | Meaning |
|--------|---------|
| 400 | Invalid or expired token |
| 422 | Password doesn't meet requirements |

---

## JWT Token Structure

### Access Token

Access tokens contain:

```json
{
  "sub": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "name": "John Doe",
  "roles": ["student"],
  "defaultOrgId": "org-uuid",
  "iat": 1705320000,
  "exp": 1705323600
}
```

**Token Lifetime:**

| Token Type | Default Lifetime | Configurable |
|------------|------------------|--------------|
| Access Token | 1 hour | `JWT_EXPIRATION` |
| Refresh Token | 7 days | `JWT_REFRESH_EXPIRATION` |

### Using Tokens

Include the access token in the `Authorization` header:

```bash
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
     https://api.example.com/api/v1/labs
```

---

## LTI 1.3 Integration

Kootenai supports LTI 1.3 for Canvas LMS integration.

### LTI Launch Flow

```mermaid
sequenceDiagram
    participant Canvas
    participant Browser
    participant API

    Canvas->>Browser: Initiate LTI Launch
    Browser->>API: GET /lti/login
    API-->>Browser: Redirect to Canvas auth
    Browser->>Canvas: Authorization request
    Canvas-->>Browser: Auth code
    Browser->>API: POST /lti/callback
    API->>Canvas: Validate ID token
    Canvas-->>API: Token validated
    API-->>Browser: Set session, redirect to app
```

### LTI Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /lti/login` | OIDC login initiation |
| `POST /lti/callback` | OIDC callback handler |
| `GET /lti/jwks` | Tool public keys |
| `POST /lti/launch` | Resource link launch |
| `GET /lti/deep-link` | Deep linking selection |

### LTI Configuration

See [Canvas LMS Integration](../admin/canvas-lms-integration.md) for full setup instructions.

---

## Demo Mode

For development and testing, demo mode allows access without authentication.

!!! danger "Security Warning"
    Never enable demo mode in production!

### Enabling Demo Mode

```bash
export AUTH_DEMO_MODE=true
export AUTH_DEMO_MODE_CONFIRM=I_UNDERSTAND_THE_RISKS
export AUTH_DEMO_ROLE=student  # or instructor, admin
```

### Demo Mode Behavior

- No login required
- All requests treated as demo user
- Role determined by `AUTH_DEMO_ROLE`
- Full API access for that role

---

## Roles and Permissions

### Role Hierarchy

```mermaid
graph TD
    A[Admin] -->|inherits| B[Instructor]
    B -->|inherits| C[Student]
```

### Role Permissions

| Permission | Student | Instructor | Admin |
|------------|---------|------------|-------|
| View labs | Yes | Yes | Yes |
| Create pods | Yes | Yes | Yes |
| View own sessions | Yes | Yes | Yes |
| View team sessions | No | Yes | Yes |
| Manage labs | No | Yes | Yes |
| Manage users | No | No | Yes |
| System settings | No | No | Yes |

---

## Security Best Practices

### Token Storage

=== "Web Applications"
    - Store access token in memory (not localStorage)
    - Store refresh token in httpOnly cookie
    - Clear tokens on logout

=== "Mobile/Native Apps"
    - Use secure storage (Keychain/Keystore)
    - Implement token refresh on 401 responses
    - Clear tokens on logout/uninstall

### Request Signing

All requests should use HTTPS in production to protect tokens in transit.

### Rate Limiting

Authentication endpoints are rate limited:

| Endpoint | Limit |
|----------|-------|
| `/auth/login` | 5 attempts/minute/IP |
| `/auth/refresh` | 10 requests/minute |
| `/auth/password/*` | 3 requests/hour/email |
