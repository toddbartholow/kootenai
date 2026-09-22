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

This section covers authentication endpoints, token handling, and LTI 1.3 integration.

:::caution There are no refresh tokens
This page previously described an access/refresh token pair. **The platform has no
refresh-token concept.** `POST /auth/login` returns a single JWT; `POST /auth/refresh`
exchanges that same JWT for a new one of the same kind. The only `refresh_token` field
anywhere in the tree belongs to the OAuth2 provider client
(`api/internal/auth/oauth2/provider.go:82`), which is never wired into the running server.
:::

## Overview

Kootenai supports several authentication methods:

| Method | Use Case | Status |
|--------|----------|--------|
| Email + password | Direct login | Working |
| JWT bearer token or HttpOnly cookie | API access | Working |
| LDAP | Institutional directory | Working |
| LTI 1.3 | Canvas LMS integration | Implemented, **gated off** — see [Enterprise Edition](../enterprise.md) |
| OAuth2 / OIDC | External identity providers | Implemented, **never mounted** — no `/oauth2/*` route exists on a running server |
| Demo Mode | Development/testing | Working |

## Authentication Flow

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant DB

    Client->>API: POST /api/v1/auth/login {email, password}
    API->>DB: Validate credentials
    DB-->>API: User found
    API-->>Client: { token, user, mustChangePassword }

    Client->>API: GET /api/v1/labs<br/>Authorization: Bearer {token}
    API->>API: Validate token
    API-->>Client: Labs data

    Note over Client,API: Before the token expires...
    Client->>API: POST /api/v1/auth/refresh {token}
    API-->>Client: { token, user } — a new JWT of the same kind
```

## Endpoints

### Login

<span class="api-method post">POST</span> `/api/v1/auth/login`

Authenticate with **email** and password. The field is `email`, not `username` — there is
no `username` field anywhere in the auth path (`api/internal/server/auth_handlers.go:137`).

**Request Body:**

```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

`email` carries `validate:"required,email"`. `password` has no validate tag; it is checked
by hand in the handler, so an empty password returns 400 `password is required` unless demo
mode is on.

**Response (200 OK):**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "name": "John Doe",
    "roles": ["student"],
    "iat": 0,
    "defaultOrgId": "org-uuid"
  },
  "mustChangePassword": false
}
```

Note the field names: `token` (singular, no `accessToken`/`refreshToken`/`expiresIn`),
`name` rather than `displayName`, and `roles` as an **array** rather than a singular `role`.
The stored `models.User.Role` is singular and gets wrapped into a one-element slice
(`auth_handlers.go:771-776`).

`mustChangePassword` signals that the user must set a new password before proceeding.

`iat` on the **user object** is a quirk worth knowing: the field has no `omitempty` and is
never assigned, so it is always `0`. The meaningful `iat` is the one inside the JWT.

**Error Responses:**

| Status | Meaning |
|--------|---------|
| 400 | Invalid request body, or missing password |
| 401 | Invalid credentials |
| 429 | Too many login attempts |

---

### Refresh Token

<span class="api-method post">POST</span> `/api/v1/auth/refresh`

Exchange a still-valid JWT for a new one. This is a re-issue, not a refresh-token grant.

**Request Body:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

:::warning The field is `token`, not `refreshToken`
`api/internal/server/auth_handlers.go:279-281`. Sending `{"refreshToken": "..."}` decodes
to an empty `token` and then fails as if you had sent nothing — you do **not** get a
helpful 400 about the wrong field name.
:::

If the body omits the token and cookie auth is enabled, the handler falls back to the
`auth_token` cookie before giving up.

**Response (200 OK):**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": { "id": "...", "email": "...", "name": "...", "roles": ["student"] }
}
```

Two differences from the login response: there is no `mustChangePassword`, and the rebuilt
user **omits `defaultOrgId`** — refresh drops organization context from both the returned
user and the new token (`auth_handlers.go:348-353`).

**Error Responses:**

| Status | Meaning |
|--------|---------|
| 400 | Empty body (`invalid request body`), or no token in body or cookie (`token is required`) |
| 401 | Token expired or invalid |

---

### Get Current User

<span class="api-method get">GET</span> `/api/v1/auth/me`

Get the currently authenticated user's information.

**Headers:**

```
Authorization: Bearer {token}
```

**Response (200 OK):**

The user object is **nested under a `user` key**, alongside the caller's locale preference
(`auth_handlers.go:410-419`):

```json
{
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "name": "John Doe",
    "roles": ["student"],
    "iat": 0,
    "defaultOrgId": "org-uuid"
  },
  "preferredLocale": "es"
}
```

`preferredLocale` is a nullable string and serializes as `null` when unset. There are no
`createdAt` or `lastLoginAt` fields on this response.

A companion endpoint, <span class="api-method put">PUT</span>
`/api/v1/auth/me/preferred-locale`, updates that value.

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

:::note Security
The response is intentionally vague to prevent email enumeration attacks.
:::

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

### Claims

Defined at `api/internal/auth/auth.go:49-57`. The custom claims use **short names** —
`uid` and `org` — which is the most common place to get this wrong:

```json
{
  "uid": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "name": "John Doe",
  "roles": ["student"],
  "org": "org-uuid",
  "orgRole": "member",

  "sub": "550e8400-e29b-41d4-a716-446655440000",
  "jti": "random-uuid",
  "iss": "kootenai-platform",
  "iat": 1705320000,
  "exp": 1705406400
}
```

The lower block comes from the embedded `jwt.RegisteredClaims`. `sub` and `uid` carry the
same value, so reading `sub` works — but `defaultOrgId` is **not** a claim. It is the JSON
name of the field on the `auth.User` object in HTTP responses; inside the token the same
value is `org`.

**Token Lifetime:**

| Setting | Default | Configurable via |
|---------|---------|------------------|
| Token lifetime | **24 hours** | `JWT_EXPIRATION` |
| Signing secret | — | `JWT_SECRET` |

There is no `JWT_REFRESH_EXPIRATION`; the only auth-related environment variables read by
`api/internal/config/config.go` are `JWT_SECRET` and `JWT_EXPIRATION`. The default of 24
hours is set at `api/internal/auth/auth.go:34`.

### Using Tokens

Include the access token in the `Authorization` header:

```bash
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
     https://api.example.com/api/v1/labs
```

---

## LTI 1.3 Integration

Kootenai supports LTI 1.3 for Canvas LMS integration.

Kootenai supports LTI 1.3 for Canvas LMS integration.

:::danger Every route below returns 403 in this build
The `/lti/*` and `/api/v1/lti/*` groups are wrapped in
`middleware.RequireEnterprise(enterprise.FeatureLTI)`
(`api/internal/server/canvas_manager.go:150` and `:165`), which denies before any handler
runs. The LTI service is not constructed either, even with Canvas fully configured. See
[Enterprise Edition](../enterprise.md) for what opens the gate.
:::

### LTI Launch Flow

```mermaid
sequenceDiagram
    participant Canvas
    participant Browser
    participant API

    Canvas->>Browser: Initiate LTI Launch
    Browser->>API: GET or POST /lti/launch
    API-->>Browser: Redirect to Canvas auth
    Browser->>Canvas: Authorization request
    Canvas-->>Browser: id_token
    Browser->>API: POST /lti/callback
    API->>Canvas: Validate ID token against JWKS
    Canvas-->>API: Token validated
    API-->>Browser: Set session, redirect to app
```

### LTI Endpoints

The complete set, from `api/internal/server/canvas_manager.go:146-169`. There is no
`GET /lti/login` and no `GET /lti/deep-link`:

| Endpoint | Purpose |
|----------|---------|
| `GET /lti/launch`, `POST /lti/launch` | Resource link launch |
| `POST /lti/callback` | OIDC callback handler |
| `GET /lti/jwks` | Tool public keys |
| `POST /lti/token` | Token endpoint |
| `GET /lti/select` | Deep-linking selection page |
| `GET /lti/console` | Launch console |
| `GET /api/v1/lti/templates` | Templates offered for deep linking |
| `POST /api/v1/lti/deep-link/submit` | Submit a deep-linking response |

### LTI Configuration

See [Canvas LMS Integration](../admin/canvas-lms-integration.md) for full setup instructions.

---

## Demo Mode

For development and testing, demo mode allows access without authentication.

:::danger Security Warning
Never enable demo mode in production. Demo mode authenticates *every* request as a fixed
demo user without reading any token — including WebSocket upgrades.
:::

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

There is one token, so there is one thing to store.

**Web applications.** Prefer the HttpOnly cookie the API sets (`auth_token`) over handling
the token in JavaScript at all — it is the mechanism the Vue frontend uses, and it is what
makes WebSocket upgrades work without a bearer header. If you must hold the token in JS,
hold it in memory rather than `localStorage`, and clear it on logout.

**Mobile and native apps.** Use platform secure storage (Keychain / Keystore). Re-issue via
`POST /api/v1/auth/refresh` before expiry rather than waiting for a 401, since a 401 means
the token is already unusable and the endpoint requires a still-valid token. Clear on
logout and uninstall.

### Request Signing

All requests should use HTTPS in production to protect tokens in transit.

### Rate Limiting

Defaults from `DefaultAuthRateLimitConfig` (`api/internal/redis/ratelimit.go:363-376`).
Limits apply only when a Redis-backed limiter is configured:

| Endpoint | Limit |
|----------|-------|
| `/auth/login` | 5 attempts/minute/IP **and** 10 attempts/hour/email |
| `/auth/password/reset-request`, `/auth/password/reset-confirm` | 3/hour/IP **and** 3/hour/email |
| Token refresh | 30/hour/user |

Note that `/auth/refresh` and `/auth/logout` carry no route-level rate-limit middleware
(`auth_handlers.go:107-109`); the 30/hour figure is the limiter's configured value for the
refresh key, not a guarantee that it is wired to that route in every build.
